package rockhopper

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"fmt"
	"os"
	"runtime"
	"sync"
	"time"

	"github.com/pkg/errors"
	log "github.com/sirupsen/logrus"
)

// DataMigrationTableName is the table that tracks data-migration progress
// (status + checkpoint). It is intentionally separate from the binary version
// table (TableName) so the schema runner's done/not-done semantics stay intact.
const DataMigrationTableName = "rockhopper_data_migrations"

// dataMigratorComponent is the value of the "component" log field stamped on
// every data-migration log line, so operators can filter the data-migrator's
// output apart from the schema runner's.
const dataMigratorComponent = "data_migrator"

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

// DefaultLeaseTTL is the lease duration used when a data migration does not set
// one. The lease is renewed on every batch commit, so the TTL only needs to
// comfortably exceed a single batch's duration plus its throttle pause.
const DefaultLeaseTTL = 30 * time.Second

// DefaultBackoffLimit is the number of times a failed batch is retried before
// the data migration gives up and is marked failed. It is used when a data
// migration does not set BackoffLimit.
const DefaultBackoffLimit = 3

// DefaultBackoffDelay is the base pause before the first retry of a failed
// batch. It doubles on each subsequent attempt, capped at maxBackoffDelay. It
// is used when a data migration does not set BackoffDelay.
const DefaultBackoffDelay = 1 * time.Second

// maxBackoffDelay caps the exponential retry pause so a large BackoffLimit
// cannot produce an unbounded (or overflowing) delay.
const maxBackoffDelay = 5 * time.Minute

var (
	// ErrLeaseHeld is returned when another live process holds the lease for a
	// data migration. Callers running a single driver (e.g. a Kubernetes Job
	// with parallelism 1) may treat this as a benign "someone else is on it".
	ErrLeaseHeld = errors.New("data migration lease is held by another process")

	// ErrLeaseLost is returned when the lease was taken over by another process
	// mid-run (the holder stalled past the TTL). The in-flight batch is rolled
	// back rather than committed.
	ErrLeaseLost = errors.New("data migration lease lost to another process")

	// ErrDataMigrationUnsupported is returned when the active dialect cannot honor
	// the conditional-update lease that data migrations rely on (e.g. an OLAP
	// backend such as ClickHouse, whose UPDATE is an asynchronous mutation with no
	// synchronous affected-row count). Schema migrations still work on such
	// dialects; only the data-migration runner is unavailable.
	ErrDataMigrationUnsupported = errors.New("data migrations are not supported on this dialect")
)

// leaseOwner identifies this process when claiming leases. It is computed once
// and is unique per process (and per pod, since the hostname is the pod name in
// Kubernetes).
var (
	leaseOwnerOnce  sync.Once
	leaseOwnerValue string
)

func leaseOwner() string {
	leaseOwnerOnce.Do(func() {
		host, err := os.Hostname()
		if err != nil || host == "" {
			host = "unknown"
		}

		var nonce [4]byte
		if _, err := rand.Read(nonce[:]); err != nil {
			leaseOwnerValue = fmt.Sprintf("%s:%d", host, os.Getpid())
			return
		}

		leaseOwnerValue = fmt.Sprintf("%s:%d:%s", host, os.Getpid(), hex.EncodeToString(nonce[:]))
	})

	return leaseOwnerValue
}

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
	// interrupted batch can safely be retried on resume (the runner also retries
	// a failed batch up to BackoffLimit times before giving up).
	//
	// cp is never empty unless Plan returned an empty checkpoint: a run that
	// finds an empty stored checkpoint re-invokes Plan first, so json.Unmarshal
	// on cp will not fail on an empty payload as long as Plan returns valid JSON.
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

	// After is the schema migration version that must be applied before this
	// data migration becomes eligible to run. Zero means no dependency. The
	// schema version is looked up in AfterPackage (see afterPackage).
	After int64

	// AfterPackage is the package of the schema migration named by After. When
	// empty, the dependency is resolved against this data migration's own
	// Package (see afterPackage). Set it to depend on a schema version in a
	// different package.
	AfterPackage string

	// Throttle is the pause inserted between batches to limit load and
	// replication lag. Zero means no pause.
	Throttle time.Duration

	// LeaseTTL is how long an acquired lease stays valid before another process
	// may steal it. It is renewed on every batch commit. Zero means
	// DefaultLeaseTTL. It must exceed a single batch's duration plus Throttle.
	LeaseTTL time.Duration

	// BackoffLimit is the number of times a failed batch is retried (with an
	// exponential backoff pause) within a single run before the migration gives
	// up and is marked failed. Zero means DefaultBackoffLimit; a negative value
	// disables retries (the batch fails on its first error).
	BackoffLimit int

	// BackoffDelay is the base pause before the first retry; it doubles on each
	// subsequent attempt, capped internally. Zero means DefaultBackoffDelay.
	BackoffDelay time.Duration
}

// backoffLimit returns the configured retry count, applying DefaultBackoffLimit
// for the zero value and clamping a negative value to zero (no retries).
func (dm *DataMigration) backoffLimit() int {
	if dm.BackoffLimit == 0 {
		return DefaultBackoffLimit
	}

	if dm.BackoffLimit < 0 {
		return 0
	}

	return dm.BackoffLimit
}

// backoffDelay returns the pause before the given 1-based retry attempt: the
// base delay doubled (attempt-1) times, capped at maxBackoffDelay.
func (dm *DataMigration) backoffDelay(attempt int) time.Duration {
	d := dm.BackoffDelay
	if d <= 0 {
		d = DefaultBackoffDelay
	}

	for i := 1; i < attempt; i++ {
		d *= 2
		if d >= maxBackoffDelay {
			return maxBackoffDelay
		}
	}

	return d
}

// afterPackage returns the package that the After schema version is looked up
// in: the explicitly configured AfterPackage when set, otherwise the data
// migration's own Package (which defaults to DefaultPackageName). This lets
// After(version) target the data migration's package without restating it,
// while After(version, pkg) targets a different package.
func (dm *DataMigration) afterPackage() string {
	if dm.AfterPackage != "" {
		return dm.AfterPackage
	}

	return dm.Package
}

func (dm *DataMigration) leaseTTL() time.Duration {
	if dm.LeaseTTL > 0 {
		return dm.LeaseTTL
	}

	return DefaultLeaseTTL
}

// logEntry returns a logrus entry pre-populated with the data-migrator
// component tag and this migration's identity, so every phase/progress line
// carries consistent, filterable structured fields.
func (dm *DataMigration) logEntry() *log.Entry {
	fields := log.Fields{
		"component": dataMigratorComponent,
		"package":   dm.Package,
		"version":   dm.Version,
	}

	if dm.Name != "" {
		fields["name"] = dm.Name
	}

	return log.WithFields(fields)
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
// migration version has been applied. Without packageName, the version is
// looked up in the data migration's own package (which defaults to
// DefaultPackageName); pass packageName to depend on a schema version in a
// different package. Only the first packageName is used.
func After(schemaVersion int64, packageName ...string) DataMigrationOption {
	return func(dm *DataMigration) {
		dm.After = schemaVersion
		if len(packageName) > 0 {
			dm.AfterPackage = packageName[0]
		}
	}
}

// WithThrottle sets the pause inserted between batches.
func WithThrottle(d time.Duration) DataMigrationOption {
	return func(dm *DataMigration) {
		dm.Throttle = d
	}
}

// WithDataMigrationName sets a human-readable description and, optionally, the
// data migration's package. Passing the package here is a convenience for the
// common case of aligning the data migration with the SQL/schema package it
// sits beside (e.g. "main"). Only the first packageName is used.
func WithDataMigrationName(name string, packageName ...string) DataMigrationOption {
	return func(dm *DataMigration) {
		dm.Name = name
		if len(packageName) > 0 {
			dm.Package = packageName[0]
		}
	}
}

// WithLeaseTTL sets how long an acquired lease stays valid before another
// process may steal it. It must exceed a single batch's duration plus the
// throttle pause.
func WithLeaseTTL(d time.Duration) DataMigrationOption {
	return func(dm *DataMigration) {
		dm.LeaseTTL = d
	}
}

// WithBackoffLimit sets how many times a failed batch is retried within a single
// run before the migration gives up. A negative value disables retries.
func WithBackoffLimit(n int) DataMigrationOption {
	return func(dm *DataMigration) {
		dm.BackoffLimit = n
	}
}

// WithBackoffDelay sets the base pause before the first retry of a failed batch;
// it doubles on each subsequent attempt, capped internally.
func WithBackoffDelay(d time.Duration) DataMigrationOption {
	return func(dm *DataMigration) {
		dm.BackoffDelay = d
	}
}

// registeredDataMigrations stores the globally registered data migrations,
// keyed the same way as registeredGoMigrations.
var registeredDataMigrations = map[RegistryKey]*DataMigration{}

// AddDataMigration registers a data migration into the global map. The version
// is parsed from the caller's source filename and the package defaults to
// DefaultPackageName, matching the default package of SQL migrations so a Go
// data migration sits in the same namespace as the scripts it accompanies. Set
// a different package with WithDataMigrationName(name, packageName) or use
// AddNamedDataMigration.
func AddDataMigration(m DataMigrator, opts ...DataMigrationOption) {
	_, filename, _, _ := runtime.Caller(1)
	AddNamedDataMigration(DefaultPackageName, filename, m, opts...)
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
