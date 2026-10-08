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
	userSvc           *UserService
	predictSvc        *PredictService
	cache             *LtvMemoryCache
	monthlySummarySvc *MonthlySummaryService
	rechargeDistSvc   *DailyDistributionService
}

func NewLtvService(
	calculator *LtvCalculator,
	ltvStatRepo *repository.LtvStatRepository,
	orderRepo *repository.OrderRepository,
	userRepo *repository.UserRepository,
	userSvc *UserService,
	predictSvc *PredictService,
	cache *LtvMemoryCache,
	monthlySummarySvc *MonthlySummaryService,
) *LtvService {
	return &LtvService{
		calculator:        calculator,
		ltvStatRepo:       ltvStatRepo,
		orderRepo:         orderRepo,
		userRepo:          userRepo,
		userSvc:           userSvc,
		predictSvc:        predictSvc,
		cache:             cache,
		monthlySummarySvc: monthlySummarySvc,
	}
}

func (s *LtvService) SetRechargeDistService(distSvc *DailyDistributionService) {
	s.rechargeDistSvc = distSvc
}

// GetLtvListResponse 兼容 Java 强类型 /api/ltv/list
func (s *LtvService) GetLtvListResponse(ctx context.Context, platformCode string, targetUserID int64) (*dto.LtvListResponseDto, error) {
	if targetUserID <= 0 {
		targetUserID = 1
	}
	pCode := strings.ToLower(strings.TrimSpace(platformCode))
	isAll := pCode == "" || pCode == "all"
	targetPlatform := "ALL"
	if !isAll {
		targetPlatform = pCode
	}

	cacheKey := fmt.Sprintf("ltv:list:%s:%d", pCode, targetUserID)
	if cached, ok := s.cache.Get(cacheKey); ok {
		if resp, valid := cached.(*dto.LtvListResponseDto); valid {
			return resp, nil
		}
	}

	startDate := s.calculator.GetLaunchStartDateForPlatform(targetPlatform)
	stats, err := s.ltvStatRepo.FindStatsByFilter(ctx, targetPlatform, []int64{targetUserID}, startDate, "")
	if err != nil {
		return nil, fmt.Errorf("find ltv stats failed: %w", err)
	}

	maxToday := timeutil.GetMaxToday()
	maxTodayStr := maxToday.Format(timeutil.DateLayout)

	allZeros := len(stats) > 0
	if allZeros {
		for _, s := range stats {
			if s.Spend.GreaterThan(decimal.Zero) || s.TotalRecharge.GreaterThan(decimal.Zero) {
				allZeros = false
				break
			}
		}
	}

	isMissingStartDate := len(stats) > 0 && stats[0].LaunchDate > startDate
	isMissingToday := len(stats) > 0 && stats[len(stats)-1].LaunchDate < maxTodayStr

	if len(stats) == 0 || allZeros || isMissingStartDate || isMissingToday {
		_ = s.CalculateLtvStatsForUserDirect(ctx, targetPlatform, targetUserID)
		stats, _ = s.ltvStatRepo.FindStatsByFilter(ctx, targetPlatform, []int64{targetUserID}, startDate, "")
	}

	totalSpend := decimal.Zero
	totalRecharge := decimal.Zero
	totalSubUsers := 0
	cohortCurves := make(map[*model.LtvDailyStat][]float64)
	minLaunchDate := maxToday

	for _, stat := range stats {
		totalSpend = totalSpend.Add(stat.Spend)
		totalRecharge = totalRecharge.Add(stat.TotalRecharge)
		totalSubUsers += stat.SubUserCount

		lDate, err := time.ParseInLocation(timeutil.DateLayout, stat.LaunchDate, timeutil.BeijingZone)
		if err == nil {
			if lDate.Before(minLaunchDate) {
				minLaunchDate = lDate
			}
			daysElapsed := int(maxToday.Sub(lDate).Hours()/24) + 1
			curve := s.predictSvc.PredictCohortDailyRechargeCurve(ctx, stat, daysElapsed)
			cohortCurves[stat] = curve
		}
	}

	overallPred := s.predictSvc.facade.AssembleOverallPrediction(totalSpend, totalRecharge, stats, cohortCurves, minLaunchDate, maxToday)

	// 获取用户落地页时区映射与实际订单，用于精确计算订阅留存与月度汇总
	userPages, lpIDs, _ := s.userSvc.GetLandingPageConfigs(ctx, targetPlatform, targetUserID)
	tzMap := make(map[string]string, len(userPages))
	for _, lp := range userPages {
		tzMap[lp.LandingPageID] = lp.Timezone
	}

	userOrders, _ := s.orderRepo.FindOrdersByLandingPageIDs(ctx, targetPlatform, lpIDs)

	// 预加载所有订阅用户的周期配置字典
	var subMemberIDs []string
	subSeen := make(map[string]bool)
	for _, o := range userOrders {
		if o.IsSubs == 1 && strings.TrimSpace(o.MemberID) != "" {
			mID := strings.TrimSpace(o.MemberID)
			if !subSeen[mID] {
				subSeen[mID] = true
				subMemberIDs = append(subMemberIDs, mID)
			}
		}
	}
	periodMap, _ := s.orderRepo.FindSubscriptionPeriodsMap(ctx, subMemberIDs)

	overallRetention := CalculateRetainedSubscribers(userOrders, tzMap, periodMap)
	monthlySummary := s.monthlySummarySvc.BuildMonthlySummary(ctx, targetPlatform, stats, userOrders, tzMap, periodMap)

	resp := &dto.LtvListResponseDto{
		Code:                          0,
		Msg:                           "success",
		Data:                          stats,
		OverallPredictedPaybackDays:   overallPred.PredictedPaybackDays,
		OverallPaybackCycleDays:       nil,
		OverallPredictedDay30Roi:      overallPred.PredictedDay30Roi,
		OverallPredictedDay60Roi:      overallPred.PredictedDay60Roi,
		OverallPredictedDay90Roi:      overallPred.PredictedDay90Roi,
		OverallPredictedDay30Recharge: overallPred.PredictedDay30Recharge,
		OverallPredictedDay60Recharge: overallPred.PredictedDay60Recharge,
		OverallPredictedDay90Recharge: overallPred.PredictedDay90Recharge,
		MonthlySummary:                monthlySummary,
		OverallRetainedSubUsers:       overallRetention.RetainedSubUsers,
		OverallRetainedRate:           overallRetention.RetainedRate,
		Total:                         len(stats),
		UserID:                        targetUserID,
	}

	s.cache.Set(cacheKey, resp)
	return resp, nil
}

// CalculateLtvStatsForUserDirect 计算指定用户的 LTV 数据并持久化
func (s *LtvService) CalculateLtvStatsForUserDirect(ctx context.Context, platformCode string, userID int64) error {
	pCode := strings.ToLower(strings.TrimSpace(platformCode))
	isAll := pCode == "" || pCode == "all"
	targetPlatform := "ALL"
	if !isAll {
		targetPlatform = pCode
	}

	user, err := s.userRepo.FindByID(ctx, userID)
	if err != nil || user == nil {
		return fmt.Errorf("user not found: %d", userID)
	}
	isMasterAcc := user.IsMaster == 1

	s.cache.Delete(fmt.Sprintf("ltv:list:%s:%d", pCode, userID))
	s.cache.Delete(fmt.Sprintf("ltv:list:all:%d", userID))

	userPages, lpIDs, err := s.userSvc.GetLandingPageConfigs(ctx, targetPlatform, userID)
	if err != nil {
		return fmt.Errorf("get landing pages failed: %w", err)
	}
	tzMap := make(map[string]string, len(userPages))
	for _, lp := range userPages {
		tzMap[lp.LandingPageID] = lp.Timezone
	}

	startDate := s.calculator.GetLaunchStartDateForPlatform(targetPlatform)

	var orders []*model.RawOrder
	if len(lpIDs) > 0 {
		queryPlatform := targetPlatform
		orders, err = s.orderRepo.FindOrdersForLtvCalculation(ctx, queryPlatform, lpIDs, startDate, "")
		if err != nil {
			return fmt.Errorf("find orders failed: %w", err)
		}
	}

	cohortMap := make(map[string][]*model.RawOrder)
	for _, o := range orders {
		regDate := GetEffectiveRegisterDate(o, tzMap)
		if len(regDate) >= 10 {
			regDate = regDate[:10]
		}
		if regDate >= startDate {
			cohortMap[regDate] = append(cohortMap[regDate], o)
		}
	}

	configsByDate := make(map[string]*model.LtvLaunchConfig)
	masterRemark := ""

	if isMasterAcc {
		subUserIDs, _ := s.userRepo.FindSubAccountIDs(ctx, userID)
		var subUsernames []string
		for _, sid := range subUserIDs {
			if u, err := s.userRepo.FindByID(ctx, sid); err == nil && u != nil {
				subUsernames = append(subUsernames, u.Username)
			}
		}
		subNamesStr := strings.Join(subUsernames, "、")
		if subNamesStr != "" {
			masterRemark = "汇总数据（子账号：" + subNamesStr + "）"
		} else {
			masterRemark = "汇总数据"
		}

		sumSpendMap := make(map[string]decimal.Decimal)
		remarkMap := make(map[string][]string)

		for _, subID := range subUserIDs {
			var subConfigs []*model.LtvLaunchConfig
			if isAll {
				subConfigs, _ = s.ltvStatRepo.FindLaunchConfigs(ctx, "", []int64{subID}, startDate, "")
			} else {
				subConfigs, _ = s.ltvStatRepo.FindLaunchConfigs(ctx, pCode, []int64{subID}, startDate, "")
			}
			for _, sc := range subConfigs {
				if strings.EqualFold(sc.PlatformCode, "ALL") {
					continue
				}
				lDate := sc.LaunchDate
				if len(lDate) >= 10 {
					lDate = lDate[:10]
				}
				if lDate != "" {
					sumSpendMap[lDate] = sumSpendMap[lDate].Add(sc.Spend)
					cleanRemark := strings.TrimSpace(sc.Remark)
					if cleanRemark != "" {
						formatted := cleanRemark
						if lDate >= model.LaunchStartDateFlicknovel {
							formatted = formatPlatformRemark(sc.PlatformCode, cleanRemark)
						}
						rList := remarkMap[lDate]
						if !containsString(rList, formatted) {
							remarkMap[lDate] = append(remarkMap[lDate], formatted)
						}
					}
				}
			}
		}

		allDates := make(map[string]bool)
		for d := range sumSpendMap {
			allDates[d] = true
		}
		for d := range remarkMap {
			allDates[d] = true
		}

		for d := range allDates {
			spend := sumSpendMap[d]
			rmk := masterRemark
			if isAll {
				if rList, ok := remarkMap[d]; ok && len(rList) > 0 {
					rmk = strings.Join(rList, " | ")
				}
			}
			configsByDate[d] = &model.LtvLaunchConfig{
				PlatformCode: targetPlatform,
				UserID:       userID,
				LaunchDate:   d,
				Spend:        spend,
				Remark:       rmk,
			}
		}
	} else {
		var list []*model.LtvLaunchConfig
		if isAll {
			list, _ = s.ltvStatRepo.FindLaunchConfigs(ctx, "", []int64{userID}, startDate, "")
			sumSpendMap := make(map[string]decimal.Decimal)
			remarkMap := make(map[string][]string)
			for _, c := range list {
				if strings.EqualFold(c.PlatformCode, "ALL") {
					continue
				}
				lDate := c.LaunchDate
				if len(lDate) >= 10 {
					lDate = lDate[:10]
				}
				if lDate != "" {
					sumSpendMap[lDate] = sumSpendMap[lDate].Add(c.Spend)
					cleanRemark := strings.TrimSpace(c.Remark)
					if cleanRemark != "" {
						formatted := cleanRemark
						if lDate >= model.LaunchStartDateFlicknovel {
							formatted = formatPlatformRemark(c.PlatformCode, cleanRemark)
						}
						rList := remarkMap[lDate]
						if !containsString(rList, formatted) {
							remarkMap[lDate] = append(remarkMap[lDate], formatted)
						}
					}
				}
			}
			allDates := make(map[string]bool)
			for d := range sumSpendMap {
				allDates[d] = true
			}
			for d := range remarkMap {
				allDates[d] = true
			}
			for d := range allDates {
				spend := sumSpendMap[d]
				rmk := ""
				if rList, ok := remarkMap[d]; ok && len(rList) > 0 {
					rmk = strings.Join(rList, " | ")
				}
				configsByDate[d] = &model.LtvLaunchConfig{
					PlatformCode: "ALL",
					UserID:       userID,
					LaunchDate:   d,
					Spend:        spend,
					Remark:       rmk,
				}
			}
		} else {
			list, _ = s.ltvStatRepo.FindLaunchConfigs(ctx, pCode, []int64{userID}, startDate, "")
			for _, c := range list {
				lDate := c.LaunchDate
				if len(lDate) >= 10 {
					lDate = lDate[:10]
				}
				configsByDate[lDate] = c
			}
		}
	}

	// 预查该平台用户全部订阅会员的周期字典 (对齐 Java preloadedSubPeriodMap 优化)
	var allSubMemberIDs []string
	subSeen := make(map[string]bool)
	for _, o := range orders {
		if o.IsSubs == 1 && strings.TrimSpace(o.MemberID) != "" {
			mID := strings.TrimSpace(o.MemberID)
			if !subSeen[mID] {
				subSeen[mID] = true
				allSubMemberIDs = append(allSubMemberIDs, mID)
			}
		}
	}
	periodMap, _ := s.orderRepo.FindSubscriptionPeriodsMap(ctx, allSubMemberIDs)

	maxToday := timeutil.GetMaxToday()
	startDateTime, _ := time.ParseInLocation(timeutil.DateLayout, startDate, timeutil.BeijingZone)
	totalDays := int(maxToday.Sub(startDateTime).Hours()/24) + 1

	stats := make([]*model.LtvDailyStat, 0, totalDays)
	for i := 0; i < totalDays; i++ {
		dateStr := startDateTime.AddDate(0, 0, i).Format(timeutil.DateLayout)
		cohortOrders := cohortMap[dateStr]

		spend := decimal.Zero
		remark := ""
		if cfg, ok := configsByDate[dateStr]; ok {
			spend = cfg.Spend
			remark = cfg.Remark
		} else if isMasterAcc {
			remark = masterRemark
		}

		stat := s.calculator.CalculateSingleCohort(ctx, targetPlatform, userID, dateStr, cohortOrders, spend, remark, maxToday, tzMap, periodMap)
		stats = append(stats, stat)
	}

	if err := s.ltvStatRepo.DeleteAndBatchInsert(ctx, targetPlatform, userID, stats); err != nil {
		return fmt.Errorf("delete and batch insert ltv stats failed: %w", err)
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

	// 异步触发所属父级主账号的 LTV 报表与每日充值分布报表重算 (goroutine 异步执行，不阻塞当前 HTTP 请求)
	s.asyncRecalculateMastersForSubUser(req.PlatformCode, req.UserID)

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

	// 异步触发所属父级主账号的 LTV 报表与每日充值分布报表重算 (goroutine 异步执行，不阻塞当前 HTTP 请求)
	s.asyncRecalculateMastersForSubUser(pCode, userID)

	return count, nil
}

// CalculateLtvStatsForUser 重新计算指定用户/平台的 LTV 统计，并级联重算 ALL 和主账号
func (s *LtvService) CalculateLtvStatsForUser(ctx context.Context, platformCode string, userID int64) error {
	pCode := strings.ToLower(strings.TrimSpace(platformCode))
	if pCode == "" {
		pCode = "all"
	}
	if userID <= 0 {
		userID = 1
	}

	_ = s.CalculateLtvStatsForUserDirect(ctx, pCode, userID)
	if pCode != "all" {
		_ = s.CalculateLtvStatsForUserDirect(ctx, "all", userID)
	}

	// 异步触发所属主账号的报表重算 (对齐 Java asyncRecalculateService.asyncRecalculateMastersForSubUser)
	s.asyncRecalculateMastersForSubUser(pCode, userID)

	return nil
}

// asyncRecalculateMastersForSubUser 异步触发子账号关联的所有父级主账号进行报表重算 (LTV 报表 + 每日充值分布报表)
func (s *LtvService) asyncRecalculateMastersForSubUser(platformCode string, subUserID int64) {
	if subUserID <= 0 {
		return
	}
	go func(pCode string, subUID int64) {
		bgCtx := context.Background()
		masters, err := s.userRepo.FindMasterUserIDs(bgCtx, subUID)
		if err != nil || len(masters) == 0 {
			return
		}

		for _, mID := range masters {
			// 1. 级联重算所属父级主账号的 LTV 报表 (当前平台 + ALL 平台)
			_ = s.CalculateLtvStatsForUserDirect(bgCtx, pCode, mID)
			_ = s.CalculateLtvStatsForUserDirect(bgCtx, "all", mID)

			// 2. 级联重算所属父级主账号的每日充值分布报表 (当前平台 + ALL 平台)
			if s.rechargeDistSvc != nil {
				_ = s.rechargeDistSvc.CalculateDailyDistributionForUser(bgCtx, pCode, mID)
				_ = s.rechargeDistSvc.CalculateDailyDistributionForUser(bgCtx, "all", mID)
			}

			// 3. 清理缓存
			s.cache.Delete(fmt.Sprintf("%s:%d", pCode, mID))
			s.cache.Delete(fmt.Sprintf("all:%d", mID))
		}
	}(platformCode, subUserID)
}

// CalculateAllLtvStats 计算所有用户的 LTV
func (s *LtvService) CalculateAllLtvStats(ctx context.Context) error {
	users, err := s.userRepo.FindAll(ctx)
	if err != nil {
		return err
	}
	platforms := []string{model.PlatformAll, model.PlatformRocnovel, model.PlatformFlicknovel}
	for _, u := range users {
		for _, p := range platforms {
			_ = s.CalculateLtvStatsForUserDirect(ctx, p, u.ID)
		}
	}
	return nil
}

func containsString(list []string, item string) bool {
	for _, s := range list {
		if s == item {
			return true
		}
	}
	return false
}

func formatPlatformRemark(platformCode, rawRemark string) string {
	if rawRemark == "" {
		return ""
	}
	pName := platformCode
	switch strings.ToLower(strings.TrimSpace(platformCode)) {
	case model.PlatformRocnovel:
		pName = "洛奇小说"
	case model.PlatformFlicknovel:
		pName = "番茄司南"
	}
	prefix := pName + "："
	if strings.HasPrefix(rawRemark, prefix) {
		return rawRemark
	}
	return prefix + rawRemark
}
