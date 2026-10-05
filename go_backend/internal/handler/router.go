package handler

import (
	"go_backend/internal/middleware"

	"github.com/gin-gonic/gin"
)

type RouterParams struct {
	AuthHandler       *AuthHandler
	LtvHandler        *LtvHandler
	UserHandler       *UserHandler
	SettlementHandler *SettlementHandler
	PlatformHandler   *PlatformHandler
}

func SetupRouter(p RouterParams) *gin.Engine {
	gin.SetMode(gin.ReleaseMode)
	r := gin.New()
	r.Use(gin.Recovery())
	r.Use(middleware.CORSMiddleware())

	api := r.Group("/api")
	{
		// 1. 认证公开接口
		auth := api.Group("/auth")
		{
			auth.POST("/login", p.AuthHandler.Login)
			auth.GET("/check", p.AuthHandler.Check)
		}

		// 2. 需要 Token 鉴权的受保护接口
		protected := api.Group("/")
		protected.Use(middleware.AuthMiddleware())
		{
			// LTV 报表
			ltv := protected.Group("/ltv")
			{
				ltv.GET("/daily-stats", p.LtvHandler.GetDailyStats)
				ltv.POST("/recalculate", p.LtvHandler.Recalculate)
				ltv.POST("/launch-config", p.LtvHandler.SaveLaunchConfig)
				ltv.GET("/daily-distribution", p.LtvHandler.GetDailyDistribution)
			}

			// 用户落地页配置
			userConfig := protected.Group("/user/config")
			{
				userConfig.GET("/landing-pages", p.UserHandler.GetLandingPages)
				userConfig.POST("/landing-pages", p.UserHandler.UpdateLandingPages)
			}

			// 管理员接口
			admin := protected.Group("/admin")
			{
				admin.GET("/users", p.UserHandler.ListUsers)
				admin.POST("/users", p.UserHandler.CreateUser)
			}

			// 月份结算
			settle := protected.Group("/settlement")
			{
				settle.GET("/configs", p.SettlementHandler.GetConfigs)
				settle.POST("/save", p.SettlementHandler.SaveConfig)
			}

			// 多平台与同步
			platform := protected.Group("/platform")
			{
				platform.GET("/list", p.PlatformHandler.ListPlatforms)
				platform.POST("/sync", p.PlatformHandler.TriggerSync)
			}
		}
	}

	return r
}
