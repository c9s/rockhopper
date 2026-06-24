package rockhopper

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/pkg/errors"
	log "github.com/sirupsen/logrus"

	"github.com/c9s/rockhopper/v2/pkg/dialect"
)

// TouchDataMigrationTable creates the data-migration state table if it does not
// exist yet.
func (db *DB) TouchDataMigrationTable(ctx context.Context) error {
	if _, err := db.ExecContext(ctx, db.dialect.CreateTable(dataMigrationSchema(DataMigrationTableName))); err != nil {
		return errors.Wrap(err, "failed to create data migration table")
	}

	return nil
}

// dataMigrationSchema describes the data-migration state table.
func dataMigrationSchema(tableName string) dialect.Schema {
	return dialect.Schema{
		Table: tableName,
		Columns: []dialect.Column{
			{Name: "id", Type: dialect.ColSerial, PrimaryKey: true},
			{Name: "package", Type: dialect.ColVarchar, Size: packageColumnSize, NotNull: true, Default: "'main'"},
			{Name: "version_id", Type: dialect.ColBigInt, NotNull: true},
			{Name: "name", Type: dialect.ColVarchar, Size: 255, NotNull: true, Default: "''"},
			{Name: "status", Type: dialect.ColVarchar, Size: 32, NotNull: true, Default: "'pending'"},
			{Name: "checkpoint", Type: dialect.ColText},
			{Name: "lease_owner", Type: dialect.ColVarchar, Size: 255},
			{Name: "lease_expires_at", Type: dialect.ColBigInt, NotNull: true, Default: "0"},
			{Name: "created_at", Type: dialect.ColTimestamp, NotNull: true, Default: dialect.DefaultNow},
			{Name: "updated_at", Type: dialect.ColTimestamp, NotNull: true, Default: dialect.DefaultNow},
		},
		Unique: [][]string{{"package", "version_id"}},
	}
}

// leaseBuilder returns the dialect's data-migration lease capability, or
// ErrDataMigrationUnsupported when the dialect cannot honor the conditional
// lease (e.g. ClickHouse). It is the single gate that keeps the data-migration
// runner from emitting lease SQL a dialect cannot execute.
func (db *DB) leaseBuilder() (dialect.LeaseBuilder, error) {
	if lb, ok := db.dialect.(dialect.LeaseBuilder); ok {
		return lb, nil
	}

	return nil, ErrDataMigrationUnsupported
}

// loadDataMigrationState loads the persisted status and checkpoint for a data
// migration. found is false when no row exists yet.
func (db *DB) loadDataMigrationState(ctx context.Context, pkgName string, version int64) (status string, cp Checkpoint, found bool, err error) {
	q, args := db.dialect.Select(DataMigrationTableName,
		[]string{"status", "checkpoint"},
		[]dialect.Col{
			{Name: "package", Val: pkgName},
			{Name: "version_id", Val: version},
		},
		dialect.SelectOpt{})
	row := db.QueryRowContext(ctx, q, args...)

	var checkpoint sql.NullString
	if err := row.Scan(&status, &checkpoint); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return "", nil, false, nil
		}

		return "", nil, false, errors.Wrap(err, "failed to load data migration state")
	}

	if checkpoint.Valid && checkpoint.String != "" {
		cp = Checkpoint(checkpoint.String)
	}

	return status, cp, true, nil
}

// insertDataMigrationState inserts the initial state row for a data migration.
func (db *DB) insertDataMigrationState(ctx context.Context, exec SQLExecutor, dm *DataMigration, status string, cp Checkpoint) error {
	q, args := db.dialect.Insert(DataMigrationTableName, []dialect.Col{
		{Name: "package", Val: dm.Package},
		{Name: "version_id", Val: dm.Version},
		{Name: "name", Val: dm.Name},
		{Name: "status", Val: status},
		{Name: "checkpoint", Val: string(cp)},
	})
	if _, err := exec.ExecContext(ctx, q, args...); err != nil {
		return errors.Wrap(err, "failed to insert data migration state")
	}

	return nil
}

// acquireDataMigrationLease attempts to claim the lease for a data migration.
// It succeeds when the lease is unowned, already owned by this process, or
// expired. It returns false (without error) when another live process holds it.
func (db *DB) acquireDataMigrationLease(ctx context.Context, dm *DataMigration, owner string, ttl time.Duration) (bool, error) {
	lb, err := db.leaseBuilder()
	if err != nil {
		return false, err
	}

	now := time.Now()
	expiresAt := now.Add(ttl).Unix()

	q, args := lb.AcquireLease(DataMigrationTableName,
		[]dialect.Col{
			{Name: "package", Val: dm.Package},
			{Name: "version_id", Val: dm.Version},
		},
		owner, expiresAt, now.Unix())

	res, err := db.ExecContext(ctx, q, args...)
	if err != nil {
		return false, errors.Wrap(err, "failed to acquire data migration lease")
	}

	affected, err := res.RowsAffected()
	if err != nil {
		return false, errors.Wrap(err, "failed to read affected rows for lease acquisition")
	}

	return affected == 1, nil
}

// acquireDataMigrationLeaseWaiting claims the lease, retrying for up to
// dm.leaseWait() when another process holds it. A crashed predecessor leaves a
// lease that becomes acquirable once it expires (LeaseTTL after its last batch),
// so waiting longer than the TTL lets a fresh process take over a dead holder's
// work while still correctly yielding to a live holder, which keeps renewing and
// is never reclaimed within the window. It returns false (no error) if the lease
// is still held when the wait elapses.
func (db *DB) acquireDataMigrationLeaseWaiting(ctx context.Context, dm *DataMigration, owner string, ttl time.Duration, logger *log.Entry) (bool, error) {
	acquired, err := db.acquireDataMigrationLease(ctx, dm, owner, ttl)
	if err != nil || acquired {
		return acquired, err
	}

	wait := dm.leaseWait()
	if wait <= 0 {
		return false, nil
	}

	interval := dm.leasePollInterval()
	start := time.Now()
	deadline := start.Add(wait)
	logger.WithFields(log.Fields{"wait": wait, "poll_interval": interval}).
		Info("data migration lease held by another process, waiting for it to be released or to expire")

	for {
		remaining := time.Until(deadline)
		if remaining <= 0 {
			return false, nil
		}

		sleep := interval
		if sleep > remaining {
			sleep = remaining
		}

		select {
		case <-ctx.Done():
			return false, ctx.Err()
		case <-time.After(sleep):
		}

		acquired, err := db.acquireDataMigrationLease(ctx, dm, owner, ttl)
		if err != nil {
			return false, err
		}

		if acquired {
			logger.WithField("waited", time.Since(start)).
				Info("data migration lease acquired after waiting (took over a stale lease)")
			return true, nil
		}
	}
}

// releaseDataMigrationLease sets a terminal status and clears the lease, guarded
// by ownership (a process that no longer holds the lease is a no-op).
func (db *DB) releaseDataMigrationLease(ctx context.Context, dm *DataMigration, owner, status string) error {
	lb, err := db.leaseBuilder()
	if err != nil {
		return err
	}

	q, args := lb.ReleaseLease(DataMigrationTableName, status,
		[]dialect.Col{
			{Name: "package", Val: dm.Package},
			{Name: "version_id", Val: dm.Version},
		},
		owner)

	if _, err := db.ExecContext(ctx, q, args...); err != nil {
		return errors.Wrap(err, "failed to release data migration lease")
	}

	return nil
}

// isSchemaVersionApplied reports whether the given schema version of a package
// has been applied in the version table.
func (db *DB) isSchemaVersionApplied(ctx context.Context, pkgName string, version int64) (bool, error) {
	m, err := db.LoadMigration(ctx, &Migration{Package: pkgName, Version: version})
	if err != nil {
		return false, err
	}

	return m != nil && m.Record != nil && m.Record.IsApplied, nil
}

// RunDataMigration drives a single data migration to completion. It is safe to
// call repeatedly: a completed migration is skipped, and an interrupted one
// resumes from its last persisted checkpoint.
//
// Exactly one process drives a migration at a time, enforced by a lease in the
// state table. If another live process already holds the lease, ErrLeaseHeld is
// returned; if the lease is stolen mid-run (this process stalled past the TTL),
// ErrLeaseLost is returned and the in-flight batch is rolled back.
//
// Each batch, its checkpoint advance and the lease renewal commit together in
// one transaction, so a process that dies mid-batch rolls back cleanly and
// resumes without double-applying committed work.
func RunDataMigration(ctx context.Context, db *DB, dm *DataMigration) error {
	if dm.Migrator == nil {
		return fmt.Errorf("data migration %s has no migrator", dm)
	}

	// fail fast (before creating any table) when the dialect cannot honor the
	// lease that the data-migration runner depends on.
	if _, err := db.leaseBuilder(); err != nil {
		return fmt.Errorf("data migration %s: %w", dm, err)
	}

	// ensure both the version table (for the dependency check) and the
	// data-migration state table exist.
	if err := db.Touch(ctx); err != nil {
		return err
	}

	if err := db.TouchDataMigrationTable(ctx); err != nil {
		return err
	}

	logger := dm.logEntry()
	logger.Debug("starting data migration run")

	// dependency gate: the mapped schema migration must be applied first.
	if dm.After > 0 {
		afterPkg := dm.afterPackage()
		applied, err := db.isSchemaVersionApplied(ctx, afterPkg, dm.After)
		if err != nil {
			return err
		}

		if !applied {
			logger.WithFields(log.Fields{"after_package": afterPkg, "after_version": dm.After}).
				Warn("data migration blocked: schema dependency not applied yet")
			return fmt.Errorf("data migration %s depends on schema version %s:%d which is not applied yet", dm, afterPkg, dm.After)
		}

		logger.WithFields(log.Fields{"after_package": afterPkg, "after_version": dm.After}).
			Debug("schema dependency satisfied")
	}

	status, _, found, err := db.loadDataMigrationState(ctx, dm.Package, dm.Version)
	if err != nil {
		return err
	}

	if found && status == DataMigrationCompleted {
		logger.Info("data migration already completed, skipping")
		return nil
	}

	// make sure a row exists so the lease can be claimed.
	if !found {
		if err := db.insertDataMigrationState(ctx, db.DB, dm, DataMigrationPending, nil); err != nil {
			// another process may have inserted concurrently; re-check.
			status, _, found, lerr := db.loadDataMigrationState(ctx, dm.Package, dm.Version)
			if lerr != nil {
				return lerr
			}

			if !found {
				return err
			}

			if status == DataMigrationCompleted {
				logger.Info("data migration already completed, skipping")
				return nil
			}
		}
	}

	owner := leaseOwner()
	ttl := dm.leaseTTL()

	acquired, err := db.acquireDataMigrationLeaseWaiting(ctx, dm, owner, ttl, logger)
	if err != nil {
		return err
	}

	if !acquired {
		logger.Info("data migration is driven by another process, skipping")
		return ErrLeaseHeld
	}

	logger.WithFields(log.Fields{"lease_owner": owner, "lease_ttl": ttl}).Debug("data migration lease acquired")

	// we hold the lease; reload the authoritative status and checkpoint (a
	// stolen lease resumes from the previous owner's last committed batch).
	status, cp, _, err := db.loadDataMigrationState(ctx, dm.Package, dm.Version)
	if err != nil {
		return err
	}

	if status == DataMigrationCompleted {
		logger.Info("data migration already completed, skipping")
		return db.releaseDataMigrationLease(ctx, dm, owner, DataMigrationCompleted)
	}

	if status == DataMigrationPending || len(cp) == 0 {
		// First run, or a prior attempt failed before persisting any progress:
		// (re)compute the starting checkpoint. Plan is read-only and idempotent,
		// so repeating it is safe, and it guarantees Batch never receives an
		// empty checkpoint unless the migrator's own Plan returns one — sparing
		// callers from json.Unmarshal failing on an empty payload.
		logger.Info("planning data migration")
		cp, err = dm.Migrator.Plan(ctx, db.DB)
		if err != nil {
			if rerr := db.releaseDataMigrationLease(ctx, dm, owner, DataMigrationFailed); rerr != nil {
				logger.WithError(rerr).Warn("failed to release lease after plan error")
			}

			return fmt.Errorf("data migration %s: plan failed: %w", dm, err)
		}

		logger.WithField("checkpoint_bytes", len(cp)).Debug("data migration planned")
	} else {
		logger.WithField("checkpoint_bytes", len(cp)).Info("resuming data migration from stored checkpoint")
	}

	// attempts counts consecutive batch failures; it resets whenever a batch
	// commits successfully, so the backoff limit bounds retries of a stuck
	// batch, not transient failures spread across a long migration. batches
	// counts committed batches for progress logging.
	attempts := 0
	batches := 0
	startedAt := time.Now()
	for {
		if err := ctx.Err(); err != nil {
			return err
		}

		next, done, err := db.runDataBatch(ctx, dm, owner, ttl, cp)
		if err != nil {
			if errors.Is(err, ErrLeaseLost) {
				// another process owns the migration now; leave its state alone.
				logger.WithField("batches", batches).Warn("data migration lease lost to another process")
				return err
			}

			limit := dm.backoffLimit()
			attempts++
			if attempts <= limit {
				delay := dm.backoffDelay(attempts)
				logger.WithError(err).WithFields(log.Fields{"attempt": attempts, "limit": limit, "retry_in": delay}).
					Warn("data migration batch failed, retrying after backoff")
				select {
				case <-ctx.Done():
					return ctx.Err()
				case <-time.After(delay):
				}

				continue
			}

			if rerr := db.releaseDataMigrationLease(ctx, dm, owner, DataMigrationFailed); rerr != nil {
				logger.WithError(rerr).Warn("failed to mark data migration as failed")
			}

			logger.WithError(err).WithFields(log.Fields{"attempts": attempts, "batches": batches}).
				Error("data migration failed: batch retries exhausted")
			return fmt.Errorf("data migration %s: batch failed after %d attempt(s): %w", dm, attempts, err)
		}

		attempts = 0
		batches++
		cp = next

		logger.WithFields(log.Fields{"batch": batches, "checkpoint_bytes": len(cp), "done": done}).
			Debug("data migration batch committed")

		if done {
			logger.WithFields(log.Fields{"batches": batches, "elapsed": time.Since(startedAt)}).
				Info("data migration completed")
			return db.releaseDataMigrationLease(ctx, dm, owner, DataMigrationCompleted)
		}

		if dm.Throttle > 0 {
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(dm.Throttle):
			}
		}
	}
}

// runDataBatch runs one batch and persists its checkpoint while renewing the
// lease, all in a single transaction. It returns ErrLeaseLost if ownership was
// taken over before the batch could commit.
func (db *DB) runDataBatch(ctx context.Context, dm *DataMigration, owner string, ttl time.Duration, cp Checkpoint) (next Checkpoint, done bool, err error) {
	lb, err := db.leaseBuilder()
	if err != nil {
		return nil, false, err
	}

	tx, err := db.Begin()
	if err != nil {
		return nil, false, err
	}

	next, done, err = dm.Migrator.Batch(ctx, tx, cp)
	if err != nil {
		return nil, false, rollbackAndLogErr(err, tx, "data migration batch failed")
	}

	status := DataMigrationRunning
	if done {
		status = DataMigrationCompleted
	}

	expiresAt := time.Now().Add(ttl).Unix()

	q, args := lb.CommitLease(DataMigrationTableName,
		[]dialect.Col{
			{Name: "status", Val: status},
			{Name: "checkpoint", Val: string(next)},
			{Name: "lease_expires_at", Val: expiresAt},
		},
		[]dialect.Col{
			{Name: "package", Val: dm.Package},
			{Name: "version_id", Val: dm.Version},
		},
		owner)

	res, err := tx.ExecContext(ctx, q, args...)
	if err != nil {
		return nil, false, rollbackAndLogErr(err, tx, "failed to persist data migration checkpoint")
	}

	affected, err := res.RowsAffected()
	if err != nil {
		return nil, false, rollbackAndLogErr(err, tx, "failed to read affected rows for batch commit")
	}

	if affected == 0 {
		// the lease was stolen; discard this batch's work.
		return nil, false, rollbackAndLogErr(ErrLeaseLost, tx, "data migration lease lost")
	}

	if err := tx.Commit(); err != nil {
		return nil, false, errors.Wrap(err, "failed to commit data migration batch")
	}

	return next, done, nil
}

// RunDataMigrations runs the given data migrations in version order.
func RunDataMigrations(ctx context.Context, db *DB, dms []*DataMigration) error {
	log.WithFields(log.Fields{"component": dataMigratorComponent, "count": len(dms)}).
		Debug("running data migrations")

	for _, dm := range dms {
		if err := RunDataMigration(ctx, db, dm); err != nil {
			return err
		}
	}

	return nil
}

// RunRegisteredDataMigrations runs all the registered data migrations for the
// given packages (or every package when none is given), in version order.
func RunRegisteredDataMigrations(ctx context.Context, db *DB, packages ...string) error {
	var dms []*DataMigration
	if len(packages) == 0 {
		dms = DataMigrations()
	} else {
		for _, pkg := range packages {
			dms = append(dms, DataMigrationsByPackage(pkg)...)
		}

		sortDataMigrations(dms)
	}

	return RunDataMigrations(ctx, db, dms)
}
