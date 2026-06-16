package migrations

import (
	"github.com/c9s/rockhopper/v2"
)

// This migration was compiled from migrations/mysql/app1/20240116231445_create_table_1.sql.
// The SQL statements are registered as data so they can be previewed in the
// console while the migration runs, exactly like a raw .sql migration.
func init() {
	AddStatementMigration("app1", 20240116231445, "migrations/mysql/app1/20240116231445_create_table_1.sql", true,
		[]rockhopper.Statement{
			{Direction: rockhopper.DirectionUp, SQL: "CREATE TABLE app1_a\n(\n    `gid`            BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,\n    `id`             BIGINT UNSIGNED,\n    `order_id`       BIGINT UNSIGNED NOT NULL,\n    `exchange`       VARCHAR(24) NOT NULL DEFAULT '',\n    `symbol`         VARCHAR(20) NOT NULL,\n    `price`          DECIMAL(16, 8) UNSIGNED NOT NULL,\n    `quantity`       DECIMAL(16, 8) UNSIGNED NOT NULL,\n    `quote_quantity` DECIMAL(16, 8) UNSIGNED NOT NULL,\n    `fee`            DECIMAL(16, 8) UNSIGNED NOT NULL,\n    `fee_currency`   VARCHAR(10) NOT NULL,\n    `is_buyer`       BOOLEAN     NOT NULL DEFAULT FALSE,\n    `is_maker`       BOOLEAN     NOT NULL DEFAULT FALSE,\n    `side`           VARCHAR(4)  NOT NULL DEFAULT '',\n    `traded_at`      DATETIME(3) NOT NULL,\n    `is_margin`      BOOLEAN     NOT NULL DEFAULT FALSE,\n    `is_isolated`    BOOLEAN     NOT NULL DEFAULT FALSE,\n    `strategy`       VARCHAR(32) NULL,\n    `pnl`            DECIMAL NULL,\n    PRIMARY KEY (`gid`),\n    UNIQUE KEY `id` (`exchange`, `symbol`, `side`, `id`)\n);"},
			{Direction: rockhopper.DirectionUp, SQL: "ALTER TABLE app1_a ADD COLUMN foo INT DEFAULT 0;"},
		},
		[]rockhopper.Statement{
			{Direction: rockhopper.DirectionDown, SQL: "ALTER TABLE app1_a DROP COLUMN foo;"},
			{Direction: rockhopper.DirectionDown, SQL: "DROP TABLE app1_a;"},
		},
	)
}
