package services

import (
	"context"
	"fmt"
	"net/mail"
	"strings"
	"unicode/utf8"

	model "ggg/models"
	"ggg/repositories"
	"golang.org/x/crypto/bcrypt"
)

// RegisterUserInput 表示用户注册所需的业务参数。
type RegisterUserInput struct{ Username, Email, Password string }

// UserService 负责用户相关业务规则。
type UserService struct{ repository repositories.UserRepository }

// NewUserService 创建用户业务服务。
func NewUserService(repository repositories.UserRepository) *UserService {
	return &UserService{repository: repository}
}

// Register 校验注册参数、加密密码并创建用户。
func (s *UserService) Register(ctx context.Context, input RegisterUserInput) (model.User, error) {
	username := strings.TrimSpace(input.Username)
	if n := utf8.RuneCountInString(username); n < 3 || n > 50 {
		return model.User{}, model.ErrInvalidUsername
	}
	email := strings.ToLower(strings.TrimSpace(input.Email))
	if _, err := mail.ParseAddress(email); err != nil {
		return model.User{}, model.ErrInvalidEmail
	}
	if n := utf8.RuneCountInString(input.Password); n < 8 || n > 72 {
		return model.User{}, model.ErrInvalidPassword
	}
	if _, err := s.repository.GetUserByUsername(ctx, username); err == nil {
		return model.User{}, model.ErrUsernameConflict
	} else if err != model.ErrUserNotFound {
		return model.User{}, fmt.Errorf("检查用户名：%w", err)
	}
	if _, err := s.repository.GetUserByEmail(ctx, email); err == nil {
		return model.User{}, model.ErrEmailConflict
	} else if err != model.ErrUserNotFound {
		return model.User{}, fmt.Errorf("检查邮箱：%w", err)
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(input.Password), bcrypt.DefaultCost)
	if err != nil {
		return model.User{}, fmt.Errorf("加密密码：%w", err)
	}
	user, err := s.repository.CreateUser(ctx, model.User{Username: username, Email: email, PasswordHash: string(hash), Status: model.UserStatusActive})
	if err != nil {
		return model.User{}, fmt.Errorf("注册用户：%w", err)
	}
	return user, nil
}
