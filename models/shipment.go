package models

import (
	"time"

	"gorm.io/gorm"
)

// Shipment 是一条发货记录（PRD-009）。
//
// ── 为什么是独立表而不是 orders 上加字段 ──
// 一个订单可能有多次发货（部分发货场景：A 现货先发、B 缺货后发）。
// 当前业务只会写一条，但表结构按一对多设计，将来支持拆单发货不用改 schema。
//
// ── 幂等怎么做 ──
// ShippingKey 由调用方（运营后台）提供，标识"这一批发货动作"。
// 唯一键 uk_shipments_order_key = (order_id, shipping_key, deleted_at)：
//
//	· 作用域限定在订单内 —— 不同订单各自发货，共用全局 key 空间没有业务意义
//	  （对照下单幂等键 uk_orders_user_idempotency 也是"作用域唯一"）
//	· 带 deleted_at —— 软删除的旧记录不再占用 key，允许"作废后重新发货"
type Shipment struct {
	ID      uint64 `json:"id" gorm:"primaryKey;autoIncrement"`
	OrderID uint64 `json:"order_id" gorm:"not null;uniqueIndex:uk_shipments_order_key,priority:1;index:idx_shipments_order"`
	// ShippingKey 存的是客户端传入值的 sha256 摘要（定长 hex），
	// 与下单幂等键同一套处理：客户端可能传任意字符串，入库前统一成定长安全值。
	// json:"-" 表示不返回给前端——它是内部去重凭证，暴露出去反而鼓励客户端复用。
	ShippingKey string `json:"-" gorm:"column:shipping_key;size:64;not null;uniqueIndex:uk_shipments_order_key,priority:2"`
	Carrier     string `json:"carrier" gorm:"size:50;not null"`
	TrackingNo  string `json:"tracking_no" gorm:"column:tracking_no;size:64;not null"`
	// ShippedBy 是发货操作人的用户 ID（运营账号），外键指向 users。
	ShippedBy uint64    `json:"shipped_by" gorm:"not null"`
	ShippedAt time.Time `json:"shipped_at" gorm:"not null"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
	// DeletedAt 必须是 gorm.DeletedAt 类型，GORM 才认它是软删除列：
	// 查询自动加 WHERE deleted_at IS NULL，Delete() 变成 UPDATE 打时间戳。
	// 写成 time.Time 这些行为全部失效——很隐蔽的坑。
	DeletedAt gorm.DeletedAt `json:"-" gorm:"index:uk_shipments_order_key,priority:3;index"`
}

// TableName 显式指定 Shipment 对应的 MySQL 表名。
// ⚠️ 必须有 (Shipment) 接收者：没有接收者的 func TableName() 只是普通函数，
// GORM 通过接口方法查找表名，写成普通函数等于没实现。
func (Shipment) TableName() string { return "shipments" }
