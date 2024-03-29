-- @package app1
-- +up
-- +begin
CREATE TABLE app1_a
(
    `gid`            BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,

    `id`             BIGINT UNSIGNED,
    `order_id`       BIGINT UNSIGNED NOT NULL,
    `exchange`       VARCHAR(24) NOT NULL DEFAULT '',
    `symbol`         VARCHAR(20) NOT NULL,
    `price`          DECIMAL(16, 8) UNSIGNED NOT NULL,
    `quantity`       DECIMAL(16, 8) UNSIGNED NOT NULL,
    `quote_quantity` DECIMAL(16, 8) UNSIGNED NOT NULL,
    `fee`            DECIMAL(16, 8) UNSIGNED NOT NULL,
    `fee_currency`   VARCHAR(10) NOT NULL,
    `is_buyer`       BOOLEAN     NOT NULL DEFAULT FALSE,
    `is_maker`       BOOLEAN     NOT NULL DEFAULT FALSE,
    `side`           VARCHAR(4)  NOT NULL DEFAULT '',
    `traded_at`      DATETIME(3) NOT NULL,

    `is_margin`      BOOLEAN     NOT NULL DEFAULT FALSE,
    `is_isolated`    BOOLEAN     NOT NULL DEFAULT FALSE,

    `strategy`       VARCHAR(32) NULL,
    `pnl`            DECIMAL NULL,

    PRIMARY KEY (`gid`),
    UNIQUE KEY `id` (`exchange`, `symbol`, `side`, `id`)
);
-- +end
-- +begin
ALTER TABLE app1_a ADD COLUMN foo INT DEFAULT 0;
-- +end

-- +down
-- +begin
ALTER TABLE app1_a DROP COLUMN foo;
-- +end
-- +begin
DROP TABLE app1_a;
-- +end
