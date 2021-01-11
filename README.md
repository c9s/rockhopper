rockhopper
======================

rockhopper is an embeddable migration tool written in Go, which can embed your migration files into a package with an easy-to-use API.

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




# License

MIT License

