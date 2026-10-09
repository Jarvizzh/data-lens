package middleware

import (
	"strings"

	"go_backend/internal/config"
	"go_backend/internal/pkg/crypto"
	"go_backend/internal/pkg/response"

	"github.com/gin-gonic/gin"
)

const ContextUserKey = "currentUser"

// AuthMiddleware 验证 Authorization Token (与 Java AuthInterceptor 逻辑对齐)
func AuthMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		if c.Request.Method == "OPTIONS" {
			c.Next()
			return
		}

		authHeader := c.GetHeader("Authorization")
		token := ""
		if strings.HasPrefix(authHeader, "Bearer ") {
			token = strings.TrimSpace(authHeader[7:])
		} else if authHeader != "" {
			token = strings.TrimSpace(authHeader)
		}

		tokenInfo := crypto.ParseToken(token, config.GetGlobalConfig().App.Auth.SecretKey)
		if !tokenInfo.Valid {
			response.Unauthorized(c, "未登录或 Token 已过期 (3天)，请重新登录")
			c.Abort()
			return
		}

		c.Set(ContextUserKey, tokenInfo)
		c.Next()
	}
}

// GetCurrentUser 从 Gin Context 提取已登录用户信息
func GetCurrentUser(c *gin.Context) *crypto.TokenInfo {
	if val, ok := c.Get(ContextUserKey); ok {
		if info, ok := val.(crypto.TokenInfo); ok {
			return &info
		}
	}
	return nil
}
