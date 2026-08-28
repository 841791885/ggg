package models

import (
	"time"

	"gorm.io/gorm"
)

// ProductStatus 表示商品当前的销售状态。
type ProductStatus string

const (
	ProductStatusDraft   ProductStatus = "draft"
	ProductStatusOnSale  ProductStatus = "on_sale"
	ProductStatusOffSale ProductStatus = "off_sale"
)

// SKUStatus 表示 SKU 当前是否可参与销售。
type SKUStatus string

const (
	SKUStatusActive   SKUStatus = "active"
	SKUStatusInactive SKUStatus = "inactive"
)

// Product 表示商城中的商品基础信息。
//
// json tag 决定 HTTP JSON 的字段名；gorm tag 描述数据库主键、索引和约束。
// DeletedAt 使用 GORM 的特殊类型后，普通查询会自动排除已软删除的数据。
type Product struct {
	ID          uint64         `json:"id" gorm:"primaryKey;autoIncrement"`
	Name        string         `json:"name" gorm:"size:100;not null"`
	Description string         `json:"description" gorm:"size:2000;not null;default:''"`
	Status      ProductStatus  `json:"status" gorm:"type:enum('draft','on_sale','off_sale');not null;index:idx_products_status_created"`
	CreatedAt   time.Time      `json:"created_at" gorm:"index:idx_products_status_created"`
	UpdatedAt   time.Time      `json:"updated_at"`
	DeletedAt   gorm.DeletedAt `json:"-" gorm:"index"`
}

// TableName 显式指定 Product 对应的 MySQL 表名。
func (Product) TableName() string {
	return "products"
}

// SKU 表示可以独立定价和计库存的最小销售单元。
// Specs 使用 JSON 序列化保存规格，例如 {"颜色":"黑色","容量":"256GB"}。
type SKU struct {
	ID        uint64            `json:"id" gorm:"primaryKey;autoIncrement"`
	ProductID uint64            `json:"product_id" gorm:"not null;index:idx_skus_product_created"`
	Code      string            `json:"code" gorm:"size:32;not null;uniqueIndex:uk_skus_code"`
	Specs     map[string]string `json:"specs" gorm:"type:json;serializer:json;not null"`
	PriceCent int64             `json:"price_cent" gorm:"not null;check:chk_skus_price_positive,price_cent > 0"`
	Stock     int64             `json:"stock" gorm:"not null;check:chk_skus_stock_non_negative,stock >= 0"`
	Status    SKUStatus         `json:"status" gorm:"type:enum('active','inactive');not null"`
	CreatedAt time.Time         `json:"created_at" gorm:"index:idx_skus_product_created"`
	UpdatedAt time.Time         `json:"updated_at"`
	DeletedAt gorm.DeletedAt    `json:"-" gorm:"index"`
}

// TableName 显式指定 SKU 对应的 MySQL 表名。
func (SKU) TableName() string {
	return "skus"
}
