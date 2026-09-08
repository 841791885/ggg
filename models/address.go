package models

import (
	"time"

	"gorm.io/gorm"
)

// Address 表示用户的收货地址。
// IsDefault 为 true 时该用户不允许存在第二条有效默认地址，
// 并发保证由 repository 层事务和数据库生成列唯一键共同完成。
type Address struct {
	ID        uint64         `json:"id" gorm:"primaryKey;autoIncrement"`
	UserID    uint64         `json:"user_id" gorm:"not null;index:idx_addresses_user_id"`
	Recipient string         `json:"recipient" gorm:"size:30;not null"`
	Phone     string         `json:"phone" gorm:"size:20;not null"`
	Province  string         `json:"province" gorm:"size:50;not null"`
	City      string         `json:"city" gorm:"size:50;not null"`
	District  string         `json:"district" gorm:"size:50;not null"`
	Detail    string         `json:"detail" gorm:"size:200;not null"`
	IsDefault bool           `json:"is_default" gorm:"not null;default:false"`
	CreatedAt time.Time      `json:"created_at"`
	UpdatedAt time.Time      `json:"updated_at"`
	DeletedAt gorm.DeletedAt `json:"-" gorm:"index"`
}

// TableName 显式指定 Address 对应的 MySQL 表名。
func (Address) TableName() string { return "addresses" }
