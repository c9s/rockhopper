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

type Migration struct {
	Name    string
	Source  string // path to .sql script
	Version int64
	UseTx   bool

	Next     *Migration
	Previous *Migration

	Registered bool
	Package    string

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

func (m *Migration) run(ctx context.Context, db *DB, direction Direction) error {
	var err error
	var tx *sql.Tx = nil
	var rollback = func() {}
	var executor SQLExecutor = db

	if m.UseTx {
		log.Infof("migration transaction is enabled, starting transaction...")

		tx, err = db.BeginTx(ctx, nil)
		if err != nil {
			return errors.Wrap(err, "failed to begin transaction")
		}

		executor = tx
		rollback = func() {
			log.Infof("rolling back transaction")
			if err := tx.Rollback(); err != nil {
				log.WithError(err).Error("rollback error")
			}
		}
	} else {
		log.Debugf("transaction is disabled in migration version: %d", m.Version)
	}

	switch direction {

	case DirectionUp:
		if m.UpFn != nil {
			if err := m.UpFn(ctx, executor); err != nil {
				rollback()
				return err
			}
		} else {
			if err := runStatements(ctx, executor, m.UpStatements); err != nil {
				rollback()
				return err
			}
		}

		if err := db.insertVersion(ctx, executor, m.Version); err != nil {
			rollback()
			return errors.Wrap(err, "failed to insert new goose version")
		}

	case DirectionDown:
		if m.DownFn != nil {
			if err := m.DownFn(ctx, executor); err != nil {
				rollback()
				return err
			}
		} else {
			if err := runStatements(ctx, executor, m.DownStatements); err != nil {
				rollback()
				return err
			}
		}

		if err := db.deleteVersion(ctx, executor, m.Version); err != nil {
			rollback()
			return errors.Wrap(err, "failed to delete version")
		}
	}

	if m.UseTx && tx != nil {
		log.Info("committing transaction...")
		if err := tx.Commit(); err != nil {
			return errors.Wrap(err, "failed to commit transaction")
		}
	}

	return nil
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

// helpers so we can use pkg sort
func (ms MigrationSlice) Len() int      { return len(ms) }
func (ms MigrationSlice) Swap(i, j int) { ms[i], ms[j] = ms[j], ms[i] }
func (ms MigrationSlice) Less(i, j int) bool {
	if ms[i].Version == ms[j].Version {
		panic(fmt.Sprintf("duplicate migration version %v detected:\n%v\n%v", ms[i].Version, ms[i].Source, ms[j].Source))
	}

	return ms[i].Version < ms[j].Version
}

// Find finds the migration by version
func (ms MigrationSlice) Find(version int64) (*Migration, error) {
	for i, migration := range ms {
		if migration.Version == version {
			return ms[i], nil
		}
	}

	return nil, ErrVersionNotFound
}

func (ms MigrationSlice) SortAndConnect() MigrationSlice {
	sort.Sort(ms)

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
