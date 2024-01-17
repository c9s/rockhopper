package rockhopper

import (
	"context"
	"database/sql"
	"fmt"
	"regexp"
	"sort"
	"strings"

	"github.com/pkg/errors"
	log "github.com/sirupsen/logrus"
)

const DefaultPackageName = "main"
const CorePackageName = "rockhopper"

// Migration presents the migration script object as a linked-list node.
// It could link to the next migration object and the previous migration object
type Migration struct {
	Name    string
	Source  string // path to .sql script
	Version int64
	UseTx   bool

	Chunk *MigrationScriptChunk

	// Next is the next migration to apply (newer migration)
	Next *Migration

	// Previous is the previous migration (older migration)
	Previous *Migration

	Registered bool

	// Package is the migration script package name
	Package string

	UpFn   TransactionHandler // Up go migration function
	DownFn TransactionHandler // Down go migration function

	UpStatements   []Statement
	DownStatements []Statement
}

func (m *Migration) String() string {
	return fmt.Sprintf(m.Source)
}

// Up runs an up migration.
func (m *Migration) Up(ctx context.Context, db *DB) error {
	return m.run(ctx, db, DirectionUp)
}

// Down runs a down migration.
func (m *Migration) Down(ctx context.Context, db *DB) error {
	return m.run(ctx, db, DirectionDown)
}

type statementExecutorFunc func(ctx context.Context, db *sql.DB, callbacks ...TransactionHandler) error

func withoutTransaction(ctx context.Context, db *sql.DB, callbacks ...TransactionHandler) error {
	for _, cb := range callbacks {
		if err2 := cb(ctx, db); err2 != nil {
			return err2
		}
	}

	return nil
}

func withTransaction(ctx context.Context, db *sql.DB, callbacks ...TransactionHandler) error {
	tx, err := db.Begin()
	if err != nil {
		return err
	}

	for _, cb := range callbacks {
		if err2 := cb(ctx, tx); err2 != nil {
			return rollbackAndLogErr(err, tx, "")
		}
	}

	return tx.Commit()
}

func (m *Migration) getStmtExecutor() statementExecutorFunc {
	if m.UseTx {
		return withTransaction
	}
	return withoutTransaction
}

func (m *Migration) runUp(ctx context.Context, db *DB) error {
	fn := withDefault[TransactionHandler](m.UpFn, func(ctx context.Context, exec SQLExecutor) error {
		return runStatements(ctx, exec, m.UpStatements)
	})
	finalizer := func(ctx context.Context, exec SQLExecutor) error {
		return db.insertVersion(ctx, db.DB, m.Package, m.Version, true)
	}

	var executor = m.getStmtExecutor()
	return executor(ctx, db.DB, fn, finalizer)
}

func (m *Migration) runDown(ctx context.Context, db *DB) error {
	fn := withDefault[TransactionHandler](m.DownFn, func(ctx context.Context, exec SQLExecutor) error {
		return runStatements(ctx, exec, m.DownStatements)
	})
	finalizer := func(ctx context.Context, exec SQLExecutor) error {
		return db.deleteVersion(ctx, db.DB, m.Package, m.Version)
	}

	var executor = m.getStmtExecutor()
	return executor(ctx, db.DB, fn, finalizer)
}

func (m *Migration) run(ctx context.Context, db *DB, direction Direction) error {
	switch direction {

	case DirectionUp:
		return m.runUp(ctx, db)

	case DirectionDown:
		return m.runDown(ctx, db)

	default:
		return fmt.Errorf("unexpected direction: %v", direction)
	}
}

var (
	matchSQLComments = regexp.MustCompile(`(?m)^--.*$[\r\n]*`)
	matchEmptyEOL    = regexp.MustCompile(`(?m)^$[\r\n]*`) // TODO: Duplicate
)

func cleanSQL(s string) string {
	s = matchSQLComments.ReplaceAllString(s, "")
	return strings.TrimSpace(matchEmptyEOL.ReplaceAllString(s, ""))
}

func runStatements(ctx context.Context, e SQLExecutor, stmts []Statement) error {
	for _, stmt := range stmts {
		log.Infof("executing statement: %s", cleanSQL(stmt.SQL))
		if _, err := e.ExecContext(ctx, stmt.SQL); err != nil {
			return errors.Wrapf(err, "failed to execute SQL query %q, error %s", cleanSQL(stmt.SQL), err.Error())
		}
	}

	return nil
}

type MigrationSlice []*Migration

func (ms MigrationSlice) Head() *Migration {
	if len(ms) == 0 {
		return nil
	}

	return ms[0]
}

func (ms MigrationSlice) Tail() *Migration {
	if len(ms) == 0 {
		return nil
	}

	return ms[len(ms)-1]
}

func (ms MigrationSlice) MapByPackage() MigrationMap {
	mm := make(MigrationMap)

	for _, m := range ms {
		if len(m.Package) == 0 {
			log.Warnf("unexpected error: found empty package name in migration script: %+v", m)
		}

		if slice, ok := mm[m.Package]; ok {
			mm[m.Package] = append(slice, m)
		} else {
			mm[m.Package] = MigrationSlice{m}
		}
	}

	return mm
}

func (ms MigrationSlice) Len() int      { return len(ms) }
func (ms MigrationSlice) Swap(i, j int) { ms[i], ms[j] = ms[j], ms[i] }
func (ms MigrationSlice) Less(i, j int) bool {
	if ms[i].Version == ms[j].Version {
		panic(fmt.Sprintf("duplicate migration version %v detected:\n%v\n%v", ms[i].Version, ms[i].Source, ms[j].Source))
	}

	return ms[i].Version < ms[j].Version
}

func (ms MigrationSlice) Versions() (versions []int64) {
	for _, migration := range ms {
		versions = append(versions, migration.Version)
	}
	return versions
}

// Find finds the migration by version
func (ms MigrationSlice) Find(version int64) (*Migration, error) {
	for i, migration := range ms {
		if migration.Version == version {
			return ms[i], nil
		}
	}

	return nil, fmt.Errorf("migration source version %d not found, available versions: %v", version, ms.Versions())
}

func (ms MigrationSlice) Sort() MigrationSlice {
	sort.Sort(ms)
	return ms
}

func (ms MigrationSlice) Connect() MigrationSlice {
	// now that we're sorted in the appropriate direction,
	// populate next and previous for each migration
	for i, m := range ms {
		var prev *Migration = nil
		if i > 0 {
			prev = ms[i-1]
			ms[i-1].Next = m
		}

		ms[i].Previous = prev
	}

	return ms
}

func (ms MigrationSlice) SortAndConnect() MigrationSlice {
	return ms.Sort().Connect()
}

func withDefault[T any](txHandler, defaultTxHandler T) T {
	if txHandler != nil {
		return txHandler
	}

	return defaultTxHandler
}
