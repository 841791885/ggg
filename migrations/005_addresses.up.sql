CREATE TABLE addresses (
    id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
    user_id BIGINT UNSIGNED NOT NULL,
    recipient VARCHAR(30) NOT NULL,
    phone VARCHAR(20) NOT NULL,
    province VARCHAR(50) NOT NULL,
    city VARCHAR(50) NOT NULL,
    district VARCHAR(50) NOT NULL,
    detail VARCHAR(200) NOT NULL,
    is_default TINYINT(1) NOT NULL DEFAULT 0,
    deleted_at DATETIME(6) NULL,
    -- 数据库层兜底：仅当地址是未删除的默认地址时生成列才等于 user_id，
    -- 同一用户出现第二条有效默认地址会命中唯一键冲突。
    -- GORM 模型不映射此列，应用正常路径靠事务保证，这里防御绕过应用的写入。
    default_user_id BIGINT UNSIGNED GENERATED ALWAYS AS (IF(is_default = 1 AND deleted_at IS NULL, user_id, NULL)) STORED,
    created_at DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
    updated_at DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6) ON UPDATE CURRENT_TIMESTAMP(6),
    PRIMARY KEY (id),
    INDEX idx_addresses_user_id (user_id),
    UNIQUE KEY uk_addresses_default_user (default_user_id),
    INDEX idx_addresses_deleted_at (deleted_at),
    CONSTRAINT fk_addresses_user FOREIGN KEY (user_id) REFERENCES users (id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;
