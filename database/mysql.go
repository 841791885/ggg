package database

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"ggg/config"

	"gorm.io/driver/mysql"
	"gorm.io/gorm"
)

// OpenMySQL 创建 GORM 和底层 database/sql 连接池，并确认数据库可以访问。
func OpenMySQL(ctx context.Context, databaseConfig config.DatabaseConfig) (*gorm.DB, *sql.DB, error) {
	db, err := gorm.Open(mysql.Open(databaseConfig.DSN()), &gorm.Config{
		// 将唯一键冲突等数据库错误转换为 GORM 的通用错误，便于 Repository 判断。
		TranslateError: true,
		// 开发阶段使用 Info，输出每次 GORM 操作生成的 SQL、耗时和影响行数。
		// Logger: logger.Default.LogMode(logger.Info),
		// GORM 自动填写 CreatedAt、UpdatedAt 时统一使用 UTC。
		NowFunc: func() time.Time {
			return time.Now().UTC()
		},
	})
	if err != nil {
		return nil, nil, fmt.Errorf("创建 GORM MySQL 连接：%w", err)
	}

	// GORM 负责 ORM 操作，底层的 database/sql.DB 才是真正管理连接池的对象。
	sqlDB, err := db.DB()
	if err != nil {
		return nil, nil, fmt.Errorf("获取 MySQL 连接池：%w", err)
	}

	sqlDB.SetMaxOpenConns(databaseConfig.MaxOpenConnections)
	sqlDB.SetMaxIdleConns(databaseConfig.MaxIdleConnections)
	sqlDB.SetConnMaxLifetime(databaseConfig.ConnectionMaxLifetime.Value())
	sqlDB.SetConnMaxIdleTime(databaseConfig.ConnectionMaxIdleTime.Value())

	// gorm.Open 不保证此刻已经成功访问数据库，Ping 才会真正验证连接。
	if err := sqlDB.PingContext(ctx); err != nil {
		sqlDB.Close()
		return nil, nil, fmt.Errorf("连接 MySQL: %w", err)
	}

	return db, sqlDB, nil
}
