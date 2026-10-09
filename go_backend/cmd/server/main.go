package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"go_backend/internal/config"
	"go_backend/internal/cron"
	"go_backend/internal/handler"
	"go_backend/internal/pkg/locker"
	"go_backend/internal/pkg/logger"
	"go_backend/internal/pkg/pool"
	"go_backend/internal/repository"
	"go_backend/internal/service"
	"go_backend/internal/service/client/flicknovel"
	"go_backend/internal/service/client/rocnovel"

	"github.com/shopspring/decimal"
	"go.uber.org/zap"
)

func main() {
	// 启用数值形式序列化 (对齐 Java Jackson BigDecimal 行为)
	decimal.MarshalJSONWithoutQuotes = true

	// 1. 加载配置
	configPath := "configs/config.yaml"
	if envPath := os.Getenv("CONFIG_PATH"); envPath != "" {
		configPath = envPath
	}
	cfg, err := config.LoadConfig(configPath)
	if err != nil {
		fmt.Printf("Load config failed: %v\n", err)
		os.Exit(1)
	}

	// 2. 初始化结构化/终端友好日志 (根据配置自动切换 Console/JSON 模式)
	logInstance, err := logger.NewLogger(&cfg.Logger)
	if err != nil {
		fmt.Printf("Init logger failed: %v\n", err)
		os.Exit(1)
	}
	defer logInstance.Sync()
	logger := logInstance

	logger.Info("Starting DataLens (data-lens) Go Backend...")

	// 2.1 初始化全局业务协程池与三方 API 协程池 (舱壁隔离架构)
	pool.InitGlobalPools(logger)

	// 3. 初始化数据库连接池
	maskedDSN := cfg.Database.MySQL.DSN
	if atIdx := strings.Index(maskedDSN, "@"); atIdx != -1 {
		maskedDSN = "***@" + maskedDSN[atIdx+1:]
	}
	logger.Info("Connecting to MySQL...", zap.String("target", maskedDSN))

	db, err := repository.InitDB(&cfg.Database.MySQL)
	if err != nil {
		logger.Fatal("Init database failed", zap.Error(err))
	}
	logger.Info("Database connected successfully.")

	// 3.1 初始化分布式/进程内双层任务排他锁管理器
	taskLocker := locker.NewTaskLocker(db, logger)

	// 4. 初始化仓储层
	userRepo := repository.NewUserRepository(db)
	orderRepo := repository.NewOrderRepository(db)
	ltvStatRepo := repository.NewLtvStatRepository(db)
	rechargeDistRepo := repository.NewRechargeDistributionRepository(db)
	platformRepo := repository.NewPlatformRepository(db)
	benchmarkRepo := repository.NewBenchmarkRepository(db)
	settleRepo := repository.NewSettlementRepository(db)
	flicknovelRepo := repository.NewFlicknovelRepository(db)

	// 5. 初始化三方客户端
	rocnovelClient := rocnovel.NewClient(&cfg.Order.API)
	flicknovelClient := flicknovel.NewClient(&cfg.Flicknovel.API)

	// 6. 初始化业务服务层
	userSvc := service.NewUserService(userRepo, orderRepo, flicknovelRepo)
	if err := userSvc.InitDefaultUsers(context.Background(), cfg.App.Auth.Username, cfg.App.Auth.Password); err != nil {
		logger.Fatal("Initialize superadmin user failed", zap.Error(err))
	}
	permSvc := service.NewUserPermissionService(userRepo)
	predictSvc := service.NewPredictService(benchmarkRepo)
	predictSvc.SetDependencies(orderRepo, userRepo, userSvc)
	ltvCache := service.NewLtvMemoryCache(30 * time.Minute)
	calculator := service.NewLtvCalculator(orderRepo, ltvStatRepo, userRepo, predictSvc)
	monthlySummarySvc := service.NewMonthlySummaryService()
	ltvSvc := service.NewLtvService(calculator, ltvStatRepo, orderRepo, userRepo, userSvc, predictSvc, ltvCache, monthlySummarySvc)
	rechargeSvc := service.NewRechargeStatService(orderRepo, userRepo, userSvc, rechargeDistRepo)
	dailyDistSvc := service.NewDailyDistributionService(rechargeDistRepo, orderRepo, userRepo, rechargeSvc)
	ltvSvc.SetRechargeDistService(dailyDistSvc)
	syncMgr := service.NewSyncManager(orderRepo, flicknovelRepo, platformRepo, rocnovelClient, flicknovelClient, logger)
	settleSvc := service.NewSettlementService(settleRepo, orderRepo, userRepo, userSvc)

	// 7. 启动定时任务调度器 (传入 taskLocker 杜绝任务重叠与重入)
	scheduler := cron.NewTaskScheduler(syncMgr, ltvSvc, rechargeSvc, userSvc, predictSvc, ltvCache, taskLocker, logger)
	if err := scheduler.Start(); err != nil {
		logger.Error("Start task scheduler failed", zap.Error(err))
	}

	flicknovelHandler := handler.NewFlicknovelHandler(syncMgr, platformRepo, taskLocker)

	// 8. 装配 HTTP 控制器与路由
	r := handler.SetupRouter(handler.RouterParams{
		AuthHandler:       handler.NewAuthHandler(userSvc),
		LtvHandler:        handler.NewLtvHandler(ltvSvc, dailyDistSvc, permSvc, syncMgr, predictSvc, taskLocker),
		UserHandler:       handler.NewUserHandler(userSvc, permSvc, ltvSvc),
		AdminHandler:      handler.NewAdminHandler(userSvc, permSvc, ltvSvc),
		SettlementHandler: handler.NewSettlementHandler(settleSvc, permSvc, userRepo),
		PlatformHandler:   handler.NewPlatformHandler(platformRepo, userRepo, syncMgr, taskLocker),
		TokenHandler:      handler.NewTokenHandler(rocnovelClient, platformRepo),
		FlicknovelHandler: flicknovelHandler,
		Logger:            logger,
	})

	srv := &http.Server{
		Addr:    fmt.Sprintf(":%d", cfg.Server.Port),
		Handler: r,
	}

	// 9. 启动 HTTP 服务
	go func() {
		logger.Info("HTTP server listening", zap.Int("port", cfg.Server.Port))
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Fatal("HTTP server listen failed", zap.Error(err))
		}
	}()

	// 10. 优雅停机 (按依赖逆序平滑释放)
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	logger.Info("Shutting down server gracefully...")

	// 对齐主流云原生 (K8s terminationGracePeriodSeconds) 标准，放宽优雅停机总超时至 30 秒
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// 10.1 停止接收新 HTTP 流量
	if err := srv.Shutdown(shutdownCtx); err != nil {
		logger.Error("HTTP server forced to shutdown", zap.Error(err))
	} else {
		logger.Info("HTTP server stopped cleanly.")
	}

	// 10.2 停止定时任务调度器并等待在途任务完成
	if err := scheduler.StopWait(shutdownCtx); err != nil {
		logger.Warn("Scheduler stop timed out or interrupted", zap.Error(err))
	}

	// 10.3 优雅关闭全局业务协程池与三方调用协程池，等待在途任务执行完成
	if err := pool.ShutdownGlobal(shutdownCtx); err != nil {
		logger.Warn("Worker pools shutdown timed out or interrupted", zap.Error(err))
	}

	// 10.4 释放数据库连接池
	if sqlDB, err := db.DB(); err == nil {
		_ = sqlDB.Close()
		logger.Info("Database connection pool closed.")
	}

	logger.Info("Server exited cleanly.")
}
