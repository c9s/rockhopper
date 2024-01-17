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
	Name string
	// Package is the migration script package name
	Package string

	// Version is the migration version
	Version int64

	// Source is the path to the .sql script
	Source string

	UseTx bool

	Chunk *MigrationScriptChunk

	// Next is the next migration to apply (newer migration)
	Next *Migration

	// Previous is the previous migration (older migration)
	Previous *Migration

	Record *MigrationRecord

	Registered bool

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
		return executeStatements(ctx, exec, m.UpStatements)
	})
	finalizer := func(ctx context.Context, exec SQLExecutor) error {
		return db.insertVersion(ctx, db.DB, m.Package, m.Version, true)
	}

	var executor = m.getStmtExecutor()
	return executor(ctx, db.DB, fn, finalizer)
}

func (m *Migration) runDown(ctx context.Context, db *DB) error {
	fn := withDefault[TransactionHandler](m.DownFn, func(ctx context.Context, exec SQLExecutor) error {
		return executeStatements(ctx, exec, m.DownStatements)
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

// cleanSQL removes the SQL comments
func cleanSQL(s string) string {
	s = matchSQLComments.ReplaceAllString(s, "")
	return strings.TrimSpace(matchEmptyEOL.ReplaceAllString(s, ""))
}

// executeStatements executes the given statements sequentially
func executeStatements(ctx context.Context, e SQLExecutor, stmts []Statement) error {
	for idx, stmt := range stmts {
		log.Debug(cleanSQL(stmt.SQL))

		p := startProfile(fmt.Sprintf("%d", idx))
		if _, err := e.ExecContext(ctx, stmt.SQL); err != nil {
			return errors.Wrapf(err, "failed to execute SQL query %q, error %s", cleanSQL(stmt.SQL), err.Error())
		}
		p.Stop()

		log.Debugf("duration: %s", p.String())

		// update duration into the statement object
		stmts[idx].Duration = p.duration
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
