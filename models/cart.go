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
	ID       uint64 `json:"id" gorm:"primaryKey;autoIncrement"`
	CartID   uint64 `json:"cart_id" gorm:"not null;index;uniqueIndex:uk_cart_items_cart_sku"`
	SKUID    uint64 `json:"sku_id" gorm:"column:sku_id;not null;index;uniqueIndex:uk_cart_items_cart_sku"`
	Quantity int64  `json:"quantity" gorm:"not null;check:chk_cart_items_quantity_positive,quantity > 0"`
	// Selected 表示该明细是否纳入下单预览；新加入的商品默认选中。
	Selected bool `json:"selected" gorm:"not null;default:true"`
	// 以下字段不落库（gorm:"-"），由 CartService 查询后填充给前端展示：
	// 购物车页需要展示商品名/单价/库存与可购买原因，这些数据属于 SKU/商品表，
	// 让 service 聚合一次，比前端逐条再查（N+1）高效得多。
	ProductID     uint64         `json:"product_id" gorm:"-"`
	ProductName   string         `json:"product_name" gorm:"-"`
	SKUCode       string         `json:"sku_code" gorm:"-"`
	UnitPriceCent int64          `json:"price_cent" gorm:"-"`
	Stock         int64          `json:"stock" gorm:"-"`
	Purchasable   bool           `json:"purchasable" gorm:"-"`
	Reason        string         `json:"reason,omitempty" gorm:"-"`
	CreatedAt     time.Time      `json:"created_at"`
	UpdatedAt     time.Time      `json:"updated_at"`
	DeletedAt     gorm.DeletedAt `json:"-" gorm:"index"`
}

// TableName 显式指定 CartItem 对应的 MySQL 表名。
func (CartItem) TableName() string { return "cart_items" }
