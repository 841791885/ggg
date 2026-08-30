package middleware

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt"
)

// JWTAuth 验证 Bearer JWT，并将用户 ID 写入 Gin Context。
func JWTAuth(secret string) gin.HandlerFunc {
	return func(c *gin.Context) {
		header := c.GetHeader("Authorization")
		parts := strings.Fields(header)
		if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"code": http.StatusUnauthorized, "message": "缺少有效的 Authorization"})
			return
		}
		token, err := jwt.Parse(parts[1], func(token *jwt.Token) (interface{}, error) {
			if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
				return nil, jwt.ErrSignatureInvalid
			}
			return []byte(secret), nil
		})
		if err != nil || !token.Valid {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"code": http.StatusUnauthorized, "message": "登录凭证无效或已过期"})
			return
		}
		claims, ok := token.Claims.(jwt.MapClaims)
		if !ok {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"code": http.StatusUnauthorized, "message": "登录凭证无效"})
			return
		}
		sub, ok := claims["sub"].(float64)
		if !ok || sub <= 0 {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"code": http.StatusUnauthorized, "message": "登录凭证缺少用户信息"})
			return
		}
		c.Set("user_id", uint64(sub))
		role, ok := claims["role"].(string)
		if !ok || role == "" {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"code": http.StatusUnauthorized, "message": "登录凭证缺少角色信息"})
			return
		}
		c.Set("user_role", role)
		c.Next()
	}
}

// RequireRole 只允许指定角色继续访问后续路由。
func RequireRole(allowedRoles ...string) gin.HandlerFunc {
	allowed := make(map[string]struct{}, len(allowedRoles))
	for _, role := range allowedRoles {
		allowed[role] = struct{}{}
	}
	return func(c *gin.Context) {
		role, exists := c.Get("user_role")
		roleName, ok := role.(string)
		if !exists || !ok {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"code": http.StatusUnauthorized, "message": "用户身份不存在"})
			return
		}
		if _, ok := allowed[roleName]; !ok {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"code": http.StatusForbidden, "message": "没有访问该资源的权限"})
			return
		}
		c.Next()
	}
}
