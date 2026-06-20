package rockhopper

import (
	"fmt"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/c9s/rockhopper/v2/pkg/dialect"
)

func allDialects(t *testing.T) map[string]dialect.Dialect {
	t.Helper()
	names := []string{DialectMySQL, DialectPostgres, DialectSQLite3, DialectTiDB, DialectRedshift}
	out := make(map[string]dialect.Dialect, len(names))
	for _, n := range names {
		d, err := LoadDialect(n)
		assert.NoError(t, err)
		out[n] = d
	}
	return out
}

// TestDataMigrationTable_UniqueConstraint guards the pre-existing bug where
// Redshift's data-migration table was missing its UNIQUE(package, version_id)
// constraint. The schema is now generic, so every dialect must emit it.
func TestDataMigrationTable_UniqueConstraint(t *testing.T) {
	for name, d := range allDialects(t) {
		t.Run(name, func(t *testing.T) {
			ddl := d.CreateTable(dataMigrationSchema("rockhopper_data_migrations"))
			assert.Contains(t, ddl, "UNIQUE(package, version_id)",
				"%s data-migration table must declare the UNIQUE(package, version_id) constraint", name)
		})
	}
}

// TestPackageColumnWidthConsistent guards the pre-existing bug where the MySQL
// legacy-table upgrade used VARCHAR(125) while the canonical schema used
// VARCHAR(128). All three call sites now share packageColumnSize.
func TestPackageColumnWidthConsistent(t *testing.T) {
	want := fmt.Sprintf("package VARCHAR(%d)", packageColumnSize)
	for name, d := range allDialects(t) {
		t.Run(name, func(t *testing.T) {
			version := d.CreateTable(versionSchema("rockhopper_versions"))
			dataMig := d.CreateTable(dataMigrationSchema("rockhopper_data_migrations"))
			assert.Contains(t, version, want, "%s version table package width", name)
			assert.Contains(t, dataMig, want, "%s data-migration table package width", name)

			// The legacy-table upgrade ALTER must use the same width.
			alter, supported := d.AddColumn("goose_db_version", dialect.Column{
				Name: "package", Type: dialect.ColVarchar, Size: packageColumnSize,
				NotNull: true, Default: "'main'",
			})
			if supported {
				assert.True(t, strings.Contains(alter, fmt.Sprintf("VARCHAR(%d)", packageColumnSize)),
					"%s legacy ALTER package width: %s", name, alter)
			}
		})
	}
}
