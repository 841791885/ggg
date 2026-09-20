CREATE TABLE shipments (
    id           BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
    order_id     BIGINT UNSIGNED NOT NULL,
    shipping_key CHAR(64) NOT NULL COMMENT '发货幂等键（哈希后），同订单内唯一',
    carrier      VARCHAR(50) NOT NULL COMMENT '承运商',
    tracking_no  VARCHAR(64) NOT NULL COMMENT '物流单号',
    shipped_by   BIGINT UNSIGNED NOT NULL COMMENT '发货人',
    shipped_at   DATETIME(6) NOT NULL,
    created_at   DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
    updated_at   DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6) ON UPDATE CURRENT_TIMESTAMP(6),
    deleted_at   DATETIME(6) NULL,
    PRIMARY KEY (id),
    UNIQUE KEY uk_shipments_order_key (order_id, shipping_key, deleted_at),
    INDEX idx_shipments_order (order_id),
    INDEX idx_shipments_deleted_at (deleted_at),
    CONSTRAINT fk_shipments_order FOREIGN KEY (order_id) REFERENCES orders (id),
    CONSTRAINT fk_shipments_operator FOREIGN KEY (shipped_by) REFERENCES users (id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;