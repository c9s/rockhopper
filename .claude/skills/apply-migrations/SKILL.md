---
name: apply-migrations
description: Apply pending database migrations (rockhopper up)
disable-model-invocation: true
argument-hint: [config_file]
---

Apply pending migrations using rockhopper.

## Steps

1. If `$ARGUMENTS` is provided, use it as the config file path. Otherwise, find all `rockhopper_*.yaml` config files and ask the user which one to use.
2. Run:
   ```bash
   rockhopper --config <config_file> up
   ```
3. Report the result.

## Environment variable overrides

If the user needs to override the DSN or driver, remind them they can set:
- `ROCKHOPPER_DRIVER` - database driver name
- `ROCKHOPPER_DIALECT` - SQL dialect name
- `ROCKHOPPER_DSN` - data source name connection string
