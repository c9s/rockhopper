---
name: create-migration
description: Create new SQL migration files for multiple database dialects (sqlite3, mysql, postgres, tidb, redshift, clickhouse)
disable-model-invocation: true
allowed-tools: Bash
argument-hint: [migration_name]
---

Create new migration files named `$ARGUMENTS` for all configured database dialects.

Run the wrapper script:

```bash
bash scripts/create-migration.sh $ARGUMENTS
```

If the script is not found, fall back to finding all `rockhopper_*.yaml` config files and running `rockhopper --config <config> create --type sql $ARGUMENTS` for each.

After creation, list the newly created files and remind the user to edit all migration files since each dialect may need different SQL syntax. OLAP dialects diverge the most — e.g. ClickHouse tables need a `MergeTree()` / `ORDER BY` engine clause instead of a primary key, use `Int64`/`String`/`DateTime` types, and have no `AUTO_INCREMENT`.

## Migration file format reference

```sql
-- @package packagename
-- +up
CREATE TABLE example (id INT PRIMARY KEY);

-- +down
DROP TABLE example;
```

For multi-statement blocks, use `-- +begin` / `-- +end`. Use `-- !txn` to disable transaction wrapping.
