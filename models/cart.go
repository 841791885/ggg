package models

import (
	"time"

	"gorm.io/gorm"
)

// Cart 表示一个用户的购物车。
// 当前项目还没有用户模块，UserID 暂时使用固定测试用户 ID。
type Cart struct {
	ID        uint64         `json:"id" gorm:"primaryKey;autoIncrement"`
	UserID    uint64         `json:"user_id" gorm:"not null;uniqueIndex:uk_carts_user_id"`
	Items     []CartItem     `json:"items,omitempty" gorm:"foreignKey:CartID"`
	CreatedAt time.Time      `json:"created_at"`
	UpdatedAt time.Time      `json:"updated_at"`
	DeletedAt gorm.DeletedAt `json:"-" gorm:"index"`
}

// TableName 显式指定 Cart 对应的 MySQL 表名。
func (Cart) TableName() string { return "carts" }

// CartItem 表示购物车中的一个 SKU 及其购买数量。
type CartItem struct {
	ID        uint64         `json:"id" gorm:"primaryKey;autoIncrement"`
	CartID    uint64         `json:"cart_id" gorm:"not null;index;uniqueIndex:uk_cart_items_cart_sku"`
	SKUID     uint64         `json:"sku_id" gorm:"column:sku_id;not null;index;uniqueIndex:uk_cart_items_cart_sku"`
	Quantity  int64          `json:"quantity" gorm:"not null;check:chk_cart_items_quantity_positive,quantity > 0"`
	CreatedAt time.Time      `json:"created_at"`
	UpdatedAt time.Time      `json:"updated_at"`
	DeletedAt gorm.DeletedAt `json:"-" gorm:"index"`
}

// TableName 显式指定 CartItem 对应的 MySQL 表名。
func (CartItem) TableName() string { return "cart_items" }
