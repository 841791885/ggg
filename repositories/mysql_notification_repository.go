package repositories

import (
	"context"
	"fmt"
	"time"

	model "ggg/models"
)

// CreateNotification 新增站内通知。
func (r *MySQLRepository) CreateNotification(ctx context.Context, notification model.Notification) (model.Notification, error) {
	if err := r.db.WithContext(ctx).Create(&notification).Error; err != nil {
		return model.Notification{}, fmt.Errorf("创建通知：%w", err)
	}
	return notification, nil
}

// ListNotifications 分页查询当前用户的通知，未读在前。
func (r *MySQLRepository) ListNotifications(ctx context.Context, userID uint64, page, pageSize int) (int64, []model.Notification, error) {
	page, pageSize = normalizePage(page, pageSize)
	tx := r.db.WithContext(ctx).Model(&model.Notification{}).Where("user_id = ?", userID)
	var total int64
	if err := tx.Count(&total).Error; err != nil {
		return 0, nil, fmt.Errorf("统计通知数量：%w", err)
	}
	var notifications []model.Notification
	if err := tx.Order("read_at IS NOT NULL, id DESC").Limit(pageSize).Offset((page - 1) * pageSize).Find(&notifications).Error; err != nil {
		return 0, nil, fmt.Errorf("查询通知列表：%w", err)
	}
	return total, notifications, nil
}

// MarkNotificationRead 把当前用户的一条未读通知标记为已读。
func (r *MySQLRepository) MarkNotificationRead(ctx context.Context, userID, notificationID uint64) error {
	now := time.Now().UTC()
	result := r.db.WithContext(ctx).Model(&model.Notification{}).
		Where("id = ? AND user_id = ? AND read_at IS NULL", notificationID, userID).
		Update("read_at", &now)
	if result.Error != nil {
		return fmt.Errorf("标记通知已读：%w", result.Error)
	}
	if result.RowsAffected == 0 {
		var exists int64
		if err := r.db.WithContext(ctx).Model(&model.Notification{}).Where("id = ? AND user_id = ?", notificationID, userID).Count(&exists).Error; err != nil {
			return fmt.Errorf("查询通知：%w", err)
		}
		if exists == 0 {
			return model.ErrNotificationNotFound
		}
	}
	return nil
}

// MarkAllNotificationsRead 把当前用户全部未读通知标记为已读。
func (r *MySQLRepository) MarkAllNotificationsRead(ctx context.Context, userID uint64) error {
	now := time.Now().UTC()
	if err := r.db.WithContext(ctx).Model(&model.Notification{}).
		Where("user_id = ? AND read_at IS NULL", userID).
		Update("read_at", &now).Error; err != nil {
		return fmt.Errorf("批量标记通知已读：%w", err)
	}
	return nil
}
