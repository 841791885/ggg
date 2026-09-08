package models

import (
	"slices"
	"time"

	"gorm.io/gorm"
)

// OrderStatus 表示订单状态机的当前状态。
type OrderStatus string

const (
	OrderStatusPendingPayment OrderStatus = "pending_payment"
	OrderStatusPaid           OrderStatus = "paid"
	OrderStatusShipped        OrderStatus = "shipped"
	OrderStatusCompleted      OrderStatus = "completed"
	OrderStatusCancelled      OrderStatus = "cancelled"
)

// orderTransitions 定义状态机允许的迁移路径，非法迁移一律拒绝。
var orderTransitions = map[OrderStatus][]OrderStatus{
	OrderStatusPendingPayment: {OrderStatusPaid, OrderStatusCancelled},
	OrderStatusPaid:           {OrderStatusShipped, OrderStatusCancelled},
	OrderStatusShipped:        {OrderStatusCompleted},
	OrderStatusCompleted:      {},
	OrderStatusCancelled:      {},
}

// CanTransitionTo 判断订单状态是否允许迁移到目标状态。
func (s OrderStatus) CanTransitionTo(target OrderStatus) bool {
	return slices.Contains(orderTransitions[s], target)
}

// OperatorType 表示状态变更的发起方。
type OperatorType string

const (
	OperatorSystem OperatorType = "system"
	OperatorUser   OperatorType = "user"
	OperatorAdmin  OperatorType = "admin"
)

// AddressSnapshot 是下单时固化的收货地址快照，后续地址修改不影响历史订单。
type AddressSnapshot struct {
	Recipient string `json:"recipient"`
	Phone     string `json:"phone"`
	Province  string `json:"province"`
	City      string `json:"city"`
	District  string `json:"district"`
	Detail    string `json:"detail"`
}

// Order 表示一张订单。金额单位为分；创建后商品与价格以订单项快照为准。
type Order struct {
	ID             uint64          `json:"id" gorm:"primaryKey;autoIncrement"`
	OrderNo        string          `json:"order_no" gorm:"size:32;not null;uniqueIndex:uk_orders_order_no"`
	UserID         uint64          `json:"user_id" gorm:"not null;index:idx_orders_user_status,priority:1"`
	IdempotencyKey string          `json:"-" gorm:"column:idempotency_key;size:64;not null;uniqueIndex:uk_orders_user_idempotency"`
	RequestHash    string          `json:"-" gorm:"column:request_hash;size:64;not null"`
	Status         OrderStatus     `json:"status" gorm:"type:enum('pending_payment','paid','shipped','completed','cancelled');not null;default:pending_payment;index:idx_orders_user_status,priority:2;index:idx_orders_status_expires,priority:1"`
	TotalCent      int64           `json:"total_cent" gorm:"not null"`
	PayCent        int64           `json:"pay_cent" gorm:"not null"`
	DiscountCent   int64           `json:"discount_cent" gorm:"not null;default:0"`
	Address        AddressSnapshot `json:"address_snapshot" gorm:"column:address_snapshot;type:json;serializer:json;not null"`
	ExpiresAt      *time.Time      `json:"expires_at" gorm:"index:idx_orders_status_expires,priority:2"`
	PaidAt         *time.Time      `json:"paid_at"`
	ShippedAt      *time.Time      `json:"shipped_at"`
	CompletedAt    *time.Time      `json:"completed_at"`
	CancelledAt    *time.Time      `json:"cancelled_at"`
	Items          []OrderItem     `json:"items,omitempty" gorm:"foreignKey:OrderID"`
	CreatedAt      time.Time       `json:"created_at"`
	UpdatedAt      time.Time       `json:"updated_at"`
	DeletedAt      gorm.DeletedAt  `json:"-" gorm:"index"`
}

// TableName 显式指定 Order 对应的 MySQL 表名。
func (Order) TableName() string { return "orders" }

// ItemRefundStatus 表示订单项的退款进度。
type ItemRefundStatus string

const (
	ItemRefundNone     ItemRefundStatus = "none"
	ItemRefundPending  ItemRefundStatus = "pending"
	ItemRefundRefunded ItemRefundStatus = "refunded"
)

// OrderItem 表示订单中的一行商品，保存下单时的商品与价格快照。
type OrderItem struct {
	ID            uint64            `json:"id" gorm:"primaryKey;autoIncrement"`
	OrderID       uint64            `json:"order_id" gorm:"not null;index"`
	SKUID         uint64            `json:"sku_id" gorm:"column:sku_id;not null;index"`
	ProductName   string            `json:"product_name" gorm:"size:100;not null"`
	SKUCode       string            `json:"sku_code" gorm:"size:32;not null"`
	Specs         map[string]string `json:"specs" gorm:"type:json;serializer:json"`
	UnitPriceCent int64             `json:"unit_price_cent" gorm:"not null"`
	Quantity      int64             `json:"quantity" gorm:"not null"`
	SubtotalCent  int64             `json:"subtotal_cent" gorm:"not null"`
	RefundStatus  ItemRefundStatus  `json:"refund_status" gorm:"type:enum('none','pending','refunded');not null;default:none"`
	CreatedAt     time.Time         `json:"created_at"`
	UpdatedAt     time.Time         `json:"updated_at"`
	DeletedAt     gorm.DeletedAt    `json:"-" gorm:"index"`
}

// TableName 显式指定 OrderItem 对应的 MySQL 表名。
func (OrderItem) TableName() string { return "order_items" }

// OrderStatusLog 记录订单每次状态变化，形成可审计的状态历史。
type OrderStatusLog struct {
	ID           uint64       `json:"id" gorm:"primaryKey;autoIncrement"`
	OrderID      uint64       `json:"order_id" gorm:"not null;index"`
	FromStatus   OrderStatus  `json:"from_status" gorm:"size:20;not null"`
	ToStatus     OrderStatus  `json:"to_status" gorm:"size:20;not null"`
	OperatorType OperatorType `json:"operator_type" gorm:"type:enum('system','user','admin');not null"`
	OperatorID   uint64       `json:"operator_id" gorm:"not null;default:0"`
	Remark       string       `json:"remark" gorm:"size:200;not null;default:"`
	CreatedAt    time.Time    `json:"created_at"`
}

// TableName 显式指定 OrderStatusLog 对应的 MySQL 表名。
func (OrderStatusLog) TableName() string { return "order_status_logs" }
