package models

import (
	"time"

	"gorm.io/gorm"
)

// PaymentStatus 表示支付单状态。
type PaymentStatus string

const (
	PaymentPending PaymentStatus = "pending"
	PaymentSuccess PaymentStatus = "success"
	PaymentFailed  PaymentStatus = "failed"
	PaymentClosed  PaymentStatus = "closed"
)

// Payment 表示一张支付单。金额由订单生成，不接受前端指定值。
// EventNo 保存渠道事件号，唯一约束保证重复回调只处理一次。
type Payment struct {
	ID         uint64         `json:"id" gorm:"primaryKey;autoIncrement"`
	PaymentNo  string         `json:"payment_no" gorm:"size:32;not null;uniqueIndex:uk_payments_payment_no"`
	OrderID    uint64         `json:"order_id" gorm:"not null;index"`
	UserID     uint64         `json:"user_id" gorm:"not null"`
	AmountCent int64          `json:"amount_cent" gorm:"not null"`
	Currency   string         `json:"currency" gorm:"size:8;not null;default:CNY"`
	Channel    string         `json:"channel" gorm:"size:20;not null;default:mock"`
	Status     PaymentStatus  `json:"status" gorm:"type:enum('pending','success','failed','closed');not null;default:pending"`
	EventNo    *string        `json:"event_no" gorm:"size:64;uniqueIndex:uk_payments_event_no"`
	PaidAt     *time.Time     `json:"paid_at"`
	CreatedAt  time.Time      `json:"created_at"`
	UpdatedAt  time.Time      `json:"updated_at"`
	DeletedAt  gorm.DeletedAt `json:"-" gorm:"index"`
}

// TableName 显式指定 Payment 对应的 MySQL 表名。
func (Payment) TableName() string { return "payments" }

// PaymentCallbackLog 保存支付渠道回调的原始报文，供对账和排障。
type PaymentCallbackLog struct {
	ID        uint64         `json:"id" gorm:"primaryKey;autoIncrement"`
	PaymentNo string         `json:"payment_no" gorm:"size:32;not null;index"`
	EventNo   string         `json:"event_no" gorm:"size:64;not null"`
	Result    string         `json:"result" gorm:"size:20;not null"`
	Payload   map[string]any `json:"payload" gorm:"type:json;serializer:json;not null"`
	Processed bool           `json:"processed" gorm:"not null;default:false"`
	CreatedAt time.Time      `json:"created_at"`
}

// TableName 显式指定 PaymentCallbackLog 对应的 MySQL 表名。
func (PaymentCallbackLog) TableName() string { return "payment_callback_logs" }
