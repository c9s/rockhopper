package rockhopper

import (
	"context"
	"database/sql"
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"

	_ "github.com/mattn/go-sqlite3"
)

func TestDB_Open(t *testing.T) {
	dialect, err := LoadDialect("sqlite3")
	assert.NoError(t, err)

	db, err := Open("sqlite3", dialect, ":memory:", TableName)
	assert.NoError(t, err)
	assert.NotNil(t, db)

	err = db.Close()
	assert.NoError(t, err)
}

func TestDB_LoadMigrations(t *testing.T) {
	dialect, err := LoadDialect("sqlite3")
	assert.NoError(t, err)

	db, err := Open("sqlite3", dialect, ":memory:", TableName)
	assert.NoError(t, err)
	assert.NotNil(t, db)

	ctx := context.Background()

	tx, err := db.DB.Begin()
	assert.NoError(t, err)

	err = db.createVersionTable(ctx, tx, 1)
	assert.NoError(t, err)

	err = tx.Commit()
	assert.NoError(t, err)

	err = db.insertVersion(ctx, db.DB, DefaultPackageName, 2, true)
	assert.NoError(t, err)

	records, err := db.LoadMigrationRecordsByPackage(ctx, DefaultPackageName)
	assert.NoError(t, err)
	if assert.Len(t, records, 1) {
		record := records[0]
		assert.False(t, record.Time.IsZero())
		assert.True(t, record.IsApplied)

		t.Logf("tstamp: %s", record.Time.String())
	}

	err = db.Close()
	assert.NoError(t, err)
}

func TestDB_LoadMigrations_Integration(t *testing.T) {
	testCases := []struct {
		Driver  string
		DSN     string
		CleanUp func(db *sql.DB) error
	}{
		{
			Driver: "mysql",
			DSN:    "",
			CleanUp: func(db *sql.DB) error {
				_, err := db.Exec("DROP TABLE " + TableName)
				return err
			},
		},
		{
			Driver: "sqlite3",
			DSN:    ":memory:",
		},
	}

	ctx := context.Background()
	for _, testCase := range testCases {
		t.Run(testCase.Driver, func(t *testing.T) {
			dialect, err := LoadDialect(testCase.Driver)
			assert.NoError(t, err)

			dsn := testCase.DSN
			if dsn == "" {
				dsn = os.Getenv("TEST_" + strings.ToUpper(testCase.Driver) + "_DSN")
			}

			if dsn == "" {
				t.Skip()
			}

			db, err := Open(testCase.Driver, dialect, dsn, TableName)
			assert.NoError(t, err)
			assert.NotNil(t, db)

			defer func() {
				err := db.Close()
				assert.NoError(t, err)
			}()

			defer func() {
				if testCase.CleanUp != nil {
					err := testCase.CleanUp(db.DB)
					assert.NoError(t, err)
				}
			}()

			tx, err := db.DB.Begin()
			assert.NoError(t, err)

			err = db.createVersionTable(ctx, tx, 1)
			assert.NoError(t, err)

			err = tx.Commit()
			assert.NoError(t, err)

			err = db.insertVersion(ctx, db.DB, DefaultPackageName, 2, true)
			if assert.NoError(t, err) {
				defer func() {
					err = db.deleteVersion(ctx, db.DB, DefaultPackageName, 2)
					assert.NoError(t, err)
				}()

				records, err := db.LoadMigrationRecordsByPackage(ctx, DefaultPackageName)
				if assert.NoError(t, err) {
					if assert.Len(t, records, 1) {
						record := records[0]
						assert.False(t, record.Time.IsZero())
						assert.True(t, record.IsApplied)

						t.Logf("%s tstamp: %s", testCase.Driver, record.Time.String())
					}
				}
			}

		})
	}

}
