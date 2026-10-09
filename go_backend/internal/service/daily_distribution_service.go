package service

import (
	"context"
	"fmt"
	"strings"
	"time"

	"go_backend/internal/model"
	"go_backend/internal/pkg/pool"
	"go_backend/internal/repository"
	"go_backend/internal/service/dto"

	"github.com/shopspring/decimal"
	"golang.org/x/sync/singleflight"
)

type DailyDistributionService struct {
	rechargeDistRepo *repository.RechargeDistributionRepository
	orderRepo        *repository.OrderRepository
	userRepo         *repository.UserRepository
	calcSvc          *RechargeStatService
	sfGroup          singleflight.Group
}

func NewDailyDistributionService(
	rechargeDistRepo *repository.RechargeDistributionRepository,
	orderRepo *repository.OrderRepository,
	userRepo *repository.UserRepository,
	calcSvc *RechargeStatService,
) *DailyDistributionService {
	return &DailyDistributionService{
		rechargeDistRepo: rechargeDistRepo,
		orderRepo:        orderRepo,
		userRepo:         userRepo,
		calcSvc:          calcSvc,
	}
}

// GetDailyDistributionResponse 获取指定用户/平台的每日充值分布与汇总 (对应 Java LtvController.getDailyDistribution, 集成 Singleflight 防重复计算)
func (s *DailyDistributionService) GetDailyDistributionResponse(
	ctx context.Context,
	platformCode string,
	targetUserID int64,
) (*dto.DailyDistributionResponseDto, error) {
	if targetUserID <= 0 {
		targetUserID = 1
	}
	pCode := "ALL"
	if platformCode != "" && !strings.EqualFold(platformCode, "ALL") {
		pCode = strings.ToLower(strings.TrimSpace(platformCode))
	}

	cacheKey := fmt.Sprintf("daily_dist:%s:%d", pCode, targetUserID)
	// Singleflight 单飞折叠：使用脱钩 context.WithoutCancel(ctx) 挂载 60s 独立硬超时
	val, err, _ := s.sfGroup.Do(cacheKey, func() (interface{}, error) {
		calcCtx, cancelCalc := context.WithTimeout(context.WithoutCancel(ctx), 60*time.Second)
		defer cancelCalc()
		return s.buildDailyDistributionResponse(calcCtx, pCode, targetUserID)
	})
	if err != nil {
		return nil, err
	}
	// 返回的指针由调用层视作只读 (Immutable)，确保 Singleflight 并发共享安全
	return val.(*dto.DailyDistributionResponseDto), nil
}

func (s *DailyDistributionService) buildDailyDistributionResponse(
	ctx context.Context,
	pCode string,
	targetUserID int64,
) (*dto.DailyDistributionResponseDto, error) {
	platformStartDate := model.GetLaunchStartDateForPlatform(pCode)
	userIDs := []int64{targetUserID}

	list, _ := s.rechargeDistRepo.FindByFilter(ctx, pCode, userIDs, platformStartDate, "")

	// 检查是否全为 0
	allZeros := len(list) > 0
	if allZeros {
		for _, item := range list {
			if item.TotalRecharge.GreaterThan(decimal.Zero) {
				allZeros = false
				break
			}
		}
	}

	today := GetTodayForPlatform(pCode)
	isMissingToday := len(list) > 0 && list[0].Date < today
	isMissingStartDate := len(list) > 0 && list[len(list)-1].Date > platformStartDate

	// 与 Java getDailyDistributionStats 判定严格一致
	if len(list) == 0 || allZeros || isMissingToday || isMissingStartDate {
		_ = s.CalculateDailyDistributionForUser(ctx, pCode, targetUserID)
		list, _ = s.rechargeDistRepo.FindByFilter(ctx, pCode, userIDs, platformStartDate, "")
	}

	// 汇总卡片严格通过原始订单计算
	summary, err := s.calcSvc.GetDailyDistributionSummary(ctx, pCode, targetUserID)
	if err != nil {
		summary = &dto.DailyDistributionSummaryDto{}
	}

	return &dto.DailyDistributionResponseDto{
		Code:    0,
		Msg:     "success",
		Data:    list,
		Summary: summary,
		Total:   len(list),
		UserID:  targetUserID,
	}, nil
}

// CalculateDailyDistributionForUser 统计指定用户/平台的每日充值分布，并异步触发关联主账号数据重算
// (对应 Java calculateDailyDistributionStatsForUser + asyncRecalculateMastersForSubUser)
func (s *DailyDistributionService) CalculateDailyDistributionForUser(ctx context.Context, platformCode string, targetUserID int64) error {
	if targetUserID <= 0 {
		targetUserID = 1
	}
	pCode := "ALL"
	if platformCode != "" && !strings.EqualFold(platformCode, "ALL") {
		pCode = strings.ToLower(strings.TrimSpace(platformCode))
	}

	if s.calcSvc != nil {
		// 计算并持久化指定用户和平台的每日充值分布
		_ = s.calcSvc.CalculateDailyDistributionForUserDirect(ctx, pCode, targetUserID)

		// 异步触发所属主账号的每日充值分布重算 (通过 BizPool 协程池执行，杜绝无界并发与单点 Panic)
		bizPool := pool.GetBizPool()
		taskName := fmt.Sprintf("recalc_daily_dist_masters_for_sub_%d_%s", targetUserID, pCode)
		task := func() {
			bgCtx := context.Background()
			if masters, err := s.userRepo.FindMasterUserIDs(bgCtx, targetUserID); err == nil {
				for _, mID := range masters {
					_ = s.calcSvc.CalculateDailyDistributionForUserDirect(bgCtx, pCode, mID)
				}
			}
		}
		if bizPool != nil {
			_ = bizPool.Submit(taskName, task)
		} else {
			pool.SafeGo(nil, taskName, task)
		}
	}
	return nil
}

// CalculateAllDailyDistribution 全量重新计算所有用户和平台的每日充值分布 (对应 Java calculateAllDailyDistributionStats)
func (s *DailyDistributionService) CalculateAllDailyDistribution(ctx context.Context) error {
	if s.calcSvc != nil {
		return s.calcSvc.CalculateAllDailyDistribution(ctx)
	}
	return nil
}

// RecalculateDailyDistribution 重新计算指定用户/平台的每日充值分布统计 (对应 Java LtvController.recalculateDailyDistributionOnly)
func (s *DailyDistributionService) RecalculateDailyDistribution(
	ctx context.Context,
	platformCode string,
	targetUserID int64,
) (*dto.DailyDistributionResponseDto, error) {
	_ = s.CalculateDailyDistributionForUser(ctx, platformCode, targetUserID)
	return s.GetDailyDistributionResponse(ctx, platformCode, targetUserID)
}

// GetGlobalDailyDistributionResponse 获取全盘每日充值分布 (对应 Java LtvController.getGlobalDailyDistribution)
func (s *DailyDistributionService) GetGlobalDailyDistributionResponse(
	ctx context.Context,
	platformCode string,
) (*dto.DailyDistributionResponseDto, error) {
	pCode := "ALL"
	if platformCode != "" && !strings.EqualFold(platformCode, "ALL") {
		pCode = strings.ToLower(strings.TrimSpace(platformCode))
	}

	list, err := s.calcSvc.GetGlobalDailyDistributionStats(ctx, pCode)
	if err != nil {
		return nil, err
	}

	summary, err := s.calcSvc.GetGlobalDailyDistributionSummary(ctx, pCode)
	if err != nil {
		return nil, err
	}

	return &dto.DailyDistributionResponseDto{
		Code:    0,
		Msg:     "success",
		Data:    list,
		Summary: summary,
		Total:   len(list),
		UserID:  0,
	}, nil
}

