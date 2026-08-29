package controllers

import (
	"time"

	model "ggg/models"
)

// ProductResponse 表示商品接口对外返回的数据。
type ProductResponse struct {
	ID          uint64              `json:"id"`
	Name        string              `json:"name"`
	Description string              `json:"description"`
	Status      model.ProductStatus `json:"status"`
	CreatedAt   time.Time           `json:"created_at"`
	UpdatedAt   time.Time           `json:"updated_at"`
}

type SKUResponse struct {
	ID        uint64            `json:"id"`
	ProductID uint64            `json:"product_id"`
	Code      string            `json:"code"`
	Specs     map[string]string `json:"specs"`
	PriceCent int64             `json:"price_cent"`
	Stock     int64             `json:"stock"`
	Status    model.SKUStatus   `json:"status"`
	CreatedAt time.Time         `json:"created_at"`
	UpdatedAt time.Time         `json:"updated_at"`
}

func newProductResponse(product *model.Product) ProductResponse {
	return ProductResponse{
		ID:          product.ID,
		Name:        product.Name,
		Description: product.Description,
		Status:      product.Status,
		CreatedAt:   product.CreatedAt,
		UpdatedAt:   product.UpdatedAt,
	}
}

func newSKUResponse(sku *model.SKU) SKUResponse {
	return SKUResponse{
		ID:        sku.ID,
		ProductID: sku.ProductID,
		Code:      sku.Code,
		Specs:     sku.Specs,
		PriceCent: sku.PriceCent,
		Stock:     sku.Stock,
		Status:    sku.Status,
		CreatedAt: sku.CreatedAt,
		UpdatedAt: sku.UpdatedAt,
	}
}
