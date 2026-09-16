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

// ClaimDueTask 领取一条到期任务：条件更新 pending→running 充当分布式锁。
// 并发 worker（或多实例）同时抢同一行时，只有先提交者 RowsAffected=1，其余拿到 0 换下一条——
// 这就是"数据库当队列"的领取协议，无需 Redis。ORDER BY next_run_at 保证先到期先执行。
func (r *MySQLRepository) ClaimDueTask(ctx context.Context, now time.Time) (model.BackgroundTask, bool, error) {
	// 第一步（乐观）：按到期时间升序找一条候选任务。
	// ⚠️ 这里只是"看到"，不是"拿到"——两个 worker 可能同时选中同一条，胜负在第二步决出。
	var candidate model.BackgroundTask
	err := r.db.WithContext(ctx).
		Where("status = ? AND next_run_at <= ?", model.TaskPending, now).
		Order("next_run_at ASC").First(&candidate).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return model.BackgroundTask{}, false, nil // 无到期任务，正常空转
	}
	if err != nil {
		return model.BackgroundTask{}, false, fmt.Errorf("查询到期任务：%w", err)
	}
	// 第二步（裁决）：条件更新即抢锁——WHERE 里的 status='pending' 是竞争点，
	// InnoDB 保证同一行的两个 UPDATE 串行执行，先提交者改成 running，后提交者的 WHERE 已不匹配。
	// attempts+1 也在这一步完成：领取即计一次尝试，进程崩溃在执行中途也算消耗过额度。
	result := r.db.WithContext(ctx).Model(&model.BackgroundTask{}).
		Where("id = ? AND status = ?", candidate.ID, model.TaskPending).
		Updates(map[string]any{"status": model.TaskRunning, "attempts": gorm.Expr("attempts + 1")})
	if result.Error != nil {
		return model.BackgroundTask{}, false, fmt.Errorf("领取任务：%w", result.Error)
	}
	if result.RowsAffected == 0 {
		// 被别的 worker 抢先一步：不算错误，本轮让出（下次心跳重新选）。
		return model.BackgroundTask{}, false, nil
	}
	// 抢到后同步内存副本与库中实际值一致，调用方拿到的 Attempts 已是本次执行的序号。
	candidate.Status = model.TaskRunning
	candidate.Attempts++
	return candidate, true, nil
}

// MarkTaskSucceeded 任务成功收尾：running → succeeded，进入终态不再被领取。
func (r *MySQLRepository) MarkTaskSucceeded(ctx context.Context, taskID uint64) error {
	// WHERE 带 running 前态：防止把已被 RevertRunningTasks 回收重跑的任务误标成功
	//（旧执行的迟到回写会匹配不到新状态，静默丢弃——与 A1/A2 同一套前态条件思想）。
	result := r.db.WithContext(ctx).Model(&model.BackgroundTask{}).
		Where("id = ? AND status = ?", taskID, model.TaskRunning).
		Update("status", model.TaskSucceeded)
	if result.Error != nil {
		return fmt.Errorf("更新任务状态：%w", result.Error)
	}
	return nil
}

// MarkTaskFailedOrRetry 失败处置：未超上限按指数退避回到 pending 等待下次领取；
// 达到 max_attempts 定格 failed，等运营在管理台重试（RetryTask 已有）。
func (r *MySQLRepository) MarkTaskFailedOrRetry(ctx context.Context, taskID uint64, lastErr string, backoff time.Duration) error {
	var task model.BackgroundTask
	if err := r.db.WithContext(ctx).First(&task, taskID).Error; err != nil {
		return fmt.Errorf("查询任务：%w", err)
	}
	next := map[string]any{"last_error": truncateErr(lastErr)}
	if task.Attempts >= task.MaxAttempts {
		next["status"] = model.TaskFailed // 额度耗尽定格失败：等运营 RetryTask 人工介入，机器不再自动重试
	} else {
		// 回到 pending 并把 next_run_at 推到未来 = "睡一觉再来"；退避时长由调用方按 attempts 指数计算。
		next["status"] = model.TaskPending
		next["next_run_at"] = time.Now().UTC().Add(backoff)
	}
	// running 前态条件同 MarkTaskSucceeded：拒绝已被回收重跑的任务的迟到回写。
	result := r.db.WithContext(ctx).Model(&model.BackgroundTask{}).
		Where("id = ? AND status = ?", taskID, model.TaskRunning).
		Updates(next)
	if result.Error != nil {
		return fmt.Errorf("回写任务失败状态：%w", result.Error)
	}
	return nil
}

// RevertRunningTasks 启动时回收崩溃遗留：上次进程死掉时处于 running 的任务重置为 pending，
// 由下一轮扫描继续（attempts 保留，防止无限重试逃逸）。
func (r *MySQLRepository) RevertRunningTasks(ctx context.Context) (int64, error) {
	// 无状态条件：worker 启动瞬间不可能有"活的"执行者（本进程还没开始领任务），
	// 此刻所有 running 都是历史遗骸，全部回收。多实例同时启动时该操作幂等，重复执行无害。
	result := r.db.WithContext(ctx).Model(&model.BackgroundTask{}).
		Where("status = ?", model.TaskRunning).
		Updates(map[string]any{"status": model.TaskPending, "next_run_at": time.Now().UTC()})
	if result.Error != nil {
		return 0, fmt.Errorf("回收 running 任务：%w", result.Error)
	}
	return result.RowsAffected, nil
}

// truncateErr 错误消息裁剪到列宽内，避免超长堆栈撑爆 last_error。
func truncateErr(s string) string {
	const max = 500
	r := []rune(s)
	if len(r) > max {
		return string(r[:max-3]) + "..."
	}
	return s
}
