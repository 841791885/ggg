package repositories

// MySQLRepository 是所有数据访问实现的基座：一个持有 GORM 连接的结构体。
//
// 各领域的方法（商品、订单、支付……）按业务拆在独立的 *_repository.go 文件里，
// 但全部挂在同一个类型上（Go 允许方法定义分散在同包不同文件）。
// 这样做的前提是跨领域事务（如"扣库存+写订单"）必须共享同一个 db 连接——
// 见 repositories/order_repository.go 里 CreateOrder 的事务实现。
//
// 若将来要换数据库，新建一个 XxxRepository 实现同一套接口即可，service 层零改动。

import (
	"gorm.io/gorm"
)

// MySQLRepository 使用 GORM 将领域模型持久化到 MySQL。
type MySQLRepository struct {
	db *gorm.DB
}

// 编译期检查 MySQLRepository 是否完整实现了 Repository 接口。
// 少写任何一个方法，这里会直接编译报错——比运行时才发现少了方法友好得多。
var _ Repository = (*MySQLRepository)(nil)

// NewMySQLRepository 创建 MySQL 仓库基座。
func NewMySQLRepository(db *gorm.DB) *MySQLRepository {
	return &MySQLRepository{db: db}
}
