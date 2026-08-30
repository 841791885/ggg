package routes

import (
	"fmt"

	"ggg/controllers"
	"ggg/middleware"

	"github.com/gin-gonic/gin"
)

// New 创建并注册应用路由。
func New(
	healthController *controllers.HealthController,
	productController *controllers.ProductController,
	cartController *controllers.CartController,
	userController *controllers.UserController,
	jwtSecret string,
) (*gin.Engine, error) {
	// gin.Default 默认安装访问日志和 panic 恢复两个中间件。
	router := gin.Default()
	// 当前服务只在本地直接访问，不信任任何反向代理传来的客户端 IP 头。
	if err := router.SetTrustedProxies(nil); err != nil {
		return nil, fmt.Errorf("配置可信代理：%w", err)
	}

	router.GET("/ping", healthController.Ping)
	router.GET("/health/ready", healthController.Ready)
	router.POST("/api/v1/auth/register", userController.Register)
	router.POST("/api/v1/auth/login", userController.Login)

	admin := router.Group("/api/v1/admin")
	admin.Use(middleware.JWTAuth(jwtSecret))
	admin.POST("/products", productController.CreateProduct)
	admin.GET("/products", productController.ListProducts)
	admin.GET("/products/:product_id", productController.GetProduct)
	admin.PATCH("/products/:product_id", productController.UpdateProduct)
	admin.DELETE("/products/:product_id", productController.DeleteProduct)

	admin.POST("/products/:product_id/skus", productController.CreateSKU)
	admin.GET("/products/:product_id/skus/:sku_id", productController.GetSKU)
	admin.GET("/products/:product_id/skus", productController.ListSKU)
	admin.PATCH("/products/:product_id/skus/:sku_id", productController.UpdateSKU)
	admin.DELETE("/products/:product_id/skus/:sku_id", productController.DeleteSKU)
	admin.PATCH("/products/:product_id/skus/:sku_id/status", productController.UpdateSKUStatus)

	admin.POST("/cart/items", cartController.AddItem)
	admin.GET("/cart", cartController.GetCart)

	return router, nil
}
