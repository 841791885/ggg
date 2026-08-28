CREATE TABLE IF NOT EXISTS products (
    id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
    name VARCHAR(100) NOT NULL,
    description VARCHAR(2000) NOT NULL DEFAULT '',
    status ENUM('draft', 'on_sale', 'off_sale') NOT NULL,
    created_at DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
    updated_at DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6) ON UPDATE CURRENT_TIMESTAMP(6),
    deleted_at DATETIME(6) NULL,
    PRIMARY KEY (id),
    INDEX idx_products_status_created (status, created_at, id),
    INDEX idx_products_deleted_at (deleted_at)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

CREATE TABLE IF NOT EXISTS skus (
    id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
    product_id BIGINT UNSIGNED NOT NULL,
    code VARCHAR(32) NOT NULL,
    specs JSON NOT NULL,
    price_cent BIGINT NOT NULL,
    stock BIGINT NOT NULL,
    status ENUM('active', 'inactive') NOT NULL,
    created_at DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
    updated_at DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6) ON UPDATE CURRENT_TIMESTAMP(6),
    deleted_at DATETIME(6) NULL,
    PRIMARY KEY (id),
    UNIQUE KEY uk_skus_code (code),
    INDEX idx_skus_product_created (product_id, created_at, id),
    INDEX idx_skus_deleted_at (deleted_at),
    CONSTRAINT fk_skus_product FOREIGN KEY (product_id) REFERENCES products (id),
    CONSTRAINT chk_skus_price_positive CHECK (price_cent > 0),
    CONSTRAINT chk_skus_stock_non_negative CHECK (stock >= 0)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

CREATE DATABASE IF NOT EXISTS gomall_test
    CHARACTER SET utf8mb4
    COLLATE utf8mb4_0900_ai_ci;
GRANT ALL PRIVILEGES ON gomall_test.* TO 'gomall'@'%';
