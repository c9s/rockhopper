#!/bin/bash
set -x
set -e
go test ./...
dotenv -f .env.local -- go run ./cmd/rockhopper --config rockhopper_mysql_local.yaml status --debug
dotenv -f .env.local -- go run ./cmd/rockhopper --config rockhopper_mysql_local.yaml down --all
dotenv -f .env.local -- go run ./cmd/rockhopper --config rockhopper_mysql_local.yaml up
dotenv -f .env.local -- go run ./cmd/rockhopper --config rockhopper_mysql_local.yaml compile --output pkg/testing/migrations
