package services

import (
	"context"
	"fmt"
	"strings"
	"unicode/utf8"

	model "ggg/models"
	"ggg/repositories"
)

// RefundService 负责退款申请与运营审核。
// 规则：只有订单所有者、已完成或已发货的订单项可申请；金额不超过该实付小计；审核人与申请人权限分离。
type RefundService struct{ repository repositories.RefundRepository }

// NewRefundService 创建退款业务服务。
func NewRefundService(repository repositories.RefundRepository) *RefundService {
	return &RefundService{repository: repository}
}

// ApplyRefundInput 表示申请退款所需的业务参数。
type ApplyRefundInput struct {
	UserID      uint64
	OrderItemID uint64
	AmountCent  int64
	Reason      string
}

// Apply 校验归属和状态后创建退款单，并把订单项标记为退款中。
func (s *RefundService) Apply(ctx context.Context, input ApplyRefundInput) (model.Refund, error) {
	if input.UserID == 0 {
		return model.Refund{}, model.ErrInvalidUserID
	}
	if input.AmountCent <= 0 {
		return model.Refund{}, model.ErrInvalidRefundAmount
	}
	reason := trimRunes(input.Reason, 200)
	item, order, err := s.repository.GetOrderItemForUser(ctx, input.UserID, input.OrderItemID)
	if err != nil {
		return model.Refund{}, fmt.Errorf("查询订单项：%w", err)
	}
	// 目的：只有真实发生过交易的订单项才能进入售后流程。
	if order.Status != model.OrderStatusShipped && order.Status != model.OrderStatusCompleted {
		return model.Refund{}, model.ErrReviewNotEligible
	}
	if item.RefundStatus == model.ItemRefundPending || item.RefundStatus == model.ItemRefundRefunded {
		return model.Refund{}, model.ErrRefundAlreadyPending
	}
	if input.AmountCent > item.SubtotalCent {
		return model.Refund{}, model.ErrInvalidRefundAmount
	}
	refund, err := s.repository.CreateRefund(ctx, model.Refund{
		RefundNo: newBizNo(), OrderID: order.ID, OrderItemID: item.ID, UserID: input.UserID,
		AmountCent: input.AmountCent, Reason: reason, Status: model.RefundPending,
	})
	if err != nil {
		return model.Refund{}, fmt.Errorf("创建退款单：%w", err)
	}
	if err := s.repository.UpdateOrderItemRefundStatus(ctx, item.ID, model.ItemRefundPending); err != nil {
		return model.Refund{}, err
	}
	return refund, nil
}

// ListByUser 查询当前用户的退款单列表。
func (s *RefundService) ListByUser(ctx context.Context, userID uint64) ([]model.Refund, error) {
	if userID == 0 {
		return nil, model.ErrInvalidUserID
	}
	refunds, err := s.repository.ListRefundsByUser(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("查询退款列表：%w", err)
	}
	return refunds, nil
}

// AdminList 管理端分页查询全部退款单。
func (s *RefundService) AdminList(ctx context.Context, query repositories.ListRefundsQuery) (int64, []model.Refund, error) {
	total, refunds, err := s.repository.AdminListRefunds(ctx, query)
	if err != nil {
		return 0, nil, fmt.Errorf("查询退款列表：%w", err)
	}
	return total, refunds, nil
}

// Approve 运营审核通过并模拟退款成功（本阶段直接置 refunded），同步更新订单项退款进度。
// TODO(PRD-009 进阶): 渠道退款应异步执行 + 回调幂等；退款金额需按优惠分摊后的实付计算（见 docs/ADVANCED-TASKS.md A4/A8）。
func (s *RefundService) Approve(ctx context.Context, operatorID, refundID uint64) (model.Refund, error) {
	return s.review(ctx, operatorID, refundID, model.Refunded, model.ItemRefundRefunded)
}

// Reject 运营驳回退款申请，订单项恢复为可再次申请状态。
func (s *RefundService) Reject(ctx context.Context, operatorID, refundID uint64) (model.Refund, error) {
	return s.review(ctx, operatorID, refundID, model.RefundRejected, model.ItemRefundNone)
}

// review 是审核的公共路径：条件更新保证同一退款单只能被审核一次。
func (s *RefundService) review(ctx context.Context, operatorID uint64, refundID uint64, to model.RefundStatus, itemStatus model.ItemRefundStatus) (model.Refund, error) {
	if operatorID == 0 {
		return model.Refund{}, model.ErrInvalidUserID
	}
	refund, err := s.repository.AdminReviewRefund(ctx, refundID, to, operatorID)
	if err != nil {
		return model.Refund{}, fmt.Errorf("审核退款单：%w", err)
	}
	if err := s.repository.UpdateOrderItemRefundStatus(ctx, refund.OrderItemID, itemStatus); err != nil {
		return model.Refund{}, err
	}
	return refund, nil
}

// trimRunes 去除首尾空白并按字符数截断，防止超长文本写入失败。
func trimRunes(value string, max int) string {
	value = strings.TrimSpace(value)
	if utf8.RuneCountInString(value) <= max {
		return value
	}
	runes := []rune(value)
	return string(runes[:max])
}
