package repositories

import (
	"context"
	"errors"
	"fmt"

	model "ggg/models"
	"gorm.io/gorm"
)

// GetUserByUsername 根据用户名查询用户。
func (r *MySQLRepository) GetUserByUsername(ctx context.Context, username string) (model.User, error) {
	var user model.User
	err := r.db.WithContext(ctx).Where("username = ?", username).First(&user).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return model.User{}, model.ErrUserNotFound
	}
	if err != nil {
		return model.User{}, fmt.Errorf("查询用户：%w", err)
	}
	return user, nil
}

// GetUserByLogin 根据用户名或邮箱查询用户。
func (r *MySQLRepository) GetUserByLogin(ctx context.Context, login string) (model.User, error) {
	var user model.User
	err := r.db.WithContext(ctx).Where("username = ? OR email = ?", login, login).First(&user).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return model.User{}, model.ErrUserNotFound
	}
	if err != nil {
		return model.User{}, fmt.Errorf("查询登录用户：%w", err)
	}
	return user, nil
}

// GetUserByEmail 根据邮箱查询用户。
func (r *MySQLRepository) GetUserByEmail(ctx context.Context, email string) (model.User, error) {
	var user model.User
	err := r.db.WithContext(ctx).Where("email = ?", email).First(&user).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return model.User{}, model.ErrUserNotFound
	}
	if err != nil {
		return model.User{}, fmt.Errorf("查询用户：%w", err)
	}
	return user, nil
}

// CreateUser 将用户持久化到 users 表，并转换唯一键冲突错误。
func (r *MySQLRepository) CreateUser(ctx context.Context, user model.User) (model.User, error) {
	if err := r.db.WithContext(ctx).Create(&user).Error; err != nil {
		if errors.Is(err, gorm.ErrDuplicatedKey) {
			return model.User{}, model.ErrUsernameConflict
		}
		return model.User{}, fmt.Errorf("创建用户：%w", err)
	}
	return user, nil
}

// 编译期检查：本文件负责的领域接口是否都实现了。
// 少写方法时错误直接指向这里，而不是 mysql_repository.go 里的全量断言。
var _ UserRepository = (*MySQLRepository)(nil)
