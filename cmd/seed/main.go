// seed 是开发环境的初始化工具命令：向数据库注入"系统内无法自助产生"的必需数据。
//
// 用法:
//
//	go run ./cmd/seed                    # 默认创建 admin / 密码取环境变量或默认值
//	GOMALL_ADMIN_PASSWORD=xxx go run ./cmd/seed
//
// 为什么需要它：注册接口（POST /auth/register）被设计成只能创建 customer——
// PRD-003 规定运营不能自助提权，所以整个系统没有任何 HTTP 途径能产生第一个管理员。
// 新环境（另一台电脑、CI、重装）跑完 init-db 后表结构齐全但无人可登录，
// 这个命令就是补上"第一颗种子"。幂等：admin 已存在则跳过，重复执行安全。
package main

import (
	"context"
	"errors"
	"fmt"
	"os"

	"golang.org/x/crypto/bcrypt"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	gormlogger "gorm.io/gorm/logger"

	appConfig "ggg/config"
	model "ggg/models"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "seed 失败:", err)
		os.Exit(1)
	}
}

func run() error {
	// 复用应用的配置读取（同一份 config.yaml），避免连接参数两处维护。
	configPath := appConfig.Path()
	if _, err := os.Stat(configPath); err != nil {
		configPath = "../config.yaml" // 从 cmd/seed 目录直接 go run 时的兜底
	}
	cfg, err := appConfig.Load(configPath)
	if err != nil {
		return fmt.Errorf("加载配置：%w", err)
	}

	password := os.Getenv("GOMALL_ADMIN_PASSWORD")
	if password == "" {
		password = "12345678" // 仅限本地开发；生产部署必须用环境变量覆盖
		fmt.Println("⚠️ 未设置 GOMALL_ADMIN_PASSWORD，使用默认开发密码 12345678")
	}
	if len(password) < 8 {
		return errors.New("密码长度至少 8 位（与注册接口的校验一致）")
	}

	db, err := gorm.Open(mysql.Open(cfg.Database.DSN()), &gorm.Config{
		Logger: gormlogger.Default.LogMode(gormlogger.Silent),
	})
	if err != nil {
		return fmt.Errorf("连接数据库：%w（先执行 ./scripts/init-db.sh）", err)
	}
	ctx := context.Background()

	var existing model.User
	err = db.WithContext(ctx).Where("username = ?", "admin").Take(&existing).Error
	switch {
	case err == nil:
		fmt.Printf("✓ admin 已存在（id=%d, role=%s），跳过\n", existing.ID, existing.Role)
		return nil
	case !errors.Is(err, gorm.ErrRecordNotFound):
		return fmt.Errorf("查询 admin：%w", err)
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return fmt.Errorf("生成密码哈希：%w", err)
	}
	admin := model.User{
		Username:     "admin",
		Email:        "admin@gomall.local",
		PasswordHash: string(hash),
		Role:         model.UserRoleAdmin, // 唯一能写出 admin 角色的地方——离线工具，绕开 HTTP 层的防提权设计
		Status:       model.UserStatusActive,
	}
	if err := db.WithContext(ctx).Create(&admin).Error; err != nil {
		return fmt.Errorf("创建 admin：%w", err)
	}
	fmt.Println("✓ admin 创建成功（用户名 admin，密码见上方提示）")
	return nil
}
