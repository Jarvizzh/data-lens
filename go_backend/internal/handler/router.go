package handler

import (
	"go_backend/internal/middleware"

	"github.com/gin-gonic/gin"
)

type RouterParams struct {
	AuthHandler       *AuthHandler
	LtvHandler        *LtvHandler
	UserHandler       *UserHandler
	AdminHandler      *AdminHandler
	SettlementHandler *SettlementHandler
	PlatformHandler   *PlatformHandler
	TokenHandler      *TokenHandler
}

func SetupRouter(p RouterParams) *gin.Engine {
	gin.SetMode(gin.ReleaseMode)
	r := gin.New()
	r.Use(gin.Logger())
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
			// 用户与落地页配置
			user := protected.Group("/user")
			{
				user.GET("/visible-accounts", p.UserHandler.GetVisibleAccounts)
				user.GET("/landing-pages", p.UserHandler.GetLandingPages)
				user.POST("/landing-pages", p.UserHandler.UpdateLandingPages)
				user.GET("/all-landing-pages", p.UserHandler.GetAllPlatformLandingPages)
			}

			// LTV 报表
			ltv := protected.Group("/ltv")
			{
				ltv.GET("/list", p.LtvHandler.GetLtvList)
				ltv.GET("/daily-stats", p.LtvHandler.GetLtvList)
				ltv.GET("/daily-distribution", p.LtvHandler.GetDailyDistribution)
				ltv.GET("/global-daily-distribution", p.LtvHandler.GetGlobalDailyDistribution)
				ltv.POST("/config", p.LtvHandler.SaveLaunchConfig)
				ltv.POST("/launch-config", p.LtvHandler.SaveLaunchConfig)
				ltv.POST("/batch-spend", p.LtvHandler.BatchSpend)
				ltv.POST("/recalculate", p.LtvHandler.Recalculate)
				ltv.POST("/recalculate-ltv", p.LtvHandler.Recalculate)
				ltv.POST("/sync-orders", p.LtvHandler.SyncOrders)
				ltv.POST("/sync-and-calc", p.LtvHandler.SyncAndCalc)
			}

			// 月份结算
			settle := protected.Group("/settlement")
			{
				settle.GET("/accounts", p.SettlementHandler.GetAccounts)
				settle.GET("/list", p.SettlementHandler.GetList)
				settle.POST("/save", p.SettlementHandler.SaveConfig)
			}

			// Token 配置
			token := protected.Group("/token")
			{
				token.GET("/get", p.TokenHandler.GetToken)
				token.POST("/update", p.TokenHandler.UpdateToken)
			}

			// 管理员接口
			admin := protected.Group("/admin")
			{
				users := admin.Group("/users")
				{
					users.GET("", p.AdminHandler.ListUsers)
					users.POST("", p.AdminHandler.CreateUser)
					users.DELETE("/:id", p.AdminHandler.DeleteUser)
					users.PUT("/:id/settlement-status", p.AdminHandler.UpdateSettlementStatus)
					users.PUT("/:id/password", p.AdminHandler.ResetPassword)
					users.PUT("/:id/role", p.AdminHandler.UpdateRole)
					users.PUT("/:id/master-status", p.AdminHandler.UpdateMasterStatus)
					users.GET("/:id/view-permissions", p.AdminHandler.GetViewPermissions)
					users.PUT("/:id/view-permissions", p.AdminHandler.UpdateViewPermissions)
					users.GET("/:id/sub-accounts", p.AdminHandler.GetSubAccounts)
					users.PUT("/:id/sub-accounts", p.AdminHandler.UpdateSubAccounts)
					users.PUT("/:id/permissions", p.AdminHandler.UpdatePermissions)
					users.GET("/:id/landing-pages", p.UserHandler.GetLandingPages)
					users.PUT("/:id/landing-pages", p.AdminHandler.UpdateLandingPages)
					users.POST("/:id/landing-pages", p.AdminHandler.UpdateLandingPages)
				}
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
