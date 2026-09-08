package services

import (
	"context"
	"fmt"

	model "ggg/models"
	"ggg/repositories"
)

// TaskService 负责后台任务的查询与失败重试。
// TODO(PRD-008 进阶): worker 消费循环（goroutine + channel + context 优雅停机）、超时关单调度、指数退避重试均未实现（见 docs/ADVANCED-TASKS.md A3）。
// 本阶段任务表作为 Outbox 事实来源，运营可查看状态、对 failed 任务发起重试。
type TaskService struct{ repository repositories.TaskRepository }

// NewTaskService 创建后台任务业务服务。
func NewTaskService(repository repositories.TaskRepository) *TaskService {
	return &TaskService{repository: repository}
}

// List 分页查询任务，可按状态和类型过滤。
func (s *TaskService) List(ctx context.Context, query repositories.ListTasksQuery) (int64, []model.BackgroundTask, error) {
	total, tasks, err := s.repository.ListTasks(ctx, query)
	if err != nil {
		return 0, nil, fmt.Errorf("查询任务列表：%w", err)
	}
	return total, tasks, nil
}

// Retry 运营重试失败任务：清零执行次数并记录操作者，便于审计"谁在什么时候重跑过"。
func (s *TaskService) Retry(ctx context.Context, operatorID, taskID uint64) (model.BackgroundTask, error) {
	if operatorID == 0 {
		return model.BackgroundTask{}, model.ErrInvalidUserID
	}
	task, err := s.repository.RetryTask(ctx, taskID, operatorID)
	if err != nil {
		return model.BackgroundTask{}, fmt.Errorf("重试任务：%w", err)
	}
	return task, nil
}
