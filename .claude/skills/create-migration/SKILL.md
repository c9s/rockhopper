---
name: create-migration
description: Create new SQL migration files for multiple database dialects (sqlite, mysql, etc.)
disable-model-invocation: true
argument-hint: [migration_name]
---

Create new migration files named `$ARGUMENTS` for all configured database dialects.

## Steps

1. Find all `rockhopper_*.yaml` config files in the project root to determine which dialects are configured.
2. For each config file found, run:
   ```bash
   rockhopper --config <config_file> create --type sql $ARGUMENTS
   ```
3. List the newly created migration files and show their paths.
4. Remind the user to edit both migration files since each dialect may need different SQL syntax.

## Migration file format reference

```sql
-- @package packagename
-- +up
CREATE TABLE example (id INT PRIMARY KEY);

-- +down
DROP TABLE example;
```

For multi-statement blocks, use `-- +begin` / `-- +end`. Use `-- !txn` to disable transaction wrapping.
