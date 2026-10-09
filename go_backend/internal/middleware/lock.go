package middleware

import (
	"net/http"
	"time"

	"go_backend/internal/pkg/locker"
	"go_backend/internal/pkg/response"

	"github.com/gin-gonic/gin"
)

// ExclusiveLock 针对 Gin 路由的声明式单键排他中间件
func ExclusiveLock(l *locker.TaskLocker, key string, ttl time.Duration, busyMsg string) gin.HandlerFunc {
	return func(c *gin.Context) {
		if l == nil {
			c.Next()
			return
		}
		unlock, ok, err := l.TryLock(c.Request.Context(), key, ttl)
		if err != nil || !ok {
			response.Error(c, http.StatusConflict, busyMsg)
			c.Abort()
			return
		}
		defer unlock()
		c.Next()
	}
}

// ExclusiveLockMulti 针对 Gin 路由的声明式复合多键排他中间件
func ExclusiveLockMulti(l *locker.TaskLocker, ttl time.Duration, busyMsg string, keys ...string) gin.HandlerFunc {
	return func(c *gin.Context) {
		if l == nil {
			c.Next()
			return
		}
		unlock, ok, err := l.TryLockMulti(c.Request.Context(), ttl, keys...)
		if err != nil || !ok {
			response.Error(c, http.StatusConflict, busyMsg)
			c.Abort()
			return
		}
		defer unlock()
		c.Next()
	}
}
