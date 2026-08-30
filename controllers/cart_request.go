package controllers

// AddCartItemRequest 表示加入购物车请求体。
type AddCartItemRequest struct {
	UserID   uint64 `json:"user_id"`
	SKUID    uint64 `json:"sku_id"`
	Quantity int64  `json:"quantity"`
}
