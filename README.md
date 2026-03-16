rockhopper
======================

[![Go](https://github.com/c9s/rockhopper/actions/workflows/go.yml/badge.svg?branch=main)](https://github.com/c9s/rockhopper/actions/workflows/go.yml)
[![Go Reference](https://pkg.go.dev/badge/github.com/c9s/rockhopper/v2.svg)](https://pkg.go.dev/github.com/c9s/rockhopper/v2)
[![Go Report Card](https://goreportcard.com/badge/github.com/c9s/rockhopper/v2)](https://goreportcard.com/report/github.com/c9s/rockhopper/v2)

rockhopper is an embeddable migration tool written in Go, which can embed your migration files into a package with an
easy-to-use API.

REF: a small penguin with a yellowish crest, breeding on subantarctic coastal cliffs which it ascends by hopping from rock to rock.

![Console Demo](https://raw.githubusercontent.com/c9s/rockhopper/main/screenshots/screenshot1.png)

## Features

- Embeddable migration script — compile SQL files into Go source and ship them in a single binary
- Package-based migration organization — group and execute migrations by package name
- Multi-dialect support: MySQL, SQLite3, PostgreSQL
- Compatible with [Goose](https://github.com/pressly/goose) migration format

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

## Using a Custom Config File

Pass `--config` to use a different config file:

```sh
rockhopper --config rockhopper_sqlite.yaml status
```

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

### Package-based migrations

When migration scripts use `-- @package <name>`, rockhopper groups and executes them per package:

1. Collect all migration scripts
2. Categorize by package name
3. Execute migrations package by package

The default package name is `main`.

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

Then import and use the compiled migrations in your application:

```go
import (
    "context"

    "github.com/c9s/rockhopper/v2"

    mysqlMigrations "github.com/c9s/bbgo/pkg/migrations/mysql"
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

## Environment Variables

You can override config file values with environment variables:

| Variable | Description |
|---|---|
| `ROCKHOPPER_DRIVER` | Database driver name (e.g. `mysql`, `sqlite3`, `postgres`) |
| `ROCKHOPPER_DIALECT` | SQL dialect for query generation |
| `ROCKHOPPER_DSN` | Data source name for database connection |

Example with [dotenv](https://github.com/joho/godotenv):

```sh
dotenv -f .env.local -- rockhopper --config rockhopper_mysql.yaml up
```

## Credit

Thanks to <https://github.com/pressly/goose>, this project was forked from goose.

## License

MIT License
