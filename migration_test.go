package rockhopper

import (
	"context"
	"testing"

	_ "github.com/mattn/go-sqlite3"
	"github.com/stretchr/testify/assert"
)

func TestLegacyGooseTableMigration_sqlite3(t *testing.T) {
	driverName := "sqlite3"
	dialect, err := LoadDialect(driverName)
	assert.NoError(t, err)

	db, err := Open(driverName, dialect, ":memory:", legacyGooseTableName)
	assert.NoError(t, err)

	defer db.Close()

	_, err = db.Exec(`CREATE TABLE IF NOT EXISTS legacyGooseTableName (
                id INTEGER PRIMARY KEY AUTOINCREMENT,
                package TEXT NOT NULL DEFAULT 'main',
                version_id INTEGER NOT NULL,
                is_applied INTEGER NOT NULL,
                tstamp TIMESTAMP DEFAULT (datetime('now'))) `)
	assert.NoError(t, err)

	ctx := context.Background()
	err = db.runCoreMigration(ctx)
	assert.NoError(t, err)

	tx, err := db.Begin()
	assert.NoError(t, err)

	err = db.insertVersion(ctx, tx, "main", "", 2000, true)
	assert.NoError(t, err)

	err = tx.Commit()
	assert.NoError(t, err)
}

func TestMigration_UpAndDown(t *testing.T) {
	driverName := "sqlite3"
	dialect, err := LoadDialect(driverName)
	if err != nil {
		t.Fatal(err)
	}

	db, err := Open(driverName, dialect, ":memory:", TableName)
	if err != nil {
		t.Fatal(err)
	}

	defer db.Close()

	ctx := context.Background()
	_, err = db.CurrentVersion(ctx, DefaultPackageName)
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

	err = Up(ctx, db, migrations.Head(), 0)
	assert.NoError(t, err)

	_, err = db.CurrentVersion(ctx, DefaultPackageName)
	assert.NoError(t, err)

	err = Down(ctx, db, migrations.Tail(), 0)
	assert.NoError(t, err)
}

// TestMigration_SkipsEmptyStatements guards that statements left empty after a
// migration's queries are merged into another file (empty string, comment-only,
// or a lone semicolon) are skipped at execution time instead of failing with
// errors like MySQL 1065 "Query was empty".
func TestMigration_SkipsEmptyStatements(t *testing.T) {
	driverName := "sqlite3"
	dialect, err := LoadDialect(driverName)
	if err != nil {
		t.Fatal(err)
	}

	db, err := Open(driverName, dialect, ":memory:", TableName)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	ctx := context.Background()
	_, err = db.CurrentVersion(ctx, DefaultPackageName)
	assert.NoError(t, err)

	migrations := MigrationSlice{
		{
			Version: 1,
			Source:  "migrations/1_merged.sql",
			UseTx:   true,
			UpStatements: []Statement{
				{Direction: DirectionUp, SQL: ""},
				{Direction: DirectionUp, SQL: "   \n  "},
				{Direction: DirectionUp, SQL: "-- queries were merged into another file"},
				{Direction: DirectionUp, SQL: ";"},
				{Direction: DirectionUp, SQL: "CREATE TABLE kept (id int)"},
			},
			DownStatements: []Statement{
				{Direction: DirectionDown, SQL: ""},
				{Direction: DirectionDown, SQL: "DROP TABLE kept"},
			},
		},
	}

	migrations = migrations.SortAndConnect()

	err = Up(ctx, db, migrations.Head(), 0)
	assert.NoError(t, err, "empty statements must be skipped, not executed")

	// the real statement still ran
	_, err = db.Exec("INSERT INTO kept (id) VALUES (1)")
	assert.NoError(t, err, "the non-empty statement should have created the table")

	err = Down(ctx, db, migrations.Tail(), 0)
	assert.NoError(t, err, "empty down statements must be skipped too")
}

// TestMigration_ErrorIncludesLocation guards that a failing statement surfaces
// the migration's source filename and version in the error message.
func TestMigration_ErrorIncludesLocation(t *testing.T) {
	driverName := "sqlite3"
	dialect, err := LoadDialect(driverName)
	if err != nil {
		t.Fatal(err)
	}

	db, err := Open(driverName, dialect, ":memory:", TableName)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	ctx := context.Background()
	_, err = db.CurrentVersion(ctx, DefaultPackageName)
	assert.NoError(t, err)

	migrations := MigrationSlice{
		{
			Version: 20240131120000,
			Source:  "migrations/20240131120000_broken.sql",
			Package: DefaultPackageName,
			UseTx:   true,
			UpStatements: []Statement{
				{Direction: DirectionUp, SQL: "THIS IS NOT VALID SQL"},
			},
		},
	}

	migrations = migrations.SortAndConnect()

	err = Up(ctx, db, migrations.Head(), 0)
	if assert.Error(t, err) {
		assert.Contains(t, err.Error(), "20240131120000_broken.sql")
		assert.Contains(t, err.Error(), "20240131120000")
	}
}

func TestIsNoOpSQL(t *testing.T) {
	tests := []struct {
		name string
		sql  string
		want bool
	}{
		{"empty", "", true},
		{"whitespace", "   \n\t ", true},
		{"lone semicolon", ";", true},
		{"multiple semicolons", "; ;\n;", true},
		{"comment only", "-- a comment", true},
		{"comment with semicolon", "-- a comment\n;", true},
		{"real statement", "CREATE TABLE t (id int);", false},
		{"statement with trailing comment", "SELECT 1; -- note", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, isNoOpSQL(tt.sql))
		})
	}
}
