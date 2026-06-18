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

// updateDataMigrationState updates the status and checkpoint of a data
// migration. It accepts an executor so the update can run inside the same
// transaction as the batch it records.
func (db *DB) updateDataMigrationState(ctx context.Context, exec SQLExecutor, dm *DataMigration, status string, cp Checkpoint) error {
	if _, err := exec.ExecContext(ctx,
		db.dialect.UpdateDataMigrationSQL(DataMigrationTableName),
		status, string(cp), dm.Package, dm.Version); err != nil {
		return errors.Wrap(err, "failed to update data migration state")
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
// Each batch and its checkpoint advance are committed together in one
// transaction, so a process that dies mid-batch rolls back cleanly and resumes
// without double-applying committed work.
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

	status, cp, found, err := db.loadDataMigrationState(ctx, dm.Package, dm.Version)
	if err != nil {
		return err
	}

	if found && status == DataMigrationCompleted {
		log.Infof("data migration %s already completed, skipping", dm)
		return nil
	}

	if !found {
		// first run: compute the starting checkpoint and persist the initial row.
		cp, err = dm.Migrator.Plan(ctx, db.DB)
		if err != nil {
			return errors.Wrapf(err, "data migration %s: plan failed", dm)
		}

		if err := db.insertDataMigrationState(ctx, db.DB, dm, DataMigrationRunning, cp); err != nil {
			return err
		}
	} else {
		log.Infof("resuming data migration %s from checkpoint", dm)
	}

	for {
		if err := ctx.Err(); err != nil {
			return err
		}

		next, done, err := db.runDataBatch(ctx, dm, cp)
		if err != nil {
			// best-effort mark as failed in a separate transaction.
			if uerr := db.updateDataMigrationState(ctx, db.DB, dm, DataMigrationFailed, cp); uerr != nil {
				log.WithError(uerr).Warnf("failed to mark data migration %s as failed", dm)
			}

			return errors.Wrapf(err, "data migration %s: batch failed", dm)
		}

		cp = next

		if done {
			log.Infof("data migration %s completed", dm)
			return nil
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

// runDataBatch runs one batch and persists its checkpoint in a single
// transaction.
func (db *DB) runDataBatch(ctx context.Context, dm *DataMigration, cp Checkpoint) (next Checkpoint, done bool, err error) {
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

	if err := db.updateDataMigrationState(ctx, tx, dm, status, next); err != nil {
		return nil, false, rollbackAndLogErr(err, tx, "failed to persist data migration checkpoint")
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
