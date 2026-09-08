package middleware

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

const (
	PermissionProductCreate  = "product.create"
	PermissionProductRead    = "product.read"
	PermissionProductUpdate  = "product.update"
	PermissionProductDelete  = "product.delete"
	PermissionSKUCreate      = "sku.create"
	PermissionSKURead        = "sku.read"
	PermissionSKUUpdate      = "sku.update"
	PermissionSKUDelete      = "sku.delete"
	PermissionCartRead       = "cart.read"
	PermissionCartAddItem    = "cart.item.create"
	PermissionCartUpdateItem = "cart.item.update"
	PermissionCartDeleteItem = "cart.item.delete"
	PermissionAddressCreate  = "address.create"
	PermissionAddressRead    = "address.read"
	PermissionAddressUpdate  = "address.update"
	PermissionAddressDelete  = "address.delete"
	// 交易链路权限：消费者私有数据（订单/支付/退款/券/评价/通知）与运营操作分离。
	PermissionOrderCreate      = "order.create"
	PermissionOrderRead        = "order.read"
	PermissionOrderCancel      = "order.cancel"
	PermissionOrderShip        = "order.ship"
	PermissionPaymentCreate    = "payment.create"
	PermissionPaymentRead      = "payment.read"
	PermissionRefundApply      = "refund.apply"
	PermissionRefundRead       = "refund.read"
	PermissionRefundReview     = "refund.review"
	PermissionCouponClaim      = "coupon.claim"
	PermissionCouponManage     = "coupon.manage"
	PermissionReviewCreate     = "review.create"
	PermissionReviewRead       = "review.read"
	PermissionReviewVisibility = "review.visibility"
	PermissionNotificationRead = "notification.read"
	PermissionTaskRead         = "task.read"
	PermissionTaskRetry        = "task.retry"
)

var rolePermissions = map[string]map[string]struct{}{
	"admin": {
		PermissionProductCreate: {}, PermissionProductRead: {}, PermissionProductUpdate: {}, PermissionProductDelete: {},
		PermissionSKUCreate: {}, PermissionSKURead: {}, PermissionSKUUpdate: {}, PermissionSKUDelete: {},
		PermissionCartRead: {}, PermissionCartAddItem: {}, PermissionCartUpdateItem: {}, PermissionCartDeleteItem: {},
		PermissionAddressCreate: {}, PermissionAddressRead: {}, PermissionAddressUpdate: {}, PermissionAddressDelete: {},
		// admin 同时持有消费者侧权限：登录用户就是自己的私有数据（订单/支付/券等）的主人，
		// 所有权仍由 repository 层 user_id 过滤保证，admin 也无法越权访问他人数据。
		PermissionOrderCreate: {}, PermissionOrderCancel: {}, PermissionPaymentCreate: {}, PermissionPaymentRead: {},
		PermissionRefundApply: {}, PermissionCouponClaim: {}, PermissionReviewCreate: {}, PermissionNotificationRead: {},
		// 运营侧：发货、退款审核、券管理、评价显隐、任务运维。
		PermissionOrderRead: {}, PermissionOrderShip: {}, PermissionRefundRead: {}, PermissionRefundReview: {},
		PermissionCouponManage: {}, PermissionReviewVisibility: {}, PermissionTaskRead: {}, PermissionTaskRetry: {},
	},
	"customer": {
		PermissionCartRead: {}, PermissionCartAddItem: {}, PermissionCartUpdateItem: {}, PermissionCartDeleteItem: {},
		PermissionAddressCreate: {}, PermissionAddressRead: {}, PermissionAddressUpdate: {}, PermissionAddressDelete: {},
		// 消费者侧：全部围绕自己的私有交易数据，所有权由 repository 层过滤。
		PermissionOrderCreate: {}, PermissionOrderRead: {}, PermissionOrderCancel: {},
		PermissionPaymentCreate: {}, PermissionPaymentRead: {},
		PermissionRefundApply: {}, PermissionRefundRead: {},
		PermissionCouponClaim: {}, PermissionReviewCreate: {}, PermissionReviewRead: {},
		PermissionNotificationRead: {},
	},
}

// RequirePermission 检查当前用户角色是否拥有指定权限点。
func RequirePermission(permission string) gin.HandlerFunc {
	return func(c *gin.Context) {
		roleValue, exists := c.Get("user_role")
		role, ok := roleValue.(string)
		if !exists || !ok {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"code": http.StatusUnauthorized, "message": "用户身份不存在"})
			return
		}
		permissions, exists := rolePermissions[role]
		if !exists {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"code": http.StatusForbidden, "message": "用户角色没有可用权限"})
			return
		}
		if _, allowed := permissions[permission]; !allowed {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"code": http.StatusForbidden, "message": "缺少权限：" + permission})
			return
		}
		c.Next()
	}
}
