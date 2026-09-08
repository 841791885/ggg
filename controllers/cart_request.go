package controllers

// AddCartItemRequest 表示加入购物车请求体。
type AddCartItemRequest struct {
	SKUID    uint64 `json:"sku_id"`
	Quantity int64  `json:"quantity"`
}

// UpdateCartItemRequest 表示修改购物车商品数量的请求体。
type UpdateCartItemRequest struct {
	Quantity int64 `json:"quantity"`
}

// UpdateCartSelectionRequest 表示整组替换购物车选中状态的请求体；item_ids 为期望选中的明细集合。
type UpdateCartSelectionRequest struct {
	ItemIDs []uint64 `json:"item_ids"`
}
