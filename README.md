rockhopper
======================

rockhopper is an embeddable migration tool written in Go, which can embed your migration files into a package with an
easy-to-use API.

# Install

```
go get github.com/c9s/rockhopper/cmd/rockhopper
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

# API

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

# License

MIT License

