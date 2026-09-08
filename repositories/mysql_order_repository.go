package repositories

import (
	"context"
	"errors"
	"fmt"
	"maps"

	model "ggg/models"
	"gorm.io/gorm"
)

// CreateOrder 在同一事务内写入订单与订单项。
func (r *MySQLRepository) CreateOrder(ctx context.Context, order model.Order) (model.Order, error) {
	if err := r.db.WithContext(ctx).Create(&order).Error; err != nil {
		return model.Order{}, fmt.Errorf("创建订单：%w", err)
	}
	return order, nil
}

// GetOrderByID 查询当前用户自己的订单及明细；他人订单统一按不存在处理。
func (r *MySQLRepository) GetOrderByID(ctx context.Context, userID, orderID uint64) (model.Order, error) {
	var order model.Order
	err := r.db.WithContext(ctx).Preload("Items").Where("id = ? AND user_id = ?", orderID, userID).First(&order).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return model.Order{}, model.ErrOrderNotFound
	}
	if err != nil {
		return model.Order{}, fmt.Errorf("查询订单：%w", err)
	}
	return order, nil
}

// GetOrderByIdempotencyKey 按用户 + 幂等键查询已有订单，用于重试返回首次结果。
func (r *MySQLRepository) GetOrderByIdempotencyKey(ctx context.Context, userID uint64, key string) (model.Order, error) {
	var order model.Order
	err := r.db.WithContext(ctx).Preload("Items").Where("user_id = ? AND idempotency_key = ?", userID, key).First(&order).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return model.Order{}, model.ErrOrderNotFound
	}
	if err != nil {
		return model.Order{}, fmt.Errorf("查询幂等订单：%w", err)
	}
	return order, nil
}

// ListOrdersByUser 分页查询当前用户的订单。
func (r *MySQLRepository) ListOrdersByUser(ctx context.Context, userID uint64, query ListOrdersQuery) (int64, []model.Order, error) {
	return r.listOrders(ctx, r.db.WithContext(ctx).Model(&model.Order{}).Where("user_id = ?", userID), query)
}

// UpdateOrderStatus 在消费者视角下推进状态：条件里带上当前状态做乐观校验，
// 状态被并发抢先变更时 RowsAffected 为 0，调用方重新读取后按非法迁移处理。
// extra 允许附带时间戳字段（paid_at 等）。
func (r *MySQLRepository) UpdateOrderStatus(ctx context.Context, userID, orderID uint64, to model.OrderStatus, extra map[string]any) (model.Order, error) {
	order, err := r.GetOrderByID(ctx, userID, orderID)
	if err != nil {
		return model.Order{}, err
	}
	updates := map[string]any{"status": to}
	maps.Copy(updates, extra)
	result := r.db.WithContext(ctx).Model(&model.Order{}).
		Where("id = ? AND user_id = ? AND status = ?", orderID, userID, order.Status).
		Updates(updates)
	if result.Error != nil {
		return model.Order{}, fmt.Errorf("更新订单状态：%w", result.Error)
	}
	if result.RowsAffected == 0 {
		return model.Order{}, model.ErrInvalidOrderTransition
	}
	return r.GetOrderByID(ctx, userID, orderID)
}

// CreateOrderStatusLog 追加一条状态变更历史。
func (r *MySQLRepository) CreateOrderStatusLog(ctx context.Context, log model.OrderStatusLog) error {
	if err := r.db.WithContext(ctx).Create(&log).Error; err != nil {
		return fmt.Errorf("记录订单状态日志：%w", err)
	}
	return nil
}

// ListOrderStatusLogs 查询订单的状态变更历史。
func (r *MySQLRepository) ListOrderStatusLogs(ctx context.Context, orderID uint64) ([]model.OrderStatusLog, error) {
	var logs []model.OrderStatusLog
	if err := r.db.WithContext(ctx).Where("order_id = ?", orderID).Order("id ASC").Find(&logs).Error; err != nil {
		return nil, fmt.Errorf("查询订单状态日志：%w", err)
	}
	return logs, nil
}

// AdminGetOrderByID 管理端查询任意订单及明细。
func (r *MySQLRepository) AdminGetOrderByID(ctx context.Context, orderID uint64) (model.Order, error) {
	var order model.Order
	err := r.db.WithContext(ctx).Preload("Items").First(&order, orderID).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return model.Order{}, model.ErrOrderNotFound
	}
	if err != nil {
		return model.Order{}, fmt.Errorf("查询订单：%w", err)
	}
	return order, nil
}

// AdminListOrders 管理端分页查询全部订单。
func (r *MySQLRepository) AdminListOrders(ctx context.Context, query ListOrdersQuery) (int64, []model.Order, error) {
	return r.listOrders(ctx, r.db.WithContext(ctx).Model(&model.Order{}), query)
}

// AdminUpdateOrderStatus 管理端推进状态，from→to 双条件保证只从合法前态迁移。
func (r *MySQLRepository) AdminUpdateOrderStatus(ctx context.Context, orderID uint64, from, to model.OrderStatus, extra map[string]any) (model.Order, error) {
	updates := map[string]any{"status": to}
	maps.Copy(updates, extra)
	result := r.db.WithContext(ctx).Model(&model.Order{}).
		Where("id = ? AND status = ?", orderID, from).
		Updates(updates)
	if result.Error != nil {
		return model.Order{}, fmt.Errorf("更新订单状态：%w", result.Error)
	}
	if result.RowsAffected == 0 {
		return model.Order{}, model.ErrInvalidOrderTransition
	}
	return r.AdminGetOrderByID(ctx, orderID)
}

// listOrders 是消费者与管理端共用的分页查询主体。
func (r *MySQLRepository) listOrders(ctx context.Context, tx *gorm.DB, query ListOrdersQuery) (int64, []model.Order, error) {
	page, pageSize := normalizePage(query.Page, query.PageSize)
	if query.Status != "" {
		tx = tx.Where("status = ?", query.Status)
	}
	var total int64
	if err := tx.Count(&total).Error; err != nil {
		return 0, nil, fmt.Errorf("统计订单数量：%w", err)
	}
	var orders []model.Order
	if err := tx.Preload("Items").Order("id DESC").Limit(pageSize).Offset((page - 1) * pageSize).Find(&orders).Error; err != nil {
		return 0, nil, fmt.Errorf("查询订单列表：%w", err)
	}
	return total, orders, nil
}

// normalizePage 把非法分页参数收敛到安全范围。
func normalizePage(page, pageSize int) (int, int) {
	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 100 {
		pageSize = 20
	}
	return page, pageSize
}
