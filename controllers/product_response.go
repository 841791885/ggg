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
