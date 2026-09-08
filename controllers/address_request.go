package controllers

// CreateAddressRequest 表示新增收货地址请求体。
// 不接受 user_id 和 is_default：归属来自 JWT，默认地址只能通过专用接口切换。
type CreateAddressRequest struct {
	Recipient string `json:"recipient"`
	Phone     string `json:"phone"`
	Province  string `json:"province"`
	City      string `json:"city"`
	District  string `json:"district"`
	Detail    string `json:"detail"`
}

// UpdateAddressRequest 表示修改收货地址请求体，nil 字段保持原值；is_default 会被忽略。
type UpdateAddressRequest struct {
	Recipient *string `json:"recipient"`
	Phone     *string `json:"phone"`
	Province  *string `json:"province"`
	City      *string `json:"city"`
	District  *string `json:"district"`
	Detail    *string `json:"detail"`
}
