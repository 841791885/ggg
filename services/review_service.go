package services

import (
	"context"
	"fmt"

	model "ggg/models"
	"ggg/repositories"
)

// ReviewService 负责评价写入、展示和运营显隐管理。
// 规则：评价必须绑定真实已完成订单项，一人一项一条；隐藏保留原始内容和操作记录用于审计。
type ReviewService struct{ repository repositories.ReviewStore }

// NewReviewService 创建评价业务服务。
func NewReviewService(repository repositories.ReviewStore) *ReviewService {
	return &ReviewService{repository: repository}
}

// CreateReviewInput 表示提交评价所需的业务参数。
type CreateReviewInput struct {
	UserID      uint64
	OrderItemID uint64
	Rating      int
	Content     string
}

// Create 校验订单项归属、订单完成状态和重复评价后写入。
func (s *ReviewService) Create(ctx context.Context, input CreateReviewInput) (model.Review, error) {
	if input.UserID == 0 {
		return model.Review{}, model.ErrInvalidUserID
	}
	if input.Rating < 1 || input.Rating > 5 {
		return model.Review{}, model.ErrReviewNotEligible
	}
	content := trimRunes(input.Content, 500)
	item, order, err := s.repository.GetOrderItemForUser(ctx, input.UserID, input.OrderItemID)
	if err != nil {
		return model.Review{}, fmt.Errorf("查询订单项：%w", err)
	}
	// 目的：只有确认收货的订单项可评价，防止未交付交易产生评价噪声。
	// 目的：退款中的订单项交易状态未定，暂不允许评价；审核驳回后 refund_status 恢复 none 可再评价。
	if order.Status != model.OrderStatusCompleted || item.RefundStatus == model.ItemRefundPending {
		return model.Review{}, model.ErrReviewNotEligible
	}
	if _, err := s.repository.GetReviewByOrderItem(ctx, item.ID); err == nil {
		return model.Review{}, model.ErrReviewDuplicate
	} else if !isModelNotFound(err) {
		return model.Review{}, fmt.Errorf("查询已有评价：%w", err)
	}
	// 目的：评价冗余存储 product_id，便于按商品聚合展示而无需再关联查询。
	var productID uint64
	if sku, err := s.repository.GetSKUByID(ctx, item.SKUID); err == nil {
		productID = sku.ProductID
	}
	review, err := s.repository.CreateReview(ctx, model.Review{
		OrderItemID: item.ID, ProductID: productID, UserID: input.UserID,
		Rating: input.Rating, Content: content, Visible: true,
	})
	if err != nil {
		return model.Review{}, fmt.Errorf("创建评价：%w", err)
	}
	return review, nil
}

// ListVisibleByProduct 消费者分页查看商品可见评价。
func (s *ReviewService) ListVisibleByProduct(ctx context.Context, productID uint64, page, pageSize int) (int64, []model.Review, error) {
	total, reviews, err := s.repository.ListVisibleReviewsByProduct(ctx, productID, page, pageSize)
	if err != nil {
		return 0, nil, fmt.Errorf("查询商品评价：%w", err)
	}
	return total, reviews, nil
}

// AdminList 管理端分页查看全部评价（含隐藏）。
func (s *ReviewService) AdminList(ctx context.Context, productID uint64, page, pageSize int) (int64, []model.Review, error) {
	total, reviews, err := s.repository.AdminListAllReviews(ctx, productID, page, pageSize)
	if err != nil {
		return 0, nil, fmt.Errorf("查询评价列表：%w", err)
	}
	return total, reviews, nil
}

// SetVisibility 运营设置评价显隐并记录操作者。
func (s *ReviewService) SetVisibility(ctx context.Context, operatorID, reviewID uint64, visible bool) (model.Review, error) {
	if operatorID == 0 {
		return model.Review{}, model.ErrInvalidUserID
	}
	review, err := s.repository.SetReviewVisibility(ctx, reviewID, visible, operatorID)
	if err != nil {
		return model.Review{}, fmt.Errorf("更新评价显示状态：%w", err)
	}
	return review, nil
}
