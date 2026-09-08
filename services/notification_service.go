package services

import (
	"context"
	"fmt"

	model "ggg/models"
	"ggg/repositories"
)

// NotificationService 负责站内通知的查询和已读标记。
// TODO(PRD-008 进阶): 交易事件自动产生通知依赖 Outbox + worker，未实现（见 docs/ADVANCED-TASKS.md A3）。
// 本阶段提供手动创建能力和用户侧读取接口。
type NotificationService struct {
	repository repositories.NotificationRepository
}

// NewNotificationService 创建通知业务服务。
func NewNotificationService(repository repositories.NotificationRepository) *NotificationService {
	return &NotificationService{repository: repository}
}

// CreateInput 表示生成一条通知所需的参数。
type CreateNotificationInput struct {
	UserID  uint64
	Type    string
	Title   string
	Content string
}

// Create 写入一条站内通知。
func (s *NotificationService) Create(ctx context.Context, input CreateNotificationInput) (model.Notification, error) {
	if input.UserID == 0 {
		return model.Notification{}, model.ErrInvalidUserID
	}
	title := trimRunes(input.Title, 50)
	if title == "" {
		return model.Notification{}, model.ErrInvalidUserID // 标题为空复用参数错误语义，避免新增过多 sentinel。
	}
	notification, err := s.repository.CreateNotification(ctx, model.Notification{
		UserID: input.UserID, Type: trimRunes(input.Type, 30), Title: title, Content: trimRunes(input.Content, 500),
	})
	if err != nil {
		return model.Notification{}, fmt.Errorf("创建通知：%w", err)
	}
	return notification, nil
}

// List 分页查询当前用户的通知，未读在前。
func (s *NotificationService) List(ctx context.Context, userID uint64, page, pageSize int) (int64, []model.Notification, error) {
	if userID == 0 {
		return 0, nil, model.ErrInvalidUserID
	}
	total, notifications, err := s.repository.ListNotifications(ctx, userID, page, pageSize)
	if err != nil {
		return 0, nil, fmt.Errorf("查询通知列表：%w", err)
	}
	return total, notifications, nil
}

// MarkRead 把一条未读通知标记为已读。
func (s *NotificationService) MarkRead(ctx context.Context, userID, notificationID uint64) error {
	if userID == 0 {
		return model.ErrInvalidUserID
	}
	if err := s.repository.MarkNotificationRead(ctx, userID, notificationID); err != nil {
		return fmt.Errorf("标记通知已读：%w", err)
	}
	return nil
}

// MarkAllRead 一键已读当前用户全部未读通知。
func (s *NotificationService) MarkAllRead(ctx context.Context, userID uint64) error {
	if userID == 0 {
		return model.ErrInvalidUserID
	}
	if err := s.repository.MarkAllNotificationsRead(ctx, userID); err != nil {
		return fmt.Errorf("批量标记通知已读：%w", err)
	}
	return nil
}
