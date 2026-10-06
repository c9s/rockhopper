// Package column names the SQL columns of rockhopper's own bookkeeping tables
// (the version table and the data-migration table). It is shared by the root
// package, which defines those tables, and pkg/dialect, which builds SQL
// against them.
package column

// Columns shared by the version table and the data-migration table.
const (
	RecordID  = "id"
	Package   = "package"
	VersionID = "version_id"
)

// Version table columns.
const (
	SourceFile = "source_file"
	IsApplied  = "is_applied"
	Timestamp  = "tstamp"
)

// Data-migration table columns.
const (
	Name           = "name"
	Status         = "status"
	Checkpoint     = "checkpoint"
	LeaseOwner     = "lease_owner"
	LeaseExpiresAt = "lease_expires_at"
	CreatedAt      = "created_at"
	UpdatedAt      = "updated_at"
)
