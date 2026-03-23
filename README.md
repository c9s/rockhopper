rockhopper
======================

[![Go](https://github.com/c9s/rockhopper/actions/workflows/go.yml/badge.svg?branch=main)](https://github.com/c9s/rockhopper/actions/workflows/go.yml)
[![Go Reference](https://pkg.go.dev/badge/github.com/c9s/rockhopper/v2.svg)](https://pkg.go.dev/github.com/c9s/rockhopper/v2)
[![Go Report Card](https://goreportcard.com/badge/github.com/c9s/rockhopper/v2)](https://goreportcard.com/report/github.com/c9s/rockhopper/v2)
[![Claude Code](https://img.shields.io/badge/Claude_Code-D97757?logo=claude&logoColor=fff)](https://claude.ai/code)

rockhopper is an embeddable migration tool written in Go, which can embed your migration files into a package with an
easy-to-use API.

REF: a small penguin with a yellowish crest, breeding on subantarctic coastal cliffs which it ascends by hopping from rock to rock.

![Console Demo](https://raw.githubusercontent.com/c9s/rockhopper/main/screenshots/screenshot1.png)

## Features

- Embeddable migration script — compile SQL files into Go source and ship them in a single binary
- Package-based migration organization — group and execute migrations by package name
- Multi-dialect support: MySQL, SQLite3, PostgreSQL
- Compatible with [Goose](https://github.com/pressly/goose) migration format
- Built-in [Claude Code](https://claude.ai/code) skills for AI-assisted migration management

## Install

```sh
go install github.com/c9s/rockhopper/v2/cmd/rockhopper@v2.0.7
```

## Quick Start

Create a config file `rockhopper.yaml`:

```yaml
---
driver: mysql
dialect: mysql
dsn: "root@tcp(localhost:3306)/rockhopper?parseTime=true"
package: myapp
migrationsDirs:
- migrations/module1
- migrations/module2
```

Create directories for your migration files:

```sh
mkdir -p migrations/{module1,module2}
```

Create a new migration:

```sh
rockhopper create -t sql --output migrations/module1 add_trades_table
```

Edit the generated migration file (see [SQL Migration Format](#sql-migration-format) below), then apply:

```sh
rockhopper up
```

Check migration status:

```sh
rockhopper status
```

```
+---------+----------------+---------------------------------------------------------+--------------------------+---------+
| PACKAGE |     VERSION ID | SOURCE FILE                                             | APPLIED AT               | CURRENT |
+---------+----------------+---------------------------------------------------------+--------------------------+---------+
| app1    | 20240116231445 | migrations/mysql/app1/20240116231445_create_table_1.sql | Fri Jan 19 15:34:51 2024 | -       |
| app1    | 20240116231513 | migrations/mysql/app1/20240116231513_create_table_2.sql | Fri Jan 19 15:34:51 2024 | *       |
+---------+----------------+---------------------------------------------------------+--------------------------+---------+
| app2    | 20240117132418 | migrations/mysql/app2/20240117132418_create_table_1.sql | Fri Jan 19 15:34:51 2024 | -       |
| app2    | 20240117132421 | migrations/mysql/app2/20240117132421_create_table_2.sql | Fri Jan 19 15:34:51 2024 | *       |
+---------+----------------+---------------------------------------------------------+--------------------------+---------+
|         |                | MIGRATIONS                                              | 4                        |         |
+---------+----------------+---------------------------------------------------------+--------------------------+---------+
```

Roll back a single migration:

```sh
rockhopper down
```

Redo the last migration (down then up):

```sh
rockhopper redo
```

## CLI Commands

### Global Flags

| Flag | Default | Description |
|---|---|---|
| `--config` | `rockhopper.yaml` | Path to config file |
| `--debug` | `false` | Enable debug logging |

### `up` — Apply pending migrations

```sh
rockhopper up                # apply all pending migrations
rockhopper up --steps 3      # apply the next 3 pending migrations
rockhopper up --to 20240117  # apply up to a specific version
```

| Flag | Description |
|---|---|
| `--steps` | Number of migrations to apply |
| `--to` | Target version to migrate up to |

### `down` — Roll back migrations

```sh
rockhopper down              # roll back the last applied migration
rockhopper down --steps 3    # roll back the last 3 migrations
rockhopper down --to 20240116  # roll back down to a specific version
rockhopper down --all        # roll back all applied migrations
```

| Flag | Description |
|---|---|
| `--steps` | Number of migrations to roll back |
| `--to` | Target version to roll back to |
| `--all` | Roll back all migrations |

### `redo` — Redo the last migration

Rolls back the last applied migration, then re-applies it:

```sh
rockhopper redo
```

### `status` — Show migration status

```sh
rockhopper status
```

### `create` — Create a new migration file

```sh
rockhopper create -t sql --output migrations/mysql add_trades_table
rockhopper create -t go --output migrations add_custom_logic
```

| Flag | Default | Description |
|---|---|---|
| `-t`, `--type` | `sql` | Migration type: `sql` or `go` |
| `-o`, `--output` | from config | Output directory for the migration file |

Migration files are named with a timestamp prefix: `{YYYYMMDDhhmmss}_{name}.sql`

### `compile` — Compile SQL migrations into Go

```sh
rockhopper compile --output pkg/migrations/mysql
rockhopper compile --output pkg/migrations/mysql --package main --package app2
rockhopper compile --output pkg/migrations/mysql --no-build
```

| Flag | Default | Description |
|---|---|---|
| `-o`, `--output` | `pkg/migrations` | Output directory for the generated Go package |
| `-p`, `--package` | all | Filter specific packages to compile (repeatable) |
| `-B`, `--no-build` | `false` | Skip building the package after compiling |

### `align` — Align migration version

Synchronize the database state to a specific migration version:

```sh
rockhopper align main 20240116231445
```

Arguments: `<packageName> <versionID>`

## Configuration

### Config File

Pass `--config` to use a different config file:

```sh
rockhopper --config rockhopper_sqlite.yaml status
```

### Config Fields

```yaml
---
driver: mysql                    # Database driver: mysql, sqlite3, postgres
dialect: mysql                   # SQL dialect (defaults to driver if omitted)
dsn: "root@tcp(localhost:3306)/mydb?parseTime=true"
package: myapp                   # Default package name (defaults to "main")
tableName: rockhopper_versions   # Version table name (defaults to "rockhopper_versions")
migrationsDirs:                  # List of migration directories
- migrations/module1
- migrations/module2
includePackages:                 # Optional: only include these packages
- main
- app2
```

| Field | Default | Description |
|---|---|---|
| `driver` | | Database driver: `mysql`, `sqlite3`, `postgres` |
| `dialect` | same as driver | SQL dialect for query generation. Also supports `tidb` (uses mysql) and `redshift` (uses postgres) |
| `dsn` | | Data source name / connection string |
| `package` | `main` | Default migration package name |
| `tableName` | `rockhopper_versions` | Migration version table name |
| `migrationsDir` | `migrations` | Single migration directory (use `migrationsDirs` for multiple) |
| `migrationsDirs` | | List of migration directories |
| `includePackages` | all | Whitelist of packages to include when loading migrations |

## SQL Migration Format

A simple migration:

```sql
-- +up
CREATE TABLE post (
    id int NOT NULL,
    title text,
    body text,
    PRIMARY KEY(id)
);

-- +down
DROP TABLE post;
```

Each migration file must have exactly one `-- +up` annotation. The `-- +down` annotation is optional. If both are present, `-- +up` must come first.

### Annotations

| Annotation | Description |
|---|---|
| `-- +up` | Statements following this are executed on upgrade |
| `-- +down` | Statements following this are executed on rollback |
| `-- +begin` / `-- +end` | Wrap multi-statement blocks (e.g. PL/pgSQL with internal semicolons) |
| `-- !txn` | Disable transaction wrapping for this file (e.g. `CREATE DATABASE`) |
| `-- @package name` | Assign this migration to a named package (default: `main`) |

### Multi-statement example

```sql
-- +up
-- +begin
create or replace procedure prac_transfer(
   sender int,
   receiver int,
   amount dec
)
language plpgsql
as $$
begin
    update accounts
    set balance = balance - amount
    where id = sender;

    update accounts
    set balance = balance + amount
    where id = receiver;

    commit;
end;$$;
-- +end
```

### Non-transactional migrations

Use `-- !txn` for statements that cannot run inside a transaction:

```sql
-- +up
-- !txn
CREATE INDEX CONCURRENTLY idx_users_email ON users (email);

-- +down
-- !txn
DROP INDEX CONCURRENTLY idx_users_email;
```

### Package-based migrations

Use `-- @package <name>` to assign migrations to named packages. Rockhopper groups and executes them per package:

```sql
-- @package billing
-- +up
CREATE TABLE invoices (id INT PRIMARY KEY, amount DECIMAL(10,2));

-- +down
DROP TABLE invoices;
```

1. Collect all migration scripts
2. Categorize by package name
3. Execute migrations package by package

The default package name is `main`. Use `includePackages` in your config to selectively apply only certain packages.

## Multi-Dialect Workflow

When supporting multiple databases (e.g. MySQL and SQLite), maintain separate config files and migration directories:

```sh
# Create migration files for each dialect
rockhopper --config rockhopper_sqlite.yaml create --type sql add_pnl_column
rockhopper --config rockhopper_mysql.yaml create --type sql add_pnl_column

# Edit both files — SQL syntax may differ between dialects

# Apply migrations
rockhopper --config rockhopper_sqlite.yaml up
rockhopper --config rockhopper_mysql.yaml up
```

## Compiling Migrations into Go

Compile SQL migrations into a Go package for embedding in your binary:

```sh
rockhopper compile --config rockhopper_mysql.yaml --output pkg/migrations/mysql
rockhopper compile --config rockhopper_sqlite.yaml --output pkg/migrations/sqlite3
```

The generated package provides:

- `Migrations()` — returns all compiled migrations as a sorted `MigrationSlice`
- `SortedMigrations()` — alias for `Migrations()`
- `GetMigrationsMap()` — returns migrations grouped by package
- `MergeMigrationsMap()` — merge additional migrations at runtime
- `AddMigration()` — register new migrations dynamically

Then import and use the compiled migrations in your application:

```go
import (
    "context"
    "database/sql"

    "github.com/c9s/rockhopper/v2"

    mysqlMigrations "github.com/yourorg/yourapp/pkg/migrations/mysql"
)

func Migrate(ctx context.Context, db *sql.DB) error {
    dialect, err := rockhopper.LoadDialect("mysql")
    if err != nil {
        return err
    }

    rh := rockhopper.New("mysql", dialect, db, rockhopper.TableName)

    if err := rh.Touch(ctx); err != nil {
        return err
    }

    migrations := mysqlMigrations.Migrations()
    migrations = migrations.FilterPackage([]string{"main"}).SortAndConnect()
    if len(migrations) == 0 {
        return nil
    }

    _, lastAppliedMigration, err := rh.FindLastAppliedMigration(ctx, migrations)
    if err != nil {
        return err
    }

    if lastAppliedMigration != nil {
        return rockhopper.Up(ctx, rh, lastAppliedMigration.Next, 0)
    }

    return rockhopper.Up(ctx, rh, migrations.Head(), 0)
}
```

## Go API

Rockhopper can be used as a library in your Go application. Below are the key APIs.

### Opening a Database Connection

```go
// From a config struct
db, err := rockhopper.OpenWithConfig(config)

// From environment variables (reads MYAPP_DRIVER, MYAPP_DIALECT, MYAPP_DSN)
db, err := rockhopper.OpenWithEnv("MYAPP")

// Manual setup
dialect, _ := rockhopper.LoadDialect("mysql")
db, err := rockhopper.Open("mysql", dialect, dsn, rockhopper.TableName)

// Wrap an existing *sql.DB
dialect, _ := rockhopper.LoadDialect("mysql")
rh := rockhopper.New("mysql", dialect, existingDB, rockhopper.TableName)
```

### Running Migrations

```go
// Apply all pending migrations (pass 0 as target version to apply all)
rockhopper.Up(ctx, db, migrations.Head(), 0)

// Apply N steps
rockhopper.UpBySteps(ctx, db, migrations.Head(), 3)

// Apply all pending migrations across all packages
rockhopper.Upgrade(ctx, db, migrations)

// Apply from compiled Go migrations by package name
rockhopper.UpgradeFromGo(ctx, db, "main", "app2")

// Roll back to a specific version (pass 0 to roll back all)
rockhopper.Down(ctx, db, migrations.Tail(), 0)

// Roll back N steps
rockhopper.DownBySteps(ctx, db, migrations.Tail(), 3)

// Redo the last migration (down then up)
rockhopper.Redo(ctx, db, lastMigration)

// Align database to a specific version
rockhopper.Align(ctx, db, versionID, migrations)
```

Migration functions accept optional callbacks that fire after each migration is applied:

```go
rockhopper.Up(ctx, db, migrations.Head(), 0, func(m *rockhopper.Migration) {
    log.Printf("applied migration %d: %s", m.Version, m.Name)
})
```

### Working with MigrationSlice

```go
migrations := mysqlMigrations.Migrations()

// Filter by package and prepare the linked list
filtered := migrations.FilterPackage([]string{"main", "app2"}).SortAndConnect()

// Traverse
first := filtered.Head()
last := filtered.Tail()
versions := filtered.Versions() // []int64

// Find a specific version
m, err := filtered.Find(20240116231445)

// Group by package
migrationMap := migrations.MapByPackage()
```

### Loading SQL Migrations at Runtime

```go
loader := rockhopper.NewSqlMigrationLoader(config)
migrations, err := loader.Load("migrations/mysql")
```

### Registering Go Migrations

For Go-based migrations (instead of SQL files), register them from `init()`:

```go
package migrations

import (
    "context"

    "github.com/c9s/rockhopper/v2"
)

func init() {
    rockhopper.AddMigration(upAddUsers, downAddUsers)
}

func upAddUsers(ctx context.Context, tx rockhopper.SQLExecutor) error {
    _, err := tx.ExecContext(ctx, "CREATE TABLE users (id INT PRIMARY KEY, name TEXT)")
    return err
}

func downAddUsers(ctx context.Context, tx rockhopper.SQLExecutor) error {
    _, err := tx.ExecContext(ctx, "DROP TABLE users")
    return err
}
```

### Database Initialization

`Touch()` automatically creates the version table if it doesn't exist, and migrates from legacy Goose tables if detected:

```go
if err := db.Touch(ctx); err != nil {
    return err
}
```

### Querying Migration State

```go
// Get current version for a package
version, err := db.CurrentVersion(ctx, "main")

// Load a specific migration's record from the database
m, err := db.LoadMigration(ctx, migration)
if m != nil && m.Record != nil && m.Record.IsApplied {
    // migration has been applied
}

// Load all records for a package
records, err := db.LoadMigrationRecordsByPackage(ctx, "main")

// Find the last applied migration from a slice
idx, lastApplied, err := db.FindLastAppliedMigration(ctx, migrations)
```

## Environment Variables

You can override config file values with environment variables:

| Variable | Description |
|---|---|
| `ROCKHOPPER_DRIVER` | Database driver: `mysql`, `sqlite3`, `postgres` |
| `ROCKHOPPER_DIALECT` | SQL dialect for query generation |
| `ROCKHOPPER_DSN` | Data source name / connection string |
| `ROCKHOPPER_MIGRATIONS_DIR` | Single migration directory |
| `ROCKHOPPER_MIGRATIONS_DIRS` | Migration directories (comma-separated) |
| `ROCKHOPPER_TABLE_NAME` | Custom version table name |

Example with [dotenv](https://github.com/joho/godotenv):

```sh
dotenv -f .env.local -- rockhopper --config rockhopper_mysql.yaml up
```

## Claude Code Support

Rockhopper ships with built-in [Claude Code](https://claude.ai/code) skills, so you can manage migrations conversationally from your terminal. The following slash commands are available when working in a rockhopper-based project:

| Command | Description |
|---|---|
| `/create-migration <name>` | Create migration files for all configured dialects at once |
| `/compile-migrations` | Compile SQL migrations into Go source for embedding |
| `/apply-migrations` | Apply pending migrations (`up`) |
| `/rollback-migration` | Roll back the last applied migration (`down`) with confirmation |
| `/migration-status` | Show migration status across all configured databases |

These skills auto-discover `rockhopper_*.yaml` config files in your project root, so they work out of the box for any multi-dialect setup.

There is also a helper script `scripts/create-migration.sh` that can be used independently:

```sh
# Create migration files for all dialects
bash scripts/create-migration.sh add_pnl_column

# Specify migration type
bash scripts/create-migration.sh -t sql add_trades_table
```

## Credit

Thanks to <https://github.com/pressly/goose>, this project was forked from goose.

## License

MIT License
