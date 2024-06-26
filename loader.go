package rockhopper

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/pkg/errors"
	log "github.com/sirupsen/logrus"
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
	ID        int64     `db:"id"`
	VersionID int64     `db:"version_id"`
	Time      time.Time `db:"time"`
	IsApplied bool      `db:"is_applied"` // was this a result of up() or down()
	Package   string    `db:"package"`
}

type TransactionHandler func(ctx context.Context, exec SQLExecutor) error

var migrationVersionRegExp = regexp.MustCompile("_?(\\d{14,})_")

func parseVersionID(name string) (string, error) {
	matches := migrationVersionRegExp.FindStringSubmatch(name)
	if len(matches) < 2 {
		return "", fmt.Errorf("version number not found in filename: %s", name)
	}

	return matches[1], nil
}

// FileNumericComponent looks for migration scripts with names in the form:
// {VersionIdTimestampFormat}_descriptivename.ext where XXX specifies the version number
// and ext specifies the type of migration
//
// See VersionIdTimestampFormat
func FileNumericComponent(name string) (int64, error) {
	base := filepath.Base(name)

	if ext := filepath.Ext(base); ext != ".go" && ext != ".sql" {
		return 0, errors.New("not a recognized migration file type")
	}

	versionID, err := parseVersionID(base)
	if err != nil {
		return 0, err
	}

	n, err := strconv.ParseInt(versionID, 10, 64)
	if err != nil {
		return 0, err
	}

	if n <= 0 {
		return 0, errors.New("migration IDs must be greater than zero")
	}

	return n, nil
}

type MigrationLoader interface{}

type GoMigrationLoader struct{}

func (loader *GoMigrationLoader) Load() (MigrationSlice, error) {
	var migrations = MigrationSlice{}
	for _, migration := range registeredGoMigrations {
		migrations = append(migrations, migration)
	}

	return migrations.Sort(), nil
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

func (m MigrationMap) FilterPackage(pkgNames []string) MigrationMap {
	newM := make(MigrationMap)
	for k, v := range m {
		if sliceContains(pkgNames, k) {
			newM[k] = v
		}
	}

	return newM
}

func (m MigrationMap) SortAndConnect() MigrationMap {
	newM := make(MigrationMap)
	for k, v := range m {
		newM[k] = v.Sort().Connect()
	}

	return newM
}

type SqlMigrationLoader struct {
	defaultPackage string

	config *Config
}

func NewSqlMigrationLoader(config *Config) *SqlMigrationLoader {
	defaultPkgName := config.Package
	if defaultPkgName == "" {
		defaultPkgName = DefaultPackageName
	}

	return &SqlMigrationLoader{
		defaultPackage: defaultPkgName,
		config:         config,
	}
}

func (loader *SqlMigrationLoader) SetDefaultPackage(pkgName string) {
	loader.defaultPackage = pkgName
}

// Load returns all the valid looking migration scripts in the
// migrations folders and go func registry, and key them by version.
// Load method always returns a sorted migration slice
func (loader *SqlMigrationLoader) Load(dirs ...string) (MigrationSlice, error) {
	log.Debugf("starting loading sql migrations from %v", dirs)

	var all MigrationSlice
	for _, d := range dirs {
		log.Debugf("loading sql migrations from %v", d)

		slice, err := loader.LoadDir(d)
		if err != nil {
			return nil, err
		}

		all = append(all, slice...)
	}

	return all.Sort(), nil
}

// LoadDir returns all the valid looking migration scripts in the
// migrations folder and go func registry, and key them by version.
func (loader *SqlMigrationLoader) LoadDir(dir string) (MigrationSlice, error) {
	if _, err := os.Stat(dir); os.IsNotExist(err) {
		return nil, fmt.Errorf("%s directory does not exists", dir)
	}

	var migrations MigrationSlice

	// SQL migration files.
	files, err := filepath.Glob(dir + "/**.sql")
	if err != nil {
		return nil, err
	}

	defaultPkgName := loader.defaultPackage
	if defaultPkgName == "" {
		defaultPkgName = DefaultPackageName
	}

	for _, file := range files {
		versionID, err := FileNumericComponent(file)
		if err != nil {
			return nil, err
		}

		name := SqlMigrationFilenamePattern.ReplaceAllString(filepath.Base(file), "$2")
		migration := &Migration{
			Package: defaultPkgName,
			Version: versionID,
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

	if loader.config != nil && len(loader.config.IncludePackages) > 0 {
		migrations = migrations.FilterPackage(loader.config.IncludePackages)
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
		return errors.Wrapf(err, "%s: failed to parse SQL migration file", filepath.Base(m.Source))
	}

	m.Chunk = chunk
	m.UseTx = chunk.UseTx
	m.UpStatements = chunk.UpStmts
	m.DownStatements = chunk.DownStmts

	if chunk.Package != "" {
		m.Package = chunk.Package
	}
	return nil
}
