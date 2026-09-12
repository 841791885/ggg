package models

import (
	"time"

	"gorm.io/gorm"
)

// StockMovementType 表示一次库存变动的业务类型。
type StockMovementType string

const (
	// StockMovementReserve 下单预占：change_qty 为负。
	StockMovementReserve StockMovementType = "reserve"
	// StockMovementRelease 取消/关单释放：change_qty 为正。
	StockMovementRelease StockMovementType = "release"
)

// StockMovement 是一条库存流水凭证：谁（订单号）、因为什么（类型）、动了多少（带符号）、动完剩多少（快照）。
// 与扣减/释放语句在同一事务内写入，保证"有变动必有流水、回滚则流水同灭"。
type StockMovement struct {
	ID           uint64            `json:"id" gorm:"primaryKey;autoIncrement"`
	SKUID        uint64            `json:"sku_id" gorm:"column:sku_id;not null;index:idx_stock_movements_sku_created,priority:1"`
	ChangeQty    int64             `json:"change_qty" gorm:"column:change_qty;not null"`
	BalanceStock int64             `json:"balance_stock" gorm:"not null"`
	Type         StockMovementType `json:"type" gorm:"type:enum('reserve','release');not null"`
	OrderNo      string            `json:"order_no" gorm:"size:32;not null;index"`
	CreatedAt    time.Time         `json:"created_at" gorm:"index:idx_stock_movements_sku_created,priority:2"`
	DeletedAt    gorm.DeletedAt    `json:"-" gorm:"index"`
}

// TableName 显式指定 StockMovement 对应的 MySQL 表名。
func (StockMovement) TableName() string { return "stock_movements" }
