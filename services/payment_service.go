package services

import (
	"context"
	"fmt"
	"time"

	model "ggg/models"
	"ggg/repositories"
)

// PaymentService 负责支付单创建、查询与渠道回调消费。
// 金额完全由订单生成（PRD-007 核心规则）。
type PaymentService struct {
	repository     repositories.PaymentStore
	callbackSecret string // HMAC 验签密钥，启动时从配置注入
}

// NewPaymentService 创建支付业务服务。
func NewPaymentService(repository repositories.PaymentStore, callbackSecret string) *PaymentService {
	return &PaymentService{repository: repository, callbackSecret: callbackSecret}
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
	// ⚠️ 已知限制（A3/A10 处理）：并发创建支付单目前无防护——"查重→插入"之间有窗口，
	// 两个请求可能各建一张 pending 单；payments 表没有 (order_id,status) 唯一键可兜底，
	// 后续用超时关单任务清理多余 pending 单，或引入活跃单唯一约束。
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

// HandleCallback 消费模拟渠道的异步支付回调（PRD-007 进阶 A2）。
// 顺序即安全边界：①验签（挡伪造）→ ②报文与支付单交叉校验（挡篡改/张冠李戴）→ ③事务内幂等消费。
// 任何一步失败都不会产生业务效果；重复回调在第三步收敛为"确认成功但零副作用"。
func (s *PaymentService) HandleCallback(ctx context.Context, callback MockPaymentCallback) (model.Payment, error) {
	// ① 先验签再解析信任：签名不对连业务都不看（PRD-007 核心规则第 4 条）。
	if !VerifyMockCallbackSignature(s.callbackSecret, callback) {
		return model.Payment{}, model.ErrInvalidCallbackSignature
	}
	if callback.Result != "success" && callback.Result != "failed" {
		return model.Payment{}, fmt.Errorf("%w：未知支付结果 %q", model.ErrCallbackAmountMismatch, callback.Result)
	}
	// ② 交叉校验：支付单存在、其背后订单号对得上、金额币种一致。
	//    目的：签名只证明"来自渠道"，不证明"内容合理"——渠道自己搞错金额时我们也要拒动。
	payment, err := s.repository.GetPaymentByNoAnyUser(ctx, callback.PaymentNo)
	if err != nil {
		return model.Payment{}, err // 未知支付单 → ErrPaymentNotFound → 404
	}
	order, err := s.repository.AdminGetOrderByID(ctx, payment.OrderID)
	if err != nil {
		return model.Payment{}, err
	}
	if callback.OrderNo != order.OrderNo {
		return model.Payment{}, model.ErrCallbackAmountMismatch // 报文张冠李戴：支付单和订单号配不上
	}
	if callback.AmountCent != payment.AmountCent || callback.Currency != payment.Currency {
		return model.Payment{}, model.ErrCallbackAmountMismatch
	}
	// ③ 事务内幂等消费（重复回调返回首次结果，firstEffect=false）。
	consumed, _, err := s.repository.ConsumePaymentCallback(ctx, repositories.ConsumeCallbackInput{
		PaymentNo:  callback.PaymentNo,
		EventNo:    callback.EventNo,
		Result:     callback.Result,
		AmountCent: callback.AmountCent,
		PaidAt:     time.Now().UTC(),
		Payload:    map[string]any{"payment_no": callback.PaymentNo, "order_no": callback.OrderNo, "event_no": callback.EventNo, "result": callback.Result, "amount_cent": callback.AmountCent, "currency": callback.Currency},
	})
	if err != nil {
		return model.Payment{}, err
	}
	return consumed, nil
}
