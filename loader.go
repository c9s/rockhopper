package rockhopper

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"time"

	"github.com/pkg/errors"
)

var (
	// ErrNoCurrentVersion when a current migration version is not found.
	ErrNoCurrentVersion = errors.New("no current version found")

	ErrVersionNotFound = errors.New("migration version not found")

	SqlMigrationFilenamePattern = regexp.MustCompile("(\\d+)_(\\w+)\\.sql$")
)

func replaceExt(s string, ext string) string {
	return regexp.MustCompile("\\.\\w+$").ReplaceAllString(s, ext)
}

// MigrationRecord struct.
type MigrationRecord struct {
	VersionID int64     `db:"version_id"`
	Time      time.Time `db:"time"`
	IsApplied bool      `db:"is_applied"` // was this a result of up() or down()
	Package   string    `db:"package"`
}

type TransactionHandler func(ctx context.Context, exec SQLExecutor) error

var registeredGoMigrations map[int64]*Migration

// AddMigration adds a migration.
func AddMigration(up, down TransactionHandler) {
	pc, filename, _, _ := runtime.Caller(1)

	funcName := runtime.FuncForPC(pc).Name()
	lastSlash := strings.LastIndexByte(funcName, '/')
	if lastSlash < 0 {
		lastSlash = 0
	}
	lastDot := strings.LastIndexByte(funcName[lastSlash:], '.') + lastSlash
	packageName := funcName[:lastDot]
	AddNamedMigration(packageName, filename, up, down)
}

// AddNamedMigration : Add a named migration.
func AddNamedMigration(packageName, filename string, up, down TransactionHandler) {
	if registeredGoMigrations == nil {
		registeredGoMigrations = make(map[int64]*Migration)
	}

	v, _ := FileNumericComponent(filename)

	migration := &Migration{
		Package:    packageName,
		Registered: true,

		Version: v,
		UpFn:    up,
		DownFn:  down,
		Source:  filename,
		UseTx:   true,
	}

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

func (loader *GoMigrationLoader) Load() (MigrationSlice, error) {
	var migrations = MigrationSlice{}
	for _, migration := range registeredGoMigrations {
		migrations = append(migrations, migration)
	}

	return migrations.SortAndConnect(), nil
}

func (loader *GoMigrationLoader) LoadByPackageSuffix(suffix string) (MigrationSlice, error) {
	var migrations = MigrationSlice{}
	for _, migration := range registeredGoMigrations {
		if strings.HasSuffix(migration.Package, suffix) {
			migrations = append(migrations, migration)
		}
	}

	return migrations.SortAndConnect(), nil
}

func (loader *GoMigrationLoader) LoadByExactPackage(packageName string) (MigrationSlice, error) {
	var migrations = MigrationSlice{}
	for _, migration := range registeredGoMigrations {
		if migration.Package == packageName {
			migrations = append(migrations, migration)
		}
	}

	return migrations.SortAndConnect(), nil
}

type MigrationMap map[string]MigrationSlice

func (m MigrationMap) SortAndConnect() MigrationMap {
	newM := make(MigrationMap)
	for k, v := range m {
		newM[k] = v.Sort().Connect()
	}

	return newM
}

type SqlMigrationLoader struct {
	parser MigrationParser
}

// Load returns all the valid looking migration scripts in the
// migrations folders and go func registry, and key them by version.
func (loader *SqlMigrationLoader) Load(dirs ...string) (MigrationSlice, error) {
	var all MigrationSlice
	for _, d := range dirs {
		slice, err := loader.LoadDir(d)
		if err != nil {
			return nil, err
		}

		all = append(all, slice...)
	}

	return all.SortAndConnect(), nil
}

// LoadDir returns all the valid looking migration scripts in the
// migrations folder and go func registry, and key them by version.
func (loader *SqlMigrationLoader) LoadDir(dir string) (MigrationSlice, error) {
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

		name := SqlMigrationFilenamePattern.ReplaceAllString(filepath.Base(file), "$2")
		migration := &Migration{
			Package: DefaultPackageName,
			Version: v,
			Name:    name,
			Source:  file,
		}

		if err := migration.readSource(); err != nil {
			return nil, err
		}

		migrations = append(migrations, migration)
	}

	// Go migrations registered via goose.AddMigration().
	for _, migration := range registeredGoMigrations {
		migrations = append(migrations, migration)
	}

	return migrations.SortAndConnect(), nil
}

func (m *Migration) readSource() error {
	f, err := os.Open(m.Source)
	if err != nil {
		return errors.Wrapf(err, "ERROR %v: failed to open SQL migration file", filepath.Base(m.Source))
	}

	defer f.Close()

	var parser MigrationParser
	chunk, err := parser.Parse(f)
	if err != nil {
		return errors.Wrapf(err, "ERROR %v: failed to parse SQL migration file", filepath.Base(m.Source))
	}

	upStmts, downStmts, useTx := chunk.UpStmts, chunk.DownStmts, chunk.UseTx

	m.UseTx = useTx
	m.UpStatements = upStmts
	m.DownStatements = downStmts
	return nil
}
