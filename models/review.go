package models

import (
	"time"

	"gorm.io/gorm"
)

// Review 表示消费者对已完成订单项的评价。一个订单项只能评价一次；
// 运营隐藏后消费者不可见，但记录与操作人保留用于审计。
type Review struct {
	ID          uint64         `json:"id" gorm:"primaryKey;autoIncrement"`
	OrderItemID uint64         `json:"order_item_id" gorm:"not null;uniqueIndex:uk_reviews_order_item"`
	ProductID   uint64         `json:"product_id" gorm:"not null;index:idx_reviews_product_visible,priority:1"`
	UserID      uint64         `json:"user_id" gorm:"not null;index"`
	Rating      int            `json:"rating" gorm:"not null;check:chk_reviews_rating_range,rating BETWEEN 1 AND 5"`
	Content     string         `json:"content" gorm:"size:500;not null;default:"`
	Visible     bool           `json:"visible" gorm:"not null;default:true;index:idx_reviews_product_visible,priority:2"`
	HiddenBy    *uint64        `json:"hidden_by"`
	CreatedAt   time.Time      `json:"created_at"`
	UpdatedAt   time.Time      `json:"updated_at"`
	DeletedAt   gorm.DeletedAt `json:"-" gorm:"index"`
}

// TableName 显式指定 Review 对应的 MySQL 表名。
func (Review) TableName() string { return "reviews" }
