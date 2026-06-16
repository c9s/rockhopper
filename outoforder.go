package rockhopper

import (
	"fmt"
	"strings"
)

// OutOfOrderError is returned when an upgrade finds pending migrations whose
// version is lower than the highest already-applied migration. Applying them
// would change history out of order, so rockhopper refuses by default and asks
// the operator to opt in explicitly.
type OutOfOrderError struct {
	// Package is the migration package the out-of-order migrations belong to.
	Package string

	// HighestAppliedVersion is the highest version that has already been applied.
	HighestAppliedVersion int64

	// Migrations holds the offending pending migrations, in ascending version order.
	Migrations MigrationSlice
}

func (e *OutOfOrderError) Error() string {
	var b strings.Builder

	fmt.Fprintf(&b,
		"out-of-order migrations detected in package %q: the following are pending but have a lower version than the highest applied migration (%d), so a normal upgrade would silently skip them:\n",
		e.Package, e.HighestAppliedVersion)

	for _, m := range e.Migrations {
		fmt.Fprintf(&b, "  - %d  %s\n", m.Version, m.Source)
	}

	b.WriteString("re-run with --allow-out-of-order to apply them anyway, or renumber them above the latest applied version")

	return b.String()
}
