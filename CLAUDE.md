# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

Rockhopper is an embeddable Go database migration tool (forked from Goose). It supports SQL and Go-based migrations with package-based organization, multi-database support (MySQL, PostgreSQL, SQLite3, MSSQL, TiDB, Redshift), and the ability to compile/embed SQL migrations into Go binaries.

Module path: `github.com/c9s/rockhopper/v2` (Go 1.21+)

## Common Commands

```bash
# Run all tests (requires SQLite3 CGO support)
go test ./...

# Run a single test
go test -run TestFunctionName -v ./...

# Run the CLI tool
go run ./cmd/rockhopper

# Lint (CI uses revive)
revive ./...

# Build
go build ./cmd/rockhopper
```

## Architecture

### Core Library (root package `rockhopper`)

- **Migration model** (`migration.go`): `Migration` struct forms a doubly-linked list. Each node has Version (int64 timestamp), Package, UpFn/DownFn callbacks, and Next/Previous pointers. `MigrationSlice` provides Sort, Connect, FilterPackage, MapByPackage.

- **DB layer** (`db.go`): `DB` wraps `sql.DB` with dialect-specific SQL. Tracks applied versions in `rockhopper_versions` table (with legacy `goose_db_version` auto-migration). Key methods: CurrentVersion, LoadMigration, FindLastAppliedMigration.

- **SQL parser** (`parser.go`): State-machine parser for migration files. Annotations: `-- +up`, `-- +down`, `-- +begin`/`-- +end` (for multi-statement blocks), `-- !txn` (disable transactions), `-- @package <name>`.

- **Loaders** (`loader.go`): `SqlMigrationLoader` reads SQL files from directories; `GoMigrationLoader` reads from the global registry. Filename pattern: `{timestamp}_{description}.sql`.

- **Dialects** (`dialect*.go`): `SQLDialect` interface with implementations for each supported database. Factory: `LoadDialect()`.

- **Code generation** (`dumper.go`): `GoMigrationDumper` converts SQL migrations into Go source files for embedding. Generates `migration_api.go` with a `Migrations()` function.

- **Registry** (`registry.go`): Global `registeredGoMigrations` map keyed by `{Package, Version}`. Used for compiled/embedded Go migrations via `AddMigration()`/`AddNamedMigration()`.

- **Migration execution** (`up.go`, `down.go`, `redo.go`): `Up()`, `UpBySteps()`, `Upgrade()`, `Down()` functions that traverse the linked list and apply/rollback migrations.

### CLI (`cmd/rockhopper/`)

Cobra-based CLI with commands: `up`, `down`, `redo`, `status`, `create`, `compile`. Configuration via `rockhopper.yaml` (YAML + env var overrides through Viper).

### Testing

Tests use testify/assert and SQLite3 in-memory databases for integration testing. Test migrations live in `pkg/testing/migrations/`.

## Typical Usage Workflow (bbgo example)

Rockhopper manages per-dialect migration files (e.g. separate sqlite and mysql directories). The typical workflow:

1. **Create migration files** for each dialect:
   ```bash
   rockhopper --config rockhopper_sqlite.yaml create --type sql add_pnl_column
   rockhopper --config rockhopper_mysql.yaml create --type sql add_pnl_column
   ```
   Or use a helper script (`examples/generate-new-migration.sh`) that creates both at once.

2. **Edit both migration files** — each dialect may need different SQL syntax.

3. **Apply migrations** to test:
   ```bash
   rockhopper --config rockhopper_sqlite.yaml up
   rockhopper --config rockhopper_mysql.yaml up
   ```

4. **Compile SQL migrations into Go** for embedding in the binary:
   ```bash
   rockhopper compile --config rockhopper_mysql.yaml --output pkg/migrations/mysql
   rockhopper compile --config rockhopper_sqlite.yaml --output pkg/migrations/sqlite3
   ```

5. **Override DSN/driver via env vars** (useful for local dev):
   ```bash
   ROCKHOPPER_DRIVER=mysql ROCKHOPPER_DIALECT=mysql ROCKHOPPER_DSN="root:pass@tcp(127.0.0.1)/dbname" \
     rockhopper --config rockhopper_mysql.yaml up
   ```

## SQL Migration File Format

```sql
-- @package mypackage
-- +up
CREATE TABLE foo (id INT PRIMARY KEY);

-- +down
DROP TABLE foo;
```

For multi-statement blocks, wrap with `-- +begin` / `-- +end`. Use `-- !txn` to disable transaction wrapping.
