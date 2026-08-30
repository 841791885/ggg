package controllers

// AddCartItemRequest 表示加入购物车请求体。
type AddCartItemRequest struct {
	SKUID    uint64 `json:"sku_id"`
	Quantity int64  `json:"quantity"`
}
