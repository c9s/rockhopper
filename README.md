rockhopper
======================

[![Go](https://github.com/c9s/rockhopper/actions/workflows/go.yml/badge.svg?branch=main)](https://github.com/c9s/rockhopper/actions/workflows/go.yml)
[![Go Reference](https://pkg.go.dev/badge/github.com/c9s/rockhopper/v2.svg)](https://pkg.go.dev/github.com/c9s/rockhopper/v2)
[![Go Report Card](https://goreportcard.com/badge/github.com/c9s/rockhopper/v2)](https://goreportcard.com/report/github.com/c9s/rockhopper/v2)

rockhopper is an embeddable migration tool written in Go, which can embed your migration files into a package with an
easy-to-use API.

REF: a small penguin with a yellowish crest, breeding on subantarctic coastal cliffs which it ascends by hopping from rock to rock.

![Console Demo](https://raw.githubusercontent.com/c9s/rockhopper/main/screenshots/screenshot1.png)

# Features

- Embeddable migration script - you can embed SQL files as go source files and compile them into a binary
- Support multiple drivers
- Modularized migration package structure
- Compatible with Goose <https://github.com/pressly/goose> 

# Supported Drivers

- mysql
- sqlite3
- postgresql
- mssql

# Install

```
go install github.com/c9s/rockhopper/v2/cmd/rockhopper@v2.0.3
```

# Quick Start

Add `rockhopper.yaml` with the following content:

```sh
---
driver: mysql
dialect: mysql
dsn: "root@tcp(localhost:3306)/rockhopper?parseTime=true"
package: myapp
migrationsDirs:
- migrations/module1
- migrations/module2
```

And create the directory structure for your migration files (or you can just use `migrations/`:

```sh
mkdir -p migrations/{module1,module2}
```

Then create a migration file with the following command:

```sh
rockhopper create -t sql --output migrations/module1 add_trades_table
```

Here is an example of the migration script (SQL format):

```sql
-- @package main
-- !txn
-- +up
-- +begin
CREATE TABLE trades
(
    `id`             BIGINT UNSIGNED,
    `order_id`       BIGINT UNSIGNED NOT NULL,
    `symbol`         VARCHAR(20) NOT NULL,
    `price`          DECIMAL(16, 8) UNSIGNED NOT NULL,
    `quantity`       DECIMAL(16, 8) UNSIGNED NOT NULL,
    `fee`            DECIMAL(16, 8) UNSIGNED NOT NULL,
    `fee_currency`   VARCHAR(10) NOT NULL,
    `side`           VARCHAR(4)  NOT NULL DEFAULT '',
    `traded_at`      DATETIME(3) NOT NULL,
    PRIMARY KEY (`gid`),
    UNIQUE KEY `id` (`exchange`, `symbol`, `side`, `id`)
);
-- +end
-- +begin
ALTER TABLE trades ADD COLUMN foo INT DEFAULT 0;
-- +end

-- +down
-- +begin
ALTER TABLE trades DROP COLUMN foo;
-- +end
-- +begin
DROP TABLE trades;
-- +end
```

After editing, you can check your migration status:

```sh
rockhopper status
```

To upgrade:

```shell
rockhopper up
```

To downgrade:

```shell
rockhopper down
```



# Usage

Create a directory for your migrations:

```sh
mkdir migrations
```

Add `rockhopper.yaml` with the following content:

```sh
---
driver: mysql
dialect: mysql
dsn: "root@tcp(localhost:3306)/rockhopper?parseTime=true"
migrationsDir: migrations
```

To add new migration scripts:

```sh
rockhopper create -t sql my_first_migration
```

Or, more advanced:

```sh
rockhopper --config rockhopper_mysql_local.yaml create -o migrations/mysql/app1 -t sql "create table 1"
```

## Status

To check migration script status:

```sh
rockhopper status
```

## Upgrade

Apply all available migrations:

```shell
rockhopper up
```

When marking the migration script with `@package app1`, the migration scripts will be executed per package,
here is the flow:

- Collect all migration scripts
- Categorize the migration scripts by their package name
- Iterate the migration scripts by package

The default package name is set to `main`

## Downgrade

Roll back a single migration from the current version:

```shell
rockhopper down
```

## Redo

To redo a migration:

```shell
rockhopper redo
```

You can compile your SQL migrations into a go package:

```shell
rockhopper compile -o pkg/migrations
```

You can change the default config file name by passing the `--config` parameter:

```shell
rockhooper --config rockhopper_sqlite3.yaml status
```

## Status

```sh
$ rockhopper status

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

# Environment Variables

```azure
ROCKHOPPER_DRIVER=mysql
ROCKHOPPER_DIALECT=mysql
ROCKHOPPER_DSN="root:root@unix(/opt/local/var/run/mysql57/mysqld.sock)/bbgo"
```

`ROCKHOPPER_DRIVER` is the db driver name that will be used for the protocol.

`ROCKHOPPER_DIALECT` is the dialect name that will be used for generating different kinds of SQL, e.g. mysql, sqlite3, postgresql...

`ROCKHOPPER_DSN` is the DSN used for connecting to the database.


# Migrations

rockhopper supports migrations written in SQL or in Go.

## SQL Migrations

A simple SQL migration looks like:

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

Each migration file must have exactly one `-- +up` annotation.
The `-- +down` annotation is optional.
If the file has both annotations, then the `-- +up` annotation **must** come first.

Notice the annotations in the comments.
Any statements following `-- +up` will be executed as part of a forward migration,
and any statements following `-- +down` will be executed as part of a rollback.

By default, all migrations are run within a transaction.
Some statements like `CREATE DATABASE`, however, cannot be run within a transaction,
You may optionally add `-- !txn` to the top of your migration file in order to skip transactions within that specific migration file.
Both Up and Down migrations within this file will be run without transactions.

By default, SQL statements are delimited by semicolons - in fact, query statements must end with a semicolon to be properly recognized by rockhopper.

More complex statements (PL/pgSQL) that have semicolons within them must be annotated with `-- +begin` and `-- +end` to be properly recognized. For example:

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
    -- subtracting the amount from the sender's account 
    update accounts 
    set balance = balance - amount 
    where id = sender;

    -- adding the amount to the receiver's account
    update accounts 
    set balance = balance + amount 
    where id = receiver;

    commit;
end;$$;-- +end
```


# Embedded SQL migrations

With rockhopper, you can embed the SQL migration files into your application,
simply run:

```sh
rockhopper compile --output pkg/migrations
```

the SQL migration files will be compiled as a GO package, you can simply import the package to load these migrations,
with the following example:

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

	rh := rockhopper.New(s.Driver, dialect, db, rockhopper.TableName)
	
	if err := rh.Touch(ctx); err != nil {
		return err
	}

    migrations = mysqlMigrations.Migrations()
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


# API

If you need to integrate rockhopper API, for example, controlling the upgrade/downgrade process from your application,
you can call the rockhopper API to do these things:

With config file:

```go
// load config into the global instance
config, err = rockhopper.LoadConfig(configFile)
if err != nil {
    log.Fatal(err)
}

db, err := rockhopper.OpenByConfig(config)
if err != nil {
    return err
}

defer db.Close()

currentVersion, err = db.CurrentVersion()
if err != nil {
    return err
}

loader := &rockhopper.SqlMigrationLoader{}
migrations, err := loader.Load(config.MigrationsDir)
if err != nil {
    return err
}

for _, m := range migrations {
	// ....
}

err = rockhopper.Up(ctx, db, migrations, currentVersion, to, func(m *rockhopper.Migration) {
    log.Infof("migration %v is applied", m.Version)
})
```

Without config file:

```go
driver := os.Getenv("DB_DRIVER")

dialect, err := rockhopper.LoadDialect(driver)
if err != nil {
	return err
}

var migrations rockhopper.MigrationSlice

switch s.Driver {
	case "sqlite3":
		migrations = sqlite3Migrations.Migrations()
	case "mysql":
		migrations = mysqlMigrations.Migrations()
}

// sqlx.DB is different from sql.DB
rh := rockhopper.New(s.Driver, dialect, s.DB.DB)

currentVersion, err := rh.CurrentVersion()
if err != nil {
	return err
}

if err := rockhopper.Up(ctx, rh, migrations, currentVersion, 0); err != nil {
	return err
}
```

# Credit

Thanks to <https://github.com/pressly/goose>, this project was forked from goose.

# License

MIT License

