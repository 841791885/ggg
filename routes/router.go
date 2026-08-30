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

	authenticated := router.Group("/api/v1")
	authenticated.Use(middleware.JWTAuth(jwtSecret))
	authenticated.POST("/cart/items", middleware.RequirePermission(middleware.PermissionCartAddItem), cartController.AddItem)
	authenticated.GET("/cart", middleware.RequirePermission(middleware.PermissionCartRead), cartController.GetCart)

	admin := authenticated.Group("/admin")
	admin.POST("/products", middleware.RequirePermission(middleware.PermissionProductCreate), productController.CreateProduct)
	admin.GET("/products", middleware.RequirePermission(middleware.PermissionProductRead), productController.ListProducts)
	admin.GET("/products/:product_id", middleware.RequirePermission(middleware.PermissionProductRead), productController.GetProduct)
	admin.PATCH("/products/:product_id", middleware.RequirePermission(middleware.PermissionProductUpdate), productController.UpdateProduct)
	admin.DELETE("/products/:product_id", middleware.RequirePermission(middleware.PermissionProductDelete), productController.DeleteProduct)

	admin.POST("/products/:product_id/skus", middleware.RequirePermission(middleware.PermissionSKUCreate), productController.CreateSKU)
	admin.GET("/products/:product_id/skus/:sku_id", middleware.RequirePermission(middleware.PermissionSKURead), productController.GetSKU)
	admin.GET("/products/:product_id/skus", middleware.RequirePermission(middleware.PermissionSKURead), productController.ListSKU)
	admin.PATCH("/products/:product_id/skus/:sku_id", middleware.RequirePermission(middleware.PermissionSKUUpdate), productController.UpdateSKU)
	admin.DELETE("/products/:product_id/skus/:sku_id", middleware.RequirePermission(middleware.PermissionSKUDelete), productController.DeleteSKU)
	admin.PATCH("/products/:product_id/skus/:sku_id/status", middleware.RequirePermission(middleware.PermissionSKUUpdate), productController.UpdateSKUStatus)

	return router, nil
}
