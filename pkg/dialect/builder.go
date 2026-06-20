package dialect

import (
	"fmt"
	"strings"
)

// Tokens is the minimal set of dialect-specific lexical choices the CRUD builder
// needs to render portable single-table statements. Each dialect implements it;
// it is wired into the embedded CRUD so the builder can render placeholders and
// the "current timestamp" expression correctly per database.
type Tokens interface {
	// Placeholder returns the bind marker for the n-th (1-based) argument,
	// e.g. "?" for MySQL/SQLite or "$1" for PostgreSQL.
	Placeholder(n int) string

	// NowExpr returns the expression for the current timestamp as used inside
	// DML (e.g. "NOW()", "sysdate", "datetime('now')").
	NowExpr() string
}

// Col is a column name paired with a bind value.
type Col struct {
	Name string
	Val  any
}

// Order is a single ORDER BY term.
type Order struct {
	Col  string
	Desc bool
}

// SelectOpt carries the optional clauses of a Select.
type SelectOpt struct {
	OrderBy []Order
	Limit   int // 0 means no LIMIT
}

// UpdateOpt carries the optional clauses of an Update.
type UpdateOpt struct {
	NowCols  []string // columns assigned the dialect's NowExpr()
	NullCols []string // columns assigned NULL
	Lock     *Col     // optional guard appended to WHERE as "<Name> = <placeholder>"
}

// Builder is the set of simple single-table statement shapes rockhopper needs.
// The table name and columns are arguments, so one method serves every table,
// and each method returns the SQL together with its bind arguments in the same
// order the placeholders were emitted — there is no separate "argument order" to
// keep in sync by hand.
type Builder interface {
	Insert(table string, cols []Col) (string, []any)
	Delete(table string, keys []Col) (string, []any)
	Select(table string, cols []string, keys []Col, opt SelectOpt) (string, []any)
	Update(table string, set, keys []Col, opt UpdateOpt) (string, []any)

	// AcquireLease conditionally claims the data-migration lease (when unowned,
	// already owned by owner, or expired).
	AcquireLease(table string, keys []Col, owner string, expiresAt, now int64) (string, []any)
	// CommitLease persists a batch's columns while renewing the lease, guarded by
	// ownership.
	CommitLease(table string, set, keys []Col, owner string) (string, []any)
	// ReleaseLease sets a terminal status and clears the lease, guarded by
	// ownership.
	ReleaseLease(table, status string, keys []Col, owner string) (string, []any)
}

// CRUD renders the Builder shapes from a dialect's Tokens. It is embedded in each
// dialect so callers can write d.Insert(...), d.Update(...), etc.
type CRUD struct {
	t Tokens
}

// NewCRUD binds a CRUD builder to a dialect's tokens.
func NewCRUD(t Tokens) CRUD { return CRUD{t: t} }

func (c CRUD) Insert(table string, cols []Col) (string, []any) {
	names := make([]string, len(cols))
	marks := make([]string, len(cols))
	args := make([]any, len(cols))
	for i, col := range cols {
		names[i] = col.Name
		marks[i] = c.t.Placeholder(i + 1)
		args[i] = col.Val
	}

	q := fmt.Sprintf("INSERT INTO %s (%s) VALUES (%s)",
		table, strings.Join(names, ", "), strings.Join(marks, ", "))
	return q, args
}

func (c CRUD) Delete(table string, keys []Col) (string, []any) {
	where, args := c.eqClauses(keys, 0, " AND ")
	return fmt.Sprintf("DELETE FROM %s WHERE %s", table, where), args
}

func (c CRUD) Select(table string, cols []string, keys []Col, opt SelectOpt) (string, []any) {
	var b strings.Builder
	fmt.Fprintf(&b, "SELECT %s FROM %s", strings.Join(cols, ", "), table)

	var args []any
	if len(keys) > 0 {
		where, wargs := c.eqClauses(keys, 0, " AND ")
		fmt.Fprintf(&b, " WHERE %s", where)
		args = wargs
	}

	if len(opt.OrderBy) > 0 {
		terms := make([]string, len(opt.OrderBy))
		for i, o := range opt.OrderBy {
			if o.Desc {
				terms[i] = o.Col + " DESC"
			} else {
				terms[i] = o.Col
			}
		}
		fmt.Fprintf(&b, " ORDER BY %s", strings.Join(terms, ", "))
	}

	if opt.Limit > 0 {
		fmt.Fprintf(&b, " LIMIT %d", opt.Limit)
	}

	return b.String(), args
}

func (c CRUD) Update(table string, set, keys []Col, opt UpdateOpt) (string, []any) {
	var assigns []string
	var args []any
	n := 0

	for _, s := range set {
		n++
		assigns = append(assigns, fmt.Sprintf("%s = %s", s.Name, c.t.Placeholder(n)))
		args = append(args, s.Val)
	}
	for _, col := range opt.NowCols {
		assigns = append(assigns, fmt.Sprintf("%s = %s", col, c.t.NowExpr()))
	}
	for _, col := range opt.NullCols {
		assigns = append(assigns, col+" = NULL")
	}

	var conds []string
	for _, k := range keys {
		n++
		conds = append(conds, fmt.Sprintf("%s = %s", k.Name, c.t.Placeholder(n)))
		args = append(args, k.Val)
	}
	if opt.Lock != nil {
		n++
		conds = append(conds, fmt.Sprintf("%s = %s", opt.Lock.Name, c.t.Placeholder(n)))
		args = append(args, opt.Lock.Val)
	}

	q := fmt.Sprintf("UPDATE %s SET %s WHERE %s",
		table, strings.Join(assigns, ", "), strings.Join(conds, " AND "))
	return q, args
}

func (c CRUD) AcquireLease(table string, keys []Col, owner string, expiresAt, now int64) (string, []any) {
	n := 0
	next := func() string { n++; return c.t.Placeholder(n) }

	var args []any
	set := fmt.Sprintf("lease_owner = %s, lease_expires_at = %s, updated_at = %s",
		next(), next(), c.t.NowExpr())
	args = append(args, owner, expiresAt)

	conds := make([]string, len(keys))
	for i, k := range keys {
		conds[i] = fmt.Sprintf("%s = %s", k.Name, next())
		args = append(args, k.Val)
	}

	guard := fmt.Sprintf("(lease_owner IS NULL OR lease_owner = %s OR lease_expires_at < %s)",
		next(), next())
	args = append(args, owner, now)

	q := fmt.Sprintf("UPDATE %s SET %s WHERE %s AND %s",
		table, set, strings.Join(conds, " AND "), guard)
	return q, args
}

func (c CRUD) CommitLease(table string, set, keys []Col, owner string) (string, []any) {
	return c.Update(table, set, keys, UpdateOpt{
		NowCols: []string{"updated_at"},
		Lock:    &Col{Name: "lease_owner", Val: owner},
	})
}

func (c CRUD) ReleaseLease(table, status string, keys []Col, owner string) (string, []any) {
	return c.Update(table,
		[]Col{{Name: "status", Val: status}, {Name: "lease_expires_at", Val: int64(0)}},
		keys,
		UpdateOpt{
			NowCols:  []string{"updated_at"},
			NullCols: []string{"lease_owner"},
			Lock:     &Col{Name: "lease_owner", Val: owner},
		})
}

// eqClauses renders "col = <placeholder>" for each column joined by sep, numbering
// placeholders starting at start+1, and returns the matching args.
func (c CRUD) eqClauses(cols []Col, start int, sep string) (string, []any) {
	conds := make([]string, len(cols))
	args := make([]any, len(cols))
	for i, col := range cols {
		conds[i] = fmt.Sprintf("%s = %s", col.Name, c.t.Placeholder(start+i+1))
		args[i] = col.Val
	}
	return strings.Join(conds, sep), args
}
