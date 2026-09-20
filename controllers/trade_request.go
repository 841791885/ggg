package controllers

import "time"

// CreateOrderRequest 表示创建订单请求体。
// 只接受地址 ID：商品、数量和金额全部由服务端从购物车与 SKU 重新计算，杜绝前端篡改价格。
type CreateOrderRequest struct {
	AddressID uint64 `json:"address_id"`
}

// ApplyRefundRequest 表示申请退款请求体。
type ApplyRefundRequest struct {
	AmountCent int64  `json:"amount_cent"`
	Reason     string `json:"reason"`
}

// ReviewRequest 表示审核评价显隐请求体（运营端）。
type SetReviewVisibilityRequest struct {
	Visible bool `json:"visible"`
}

// CreateCouponTemplateRequest 表示创建优惠券模板请求体。
type CreateCouponTemplateRequest struct {
	Name          string    `json:"name"`
	ThresholdCent int64     `json:"threshold_cent"`
	DiscountCent  int64     `json:"discount_cent"`
	TotalCount    int       `json:"total_count"`
	StartsAt      time.Time `json:"starts_at"`
	EndsAt        time.Time `json:"ends_at"`
}

// UpdateCouponTemplateRequest 表示更新优惠券模板请求体，nil 字段保持原值。
type UpdateCouponTemplateRequest struct {
	Name          *string    `json:"name"`
	ThresholdCent *int64     `json:"threshold_cent"`
	DiscountCent  *int64     `json:"discount_cent"`
	TotalCount    *int       `json:"total_count"`
	Remaining     *int       `json:"remaining"`
	StartsAt      *time.Time `json:"starts_at"`
	EndsAt        *time.Time `json:"ends_at"`
	Status        *string    `json:"status"`
}

// CreateNotificationRequest 表示生成站内通知请求体（本阶段供联调造数使用）。
type CreateNotificationRequest struct {
	Type    string `json:"type"`
	Title   string `json:"title"`
	Content string `json:"content"`
}

// ShipOrderRequest 表示运营发货的请求体（PRD-009 A7）。
// ShippingKey 是幂等键：客户端为"这一批发货动作"生成的唯一标识，
// 重试时复用同一个 key，服务端据此返回首次结果而不重复发货。
type ShipOrderRequest struct {
	ShippingKey string `json:"shipping_key"`
	Carrier     string `json:"carrier"`
	TrackingNo  string `json:"tracking_no"`
}
