package rockhopper

import "fmt"

type Migration struct {
	Version  int64
	Next     *Migration
	Previous *Migration

	Source     string // path to .sql script
	Registered bool
	UpFn       TransactionHandler // Up go migration function
	DownFn     TransactionHandler // Down go migration function

	UpStatements   []Statement
	DownStatements []Statement
}

func (m *Migration) String() string {
	return fmt.Sprintf(m.Source)
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

	return nil, ErrNoCurrentVersion
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
