package controllers

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
