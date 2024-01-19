rockhopper
======================

[![Go](https://github.com/c9s/rockhopper/actions/workflows/go.yml/badge.svg?branch=main)](https://github.com/c9s/rockhopper/actions/workflows/go.yml)

rockhopper is an embeddable migration tool written in Go, which can embed your migration files into a package with an
easy-to-use API.

REF: a small penguin with a yellowish crest, breeding on subantarctic coastal cliffs which it ascends by hopping from rock to rock.

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
go install github.com/c9s/rockhopper/cmd/rockhopper@latest
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

Now you can check your migration status:

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

To check migration status:

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

# API

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

# License

MIT License

