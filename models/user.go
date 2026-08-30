package models

import (
	"time"

	"gorm.io/gorm"
)

// UserStatus 表示用户当前是否允许登录。
type UserStatus string

const (
	// UserStatusActive 表示用户可以正常登录和使用服务。
	UserStatusActive UserStatus = "active"
	// UserStatusDisabled 表示用户已被禁用，不能登录。
	UserStatusDisabled UserStatus = "disabled"
)

// User 表示商城用户的持久化数据。
// PasswordHash 只保存密码哈希，绝不保存用户明文密码。
type User struct {
	ID           uint64         `json:"id" gorm:"primaryKey;autoIncrement"`
	Username     string         `json:"username" gorm:"size:50;not null;uniqueIndex:uk_users_username"`
	Email        string         `json:"email" gorm:"size:255;not null;uniqueIndex:uk_users_email"`
	PasswordHash string         `json:"-" gorm:"size:255;not null"`
	Status       UserStatus     `json:"status" gorm:"type:enum('active','disabled');not null;default:active;index:idx_users_status"`
	CreatedAt    time.Time      `json:"created_at"`
	UpdatedAt    time.Time      `json:"updated_at"`
	DeletedAt    gorm.DeletedAt `json:"-" gorm:"index"`
}

// TableName 显式指定 User 对应的 MySQL 表名。
func (User) TableName() string { return "users" }
