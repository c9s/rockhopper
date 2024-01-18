package rockhopper

import (
	"context"
	"database/sql"
	"fmt"
	"reflect"
	"sort"

	"github.com/jedib0t/go-pretty/v6/text"
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
	return m.runUp(ctx, db)
}

// Down runs a down migration.
func (m *Migration) Down(ctx context.Context, db *DB) error {
	return m.runDown(ctx, db)
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

type statementExecution func(ctx context.Context, e SQLExecutor, stmt *Statement) error

func withStatementDebug(next statementExecution) statementExecution {
	return func(ctx context.Context, e SQLExecutor, stmt *Statement) error {
		log.Debug(cleanSQL(stmt.SQL))
		return next(ctx, e, stmt)
	}
}

func withStatementPrettyLog(next statementExecution) statementExecution {
	return func(ctx context.Context, e SQLExecutor, stmt *Statement) error {
		fmt.Print(text.Colors{text.FgGreen}.Sprint("EXECUTING: "))
		fmt.Print(text.Colors{text.FgHiWhite}.Sprint(previewSQL(stmt.SQL)), " ")

		err := next(ctx, e, stmt)

		fmt.Printf("[  %6s  ]", text.Colors{text.FgHiGreen}.Sprintf("OK"))
		fmt.Printf(" ---- %s", text.Colors{text.FgWhite, text.BgBlack}.Sprintf(stmt.Duration.String()))
		fmt.Print("\n")
		return err
	}
}

func withStatementProfile(next statementExecution) statementExecution {
	return func(ctx context.Context, e SQLExecutor, stmt *Statement) error {
		p := startProfile(fmt.Sprintf("stmt: %x", stmt))
		err := next(ctx, e, stmt)
		p.Stop()
		log.Debugf("query done, duration: %s", p.String())
		stmt.Duration = p.duration
		return err
	}
}

func executeStatement(ctx context.Context, e SQLExecutor, stmt *Statement) error {
	var fn statementExecution = func(ctx context.Context, e SQLExecutor, stmt *Statement) error {
		if _, err := e.ExecContext(ctx, stmt.SQL); err != nil {
			return errors.Wrapf(err, "failed to execute SQL query %q, error %s", cleanSQL(stmt.SQL), err.Error())
		}

		return nil
	}

	fn = withStatementProfile(fn)
	if log.GetLevel() == log.DebugLevel {
		fn = withStatementDebug(fn)
	} else {
		fn = withStatementPrettyLog(fn)
	}

	return fn(ctx, e, stmt)
}

// executeStatements executes the given statements sequentially
func executeStatements(ctx context.Context, e SQLExecutor, stmts []Statement) error {
	for _, stmt := range stmts {
		if err := executeStatement(ctx, e, &stmt); err != nil {
			return err
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

func (ms MigrationSlice) FilterPackage(pkgs []string) (slice MigrationSlice) {
	for _, s := range ms {
		if sliceContains(pkgs, s.Package) {
			slice = append(slice, s)
		}
	}

	return slice
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
	for i := 0; i < len(ms); i++ {
		m := ms[i]

		if i < len(ms)-1 {
			m.Next = ms[i+1]
		}
		if i > 0 {
			m.Previous = ms[i-1]
		}

		ms[i] = m
	}

	return ms
}

func (ms MigrationSlice) SortAndConnect() MigrationSlice {
	return ms.Sort().Connect()
}

func withDefault[T any](a, def T) T {
	v := reflect.ValueOf(a)

	if !v.IsNil() {
		return a
	}

	return def
}
