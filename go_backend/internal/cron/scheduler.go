package cron

import (
	"context"
	"fmt"
	"time"

	"go_backend/internal/pkg/timeutil"
	"go_backend/internal/service"

	"github.com/robfig/cron/v3"
	"go.uber.org/zap"
)

type TaskScheduler struct {
	cron        *cron.Cron
	syncMgr     *service.SyncManager
	ltvSvc      *service.LtvService
	rechargeSvc *service.RechargeStatService
	userSvc     *service.UserService
	predictSvc  *service.PredictService
	cache       *service.LtvMemoryCache
	logger      *zap.Logger
}

type cronZapLogger struct {
	logger *zap.Logger
}

func newCronZapLogger(l *zap.Logger) cron.Logger {
	if l == nil {
		l = zap.NewNop()
	}
	return &cronZapLogger{logger: l.Named("cron")}
}

func (l *cronZapLogger) Info(msg string, keysAndValues ...interface{}) {
	l.logger.Sugar().Infow(msg, keysAndValues...)
}

func (l *cronZapLogger) Error(err error, msg string, keysAndValues ...interface{}) {
	l.logger.Sugar().Errorw(msg, append(keysAndValues, "error", err)...)
}

func NewTaskScheduler(
	syncMgr *service.SyncManager,
	ltvSvc *service.LtvService,
	rechargeSvc *service.RechargeStatService,
	userSvc *service.UserService,
	predictSvc *service.PredictService,
	cache *service.LtvMemoryCache,
	logger *zap.Logger,
) *TaskScheduler {
	cl := newCronZapLogger(logger)
	c := cron.New(
		cron.WithLocation(timeutil.BeijingZone),
		cron.WithChain(
			cron.Recover(cl),            // 捕获任务 panic，防止调度器循环崩溃
			cron.SkipIfStillRunning(cl), // 若前序任务仍在执行，自动跳过本次调度，杜绝重叠！
		),
	)
	return &TaskScheduler{
		cron:        c,
		syncMgr:     syncMgr,
		ltvSvc:      ltvSvc,
		rechargeSvc: rechargeSvc,
		userSvc:     userSvc,
		predictSvc:  predictSvc,
		cache:       cache,
		logger:      logger,
	}
}

func (s *TaskScheduler) Start() error {
	ctx := context.Background()

	// 1. 每 4 小时 15 分 (Java 为 0 */4 * * *): 拉取番茄司南所有推广链接与充值模板，并自动导入全量推广ID给管理员
	_, err := s.cron.AddFunc("15 */4 * * *", func() {
		s.logger.Info("Starting scheduled Flicknovel promotions & templates sync (every 4h +15m)...")
		if err := s.syncMgr.SyncFlicknovelPromotionsAndTemplates(ctx); err != nil {
			s.logger.Error("Scheduled Flicknovel promotions sync failed", zap.Error(err))
		}
		if s.userSvc != nil {
			if count, err := s.userSvc.AutoImportFlicknovelLandingPagesForAdmins(ctx); err == nil && count > 0 {
				s.logger.Info("Scheduled Flicknovel admin landing page auto-import completed", zap.Int("imported", count))
			}
		}
	})
	if err != nil {
		return fmt.Errorf("register cron job 1 failed: %w", err)
	}

	// 2. 每小时 20 分 (Java 为 xx:05): 拉取过去 2 天全量增量订单与染色归因
	_, err = s.cron.AddFunc("20 * * * *", func() {
		s.logger.Info("Starting scheduled order fetch at xx:20 BJ Time (past 2 days)...")
		today := time.Now().In(timeutil.BeijingZone)
		start := today.AddDate(0, 0, -2).Format(timeutil.DateLayout)
		end := today.Format(timeutil.DateLayout)
		if err := s.syncMgr.SyncOrdersAllPlatforms(ctx, start, end); err != nil {
			s.logger.Error("Scheduled order fetch failed", zap.Error(err))
		}
		if _, err := s.syncMgr.SyncFlicknovelRelations(ctx, start, end); err != nil {
			s.logger.Error("Scheduled Flicknovel relations fetch failed", zap.Error(err))
		}
	})
	if err != nil {
		return fmt.Errorf("register cron job 2 failed: %w", err)
	}

	// 3. 每天凌晨 00:55 (Java 为 00:40): 全量拉取历史订单与染色归因
	_, err = s.cron.AddFunc("55 0 * * *", func() {
		s.logger.Info("Starting daily full order fetch at 00:55 BJ Time...")
		today := time.Now().In(timeutil.BeijingZone)
		todayStr := today.Format(timeutil.DateLayout)
		if err := s.syncMgr.SyncOrdersAllPlatforms(ctx, "", todayStr); err != nil {
			s.logger.Error("Daily full order fetch failed", zap.Error(err))
		}
		relStart := today.AddDate(0, 0, -30).Format(timeutil.DateLayout)
		if _, err := s.syncMgr.SyncFlicknovelRelations(ctx, relStart, todayStr); err != nil {
			s.logger.Error("Scheduled Flicknovel full relations fetch failed", zap.Error(err))
		}
		if s.userSvc != nil {
			if count, err := s.userSvc.AutoImportFlicknovelLandingPagesForAdmins(ctx); err == nil && count > 0 {
				s.logger.Info("Daily full admin landing page auto-import completed", zap.Int("imported", count))
			}
		}
	})
	if err != nil {
		return fmt.Errorf("register cron job 3 failed: %w", err)
	}

	// 4. 每小时 35 分 (Java 为 xx:20): 定时统计【每日充值分布】数据并落库
	_, err = s.cron.AddFunc("35 * * * *", func() {
		s.logger.Info("Starting hourly daily distribution calculation at xx:35 BJ Time...")
		if err := s.rechargeSvc.CalculateAllDailyDistribution(ctx); err != nil {
			s.logger.Error("Hourly daily distribution calculation failed", zap.Error(err))
		}
	})
	if err != nil {
		return fmt.Errorf("register cron job 4 failed: %w", err)
	}

	// 5. 每小时 45 分 (Java 为 xx:30): 定时统计 LTV 数据
	_, err = s.cron.AddFunc("45 * * * *", func() {
		s.logger.Info("Starting hourly LTV calculation at xx:45 BJ Time...")
		if err := s.ltvSvc.CalculateAllLtvStats(ctx); err != nil {
			s.logger.Error("Hourly LTV calculation failed", zap.Error(err))
		}
	})
	if err != nil {
		return fmt.Errorf("register cron job 5 failed: %w", err)
	}

	// 6. 每小时 15 分与 45 分 (Java 为每 30 分钟整点/半点 0/30): 定期清理过期报表缓存
	_, err = s.cron.AddFunc("15,45 * * * *", func() {
		s.cache.CleanExpired()
	})
	if err != nil {
		return fmt.Errorf("register cron job 6 failed: %w", err)
	}

	// 7. 每 6 小时 15 分 (Java 为 0 */6 * * *): 自动同步中文在线落地页配置与订阅套餐明细版本 (0:15, 6:15, 12:15, 18:15)
	_, err = s.cron.AddFunc("15 */6 * * *", func() {
		s.logger.Info("Starting scheduled 6-hour sync for Rocnovel landing page & subscribe configs (+15m)...")
		if _, err := s.syncMgr.SyncRocnovelSubscribeConfigs(ctx); err != nil {
			s.logger.Error("Scheduled Rocnovel subscribe configs sync failed", zap.Error(err))
		}
	})
	if err != nil {
		return fmt.Errorf("register cron job 7 failed: %w", err)
	}

	// 8. 每天凌晨 03:15 (Java 为 03:00): 定时重算 LTV 预测基准库
	_, err = s.cron.AddFunc("15 3 * * *", func() {
		s.logger.Info("Starting scheduled LTV prediction benchmark recalculation at 03:15 BJ Time...")
		if s.predictSvc != nil {
			if err := s.predictSvc.RecalculateAllBenchmarks(ctx); err != nil {
				s.logger.Error("Scheduled LTV prediction benchmark recalculation failed", zap.Error(err))
			}
		}
	})
	if err != nil {
		return fmt.Errorf("register cron job 8 failed: %w", err)
	}

	s.cron.Start()
	s.logger.Info("Task scheduler started successfully.")
	return nil
}

// StopWait 优雅停止定时调度器并等待在途任务执行完成
func (s *TaskScheduler) StopWait(ctx context.Context) error {
	if s.cron == nil {
		return nil
	}
	s.logger.Info("Stopping task scheduler and waiting for in-flight jobs...")
	cronCtx := s.cron.Stop()
	select {
	case <-cronCtx.Done():
		s.logger.Info("Task scheduler stopped cleanly.")
		return nil
	case <-ctx.Done():
		s.logger.Warn("Timed out waiting for in-flight cron jobs to finish during stop", zap.Error(ctx.Err()))
		return ctx.Err()
	}
}

func (s *TaskScheduler) Stop() {
	_ = s.StopWait(context.Background())
}
