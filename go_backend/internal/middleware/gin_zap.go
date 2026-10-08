package middleware

import (
	"net"
	"net/http"
	"net/http/httputil"
	"os"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

// GinLogger 使用 Zap 记录 Gin HTTP 请求访问日志，并根据 HTTP 响应状态码自动分流级别:
// - status < 400: DEBUG 级别 (系统默认 info 级别时自动过滤常规 200 请求，需要排查耗时可开 debug)
// - 400 <= status < 500: WARN 级别 (客户端错误，如 401, 404, 400)
// - status >= 500: ERROR 级别 (服务端错误)
func GinLogger(logger *zap.Logger) gin.HandlerFunc {
	if logger == nil {
		logger = zap.NewNop()
	}

	return func(c *gin.Context) {
		start := time.Now()
		path := c.Request.URL.Path
		query := c.Request.URL.RawQuery

		c.Next()

		cost := time.Since(start)
		status := c.Writer.Status()
		clientIP := c.ClientIP()
		method := c.Request.Method

		// 收集结构化字段
		fields := []zap.Field{
			zap.Int("status", status),
			zap.String("method", method),
			zap.String("path", path),
			zap.Duration("cost", cost),
			zap.String("ip", clientIP),
		}

		if query != "" {
			fields = append(fields, zap.String("query", query))
		}

		if len(c.Errors) > 0 {
			fields = append(fields, zap.String("errors", c.Errors.ByType(gin.ErrorTypePrivate).String()))
		}

		switch {
		case status >= http.StatusInternalServerError:
			logger.Error("HTTP Request Error", fields...)
		case status >= http.StatusBadRequest:
			logger.Warn("HTTP Request Warning", fields...)
		default:
			logger.Debug("HTTP Request", fields...)
		}
	}
}

// GinRecovery 使用 Zap 捕获 HTTP 处理中的 panic，记录堆栈并返回 500
func GinRecovery(logger *zap.Logger, stack bool) gin.HandlerFunc {
	if logger == nil {
		logger = zap.NewNop()
	}

	return func(c *gin.Context) {
		defer func() {
			if err := recover(); err != nil {
				// 检查连接是否断开 (broken pipe)
				var brokenPipe bool
				if ne, ok := err.(*net.OpError); ok {
					if se, ok := ne.Err.(*os.SyscallError); ok {
						if strings.Contains(strings.ToLower(se.Error()), "broken pipe") ||
							strings.Contains(strings.ToLower(se.Error()), "connection reset by peer") {
							brokenPipe = true
						}
					}
				}

				httpRequest, _ := httputil.DumpRequest(c.Request, false)
				if brokenPipe {
					logger.Error("Broken pipe in HTTP request",
						zap.Any("error", err),
						zap.String("request", string(httpRequest)),
					)
					_ = c.Error(err.(error))
					c.Abort()
					return
				}

				fields := []zap.Field{
					zap.Any("error", err),
					zap.String("request", string(httpRequest)),
				}
				if stack {
					fields = append(fields, zap.Stack("stacktrace"))
				}

				logger.Error("HTTP Panic Recovered", fields...)
				c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{
					"code": 500,
					"msg":  "服务器内部异常",
				})
			}
		}()
		c.Next()
	}
}
