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
	cache       *service.LtvMemoryCache
	logger      *zap.Logger
}

func NewTaskScheduler(
	syncMgr *service.SyncManager,
	ltvSvc *service.LtvService,
	rechargeSvc *service.RechargeStatService,
	userSvc *service.UserService,
	cache *service.LtvMemoryCache,
	logger *zap.Logger,
) *TaskScheduler {
	c := cron.New(cron.WithLocation(timeutil.BeijingZone))
	return &TaskScheduler{
		cron:        c,
		syncMgr:     syncMgr,
		ltvSvc:      ltvSvc,
		rechargeSvc: rechargeSvc,
		userSvc:     userSvc,
		cache:       cache,
		logger:      logger,
	}
}

func (s *TaskScheduler) Start() error {
	ctx := context.Background()

	// 1. 每 4 小时整点: 拉取番茄司南所有推广链接与充值模板，并自动导入全量推广ID给管理员
	_, err := s.cron.AddFunc("0 */4 * * *", func() {
		s.logger.Info("Starting scheduled Flicknovel promotions & templates sync (every 4h)...")
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

	// 2. 每小时 05 分: 拉取过去 2 天全量增量订单与染色归因
	_, err = s.cron.AddFunc("5 * * * *", func() {
		s.logger.Info("Starting scheduled order fetch at xx:05 BJ Time (past 2 days)...")
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

	// 3. 每天凌晨 00:40: 全量拉取历史订单与染色归因
	_, err = s.cron.AddFunc("40 0 * * *", func() {
		s.logger.Info("Starting daily full order fetch at 00:40 BJ Time...")
		today := time.Now().In(timeutil.BeijingZone)
		todayStr := today.Format(timeutil.DateLayout)
		if err := s.syncMgr.SyncOrdersAllPlatforms(ctx, "2026-07-10", todayStr); err != nil {
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

	// 4. 每小时 20 分: 定时统计【每日充值分布】数据并落库
	_, err = s.cron.AddFunc("20 * * * *", func() {
		s.logger.Info("Starting hourly daily distribution calculation at xx:20 BJ Time...")
		if err := s.rechargeSvc.CalculateAllDailyDistribution(ctx); err != nil {
			s.logger.Error("Hourly daily distribution calculation failed", zap.Error(err))
		}
	})
	if err != nil {
		return fmt.Errorf("register cron job 4 failed: %w", err)
	}

	// 5. 每小时 30 分: 定时统计 LTV 数据
	_, err = s.cron.AddFunc("30 * * * *", func() {
		s.logger.Info("Starting hourly LTV calculation at xx:30 BJ Time...")
		if err := s.ltvSvc.CalculateAllLtvStats(ctx); err != nil {
			s.logger.Error("Hourly LTV calculation failed", zap.Error(err))
		}
	})
	if err != nil {
		return fmt.Errorf("register cron job 5 failed: %w", err)
	}

	// 6. 每 30 分钟: 定期清理过期报表缓存
	_, err = s.cron.AddFunc("*/30 * * * *", func() {
		s.cache.CleanExpired()
	})
	if err != nil {
		return fmt.Errorf("register cron job 6 failed: %w", err)
	}

	s.cron.Start()
	s.logger.Info("Task scheduler started successfully.")
	return nil
}

func (s *TaskScheduler) Stop() {
	if s.cron != nil {
		s.cron.Stop()
	}
}
