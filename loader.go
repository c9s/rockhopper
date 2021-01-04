package rockhopper

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/pkg/errors"
)

var (
	// ErrNoCurrentVersion when a current migration version is not found.
	ErrNoCurrentVersion = errors.New("no current version found")

	// ErrNoNextVersion when the next migration version is not found.
	ErrNoNextVersion = errors.New("no next version found")
)

// MigrationRecord struct.
type MigrationRecord struct {
	VersionID int64
	Time      time.Time
	IsApplied bool // was this a result of up() or down()
}

type TransactionHandler func(tx *sql.Tx) error

var registeredGoMigrations map[int64]*Migration

// AddMigration adds a migration.
func AddMigration(up, down TransactionHandler) {
	_, filename, _, _ := runtime.Caller(1)
	AddNamedMigration(filename, up, down)
}

// AddNamedMigration : Add a named migration.
func AddNamedMigration(filename string, up, down TransactionHandler) {
	v, _ := FileNumericComponent(filename)

	migration := &Migration{Version: v, Registered: true, UpFn: up, DownFn: down, Source: filename}
	if existing, ok := registeredGoMigrations[v]; ok {
		panic(fmt.Sprintf("failed to add migration %q: version conflicts with %q", filename, existing.Source))
	}
	registeredGoMigrations[v] = migration
}

// FileNumericComponent looks for migration scripts with names in the form:
// XXX_descriptivename.ext where XXX specifies the version number
// and ext specifies the type of migration
func FileNumericComponent(name string) (int64, error) {
	base := filepath.Base(name)

	if ext := filepath.Ext(base); ext != ".go" && ext != ".sql" {
		return 0, errors.New("not a recognized migration file type")
	}

	idx := strings.Index(base, "_")
	if idx < 0 {
		return 0, errors.New("no separator found")
	}

	n, e := strconv.ParseInt(base[:idx], 10, 64)
	if e == nil && n <= 0 {
		return 0, errors.New("migration IDs must be greater than zero")
	}

	return n, e
}

func filterMigrationVersions(v, current, target int64) bool {
	if target > current {
		return v > current && v <= target
	}

	if target < current {
		return v <= current && v > target
	}

	return false
}

type MigrationLoader interface{}

type GoMigrationLoader struct{}

func (loader *GoMigrationLoader) Load(dir string) (MigrationSlice, error) {
	if _, err := os.Stat(dir); os.IsNotExist(err) {
		return nil, fmt.Errorf("%s directory does not exists", dir)
	}

	var migrations = MigrationSlice{}

	// Go migration files
	goMigrationFiles, err := filepath.Glob(dir + "/**.go")
	if err != nil {
		return nil, err
	}
	for _, file := range goMigrationFiles {
		v, err := FileNumericComponent(file)
		if err != nil {
			continue // Skip any files that don't have version prefix.
		}

		// Skip migrations already existing migrations registered via goose.AddMigration().
		if _, ok := registeredGoMigrations[v]; ok {
			continue
		}

		migration := &Migration{Version: v, Source: file, Registered: false}
		migrations = append(migrations, migration)
	}

	return migrations.Connect(), nil
}

type SqlMigrationLoader struct {
	parser MigrationParser
}

// CollectMigrations returns all the valid looking migration scripts in the
// migrations folder and go func registry, and key them by version.
func (loader *SqlMigrationLoader) Load(dir string) (MigrationSlice, error) {
	if _, err := os.Stat(dir); os.IsNotExist(err) {
		return nil, fmt.Errorf("%s directory does not exists", dir)
	}

	var migrations = MigrationSlice{}

	// SQL migration files.
	files, err := filepath.Glob(dir + "/**.sql")
	if err != nil {
		return nil, err
	}

	for _, file := range files {
		v, err := FileNumericComponent(file)
		if err != nil {
			return nil, err
		}

		migration := &Migration{Version: v, Source: file}

		if err := loader.readSource(migration); err != nil {
			return nil, err
		}

		migrations = append(migrations, migration)
	}

	// Go migrations registered via goose.AddMigration().
	for _, migration := range registeredGoMigrations {
		migrations = append(migrations, migration)
	}

	sort.Sort(migrations)
	return migrations.Connect(), nil
}

func (loader *SqlMigrationLoader) readSource(m *Migration) error {
	f, err := os.Open(m.Source)
	if err != nil {
		return errors.Wrapf(err, "ERROR %v: failed to open SQL migration file", filepath.Base(m.Source))
	}

	defer f.Close()

	upStmts, downStmts, err := loader.parser.Parse(f)
	if err != nil {
		return errors.Wrapf(err, "ERROR %v: failed to parse SQL migration file", filepath.Base(m.Source))
	}

	m.UpStatements = upStmts
	m.DownStatements = downStmts
	return nil
}
