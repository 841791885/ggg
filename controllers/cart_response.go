package controllers

import model "ggg/models"

// CartItemResponse 表示购物车明细的对外响应结构。
type CartItemResponse struct {
	ID       uint64 `json:"id"`
	CartID   uint64 `json:"cart_id"`
	SKUID    uint64 `json:"sku_id"`
	Quantity int64  `json:"quantity"`
	Selected bool   `json:"selected"`
	// 以下为服务端补齐的展示信息（购物车页要显示"什么商品、多少钱、还有货吗"，
	// 只给 sku_id 的话前端只能再逐个查——把聚合放在服务端是唯一正确的位置）。
	ProductID     uint64 `json:"product_id"`
	ProductName   string `json:"product_name"`
	SKUCode       string `json:"sku_code"`
	UnitPriceCent int64  `json:"price_cent"`
	Stock         int64  `json:"stock"`
	Purchasable   bool   `json:"purchasable"`
	Reason        string `json:"reason,omitempty"`
}

// newCartItemResponse 将购物车明细模型转换为对外响应结构（展示字段由 service 预填在模型上）。
func newCartItemResponse(item *model.CartItem) CartItemResponse {
	return CartItemResponse{
		ID: item.ID, CartID: item.CartID, SKUID: item.SKUID, Quantity: item.Quantity, Selected: item.Selected,
		ProductID: item.ProductID, ProductName: item.ProductName, SKUCode: item.SKUCode,
		UnitPriceCent: item.UnitPriceCent, Stock: item.Stock, Purchasable: item.Purchasable, Reason: item.Reason,
	}
}
