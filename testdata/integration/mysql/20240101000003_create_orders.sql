-- @package integration
-- +up
-- +begin
CREATE TABLE orders
(
    `id`          BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
    `user_id`     BIGINT UNSIGNED NOT NULL,
    `product_id`  BIGINT UNSIGNED NOT NULL,
    `quantity`    INT             NOT NULL DEFAULT 1,
    `total`       DECIMAL(20, 8)  NOT NULL,
    `is_paid`     BOOLEAN         NOT NULL DEFAULT FALSE,
    `ordered_at`  DATETIME(3)     NOT NULL,
    `created_at`  TIMESTAMP       NOT NULL DEFAULT CURRENT_TIMESTAMP,

    PRIMARY KEY (`id`),
    KEY `idx_user_id` (`user_id`),
    KEY `idx_product_id` (`product_id`),
    CONSTRAINT `fk_orders_user` FOREIGN KEY (`user_id`) REFERENCES users (`id`),
    CONSTRAINT `fk_orders_product` FOREIGN KEY (`product_id`) REFERENCES products (`id`)
);
-- +end

-- +down
-- +begin
DROP TABLE orders;
-- +end
