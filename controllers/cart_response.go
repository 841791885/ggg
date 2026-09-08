package controllers

import model "ggg/models"

// CartItemResponse 表示购物车明细的对外响应结构。
type CartItemResponse struct {
	ID       uint64 `json:"id"`
	CartID   uint64 `json:"cart_id"`
	SKUID    uint64 `json:"sku_id"`
	Quantity int64  `json:"quantity"`
	Selected bool   `json:"selected"`
}

// newCartItemResponse 将购物车明细模型转换为对外响应结构。
func newCartItemResponse(item *model.CartItem) CartItemResponse {
	return CartItemResponse{ID: item.ID, CartID: item.CartID, SKUID: item.SKUID, Quantity: item.Quantity, Selected: item.Selected}
}
