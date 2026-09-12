-- PRD-005 进阶 A1：库存流水表。每次预占/释放都留一行凭证，用于对账与审计。
-- change_qty 带符号（预占为负、释放为正），balance_stock 记录变动后的库存快照，便于事后核对不依赖当前值反推。
CREATE TABLE stock_movements (
    id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
    sku_id BIGINT UNSIGNED NOT NULL COMMENT '变动的 SKU',
    change_qty BIGINT NOT NULL COMMENT '变动数量，带符号：预占为负、释放为正',
    balance_stock BIGINT NOT NULL COMMENT '本次变动后的库存快照，用于对账不依赖当前值反推',
    type ENUM('reserve', 'release') NOT NULL COMMENT '业务类型：下单预占/取消或关单释放',
    order_no VARCHAR(32) NOT NULL COMMENT '关联订单号，凭证署名',
    created_at DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
    deleted_at DATETIME(6) NULL,
    PRIMARY KEY (id),
    INDEX idx_stock_movements_sku_created (sku_id, created_at),
    INDEX idx_stock_movements_order_no (order_no),
    INDEX idx_stock_movements_deleted_at (deleted_at),
    CONSTRAINT fk_stock_movements_sku FOREIGN KEY (sku_id) REFERENCES skus (id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci COMMENT='库存流水表：每次预占/释放留一行凭证（A1）';
