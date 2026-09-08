package routes

import (
	"fmt"
	"net/http"

	"ggg/controllers"
	"ggg/middleware"

	"github.com/gin-gonic/gin"
)

// New 创建并注册应用路由。
func New(
	healthController *controllers.HealthController,
	productController *controllers.ProductController,
	cartController *controllers.CartController,
	addressController *controllers.AddressController,
	tradeController *controllers.TradeController,
	userController *controllers.UserController,
	jwtSecret string,
) (*gin.Engine, error) {
	// 使用自定义 Zap 请求日志，并保留 Gin 的 panic 恢复中间件。
	router := gin.New()
	router.Use(middleware.RequestLogger(), gin.Recovery())
	// 当前服务只在本地直接访问，不信任任何反向代理传来的客户端 IP 头。
	if err := router.SetTrustedProxies(nil); err != nil {
		return nil, fmt.Errorf("配置可信代理：%w", err)
	}

	// 浏览器会自动请求站点图标，显式返回 204 避免控制台出现 404 噪声；其余未匹配路径统一 JSON 404。
	router.NoRoute(func(c *gin.Context) {
		if c.Request.Method == http.MethodGet && c.Request.URL.Path == "/favicon.ico" {
			c.Status(http.StatusNoContent)
			return
		}
		c.JSON(http.StatusNotFound, gin.H{"code": http.StatusNotFound, "message": "资源不存在"})
	})

	router.GET("/ping", healthController.Ping)
	router.GET("/health/ready", healthController.Ready)
	// 管理平台和 API 由同一个服务提供，因此前端可以直接请求当前域名下的接口。
	// 静态文件本身不含敏感数据（浏览器里看不到他人数据），访问控制由页面内每个 API 调用的
	// JWT + 所有权校验保证，因此这里不做认证，避免 <link> 标签无法携带 Authorization 头的问题。
	router.Static("/admin-ui", "./admin-ui")
	router.GET("/admin", func(c *gin.Context) {
		c.Redirect(http.StatusTemporaryRedirect, "/admin-ui/")
	})
	router.POST("/api/v1/auth/register", userController.Register)
	router.POST("/api/v1/auth/login", userController.Login)

	authenticated := router.Group("/api/v1")
	authenticated.Use(middleware.JWTAuth(jwtSecret))
	authenticated.POST("/cart/items", middleware.RequirePermission(middleware.PermissionCartAddItem), cartController.AddItem)
	authenticated.GET("/cart", middleware.RequirePermission(middleware.PermissionCartRead), cartController.GetCart)
	authenticated.PATCH("/cart/items/:item_id", middleware.RequirePermission(middleware.PermissionCartUpdateItem), cartController.UpdateItemQuantity)
	authenticated.DELETE("/cart/items/:item_id", middleware.RequirePermission(middleware.PermissionCartDeleteItem), cartController.RemoveItem)
	authenticated.POST("/addresses", middleware.RequirePermission(middleware.PermissionAddressCreate), addressController.Create)
	authenticated.GET("/addresses", middleware.RequirePermission(middleware.PermissionAddressRead), addressController.List)
	authenticated.PATCH("/addresses/:id", middleware.RequirePermission(middleware.PermissionAddressUpdate), addressController.Update)
	authenticated.PUT("/addresses/:id/default", middleware.RequirePermission(middleware.PermissionAddressUpdate), addressController.SetDefault)
	authenticated.DELETE("/addresses/:id", middleware.RequirePermission(middleware.PermissionAddressDelete), addressController.Delete)

	// 购物车选择与下单预览（PRD-004 收尾）。
	authenticated.PUT("/cart/selection", middleware.RequirePermission(middleware.PermissionCartUpdateItem), cartController.SetSelection)
	authenticated.POST("/cart/preview", middleware.RequirePermission(middleware.PermissionCartRead), tradeController.CartPreview)

	// 消费者交易链路：订单、支付、退款、优惠券、评价、通知（PRD-005~009）。
	authenticated.POST("/orders", middleware.RequirePermission(middleware.PermissionOrderCreate), tradeController.CreateOrder)
	authenticated.GET("/orders", middleware.RequirePermission(middleware.PermissionOrderRead), tradeController.ListMyOrders)
	authenticated.GET("/orders/:id", middleware.RequirePermission(middleware.PermissionOrderRead), tradeController.GetMyOrder)
	// cancel 与 confirm-receipt 是订单生命周期动作，归 order.cancel 权限；支付单创建归 payment.create。
	authenticated.POST("/orders/:id/cancel", middleware.RequirePermission(middleware.PermissionOrderCancel), tradeController.CancelOrder)
	authenticated.POST("/orders/:id/confirm-receipt", middleware.RequirePermission(middleware.PermissionOrderCancel), tradeController.ConfirmReceipt)
	authenticated.POST("/orders/:id/payments", middleware.RequirePermission(middleware.PermissionPaymentCreate), tradeController.CreatePayment)
	// "按订单查支付单"使用独立的 /payments/:payment_no（凭支付单号查询），不挂在 /orders 路径下。
	authenticated.GET("/payments/:payment_no", middleware.RequirePermission(middleware.PermissionPaymentRead), tradeController.GetMyPayment)
	authenticated.POST("/order-items/:id/refunds", middleware.RequirePermission(middleware.PermissionRefundApply), tradeController.ApplyRefund)
	authenticated.GET("/refunds", middleware.RequirePermission(middleware.PermissionRefundRead), tradeController.ListMyRefunds)
	authenticated.POST("/coupons/:template_id/claim", middleware.RequirePermission(middleware.PermissionCouponClaim), tradeController.ClaimCoupon)
	authenticated.GET("/coupons", middleware.RequirePermission(middleware.PermissionCouponClaim), tradeController.ListMyCoupons)
	authenticated.POST("/order-items/:id/reviews", middleware.RequirePermission(middleware.PermissionReviewCreate), tradeController.CreateReview)
	authenticated.GET("/notifications", middleware.RequirePermission(middleware.PermissionNotificationRead), tradeController.ListNotifications)
	authenticated.PUT("/notifications/:id/read", middleware.RequirePermission(middleware.PermissionNotificationRead), tradeController.MarkNotificationRead)
	authenticated.PUT("/notifications/read-all", middleware.RequirePermission(middleware.PermissionNotificationRead), tradeController.MarkAllNotificationsRead)
	authenticated.POST("/notifications", middleware.RequirePermission(middleware.PermissionNotificationRead), tradeController.CreateNotification)

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

	// 运营端：订单、退款审核、优惠券模板、评价显隐、任务运维（PRD-005~009）。
	admin.GET("/orders", middleware.RequirePermission(middleware.PermissionOrderRead), tradeController.AdminListOrders)
	admin.GET("/orders/:id", middleware.RequirePermission(middleware.PermissionOrderRead), tradeController.AdminGetOrder)
	admin.POST("/orders/:id/ship", middleware.RequirePermission(middleware.PermissionOrderShip), tradeController.AdminShipOrder)
	admin.GET("/refunds", middleware.RequirePermission(middleware.PermissionRefundRead), tradeController.AdminListRefunds)
	admin.POST("/refunds/:id/approve", middleware.RequirePermission(middleware.PermissionRefundReview), tradeController.AdminApproveRefund)
	admin.POST("/refunds/:id/reject", middleware.RequirePermission(middleware.PermissionRefundReview), tradeController.AdminRejectRefund)
	admin.POST("/coupon-templates", middleware.RequirePermission(middleware.PermissionCouponManage), tradeController.CreateCouponTemplate)
	admin.GET("/coupon-templates", middleware.RequirePermission(middleware.PermissionCouponManage), tradeController.ListCouponTemplates)
	admin.PATCH("/coupon-templates/:id", middleware.RequirePermission(middleware.PermissionCouponManage), tradeController.UpdateCouponTemplate)
	admin.DELETE("/coupon-templates/:id", middleware.RequirePermission(middleware.PermissionCouponManage), tradeController.DeleteCouponTemplate)
	admin.GET("/reviews", middleware.RequirePermission(middleware.PermissionReviewRead), tradeController.AdminListReviews)
	admin.PUT("/reviews/:id/visibility", middleware.RequirePermission(middleware.PermissionReviewVisibility), tradeController.SetReviewVisibility)
	admin.GET("/tasks", middleware.RequirePermission(middleware.PermissionTaskRead), tradeController.AdminListTasks)
	admin.POST("/tasks/:id/retry", middleware.RequirePermission(middleware.PermissionTaskRetry), tradeController.AdminRetryTask)

	// 商品公开评价列表不要求 JWT：游客也应能查看口碑，与商品目录的公开语义一致。
	router.GET("/api/v1/products/:id/reviews", tradeController.ListProductReviews)

	return router, nil
}
