-- @package integration
-- +up
-- +begin
CREATE TABLE users
(
    `id`         BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
    `name`       VARCHAR(64)     NOT NULL,
    `email`      VARCHAR(128)    NOT NULL,
    `is_active`  BOOLEAN         NOT NULL DEFAULT TRUE,
    `balance`    DECIMAL(20, 8)  NOT NULL DEFAULT 0,
    `signup_at`  DATETIME(3)     NOT NULL,
    `created_at` TIMESTAMP       NOT NULL DEFAULT CURRENT_TIMESTAMP,

    PRIMARY KEY (`id`),
    UNIQUE KEY `uniq_email` (`email`)
);
-- +end

-- +down
-- +begin
DROP TABLE users;
-- +end
