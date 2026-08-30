package middleware

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

const (
	PermissionProductCreate = "product.create"
	PermissionProductRead   = "product.read"
	PermissionProductUpdate = "product.update"
	PermissionProductDelete = "product.delete"
	PermissionSKUCreate     = "sku.create"
	PermissionSKURead       = "sku.read"
	PermissionSKUUpdate     = "sku.update"
	PermissionSKUDelete     = "sku.delete"
	PermissionCartRead      = "cart.read"
	PermissionCartAddItem   = "cart.item.create"
)

var rolePermissions = map[string]map[string]struct{}{
	"admin": {
		PermissionProductCreate: {}, PermissionProductRead: {}, PermissionProductUpdate: {}, PermissionProductDelete: {},
		PermissionSKUCreate: {}, PermissionSKURead: {}, PermissionSKUUpdate: {}, PermissionSKUDelete: {},
		PermissionCartRead: {}, PermissionCartAddItem: {},
	},
	"customer": {
		PermissionCartRead: {}, PermissionCartAddItem: {},
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
