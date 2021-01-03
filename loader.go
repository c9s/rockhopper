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

	"github.com/pkg/errors"
)

var (
	// ErrNoCurrentVersion when a current migration version is not found.
	ErrNoCurrentVersion = errors.New("no current version found")
	// ErrNoNextVersion when the next migration version is not found.
	ErrNoNextVersion = errors.New("no next version found")
)

type Migration struct {
	Version    int64
	Next       *Migration
	Previous   *Migration

	Source     string // path to .sql script
	Registered bool
	UpFn       func(*sql.Tx) error // Up go migration function
	DownFn     func(*sql.Tx) error // Down go migration function
}

type MigrationSlice []*Migration

// helpers so we can use pkg sort
func (ms MigrationSlice) Len() int      { return len(ms) }
func (ms MigrationSlice) Swap(i, j int) { ms[i], ms[j] = ms[j], ms[i] }
func (ms MigrationSlice) Less(i, j int) bool {
	if ms[i].Version == ms[j].Version {
		panic(fmt.Sprintf("goose: duplicate version %v detected:\n%v\n%v", ms[i].Version, ms[i].Source, ms[j].Source))
	}
	return ms[i].Version < ms[j].Version
}

// Current gets the current migration.
func (ms MigrationSlice) Current(current int64) (*Migration, error) {
	for i, migration := range ms {
		if migration.Version == current {
			return ms[i], nil
		}
	}

	return nil, ErrNoCurrentVersion
}

// Next gets the next migration.
func (ms MigrationSlice) Next(current int64) (*Migration, error) {
	for i, migration := range ms {
		if migration.Version > current {
			return ms[i], nil
		}
	}

	return nil, ErrNoNextVersion
}

// Previous : Get the previous migration.
func (ms MigrationSlice) Previous(current int64) (*Migration, error) {
	for i := len(ms) - 1; i >= 0; i-- {
		if ms[i].Version < current {
			return ms[i], nil
		}
	}

	return nil, ErrNoNextVersion
}

// Last gets the last migration.
func (ms MigrationSlice) Last() (*Migration, error) {
	if len(ms) == 0 {
		return nil, ErrNoNextVersion
	}

	return ms[len(ms)-1], nil
}

var registeredGoMigrations map[int64]*Migration

// AddMigration adds a migration.
func AddMigration(up func(*sql.Tx) error, down func(*sql.Tx) error) {
	_, filename, _, _ := runtime.Caller(1)
	AddNamedMigration(filename, up, down)
}

// AddNamedMigration : Add a named migration.
func AddNamedMigration(filename string, up func(*sql.Tx) error, down func(*sql.Tx) error) {
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

type MigrationLoader interface{}

type GoMigrationLoader struct{}

func (loader *GoMigrationLoader) Load(dir string) (MigrationSlice, error) {
	if _, err := os.Stat(dir); os.IsNotExist(err) {
		return nil, fmt.Errorf("%s directory does not exists", dir)
	}

	var migrations MigrationSlice

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

	return sortAndConnectMigrations(migrations), nil
}

type SqlMigrationLoader struct{}

// CollectMigrations returns all the valid looking migration scripts in the
// migrations folder and go func registry, and key them by version.
func (loader *SqlMigrationLoader) Load(dir string) (MigrationSlice, error) {
	if _, err := os.Stat(dir); os.IsNotExist(err) {
		return nil, fmt.Errorf("%s directory does not exists", dir)
	}

	var migrations MigrationSlice

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

		migrations = append(migrations, &Migration{Version: v, Source: file})
	}

	// Go migrations registered via goose.AddMigration().
	for _, migration := range registeredGoMigrations {
		migrations = append(migrations, migration)
	}

	return sortAndConnectMigrations(migrations), nil
}

func sortAndConnectMigrations(migrations MigrationSlice) MigrationSlice {
	sort.Sort(migrations)

	// now that we're sorted in the appropriate direction,
	// populate next and previous for each migration
	for i, m := range migrations {
		var prev *Migration = nil
		if i > 0 {
			prev = migrations[i-1]
			migrations[i-1].Next = m
		}

		migrations[i].Previous = prev
	}

	return migrations
}
