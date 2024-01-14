package rockhopper

import (
	"context"
	"testing"

	_ "github.com/mattn/go-sqlite3"
	"github.com/stretchr/testify/assert"
)

func TestMigration_UpAndDown(t *testing.T) {
	driverName := "sqlite3"

	dialect, err := LoadDialect(driverName)
	if err != nil {
		t.Fatal(err)
	}

	db, err := Open(driverName, dialect, ":memory:", legacyGooseTableName)
	if err != nil {
		t.Fatal(err)
	}

	defer db.Close()

	currentVersion, err := db.CurrentVersion()
	assert.NoError(t, err)

	migrations := MigrationSlice{
		{
			Version: 1,
			UseTx:   true,
			UpStatements: []Statement{
				{Direction: DirectionUp, SQL: "CREATE TABLE a (id int)"},
			},
			DownStatements: []Statement{
				{Direction: DirectionDown, SQL: "DROP TABLE a"},
			},
		},
		{
			Version: 2,
			UseTx:   true,
			UpStatements: []Statement{
				{Direction: DirectionUp, SQL: "CREATE TABLE b (id int)"},
			},
			DownStatements: []Statement{
				{Direction: DirectionDown, SQL: "DROP TABLE b"},
			},
		},
	}

	migrations = migrations.SortAndConnect()
	assert.NotEmpty(t, migrations)

	ctx := context.Background()

	err = Up(ctx, db, migrations, currentVersion, 0)
	assert.NoError(t, err)

	currentVersion, err = db.CurrentVersion()
	assert.NoError(t, err)

	err = Down(ctx, db, migrations, currentVersion, 0)
	assert.NoError(t, err)
}
