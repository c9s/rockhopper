package rockhopper

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/pkg/errors"
	log "github.com/sirupsen/logrus"
)

// TouchDataMigrationTable creates the data-migration state table if it does not
// exist yet.
func (db *DB) TouchDataMigrationTable(ctx context.Context) error {
	if _, err := db.ExecContext(ctx, db.dialect.CreateDataMigrationTableSQL(DataMigrationTableName)); err != nil {
		return errors.Wrap(err, "failed to create data migration table")
	}

	return nil
}

// loadDataMigrationState loads the persisted status and checkpoint for a data
// migration. found is false when no row exists yet.
func (db *DB) loadDataMigrationState(ctx context.Context, pkgName string, version int64) (status string, cp Checkpoint, found bool, err error) {
	row := db.QueryRowContext(ctx, db.dialect.SelectDataMigrationSQL(DataMigrationTableName), pkgName, version)

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
	if _, err := exec.ExecContext(ctx,
		db.dialect.InsertDataMigrationSQL(DataMigrationTableName),
		dm.Package, dm.Version, dm.Name, status, string(cp)); err != nil {
		return errors.Wrap(err, "failed to insert data migration state")
	}

	return nil
}

// acquireDataMigrationLease attempts to claim the lease for a data migration.
// It succeeds when the lease is unowned, already owned by this process, or
// expired. It returns false (without error) when another live process holds it.
func (db *DB) acquireDataMigrationLease(ctx context.Context, dm *DataMigration, owner string, ttl time.Duration) (bool, error) {
	now := time.Now()
	expiresAt := now.Add(ttl).Unix()

	res, err := db.ExecContext(ctx,
		db.dialect.AcquireDataMigrationLeaseSQL(DataMigrationTableName),
		owner, expiresAt, dm.Package, dm.Version, owner, now.Unix())
	if err != nil {
		return false, errors.Wrap(err, "failed to acquire data migration lease")
	}

	affected, err := res.RowsAffected()
	if err != nil {
		return false, errors.Wrap(err, "failed to read affected rows for lease acquisition")
	}

	return affected == 1, nil
}

// releaseDataMigrationLease sets a terminal status and clears the lease, guarded
// by ownership (a process that no longer holds the lease is a no-op).
func (db *DB) releaseDataMigrationLease(ctx context.Context, dm *DataMigration, owner, status string) error {
	if _, err := db.ExecContext(ctx,
		db.dialect.ReleaseDataMigrationLeaseSQL(DataMigrationTableName),
		status, dm.Package, dm.Version, owner); err != nil {
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

	// ensure both the version table (for the dependency check) and the
	// data-migration state table exist.
	if err := db.Touch(ctx); err != nil {
		return err
	}

	if err := db.TouchDataMigrationTable(ctx); err != nil {
		return err
	}

	// dependency gate: the mapped schema migration must be applied first.
	if dm.After > 0 {
		applied, err := db.isSchemaVersionApplied(ctx, dm.Package, dm.After)
		if err != nil {
			return err
		}

		if !applied {
			return fmt.Errorf("data migration %s depends on schema version %d which is not applied yet", dm, dm.After)
		}
	}

	status, _, found, err := db.loadDataMigrationState(ctx, dm.Package, dm.Version)
	if err != nil {
		return err
	}

	if found && status == DataMigrationCompleted {
		log.Infof("data migration %s already completed, skipping", dm)
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
				log.Infof("data migration %s already completed, skipping", dm)
				return nil
			}
		}
	}

	owner := leaseOwner()
	ttl := dm.leaseTTL()

	acquired, err := db.acquireDataMigrationLease(ctx, dm, owner, ttl)
	if err != nil {
		return err
	}

	if !acquired {
		log.Infof("data migration %s is driven by another process, skipping", dm)
		return ErrLeaseHeld
	}

	// we hold the lease; reload the authoritative status and checkpoint (a
	// stolen lease resumes from the previous owner's last committed batch).
	status, cp, _, err := db.loadDataMigrationState(ctx, dm.Package, dm.Version)
	if err != nil {
		return err
	}

	if status == DataMigrationCompleted {
		log.Infof("data migration %s already completed, skipping", dm)
		return db.releaseDataMigrationLease(ctx, dm, owner, DataMigrationCompleted)
	}

	if status == DataMigrationPending {
		// first run: compute the starting checkpoint.
		cp, err = dm.Migrator.Plan(ctx, db.DB)
		if err != nil {
			if rerr := db.releaseDataMigrationLease(ctx, dm, owner, DataMigrationFailed); rerr != nil {
				log.WithError(rerr).Warnf("failed to release lease for data migration %s after plan error", dm)
			}

			return errors.Wrapf(err, "data migration %s: plan failed", dm)
		}
	} else {
		log.Infof("resuming data migration %s from checkpoint", dm)
	}

	for {
		if err := ctx.Err(); err != nil {
			return err
		}

		next, done, err := db.runDataBatch(ctx, dm, owner, ttl, cp)
		if err != nil {
			if errors.Is(err, ErrLeaseLost) {
				// another process owns the migration now; leave its state alone.
				return err
			}

			if rerr := db.releaseDataMigrationLease(ctx, dm, owner, DataMigrationFailed); rerr != nil {
				log.WithError(rerr).Warnf("failed to mark data migration %s as failed", dm)
			}

			return errors.Wrapf(err, "data migration %s: batch failed", dm)
		}

		cp = next

		if done {
			log.Infof("data migration %s completed", dm)
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

	res, err := tx.ExecContext(ctx,
		db.dialect.CommitDataBatchSQL(DataMigrationTableName),
		status, string(next), expiresAt, dm.Package, dm.Version, owner)
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
