package controllers

import (
	model "ggg/models"
	"time"
)

// UserResponse 表示用户公开信息，不包含密码哈希。
type UserResponse struct {
	ID        uint64           `json:"id"`
	Username  string           `json:"username"`
	Email     string           `json:"email"`
	Status    model.UserStatus `json:"status"`
	CreatedAt time.Time        `json:"created_at"`
	UpdatedAt time.Time        `json:"updated_at"`
}

// newUserResponse 将用户模型转换为对外响应结构。
func newUserResponse(user *model.User) UserResponse {
	return UserResponse{ID: user.ID, Username: user.Username, Email: user.Email, Status: user.Status, CreatedAt: user.CreatedAt, UpdatedAt: user.UpdatedAt}
}
