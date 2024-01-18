#!/bin/bash
set -e
dotenv -f .env.local -- go run ./cmd/rockhopper --config rockhopper_mysql_local.yaml status --debug
dotenv -f .env.local -- go run ./cmd/rockhopper --config rockhopper_mysql_local.yaml down --all
dotenv -f .env.local -- go run ./cmd/rockhopper --config rockhopper_mysql_local.yaml up
