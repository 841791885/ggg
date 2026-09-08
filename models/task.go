package models

import (
	"time"

	"gorm.io/gorm"
)

// TaskStatus 表示后台任务的执行状态。worker 消费属于 PRD-008 进阶阶段，
// 本阶段提供任务表结构与运营查询/重试接口。
type TaskStatus string

const (
	TaskPending   TaskStatus = "pending"
	TaskRunning   TaskStatus = "running"
	TaskSucceeded TaskStatus = "succeeded"
	TaskFailed    TaskStatus = "failed"
)

// BackgroundTask 表示一条持久化后台任务（MySQL Outbox 事实来源）。
// 只有 failed 状态允许运营重试，重试会清零 attempts 并记录操作者。
type BackgroundTask struct {
	ID              uint64         `json:"id" gorm:"primaryKey;autoIncrement"`
	TaskType        string         `json:"task_type" gorm:"size:30;not null"`
	Payload         map[string]any `json:"payload" gorm:"type:json;serializer:json;not null"`
	Status          TaskStatus     `json:"status" gorm:"type:enum('pending','running','succeeded','failed');not null;default:pending;index:idx_background_tasks_status_next_run,priority:1"`
	Attempts        int            `json:"attempts" gorm:"not null;default:0"`
	MaxAttempts     int            `json:"max_attempts" gorm:"not null;default:5"`
	NextRunAt       time.Time      `json:"next_run_at" gorm:"not null;index:idx_background_tasks_status_next_run,priority:2"`
	LastError       string         `json:"last_error" gorm:"size:500;not null;default:"`
	RetryOperatorID *uint64        `json:"retry_operator_id"`
	CreatedAt       time.Time      `json:"created_at"`
	UpdatedAt       time.Time      `json:"updated_at"`
	DeletedAt       gorm.DeletedAt `json:"-" gorm:"index"`
}

// TableName 显式指定 BackgroundTask 对应的 MySQL 表名。
func (BackgroundTask) TableName() string { return "background_tasks" }
