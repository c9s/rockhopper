package rockhopper

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/c9s/rockhopper/v2/pkg/dialect"
)

// noopMigrator is a DataMigrator that does nothing; it only needs to be non-nil
// so RunDataMigration gets past its migrator check and reaches the dialect gate.
type noopMigrator struct{}

func (noopMigrator) Plan(context.Context, Queryer) (Checkpoint, error) { return nil, nil }
func (noopMigrator) Batch(context.Context, BatchExecutor, Checkpoint) (Checkpoint, bool, error) {
	return nil, true, nil
}

func TestClickHouse_DataMigrationUnsupported(t *testing.T) {
	// The capability gate runs before any DB access, so a nil *sql.DB is fine.
	db := New(DialectClickHouse, dialect.NewClickHouseDialect(), nil, "rockhopper_versions")

	_, err := db.leaseBuilder()
	assert.ErrorIs(t, err, ErrDataMigrationUnsupported)

	dm := &DataMigration{Package: "main", Version: 1, Name: "x", Migrator: noopMigrator{}}
	err = RunDataMigration(context.Background(), db, dm)
	assert.ErrorIs(t, err, ErrDataMigrationUnsupported,
		"RunDataMigration must refuse to run on a dialect without the lease capability")
}

func TestOLTP_DataMigrationSupported(t *testing.T) {
	d, err := LoadDialect(DialectSQLite3)
	assert.NoError(t, err)

	db := New(DialectSQLite3, d, nil, "rockhopper_versions")
	lb, err := db.leaseBuilder()
	assert.NoError(t, err)
	assert.NotNil(t, lb)
}
