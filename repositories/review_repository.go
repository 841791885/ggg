package repositories

import (
	"context"
	"errors"
	"fmt"

	model "ggg/models"
	"gorm.io/gorm"
)

// CreateReview 新增评价。
func (r *MySQLRepository) CreateReview(ctx context.Context, review model.Review) (model.Review, error) {
	if err := r.db.WithContext(ctx).Create(&review).Error; err != nil {
		// 订单项唯一键冲突表示已评价过。
		if errors.Is(err, gorm.ErrDuplicatedKey) {
			return model.Review{}, model.ErrReviewDuplicate
		}
		return model.Review{}, fmt.Errorf("创建评价：%w", err)
	}
	return review, nil
}

// GetReviewByOrderItem 查询某订单项的评价。
func (r *MySQLRepository) GetReviewByOrderItem(ctx context.Context, orderItemID uint64) (model.Review, error) {
	var review model.Review
	err := r.db.WithContext(ctx).Where("order_item_id = ?", orderItemID).First(&review).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return model.Review{}, model.ErrReviewNotFound
	}
	if err != nil {
		return model.Review{}, fmt.Errorf("查询评价：%w", err)
	}
	return review, nil
}

// ListVisibleReviewsByProduct 消费者视角：分页查询商品的可见评价。
func (r *MySQLRepository) ListVisibleReviewsByProduct(ctx context.Context, productID uint64, page, pageSize int) (int64, []model.Review, error) {
	page, pageSize = normalizePage(page, pageSize)
	tx := r.db.WithContext(ctx).Model(&model.Review{}).Where("product_id = ? AND visible = ?", productID, true)
	var total int64
	if err := tx.Count(&total).Error; err != nil {
		return 0, nil, fmt.Errorf("统计商品评价：%w", err)
	}
	var reviews []model.Review
	if err := tx.Order("id DESC").Limit(pageSize).Offset((page - 1) * pageSize).Find(&reviews).Error; err != nil {
		return 0, nil, fmt.Errorf("查询商品评价列表：%w", err)
	}
	return total, reviews, nil
}

// AdminListAllReviews 管理端分页查询全部评价（含隐藏）；productID 为 0 时查全量。
func (r *MySQLRepository) AdminListAllReviews(ctx context.Context, productID uint64, page, pageSize int) (int64, []model.Review, error) {
	page, pageSize = normalizePage(page, pageSize)
	tx := r.db.WithContext(ctx).Model(&model.Review{})
	if productID > 0 {
		tx = tx.Where("product_id = ?", productID)
	}
	var total int64
	if err := tx.Count(&total).Error; err != nil {
		return 0, nil, fmt.Errorf("统计评价：%w", err)
	}
	var reviews []model.Review
	if err := tx.Order("id DESC").Limit(pageSize).Offset((page - 1) * pageSize).Find(&reviews).Error; err != nil {
		return 0, nil, fmt.Errorf("查询评价列表：%w", err)
	}
	return total, reviews, nil
}

// SetReviewVisibility 运营设置评价显隐；隐藏时记录操作者，保留原始内容用于审计。
func (r *MySQLRepository) SetReviewVisibility(ctx context.Context, reviewID uint64, visible bool, operatorID uint64) (model.Review, error) {
	updates := map[string]any{"visible": visible}
	if !visible {
		updates["hidden_by"] = operatorID
	} else {
		updates["hidden_by"] = nil
	}
	result := r.db.WithContext(ctx).Model(&model.Review{}).Where("id = ?", reviewID).Updates(updates)
	if result.Error != nil {
		return model.Review{}, fmt.Errorf("更新评价显示状态：%w", result.Error)
	}
	if result.RowsAffected == 0 {
		return model.Review{}, model.ErrReviewNotFound
	}
	var review model.Review
	if err := r.db.WithContext(ctx).First(&review, reviewID).Error; err != nil {
		return model.Review{}, fmt.Errorf("查询评价：%w", err)
	}
	return review, nil
}

// 编译期检查：本文件负责的领域接口是否都实现了。
// 少写方法时错误直接指向这里，而不是 mysql_repository.go 里的全量断言。
var _ ReviewRepository = (*MySQLRepository)(nil)
