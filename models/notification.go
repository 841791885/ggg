package models

import (
	"time"

	"gorm.io/gorm"
)

// Notification 表示站内通知。支付成功、发货、退款等事件产生，读取时置 read_at。
type Notification struct {
	ID        uint64         `json:"id" gorm:"primaryKey;autoIncrement"`
	UserID    uint64         `json:"user_id" gorm:"not null;index:idx_notifications_user_read,priority:1"`
	Type      string         `json:"type" gorm:"size:30;not null"`
	Title     string         `json:"title" gorm:"size:50;not null"`
	Content   string         `json:"content" gorm:"size:500;not null;default:"`
	ReadAt    *time.Time     `json:"read_at" gorm:"index:idx_notifications_user_read,priority:2"`
	CreatedAt time.Time      `json:"created_at"`
	UpdatedAt time.Time      `json:"updated_at"`
	DeletedAt gorm.DeletedAt `json:"-" gorm:"index"`
}

// TableName 显式指定 Notification 对应的 MySQL 表名。
func (Notification) TableName() string { return "notifications" }
