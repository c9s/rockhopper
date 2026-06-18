-- @package integration
-- +up
-- +begin
CREATE TABLE products
(
    `id`          BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
    `sku`         VARCHAR(32)     NOT NULL,
    `name`        VARCHAR(128)    NOT NULL,
    `price`       DECIMAL(16, 4)  NOT NULL,
    `in_stock`    BOOLEAN         NOT NULL DEFAULT FALSE,
    `released_at` DATETIME        NULL,
    `created_at`  TIMESTAMP       NOT NULL DEFAULT CURRENT_TIMESTAMP,

    PRIMARY KEY (`id`),
    UNIQUE KEY `uniq_sku` (`sku`)
);
-- +end

-- +down
-- +begin
DROP TABLE products;
-- +end
