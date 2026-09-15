package controllers

import model "ggg/models"

// CreateProductRequest 表示创建商品接口允许客户端提交的 JSON 字段。
//
// 请求中没有 status，防止客户端绕过 Service 直接创建上架商品。
type CreateProductRequest struct {
	Name        string `json:"name"`
	Description string `json:"description"`
}

type UpdateProductRequest struct {
	Name        *string `json:"name"`
	Description *string `json:"description"`
}

// ListProductsQuery 表示商品列表接口的可选 Query 参数。
// 使用指针可以区分“客户端没有传”和“客户端明确传了 0”。
type ListProductsQuery struct {
	Page     *int    `form:"page"`
	PageSize *int    `form:"page_size"`
	Name     *string `form:"name"`
}

// UpdateProductStatusRequest 表示修改商品销售状态的请求体。
type UpdateProductStatusRequest struct {
	Status model.ProductStatus `json:"status"`
}

type CreateSKURequest struct {
	Code      string            `json:"code"`
	Specs     map[string]string `json:"specs"`
	PriceCent int64             `json:"price_cent"`
	Stock     int64             `json:"stock"`
}
type UpdateSKURequest struct {
	Code      string            `json:"code"`
	Specs     map[string]string `json:"specs"`
	PriceCent int64             `json:"price_cent"`
	Stock     int64             `json:"stock"`
}

// UpdateSKUStatusRequest 表示修改 SKU 启用状态的请求体。
type UpdateSKUStatusRequest struct {
	Status model.SKUStatus `json:"status"`
}

type ListSKUQuery struct {
	Page     *int    `form:"page"`
	PageSize *int    `form:"page_size"`
	Name     *string `form:"name"`
}
