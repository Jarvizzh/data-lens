package service

import (
	"context"
	"fmt"
	"strings"
	"time"

	"go_backend/internal/model"
	"go_backend/internal/pkg/timeutil"
	"go_backend/internal/repository"
	"go_backend/internal/service/dto"

	"github.com/shopspring/decimal"
)

type LtvService struct {
	calculator        *LtvCalculator
	ltvStatRepo       *repository.LtvStatRepository
	orderRepo         *repository.OrderRepository
	userRepo          *repository.UserRepository
	predictSvc        *PredictService
	cache             *LtvMemoryCache
	monthlySummarySvc *MonthlySummaryService
}

func NewLtvService(
	calculator *LtvCalculator,
	ltvStatRepo *repository.LtvStatRepository,
	orderRepo *repository.OrderRepository,
	userRepo *repository.UserRepository,
	predictSvc *PredictService,
	cache *LtvMemoryCache,
	monthlySummarySvc *MonthlySummaryService,
) *LtvService {
	return &LtvService{
		calculator:        calculator,
		ltvStatRepo:       ltvStatRepo,
		orderRepo:         orderRepo,
		userRepo:          userRepo,
		predictSvc:        predictSvc,
		cache:             cache,
		monthlySummarySvc: monthlySummarySvc,
	}
}

// GetLtvListResponse 兼容 Java 强类型 /api/ltv/list
func (s *LtvService) GetLtvListResponse(ctx context.Context, platformCode string, targetUserID int64) (*dto.LtvListResponseDto, error) {
	if targetUserID <= 0 {
		targetUserID = 1
	}
	pCode := strings.ToLower(platformCode)
	if pCode == "" {
		pCode = "all"
	}

	cacheKey := fmt.Sprintf("ltv:list:%s:%d", pCode, targetUserID)
	if cached, ok := s.cache.Get(cacheKey); ok {
		if resp, valid := cached.(*dto.LtvListResponseDto); valid {
			return resp, nil
		}
	}

	startDate := s.calculator.GetLaunchStartDateForPlatform(pCode)
	stats, err := s.ltvStatRepo.FindStatsByFilter(ctx, pCode, []int64{targetUserID}, startDate, "")
	if err != nil {
		return nil, fmt.Errorf("find ltv stats failed: %w", err)
	}

	if len(stats) == 0 {
		_ = s.CalculateLtvStatsForUserDirect(ctx, pCode, targetUserID)
		stats, _ = s.ltvStatRepo.FindStatsByFilter(ctx, pCode, []int64{targetUserID}, startDate, "")
	}

	totalSpend := decimal.Zero
	totalRecharge := decimal.Zero
	totalSubUsers := 0
	totalRetainedSubUsers := 0
	cohortCurves := make(map[*model.LtvDailyStat][]float64)
	today := time.Now().In(timeutil.BeijingZone)
	minLaunchDate := today

	for _, stat := range stats {
		totalSpend = totalSpend.Add(stat.Spend)
		totalRecharge = totalRecharge.Add(stat.TotalRecharge)
		totalSubUsers += stat.SubUserCount
		if stat.Day7SubUserCount != nil {
			totalRetainedSubUsers += *stat.Day7SubUserCount
		}

		lDate, err := time.ParseInLocation(timeutil.DateLayout, stat.LaunchDate, timeutil.BeijingZone)
		if err == nil {
			if lDate.Before(minLaunchDate) {
				minLaunchDate = lDate
			}
			daysElapsed := int(today.Sub(lDate).Hours()/24) + 1
			curve := s.predictSvc.PredictCohortDailyRechargeCurve(ctx, stat, daysElapsed)
			cohortCurves[stat] = curve
		}
	}

	overallPred := s.predictSvc.facade.AssembleOverallPrediction(totalSpend, totalRecharge, stats, cohortCurves, minLaunchDate, today)
	monthlySummary := s.monthlySummarySvc.BuildMonthlySummary(ctx, stats)

	retainedRateStr := "0.00%"
	if totalSubUsers > 0 {
		rate := decimal.NewFromInt(int64(totalRetainedSubUsers)).
			DivRound(decimal.NewFromInt(int64(totalSubUsers)), 4).
			Mul(decimal.NewFromInt(100))
		retainedRateStr = fmt.Sprintf("%.2f%%", rate.InexactFloat64())
	}

	resp := &dto.LtvListResponseDto{
		Code:                          0,
		Msg:                           "success",
		Data:                          stats,
		OverallPredictedPaybackDays:   overallPred.PredictedPaybackDays,
		OverallPaybackCycleDays:       overallPred.PredictedPaybackDays,
		OverallPredictedDay30Roi:      overallPred.PredictedDay30Roi,
		OverallPredictedDay60Roi:      overallPred.PredictedDay60Roi,
		OverallPredictedDay90Roi:      overallPred.PredictedDay90Roi,
		OverallPredictedDay30Recharge: overallPred.PredictedDay30Recharge,
		OverallPredictedDay60Recharge: overallPred.PredictedDay60Recharge,
		OverallPredictedDay90Recharge: overallPred.PredictedDay90Recharge,
		MonthlySummary:                monthlySummary,
		OverallRetainedSubUsers:       totalRetainedSubUsers,
		OverallRetainedRate:           retainedRateStr,
		Total:                         len(stats),
		UserID:                        targetUserID,
	}

	s.cache.Set(cacheKey, resp)
	return resp, nil
}

// CalculateLtvStatsForUserDirect 计算指定用户的 LTV 数据并持久化
func (s *LtvService) CalculateLtvStatsForUserDirect(ctx context.Context, platformCode string, userID int64) error {
	pCode := strings.ToLower(platformCode)
	if pCode == "" {
		pCode = "all"
	}

	landingPages, err := s.userRepo.FindLandingPages(ctx, pCode, userID)
	if err != nil {
		return fmt.Errorf("find landing pages failed: %w", err)
	}

	lpIDs := make([]string, 0, len(landingPages))
	tzMap := make(map[string]string)
	for _, lp := range landingPages {
		lpIDs = append(lpIDs, lp.LandingPageID)
		tzMap[lp.LandingPageID] = lp.Timezone
	}

	startDate := s.calculator.GetLaunchStartDateForPlatform(pCode)
	orders, err := s.orderRepo.FindOrdersForLtvCalculation(ctx, pCode, lpIDs, startDate, "")
	if err != nil {
		return fmt.Errorf("find orders failed: %w", err)
	}

	cohortMap := make(map[string][]*model.RawOrder)
	for _, o := range orders {
		regDate := o.RegisterDateET
		tz := tzMap[o.LandingPageID]
		if strings.EqualFold(tz, "CST") {
			regDate = o.RegisterTimeBJ.In(timeutil.BeijingZone).Format(timeutil.DateLayout)
		}
		if regDate >= startDate {
			cohortMap[regDate] = append(cohortMap[regDate], o)
		}
	}

	launchConfigs, _ := s.ltvStatRepo.FindLaunchConfigs(ctx, pCode, []int64{userID}, startDate, "")
	spendMap := make(map[string]*model.LtvLaunchConfig)
	for _, lc := range launchConfigs {
		spendMap[lc.LaunchDate] = lc
	}

	today := time.Now().In(timeutil.BeijingZone)
	startDateTime, _ := time.ParseInLocation(timeutil.DateLayout, startDate, timeutil.BeijingZone)
	totalDays := int(today.Sub(startDateTime).Hours()/24) + 1

	stats := make([]*model.LtvDailyStat, 0, totalDays)
	for i := 0; i < totalDays; i++ {
		dateStr := startDateTime.AddDate(0, 0, i).Format(timeutil.DateLayout)
		cohortOrders := cohortMap[dateStr]

		spend := decimal.Zero
		remark := ""
		if cfg, ok := spendMap[dateStr]; ok {
			spend = cfg.Spend
			remark = cfg.Remark
		}

		stat := s.calculator.CalculateSingleCohort(ctx, pCode, userID, dateStr, cohortOrders, spend, remark, today, tzMap)
		stats = append(stats, stat)
	}

	if err := s.ltvStatRepo.BatchUpsertLtvDailyStat(ctx, stats); err != nil {
		return fmt.Errorf("batch upsert ltv stats failed: %w", err)
	}

	s.cache.CleanExpired()
	return nil
}

// SaveLaunchConfig 保存消耗配置并触发重算
func (s *LtvService) SaveLaunchConfig(ctx context.Context, req dto.SaveLaunchConfigRequest) error {
	cfg := &model.LtvLaunchConfig{
		PlatformCode: strings.ToLower(req.PlatformCode),
		UserID:       req.UserID,
		LaunchDate:   req.LaunchDate,
		Spend:        req.Spend,
		Remark:       req.Remark,
		UpdatedAt:    time.Now(),
	}
	if err := s.ltvStatRepo.SaveLaunchConfig(ctx, cfg); err != nil {
		return err
	}
	_ = s.CalculateLtvStatsForUserDirect(ctx, req.PlatformCode, req.UserID)
	_ = s.CalculateLtvStatsForUserDirect(ctx, "all", req.UserID)
	return nil
}

// BatchSaveLaunchConfig 批量导入消耗配置
func (s *LtvService) BatchSaveLaunchConfig(ctx context.Context, platformCode string, userID int64, items []dto.BatchSpendItem) (int, error) {
	pCode := strings.ToLower(platformCode)
	count := 0
	for _, item := range items {
		if item.LaunchDate == "" {
			continue
		}
		cfg := &model.LtvLaunchConfig{
			PlatformCode: pCode,
			UserID:       userID,
			LaunchDate:   item.LaunchDate,
			Spend:        item.Spend,
			Remark:       item.Remark,
			UpdatedAt:    time.Now(),
		}
		if err := s.ltvStatRepo.SaveLaunchConfig(ctx, cfg); err == nil {
			count++
		}
	}
	_ = s.CalculateLtvStatsForUserDirect(ctx, pCode, userID)
	_ = s.CalculateLtvStatsForUserDirect(ctx, "all", userID)
	return count, nil
}

// CalculateAllLtvStats 计算所有用户的 LTV
func (s *LtvService) CalculateAllLtvStats(ctx context.Context) error {
	users, err := s.userRepo.FindAll(ctx)
	if err != nil {
		return err
	}
	platforms := []string{"all", "rocnovel", "flicknovel"}
	for _, u := range users {
		for _, p := range platforms {
			_ = s.CalculateLtvStatsForUserDirect(ctx, p, u.ID)
		}
	}
	return nil
}

