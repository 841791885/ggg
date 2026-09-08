-- PRD-004 收尾：购物车选择与下单预览的数据基础。
ALTER TABLE cart_items ADD COLUMN selected TINYINT(1) NOT NULL DEFAULT 1;

-- PRD-005：订单主表。金额使用分（int64），地址与商品快照保证历史不受后续修改影响。
CREATE TABLE orders (
    id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
    order_no VARCHAR(32) NOT NULL,
    user_id BIGINT UNSIGNED NOT NULL,
    idempotency_key CHAR(64) NOT NULL,
    request_hash CHAR(64) NOT NULL,
    status ENUM('pending_payment', 'paid', 'shipped', 'completed', 'cancelled') NOT NULL DEFAULT 'pending_payment',
    total_cent BIGINT NOT NULL,
    pay_cent BIGINT NOT NULL,
    discount_cent BIGINT NOT NULL DEFAULT 0,
    address_snapshot JSON NOT NULL,
    expires_at DATETIME(6) NULL,
    paid_at DATETIME(6) NULL,
    shipped_at DATETIME(6) NULL,
    completed_at DATETIME(6) NULL,
    cancelled_at DATETIME(6) NULL,
    created_at DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
    updated_at DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6) ON UPDATE CURRENT_TIMESTAMP(6),
    deleted_at DATETIME(6) NULL,
    PRIMARY KEY (id),
    UNIQUE KEY uk_orders_order_no (order_no),
    UNIQUE KEY uk_orders_user_idempotency (user_id, idempotency_key),
    INDEX idx_orders_user_status (user_id, status),
    INDEX idx_orders_status_expires (status, expires_at),
    INDEX idx_orders_deleted_at (deleted_at),
    CONSTRAINT fk_orders_user FOREIGN KEY (user_id) REFERENCES users (id),
    CONSTRAINT chk_orders_amount_non_negative CHECK (total_cent >= 0 AND pay_cent >= 0 AND discount_cent >= 0)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

-- PRD-005：订单项，保存商品/SKU/价格快照；退款按订单项维度申请。
CREATE TABLE order_items (
    id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
    order_id BIGINT UNSIGNED NOT NULL,
    sku_id BIGINT UNSIGNED NOT NULL,
    product_name VARCHAR(100) NOT NULL,
    sku_code VARCHAR(32) NOT NULL,
    specs JSON NULL,
    unit_price_cent BIGINT NOT NULL,
    quantity BIGINT NOT NULL,
    subtotal_cent BIGINT NOT NULL,
    refund_status ENUM('none', 'pending', 'refunded') NOT NULL DEFAULT 'none',
    created_at DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
    updated_at DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6) ON UPDATE CURRENT_TIMESTAMP(6),
    deleted_at DATETIME(6) NULL,
    PRIMARY KEY (id),
    INDEX idx_order_items_order_id (order_id),
    INDEX idx_order_items_sku_id (sku_id),
    INDEX idx_order_items_deleted_at (deleted_at),
    CONSTRAINT fk_order_items_order FOREIGN KEY (order_id) REFERENCES orders (id),
    CONSTRAINT fk_order_items_sku FOREIGN KEY (sku_id) REFERENCES skus (id),
    CONSTRAINT chk_order_items_quantity_positive CHECK (quantity > 0)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

-- PRD-005：订单状态变更历史，状态机审计依据。
CREATE TABLE order_status_logs (
    id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
    order_id BIGINT UNSIGNED NOT NULL,
    from_status VARCHAR(20) NOT NULL,
    to_status VARCHAR(20) NOT NULL,
    operator_type ENUM('system', 'user', 'admin') NOT NULL,
    operator_id BIGINT UNSIGNED NOT NULL DEFAULT 0,
    remark VARCHAR(200) NOT NULL DEFAULT '',
    created_at DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
    PRIMARY KEY (id),
    INDEX idx_order_status_logs_order_id (order_id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

-- PRD-007：支付单。金额完全由订单生成；渠道事件号唯一，重复回调靠它幂等。
CREATE TABLE payments (
    id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
    payment_no VARCHAR(32) NOT NULL,
    order_id BIGINT UNSIGNED NOT NULL,
    user_id BIGINT UNSIGNED NOT NULL,
    amount_cent BIGINT NOT NULL,
    currency VARCHAR(8) NOT NULL DEFAULT 'CNY',
    channel VARCHAR(20) NOT NULL DEFAULT 'mock',
    status ENUM('pending', 'success', 'failed', 'closed') NOT NULL DEFAULT 'pending',
    event_no VARCHAR(64) NULL,
    paid_at DATETIME(6) NULL,
    created_at DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
    updated_at DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6) ON UPDATE CURRENT_TIMESTAMP(6),
    deleted_at DATETIME(6) NULL,
    PRIMARY KEY (id),
    UNIQUE KEY uk_payments_payment_no (payment_no),
    UNIQUE KEY uk_payments_event_no (event_no),
    INDEX idx_payments_order_id (order_id),
    INDEX idx_payments_deleted_at (deleted_at),
    CONSTRAINT fk_payments_order FOREIGN KEY (order_id) REFERENCES orders (id),
    CONSTRAINT chk_payments_amount_positive CHECK (amount_cent > 0)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

-- PRD-007：回调原始报文留档，供对账与排障。
CREATE TABLE payment_callback_logs (
    id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
    payment_no VARCHAR(32) NOT NULL,
    event_no VARCHAR(64) NOT NULL,
    result VARCHAR(20) NOT NULL,
    payload JSON NOT NULL,
    processed TINYINT(1) NOT NULL DEFAULT 0,
    created_at DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
    PRIMARY KEY (id),
    INDEX idx_payment_callback_logs_payment_no (payment_no)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

-- PRD-009：退款单，按订单项申请，运营审核。
CREATE TABLE refunds (
    id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
    refund_no VARCHAR(32) NOT NULL,
    order_id BIGINT UNSIGNED NOT NULL,
    order_item_id BIGINT UNSIGNED NOT NULL,
    user_id BIGINT UNSIGNED NOT NULL,
    amount_cent BIGINT NOT NULL,
    reason VARCHAR(200) NOT NULL DEFAULT '',
    status ENUM('pending', 'approved', 'rejected', 'refunded') NOT NULL DEFAULT 'pending',
    reviewed_by BIGINT UNSIGNED NULL,
    reviewed_at DATETIME(6) NULL,
    created_at DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
    updated_at DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6) ON UPDATE CURRENT_TIMESTAMP(6),
    deleted_at DATETIME(6) NULL,
    PRIMARY KEY (id),
    UNIQUE KEY uk_refunds_refund_no (refund_no),
    -- 同一订单项同时只允许有一条待处理退款；已驳回后可再次申请。
    UNIQUE KEY uk_refunds_item_pending (order_item_id, status),
    INDEX idx_refunds_user_id (user_id),
    INDEX idx_refunds_deleted_at (deleted_at),
    CONSTRAINT fk_refunds_order FOREIGN KEY (order_id) REFERENCES orders (id),
    CONSTRAINT fk_refunds_order_item FOREIGN KEY (order_item_id) REFERENCES order_items (id),
    CONSTRAINT chk_refunds_amount_positive CHECK (amount_cent > 0)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

-- PRD-009：优惠券模板。remaining 条件更新防超发属于后续进阶点，本阶段先建结构。
CREATE TABLE coupon_templates (
    id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
    name VARCHAR(50) NOT NULL,
    type ENUM('threshold_discount') NOT NULL DEFAULT 'threshold_discount',
    threshold_cent BIGINT NOT NULL,
    discount_cent BIGINT NOT NULL,
    total_count INT NOT NULL,
    remaining INT NOT NULL,
    per_user_limit INT NOT NULL DEFAULT 1,
    starts_at DATETIME(6) NOT NULL,
    ends_at DATETIME(6) NOT NULL,
    status ENUM('active', 'inactive') NOT NULL DEFAULT 'active',
    created_at DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
    updated_at DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6) ON UPDATE CURRENT_TIMESTAMP(6),
    deleted_at DATETIME(6) NULL,
    PRIMARY KEY (id),
    INDEX idx_coupon_templates_status (status),
    INDEX idx_coupon_templates_deleted_at (deleted_at),
    CONSTRAINT chk_coupon_remaining_range CHECK (remaining >= 0 AND remaining <= total_count),
    CONSTRAINT chk_coupon_discount_positive CHECK (threshold_cent > 0 AND discount_cent > 0)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

-- PRD-009：用户持有的券。uk 先按每人每模板一张建模；放开 per_user_limit>1 时需同步调整唯一键。
CREATE TABLE user_coupons (
    id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
    user_id BIGINT UNSIGNED NOT NULL,
    template_id BIGINT UNSIGNED NOT NULL,
    status ENUM('unused', 'used', 'expired') NOT NULL DEFAULT 'unused',
    order_id BIGINT UNSIGNED NULL,
    claimed_at DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
    used_at DATETIME(6) NULL,
    created_at DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
    updated_at DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6) ON UPDATE CURRENT_TIMESTAMP(6),
    deleted_at DATETIME(6) NULL,
    PRIMARY KEY (id),
    UNIQUE KEY uk_user_coupons_user_template (user_id, template_id),
    INDEX idx_user_coupons_status (status),
    INDEX idx_user_coupons_deleted_at (deleted_at),
    CONSTRAINT fk_user_coupons_user FOREIGN KEY (user_id) REFERENCES users (id),
    CONSTRAINT fk_user_coupons_template FOREIGN KEY (template_id) REFERENCES coupon_templates (id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

-- PRD-009：评价绑定真实已完成订单项，一人一项一条。
CREATE TABLE reviews (
    id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
    order_item_id BIGINT UNSIGNED NOT NULL,
    product_id BIGINT UNSIGNED NOT NULL,
    user_id BIGINT UNSIGNED NOT NULL,
    rating TINYINT NOT NULL,
    content VARCHAR(500) NOT NULL DEFAULT '',
    visible TINYINT(1) NOT NULL DEFAULT 1,
    hidden_by BIGINT UNSIGNED NULL,
    created_at DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
    updated_at DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6) ON UPDATE CURRENT_TIMESTAMP(6),
    deleted_at DATETIME(6) NULL,
    PRIMARY KEY (id),
    UNIQUE KEY uk_reviews_order_item (order_item_id),
    INDEX idx_reviews_product_visible (product_id, visible),
    INDEX idx_reviews_deleted_at (deleted_at),
    CONSTRAINT fk_reviews_order_item FOREIGN KEY (order_item_id) REFERENCES order_items (id),
    CONSTRAINT fk_reviews_product FOREIGN KEY (product_id) REFERENCES products (id),
    CONSTRAINT chk_reviews_rating_range CHECK (rating BETWEEN 1 AND 5)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

-- PRD-008：站内通知。
CREATE TABLE notifications (
    id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
    user_id BIGINT UNSIGNED NOT NULL,
    type VARCHAR(30) NOT NULL,
    title VARCHAR(50) NOT NULL,
    content VARCHAR(500) NOT NULL DEFAULT '',
    read_at DATETIME(6) NULL,
    created_at DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
    updated_at DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6) ON UPDATE CURRENT_TIMESTAMP(6),
    deleted_at DATETIME(6) NULL,
    PRIMARY KEY (id),
    INDEX idx_notifications_user_read (user_id, read_at),
    INDEX idx_notifications_deleted_at (deleted_at)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

-- PRD-008：后台任务表（Outbox 事实来源）。worker 消费逻辑属于进阶阶段，本阶段仅提供查询与重试 API。
CREATE TABLE background_tasks (
    id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
    task_type VARCHAR(30) NOT NULL,
    payload JSON NOT NULL,
    status ENUM('pending', 'running', 'succeeded', 'failed') NOT NULL DEFAULT 'pending',
    attempts INT NOT NULL DEFAULT 0,
    max_attempts INT NOT NULL DEFAULT 5,
    next_run_at DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
    last_error VARCHAR(500) NOT NULL DEFAULT '',
    retry_operator_id BIGINT UNSIGNED NULL,
    created_at DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
    updated_at DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6) ON UPDATE CURRENT_TIMESTAMP(6),
    deleted_at DATETIME(6) NULL,
    PRIMARY KEY (id),
    INDEX idx_background_tasks_status_next_run (status, next_run_at),
    INDEX idx_background_tasks_deleted_at (deleted_at),
    CONSTRAINT chk_background_tasks_attempts_range CHECK (attempts >= 0 AND attempts <= max_attempts)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;
