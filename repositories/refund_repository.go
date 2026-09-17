package repositories

import (
	"context"
	"errors"
	"fmt"
	"time"

	model "ggg/models"
	"gorm.io/gorm"
)

// CreateRefund 新增退款单。
func (r *MySQLRepository) CreateRefund(ctx context.Context, refund model.Refund) (model.Refund, error) {
	if err := r.db.WithContext(ctx).Create(&refund).Error; err != nil {
		return model.Refund{}, fmt.Errorf("创建退款单：%w", err)
	}
	return refund, nil
}

// GetRefundByID 查询当前用户自己的退款单。
func (r *MySQLRepository) GetRefundByID(ctx context.Context, userID, refundID uint64) (model.Refund, error) {
	var refund model.Refund
	err := r.db.WithContext(ctx).Where("id = ? AND user_id = ?", refundID, userID).First(&refund).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return model.Refund{}, model.ErrRefundNotFound
	}
	if err != nil {
		return model.Refund{}, fmt.Errorf("查询退款单：%w", err)
	}
	return refund, nil
}

// ListRefundsByUser 查询当前用户的退款单列表。
func (r *MySQLRepository) ListRefundsByUser(ctx context.Context, userID uint64) ([]model.Refund, error) {
	var refunds []model.Refund
	if err := r.db.WithContext(ctx).Where("user_id = ?", userID).Order("id DESC").Find(&refunds).Error; err != nil {
		return nil, fmt.Errorf("查询退款列表：%w", err)
	}
	return refunds, nil
}

// AdminListRefunds 管理端分页查询全部退款单。
func (r *MySQLRepository) AdminListRefunds(ctx context.Context, query ListRefundsQuery) (int64, []model.Refund, error) {
	page, pageSize := normalizePage(query.Page, query.PageSize)
	tx := r.db.WithContext(ctx).Model(&model.Refund{})
	if query.Status != "" {
		tx = tx.Where("status = ?", query.Status)
	}
	var total int64
	if err := tx.Count(&total).Error; err != nil {
		return 0, nil, fmt.Errorf("统计退款数量：%w", err)
	}
	var refunds []model.Refund
	if err := tx.Order("id DESC").Limit(pageSize).Offset((page - 1) * pageSize).Find(&refunds).Error; err != nil {
		return 0, nil, fmt.Errorf("查询退款列表：%w", err)
	}
	return total, refunds, nil
}

// AdminReviewRefund 运营审核：只允许 pending → approved/rejected，状态条件防止并发重复审核。
func (r *MySQLRepository) AdminReviewRefund(ctx context.Context, refundID uint64, to model.RefundStatus, operatorID uint64) (model.Refund, error) {
	now := time.Now().UTC()
	result := r.db.WithContext(ctx).Model(&model.Refund{}).
		Where("id = ? AND status = ?", refundID, model.RefundPending).
		Updates(map[string]any{"status": to, "reviewed_by": operatorID, "reviewed_at": &now})
	if result.Error != nil {
		return model.Refund{}, fmt.Errorf("审核退款单：%w", result.Error)
	}
	if result.RowsAffected == 0 {
		return model.Refund{}, model.ErrRefundNotFound
	}
	var refund model.Refund
	if err := r.db.WithContext(ctx).First(&refund, refundID).Error; err != nil {
		return model.Refund{}, fmt.Errorf("查询退款单：%w", err)
	}
	return refund, nil
}

// UpdateOrderItemRefundStatus 同步订单项的退款进度。
func (r *MySQLRepository) UpdateOrderItemRefundStatus(ctx context.Context, orderItemID uint64, status model.ItemRefundStatus) error {
	if err := r.db.WithContext(ctx).Model(&model.OrderItem{}).Where("id = ?", orderItemID).Update("refund_status", status).Error; err != nil {
		return fmt.Errorf("更新订单项退款状态：%w", err)
	}
	return nil
}

// GetOrderItemForUser 按订单项 ID 联查订单项与所属订单，并校验归属；他人数据统一按订单不存在处理。
func (r *MySQLRepository) GetOrderItemForUser(ctx context.Context, userID, itemID uint64) (model.OrderItem, model.Order, error) {
	var item model.OrderItem
	var order model.Order
	err := r.db.WithContext(ctx).
		Joins("JOIN orders ON orders.id = order_items.order_id AND orders.deleted_at IS NULL").
		Where("order_items.id = ? AND orders.user_id = ?", itemID, userID).
		First(&item).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return model.OrderItem{}, model.Order{}, model.ErrOrderNotFound
	}
	if err != nil {
		return model.OrderItem{}, model.Order{}, fmt.Errorf("查询订单项：%w", err)
	}
	if err := r.db.WithContext(ctx).First(&order, item.OrderID).Error; err != nil {
		return model.OrderItem{}, model.Order{}, fmt.Errorf("查询订单项所属订单：%w", err)
	}
	return item, order, nil
}

// 编译期检查：本文件负责的领域接口是否都实现了。
// 少写方法时错误直接指向这里，而不是 mysql_repository.go 里的全量断言。
var _ RefundRepository = (*MySQLRepository)(nil)
