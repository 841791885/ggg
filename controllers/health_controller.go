package controllers

import (
	"context"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
)

type databasePinger interface {
	PingContext(ctx context.Context) error
}

// HealthController 处理基础存活和数据库就绪检查。
// 这里只依赖最小的 databasePinger 接口，因此测试时不需要真的启动 MySQL。
type HealthController struct {
	database databasePinger
	timeout  time.Duration
}

// NewHealthController 创建健康检查控制器。
func NewHealthController(database databasePinger, timeout time.Duration) *HealthController {
	return &HealthController{database: database, timeout: timeout}
}

// Ping 检查 HTTP 服务进程是否存活，不访问数据库。
// Ping 返回基础存活响应。
func (h *HealthController) Ping(c *gin.Context) {
	respondSuccess(c, http.StatusOK, gin.H{"message": "pong"})
}

// Ready 检查 MySQL 连接是否可用。
func (h *HealthController) Ready(c *gin.Context) {
	// 健康检查必须快速返回，防止数据库异常时请求长期占用 HTTP 连接。
	ctx, cancel := context.WithTimeout(c.Request.Context(), h.timeout)
	defer cancel()

	if err := h.database.PingContext(ctx); err != nil {
		respondError(
			c,
			http.StatusServiceUnavailable,
			"数据库暂时不可用",
		)
		return
	}
	respondSuccess(c, http.StatusOK, gin.H{"status": "ok"})
}
