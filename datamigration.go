package rockhopper

import (
	"context"
	"database/sql"
	"fmt"
	"runtime"
	"strings"
	"time"

	log "github.com/sirupsen/logrus"
)

// DataMigrationTableName is the table that tracks data-migration progress
// (status + checkpoint). It is intentionally separate from the binary version
// table (TableName) so the schema runner's done/not-done semantics stay intact.
const DataMigrationTableName = "rockhopper_data_migrations"

// Data migration status values stored in the status column.
const (
	// DataMigrationPending means the migration has a row but no batch has run yet.
	DataMigrationPending = "pending"
	// DataMigrationRunning means the migration has started and has a checkpoint.
	DataMigrationRunning = "running"
	// DataMigrationCompleted means every batch has been applied.
	DataMigrationCompleted = "completed"
	// DataMigrationFailed means a batch returned an error.
	DataMigrationFailed = "failed"
)

// Checkpoint is the opaque, serializable progress cursor of a data migration.
// The framework stores it verbatim and hands it back on resume; only the
// migrator's own code interprets its contents (commonly JSON).
type Checkpoint []byte

// Queryer is the read side of a database handle. Data migrations need to read
// rows to compute batch ranges and detect completion, which the core
// SQLExecutor (ExecContext only) does not expose. Both *sql.DB and *sql.Tx
// satisfy Queryer.
type Queryer interface {
	QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error)
	QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row
}

// BatchExecutor is handed to DataMigrator.Batch. It is bound to the transaction
// that also persists the advanced checkpoint, so a batch's writes and its
// checkpoint commit atomically. *sql.Tx satisfies this interface.
type BatchExecutor interface {
	SQLExecutor
	Queryer
}

// DataMigrator is implemented by user applications to define a long-running,
// resumable data migration (e.g. a chunked backfill). The framework owns the
// loop, checkpoint persistence, throttling and resume; the migrator owns the
// per-batch logic.
type DataMigrator interface {
	// Plan is called once before batching begins, when no checkpoint has been
	// persisted yet. It may inspect the database (read-only) to compute bounds
	// such as min/max primary keys, and returns the initial checkpoint. A nil
	// checkpoint is allowed.
	Plan(ctx context.Context, q Queryer) (Checkpoint, error)

	// Batch processes a single chunk of work starting from cp and returns the
	// advanced checkpoint and whether the migration is complete. exec is bound
	// to the transaction that persists the returned checkpoint, so Batch must
	// not commit, roll back or sleep. Batches should be idempotent so that an
	// interrupted batch can safely be retried on resume.
	Batch(ctx context.Context, exec BatchExecutor, cp Checkpoint) (next Checkpoint, done bool, err error)
}

// DataMigration is the registered descriptor for a data migration. It carries
// the migrator together with scheduling metadata (dependency, throttle).
type DataMigration struct {
	// Package is the migration package name.
	Package string

	// Version is the data-migration version (parsed from the source filename).
	Version int64

	// Name is a human-readable description.
	Name string

	// Source is the path to the file that registered the migration.
	Source string

	// Migrator holds the user-provided batch logic.
	Migrator DataMigrator

	// After is the schema migration version (same package) that must be applied
	// before this data migration becomes eligible to run. Zero means no
	// dependency.
	After int64

	// Throttle is the pause inserted between batches to limit load and
	// replication lag. Zero means no pause.
	Throttle time.Duration
}

func (dm *DataMigration) String() string {
	if dm.Name != "" {
		return fmt.Sprintf("%s:%d (%s)", dm.Package, dm.Version, dm.Name)
	}

	return fmt.Sprintf("%s:%d", dm.Package, dm.Version)
}

// DataMigrationOption customizes a registered DataMigration.
type DataMigrationOption func(*DataMigration)

// After declares that the data migration may only run once the given schema
// migration version (in the same package) has been applied.
func After(schemaVersion int64) DataMigrationOption {
	return func(dm *DataMigration) {
		dm.After = schemaVersion
	}
}

// WithThrottle sets the pause inserted between batches.
func WithThrottle(d time.Duration) DataMigrationOption {
	return func(dm *DataMigration) {
		dm.Throttle = d
	}
}

// WithDataMigrationName sets a human-readable description.
func WithDataMigrationName(name string) DataMigrationOption {
	return func(dm *DataMigration) {
		dm.Name = name
	}
}

// registeredDataMigrations stores the globally registered data migrations,
// keyed the same way as registeredGoMigrations.
var registeredDataMigrations = map[RegistryKey]*DataMigration{}

// AddDataMigration registers a data migration into the global map. The version
// is parsed from the caller's source filename and the package is derived from
// the caller's function name, mirroring AddMigration.
func AddDataMigration(m DataMigrator, opts ...DataMigrationOption) {
	pc, filename, _, _ := runtime.Caller(1)

	funcName := runtime.FuncForPC(pc).Name()
	lastSlash := strings.LastIndexByte(funcName, '/')
	if lastSlash < 0 {
		lastSlash = 0
	}

	lastDot := strings.LastIndexByte(funcName[lastSlash:], '.') + lastSlash
	packageName := funcName[:lastDot]
	AddNamedDataMigration(packageName, filename, m, opts...)
}

// AddNamedDataMigration registers a data migration with an explicit package and
// source filename. The version is parsed from the filename.
func AddNamedDataMigration(packageName, filename string, m DataMigrator, opts ...DataMigrationOption) {
	v, err := FileNumericComponent(filename)
	if err != nil {
		log.Panic(err)
	}

	dm := &DataMigration{
		Package:  packageName,
		Version:  v,
		Source:   filename,
		Migrator: m,
	}

	for _, opt := range opts {
		opt(dm)
	}

	key := RegistryKey{Package: packageName, Version: v}
	if existing, ok := registeredDataMigrations[key]; ok {
		panic(fmt.Sprintf("failed to add data migration %q: version conflicts with %q", filename, existing.Source))
	}

	registeredDataMigrations[key] = dm
}

// DataMigrations returns all the registered data migrations sorted by version.
func DataMigrations() []*DataMigration {
	var all []*DataMigration
	for _, dm := range registeredDataMigrations {
		all = append(all, dm)
	}

	sortDataMigrations(all)
	return all
}

// DataMigrationsByPackage returns the registered data migrations for the given
// package, sorted by version.
func DataMigrationsByPackage(packageName string) []*DataMigration {
	var all []*DataMigration
	for _, dm := range registeredDataMigrations {
		if dm.Package == packageName {
			all = append(all, dm)
		}
	}

	sortDataMigrations(all)
	return all
}

func sortDataMigrations(s []*DataMigration) {
	for i := 1; i < len(s); i++ {
		for j := i; j > 0 && s[j-1].Version > s[j].Version; j-- {
			s[j-1], s[j] = s[j], s[j-1]
		}
	}
}
