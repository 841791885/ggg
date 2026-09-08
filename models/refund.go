package models

import (
	"time"

	"gorm.io/gorm"
)

// RefundStatus 表示退款单状态：申请后等待运营审核，通过则进入模拟退款。
type RefundStatus string

const (
	RefundPending  RefundStatus = "pending"
	RefundApproved RefundStatus = "approved"
	RefundRejected RefundStatus = "rejected"
	Refunded       RefundStatus = "refunded"
)

// Refund 表示按订单项发起的退款单。金额不超过该订单项实付小计。
type Refund struct {
	ID          uint64         `json:"id" gorm:"primaryKey;autoIncrement"`
	RefundNo    string         `json:"refund_no" gorm:"size:32;not null;uniqueIndex:uk_refunds_refund_no"`
	OrderID     uint64         `json:"order_id" gorm:"not null;index"`
	OrderItemID uint64         `json:"order_item_id" gorm:"not null;uniqueIndex:uk_refunds_item_pending,priority:1"`
	UserID      uint64         `json:"user_id" gorm:"not null;index"`
	AmountCent  int64          `json:"amount_cent" gorm:"not null"`
	Reason      string         `json:"reason" gorm:"size:200;not null;default:"`
	Status      RefundStatus   `json:"status" gorm:"type:enum('pending','approved','rejected','refunded');not null;default:pending;uniqueIndex:uk_refunds_item_pending,priority:2"`
	ReviewedBy  *uint64        `json:"reviewed_by"`
	ReviewedAt  *time.Time     `json:"reviewed_at"`
	CreatedAt   time.Time      `json:"created_at"`
	UpdatedAt   time.Time      `json:"updated_at"`
	DeletedAt   gorm.DeletedAt `json:"-" gorm:"index"`
}

// TableName 显式指定 Refund 对应的 MySQL 表名。
func (Refund) TableName() string { return "refunds" }
