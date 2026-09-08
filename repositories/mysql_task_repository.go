package repositories

import (
	"context"
	"errors"
	"fmt"
	"time"

	model "ggg/models"
	"gorm.io/gorm"
)

// CreateTask 新增后台任务。
func (r *MySQLRepository) CreateTask(ctx context.Context, task model.BackgroundTask) (model.BackgroundTask, error) {
	if err := r.db.WithContext(ctx).Create(&task).Error; err != nil {
		return model.BackgroundTask{}, fmt.Errorf("创建后台任务：%w", err)
	}
	return task, nil
}

// ListTasks 管理端分页查询后台任务，可按状态和类型过滤。
func (r *MySQLRepository) ListTasks(ctx context.Context, query ListTasksQuery) (int64, []model.BackgroundTask, error) {
	page, pageSize := normalizePage(query.Page, query.PageSize)
	tx := r.db.WithContext(ctx).Model(&model.BackgroundTask{})
	if query.Status != "" {
		tx = tx.Where("status = ?", query.Status)
	}
	if query.TaskType != "" {
		tx = tx.Where("task_type = ?", query.TaskType)
	}
	var total int64
	if err := tx.Count(&total).Error; err != nil {
		return 0, nil, fmt.Errorf("统计任务数量：%w", err)
	}
	var tasks []model.BackgroundTask
	if err := tx.Order("id DESC").Limit(pageSize).Offset((page - 1) * pageSize).Find(&tasks).Error; err != nil {
		return 0, nil, fmt.Errorf("查询任务列表：%w", err)
	}
	return total, tasks, nil
}

// GetTaskByID 按 ID 查询任务。
func (r *MySQLRepository) GetTaskByID(ctx context.Context, taskID uint64) (model.BackgroundTask, error) {
	var task model.BackgroundTask
	err := r.db.WithContext(ctx).First(&task, taskID).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return model.BackgroundTask{}, model.ErrTaskNotFound
	}
	if err != nil {
		return model.BackgroundTask{}, fmt.Errorf("查询任务：%w", err)
	}
	return task, nil
}

// RetryTask 运营重试：只允许 failed → pending，清零执行次数并记录操作者；
// 状态条件保证并发下只有一个请求能成功重试。
func (r *MySQLRepository) RetryTask(ctx context.Context, taskID, operatorID uint64) (model.BackgroundTask, error) {
	result := r.db.WithContext(ctx).Model(&model.BackgroundTask{}).
		Where("id = ? AND status = ?", taskID, model.TaskFailed).
		Updates(map[string]any{"status": model.TaskPending, "attempts": 0, "next_run_at": time.Now().UTC(), "retry_operator_id": operatorID})
	if result.Error != nil {
		return model.BackgroundTask{}, fmt.Errorf("重试任务：%w", result.Error)
	}
	if result.RowsAffected == 0 {
		if _, err := r.GetTaskByID(ctx, taskID); err != nil {
			return model.BackgroundTask{}, err
		}
		return model.BackgroundTask{}, model.ErrTaskNotRetryable
	}
	return r.GetTaskByID(ctx, taskID)
}
