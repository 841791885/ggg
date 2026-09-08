package controllers

import (
	"time"

	model "ggg/models"
)

// OrderResponse 表示订单的对外响应结构，隐藏幂等键等内部字段。
type OrderResponse struct {
	ID           uint64                `json:"id"`
	OrderNo      string                `json:"order_no"`
	UserID       uint64                `json:"user_id"`
	Status       model.OrderStatus     `json:"status"`
	TotalCent    int64                 `json:"total_cent"`
	PayCent      int64                 `json:"pay_cent"`
	DiscountCent int64                 `json:"discount_cent"`
	Address      model.AddressSnapshot `json:"address_snapshot"`
	Items        []OrderItemResponse   `json:"items"`
	ExpiresAt    *time.Time            `json:"expires_at"`
	PaidAt       *time.Time            `json:"paid_at"`
	ShippedAt    *time.Time            `json:"shipped_at"`
	CompletedAt  *time.Time            `json:"completed_at"`
	CancelledAt  *time.Time            `json:"cancelled_at"`
	CreatedAt    time.Time             `json:"created_at"`
}

// OrderItemResponse 表示订单项快照的对外响应结构。
type OrderItemResponse struct {
	ID            uint64            `json:"id"`
	SKUID         uint64            `json:"sku_id"`
	ProductName   string            `json:"product_name"`
	SKUCode       string            `json:"sku_code"`
	Specs         map[string]string `json:"specs,omitempty"`
	UnitPriceCent int64             `json:"unit_price_cent"`
	Quantity      int64             `json:"quantity"`
	SubtotalCent  int64             `json:"subtotal_cent"`
	RefundStatus  string            `json:"refund_status"`
}

// newOrderResponse 将订单模型转换为对外响应结构。
func newOrderResponse(order *model.Order) OrderResponse {
	items := make([]OrderItemResponse, len(order.Items))
	for i, item := range order.Items {
		items[i] = OrderItemResponse{
			ID: item.ID, SKUID: item.SKUID, ProductName: item.ProductName, SKUCode: item.SKUCode,
			Specs: item.Specs, UnitPriceCent: item.UnitPriceCent, Quantity: item.Quantity,
			SubtotalCent: item.SubtotalCent, RefundStatus: string(item.RefundStatus),
		}
	}
	return OrderResponse{
		ID: order.ID, OrderNo: order.OrderNo, UserID: order.UserID, Status: order.Status,
		TotalCent: order.TotalCent, PayCent: order.PayCent, DiscountCent: order.DiscountCent,
		Address: order.Address, Items: items, ExpiresAt: order.ExpiresAt, PaidAt: order.PaidAt,
		ShippedAt: order.ShippedAt, CompletedAt: order.CompletedAt, CancelledAt: order.CancelledAt,
		CreatedAt: order.CreatedAt,
	}
}

// newOrderResponses 批量转换订单列表。
func newOrderResponses(orders []model.Order) []OrderResponse {
	result := make([]OrderResponse, len(orders))
	for i := range orders {
		result[i] = newOrderResponse(&orders[i])
	}
	return result
}

// PaymentResponse 表示支付单的对外响应结构。
type PaymentResponse struct {
	ID         uint64              `json:"id"`
	PaymentNo  string              `json:"payment_no"`
	OrderID    uint64              `json:"order_id"`
	AmountCent int64               `json:"amount_cent"`
	Currency   string              `json:"currency"`
	Channel    string              `json:"channel"`
	Status     model.PaymentStatus `json:"status"`
	PaidAt     *time.Time          `json:"paid_at"`
	CreatedAt  time.Time           `json:"created_at"`
}

// newPaymentResponse 将支付单模型转换为对外响应结构。
func newPaymentResponse(payment *model.Payment) PaymentResponse {
	return PaymentResponse{
		ID: payment.ID, PaymentNo: payment.PaymentNo, OrderID: payment.OrderID, AmountCent: payment.AmountCent,
		Currency: payment.Currency, Channel: payment.Channel, Status: payment.Status, PaidAt: payment.PaidAt,
		CreatedAt: payment.CreatedAt,
	}
}

// RefundResponse 表示退款单的对外响应结构。
type RefundResponse struct {
	ID          uint64             `json:"id"`
	RefundNo    string             `json:"refund_no"`
	OrderID     uint64             `json:"order_id"`
	OrderItemID uint64             `json:"order_item_id"`
	AmountCent  int64              `json:"amount_cent"`
	Reason      string             `json:"reason"`
	Status      model.RefundStatus `json:"status"`
	ReviewedBy  *uint64            `json:"reviewed_by"`
	ReviewedAt  *time.Time         `json:"reviewed_at"`
	CreatedAt   time.Time          `json:"created_at"`
}

// newRefundResponse 将退款单模型转换为对外响应结构。
func newRefundResponse(refund *model.Refund) RefundResponse {
	return RefundResponse{
		ID: refund.ID, RefundNo: refund.RefundNo, OrderID: refund.OrderID, OrderItemID: refund.OrderItemID,
		AmountCent: refund.AmountCent, Reason: refund.Reason, Status: refund.Status,
		ReviewedBy: refund.ReviewedBy, ReviewedAt: refund.ReviewedAt, CreatedAt: refund.CreatedAt,
	}
}

// CouponTemplateResponse 表示优惠券模板的对外响应结构。
type CouponTemplateResponse struct {
	ID            uint64                     `json:"id"`
	Name          string                     `json:"name"`
	ThresholdCent int64                      `json:"threshold_cent"`
	DiscountCent  int64                      `json:"discount_cent"`
	TotalCount    int                        `json:"total_count"`
	Remaining     int                        `json:"remaining"`
	PerUserLimit  int                        `json:"per_user_limit"`
	StartsAt      time.Time                  `json:"starts_at"`
	EndsAt        time.Time                  `json:"ends_at"`
	Status        model.CouponTemplateStatus `json:"status"`
	CreatedAt     time.Time                  `json:"created_at"`
}

// newCouponTemplateResponse 将优惠券模板模型转换为对外响应结构。
func newCouponTemplateResponse(template *model.CouponTemplate) CouponTemplateResponse {
	return CouponTemplateResponse{
		ID: template.ID, Name: template.Name, ThresholdCent: template.ThresholdCent, DiscountCent: template.DiscountCent,
		TotalCount: template.TotalCount, Remaining: template.Remaining, PerUserLimit: template.PerUserLimit,
		StartsAt: template.StartsAt, EndsAt: template.EndsAt, Status: template.Status, CreatedAt: template.CreatedAt,
	}
}

// UserCouponResponse 表示用户持有券的对外响应结构。
type UserCouponResponse struct {
	ID         uint64                 `json:"id"`
	TemplateID uint64                 `json:"template_id"`
	Status     model.UserCouponStatus `json:"status"`
	ClaimedAt  time.Time              `json:"claimed_at"`
	UsedAt     *time.Time             `json:"used_at"`
}

// newUserCouponResponse 将用户优惠券模型转换为对外响应结构。
func newUserCouponResponse(coupon *model.UserCoupon) UserCouponResponse {
	return UserCouponResponse{ID: coupon.ID, TemplateID: coupon.TemplateID, Status: coupon.Status, ClaimedAt: coupon.ClaimedAt, UsedAt: coupon.UsedAt}
}

// ReviewResponse 表示评价的对外响应结构。
type ReviewResponse struct {
	ID          uint64    `json:"id"`
	OrderItemID uint64    `json:"order_item_id"`
	ProductID   uint64    `json:"product_id"`
	Rating      int       `json:"rating"`
	Content     string    `json:"content"`
	Visible     bool      `json:"visible"`
	CreatedAt   time.Time `json:"created_at"`
}

// newReviewResponse 将评价模型转换为对外响应结构。
func newReviewResponse(review *model.Review) ReviewResponse {
	return ReviewResponse{
		ID: review.ID, OrderItemID: review.OrderItemID, ProductID: review.ProductID,
		Rating: review.Rating, Content: review.Content, Visible: review.Visible, CreatedAt: review.CreatedAt,
	}
}

// NotificationResponse 表示站内通知的对外响应结构。
type NotificationResponse struct {
	ID        uint64     `json:"id"`
	Type      string     `json:"type"`
	Title     string     `json:"title"`
	Content   string     `json:"content"`
	Read      bool       `json:"read"`
	CreatedAt time.Time  `json:"created_at"`
	ReadAt    *time.Time `json:"read_at"`
}

// newNotificationResponse 将通知模型转换为对外响应结构；Read 由 read_at 是否有值派生。
func newNotificationResponse(notification *model.Notification) NotificationResponse {
	return NotificationResponse{
		ID: notification.ID, Type: notification.Type, Title: notification.Title, Content: notification.Content,
		Read: notification.ReadAt != nil, CreatedAt: notification.CreatedAt, ReadAt: notification.ReadAt,
	}
}

// TaskResponse 表示后台任务的对外响应结构，隐藏软删除字段。
type TaskResponse struct {
	ID              uint64           `json:"id"`
	TaskType        string           `json:"task_type"`
	Payload         map[string]any   `json:"payload"`
	Status          model.TaskStatus `json:"status"`
	Attempts        int              `json:"attempts"`
	MaxAttempts     int              `json:"max_attempts"`
	NextRunAt       time.Time        `json:"next_run_at"`
	LastError       string           `json:"last_error"`
	RetryOperatorID *uint64          `json:"retry_operator_id"`
	CreatedAt       time.Time        `json:"created_at"`
}

// newTaskResponse 将后台任务模型转换为对外响应结构。
func newTaskResponse(task *model.BackgroundTask) TaskResponse {
	return TaskResponse{
		ID: task.ID, TaskType: task.TaskType, Payload: task.Payload, Status: task.Status,
		Attempts: task.Attempts, MaxAttempts: task.MaxAttempts, NextRunAt: task.NextRunAt,
		LastError: task.LastError, RetryOperatorID: task.RetryOperatorID, CreatedAt: task.CreatedAt,
	}
}
