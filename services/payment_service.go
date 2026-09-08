package services

import (
	"context"
	"fmt"

	model "ggg/models"
	"ggg/repositories"
)

// PaymentService 负责支付单创建与查询。
// 金额完全由订单生成（PRD-007 核心规则）。
// TODO(PRD-007 进阶): HMAC 回调验签、事件号幂等消费、支付成功推进订单状态需同一事务（见 docs/ADVANCED-TASKS.md A2）。
type PaymentService struct{ repository repositories.Repository }

// NewPaymentService 创建支付业务服务。
func NewPaymentService(repository repositories.Repository) *PaymentService {
	return &PaymentService{repository: repository}
}

// CreateForOrder 为待支付订单创建模拟渠道支付单；同一订单同时只允许一张待支付单。
func (s *PaymentService) CreateForOrder(ctx context.Context, userID, orderID uint64) (model.Payment, error) {
	if userID == 0 {
		return model.Payment{}, model.ErrInvalidUserID
	}
	// 目的：先校验订单归属和状态，防止为他人订单或已完成订单创建支付单。
	order, err := s.repository.GetOrderByID(ctx, userID, orderID)
	if err != nil {
		return model.Payment{}, fmt.Errorf("查询订单：%w", err)
	}
	if order.Status != model.OrderStatusPendingPayment {
		return model.Payment{}, model.ErrInvalidOrderTransition
	}
	// 已有未过期的待支付单时直接复用，避免重复创建造成对账歧义。
	if active, err := s.repository.GetActivePaymentByOrder(ctx, userID, orderID); err == nil {
		return active, nil
	} else if !isModelNotFound(err) {
		return model.Payment{}, fmt.Errorf("查询待支付支付单：%w", err)
	}
	payment := model.Payment{
		PaymentNo:  newBizNo(),
		OrderID:    order.ID,
		UserID:     userID,
		AmountCent: order.PayCent, // 目的：金额只能来自订单快照，前端无法篡改。
		Currency:   "CNY",
		Channel:    "mock",
		Status:     model.PaymentPending,
	}
	created, err := s.repository.CreatePayment(ctx, payment)
	if err != nil {
		return model.Payment{}, fmt.Errorf("创建支付单：%w", err)
	}
	return created, nil
}

// GetByPaymentNo 查询当前用户自己的支付单。
func (s *PaymentService) GetByPaymentNo(ctx context.Context, userID uint64, paymentNo string) (model.Payment, error) {
	if userID == 0 {
		return model.Payment{}, model.ErrInvalidUserID
	}
	payment, err := s.repository.GetPaymentByNo(ctx, userID, paymentNo)
	if err != nil {
		return model.Payment{}, fmt.Errorf("查询支付单：%w", err)
	}
	return payment, nil
}
