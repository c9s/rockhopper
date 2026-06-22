//go:build !no_clickhouse

package driver

import (
	// Importing the driver registers the "clickhouse" driver with database/sql.
	_ "github.com/ClickHouse/clickhouse-go/v2"
)
