rockhopper
======================


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


# License

MIT License

