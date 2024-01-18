package rockhopper

import (
	"context"

	log "github.com/sirupsen/logrus"
)

func Align(ctx context.Context, db *DB, versionID int64, migrations MigrationSlice) error {
	_, lastAppliedMigration, err := db.FindLastAppliedMigration(ctx, migrations)
	if err != nil {
		return err
	}

	if lastAppliedMigration == nil {
		return Up(ctx, db, migrations.Head(), versionID)
	}

	if versionID < lastAppliedMigration.Version {
		return Down(ctx, db, lastAppliedMigration, versionID)
	} else if lastAppliedMigration != nil && versionID > lastAppliedMigration.Version {
		return Up(ctx, db, lastAppliedMigration.Next, versionID)
	} else {
		log.Infof("the migration version is already aligned to %d", versionID)
		return nil
	}
}
