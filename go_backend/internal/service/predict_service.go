package service

import (
	"context"
	"math"
	"strconv"
	"strings"
	"time"

	"go_backend/internal/model"
	"go_backend/internal/pkg/timeutil"
	"go_backend/internal/repository"
	"go_backend/internal/service/engine"

	"github.com/shopspring/decimal"
)

// BenchmarkRepository 抽象基准线及配置版本数据源接口
type BenchmarkRepository interface {
	FindBenchmarkCurve(ctx context.Context, dimType, dimValue string, periodDays int) ([]*model.LtvPredictBenchmark, error)
	FindMatchingPagePeriodVersions(ctx context.Context, landingPageID string, periodDays int, launchTime time.Time) ([]*model.SubscriptionConfigVersion, error)
	FindMatchingPlatformPeriodVersions(ctx context.Context, platformCode string, periodDays int, launchTime time.Time) ([]*model.SubscriptionConfigVersion, error)
	FindMatchingPeriodVersions(ctx context.Context, periodDays int, launchTime time.Time) ([]*model.SubscriptionConfigVersion, error)
	BatchUpsertBenchmarks(ctx context.Context, list []*model.LtvPredictBenchmark) error
	DeleteBenchmarksByDim(ctx context.Context, dimType, dimValue string, periodDays int) error
}

type PredictService struct {
	benchmarkRepo BenchmarkRepository
	orderRepo     *repository.OrderRepository
	userRepo      *repository.UserRepository
	userSvc       *UserService
	facade        *engine.LtvPredictFacade
}

func NewPredictService(benchmarkRepo BenchmarkRepository) *PredictService {
	paybackEngine := engine.NewPaybackPredictEngine()
	roiEngine := engine.NewRoiPredictEngine()
	facade := engine.NewLtvPredictFacade(paybackEngine, roiEngine)
	return &PredictService{
		benchmarkRepo: benchmarkRepo,
		facade:        facade,
	}
}

func (s *PredictService) SetDependencies(orderRepo *repository.OrderRepository, userRepo *repository.UserRepository, userSvc *UserService) {
	s.orderRepo = orderRepo
	s.userRepo = userRepo
	s.userSvc = userSvc
}

// PredictCohort 单个 Cohort 回本与 ROI 预测
func (s *PredictService) PredictCohort(ctx context.Context, stat *model.LtvDailyStat, daysElapsed int) engine.PredictionResult {
	if stat == nil || stat.Spend.LessThanOrEqual(decimal.Zero) {
		return engine.PredictionResult{}
	}
	cumRecharge := s.PredictCohortDailyRechargeCurve(ctx, stat, daysElapsed)
	return s.facade.AssembleCohortPrediction(stat, daysElapsed, cumRecharge)
}

// PredictCohortDailyRechargeCurve 推导 365 天每日累计充值曲线
func (s *PredictService) PredictCohortDailyRechargeCurve(ctx context.Context, stat *model.LtvDailyStat, daysElapsed int) []float64 {
	cumRecharge := make([]float64, 367)
	if stat == nil || stat.Spend.LessThanOrEqual(decimal.Zero) {
		return cumRecharge
	}

	maxDays := daysElapsed
	if maxDays > 60 {
		maxDays = 60
	}
	spendVal, _ := stat.Spend.Float64()

	// 填充历史真实数据
	for d := 1; d <= maxDays; d++ {
		r := engine.GetRechargeForDay(stat, d)
		rVal, _ := r.Float64()
		cumRecharge[d] = rVal
	}

	actualRecharge := cumRecharge[maxDays]
	subUserCount := stat.SubUserCount
	if subUserCount <= 0 && actualRecharge <= 0 {
		for d := maxDays + 1; d <= 365; d++ {
			cumRecharge[d] = actualRecharge
		}
		return cumRecharge
	}

	if engine.IsSubscriptionStagnant(stat, maxDays) {
		for d := maxDays + 1; d <= 365; d++ {
			cumRecharge[d] = actualRecharge
		}
		return cumRecharge
	}

	effectiveSubUserCount := subUserCount
	if effectiveSubUserCount < 1 {
		effectiveSubUserCount = 1
	}

	dimType := "ALL"
	dimValue := "DEFAULT"
	if stat.UserID > 0 {
		dimType = "USER"
		dimValue = strconv.FormatInt(stat.UserID, 10)
	}

	subPeriodDays := stat.SubPeriodDays
	if subPeriodDays < 1 {
		subPeriodDays = 1
	}

	launchTime, err := time.ParseInLocation(timeutil.DateLayout, stat.LaunchDate, timeutil.BeijingZone)
	if err != nil {
		launchTime = time.Now().In(timeutil.BeijingZone)
	}

	// 解析订阅周期分布
	periodDistMap := s.parsePeriodDistribution(stat.SubPeriodDistribution, subPeriodDays, effectiveSubUserCount)
	type periodCtx struct {
		periodDays     int
		userCount      int
		configRenewUsd *float64
		configFirstUsd *float64
		baseRet        [367]float64
		baseArpu       [367]float64
		effectiveArpu  float64
	}

	contexts := make([]*periodCtx, 0, len(periodDistMap))

	for period, count := range periodDistMap {
		pCtx := &periodCtx{
			periodDays: period,
			userCount:  count,
		}

		// 匹配价格版本 (工业级 3 级穿透兜底: 落地页维度精准绑定 > 平台维度匹配 > 全局确定性兜底)
		var versions []*model.SubscriptionConfigVersion
		if stat.DominantLandingPageID != "" {
			versions, _ = s.benchmarkRepo.FindMatchingPagePeriodVersions(ctx, stat.DominantLandingPageID, period, launchTime)
		}
		if len(versions) == 0 && stat.PlatformCode != "" && !strings.EqualFold(stat.PlatformCode, "ALL") {
			versions, _ = s.benchmarkRepo.FindMatchingPlatformPeriodVersions(ctx, stat.PlatformCode, period, launchTime)
		}
		if len(versions) == 0 {
			versions, _ = s.benchmarkRepo.FindMatchingPeriodVersions(ctx, period, launchTime)
		}

		if len(versions) > 0 {
			v := versions[0]
			if v.RenewPriceCent > 0 {
				renewVal := float64(v.RenewPriceCent) / 100.0
				pCtx.configRenewUsd = &renewVal
			}
			if v.FirstPriceCent > 0 {
				firstVal := float64(v.FirstPriceCent) / 100.0
				pCtx.configFirstUsd = &firstVal
			}
		}

		// 查询基准线 (对齐 Java 4 级回退策略)
		benchmarks, _ := s.GetBenchmarkCurve(ctx, stat.PlatformCode, dimType, dimValue, period)

		if len(benchmarks) > 0 {
			for _, b := range benchmarks {
				idx := b.DayIndex
				if idx >= 1 && idx <= 365 {
					retVal, _ := b.BaseRetentionRate.Float64()
					arpuVal, _ := b.BaseArpu.Float64()
					pCtx.baseRet[idx] = retVal
					pCtx.baseArpu[idx] = arpuVal
				}
			}
		} else {
			// 标准合成基准线兜底 (Synthetic Standard Benchmark Fallback)
			defaultUnitPrice := engine.DefaultWeeklySubPrice
			if period == 1 {
				defaultUnitPrice = engine.DefaultDailySubPrice
			}
			unitPrice := defaultUnitPrice
			if pCtx.configRenewUsd != nil {
				unitPrice = *pCtx.configRenewUsd
			} else if pCtx.configFirstUsd != nil {
				unitPrice = *pCtx.configFirstUsd
			}

			for d := 1; d <= 90; d++ {
				if period > 1 {
					if (d-1)%period == 0 {
						cycleIdx := (d-1)/period + 1
						pCtx.baseRet[d] = math.Pow(0.55, float64(cycleIdx-1))
					} else {
						pCtx.baseRet[d] = 0.0
					}
				} else {
					pCtx.baseRet[d] = 1.0 / math.Pow(float64(d), 0.75)
				}
				pCtx.baseArpu[d] = unitPrice
			}
		}

		// 91~365 远期留存衰减推导
		anchorDay := 90
		if period > 1 {
			for anchorDay >= 1 && (anchorDay-1)%period != 0 {
				anchorDay--
			}
		}
		if anchorDay < 1 {
			anchorDay = 1
		}
		lastRet := pCtx.baseRet[anchorDay]
		lastArpu := pCtx.baseArpu[anchorDay]
		for d := 91; d <= 365; d++ {
			if pCtx.baseRet[d] == 0.0 {
				if period > 1 && (d-1)%period != 0 {
					pCtx.baseRet[d] = 0.0
				} else {
					pCtx.baseRet[d] = lastRet * math.Pow(float64(anchorDay)/float64(d), 1.2)
				}
				pCtx.baseArpu[d] = lastArpu
			}
		}

		contexts = append(contexts, pCtx)
	}

	hasValidBenchmark := false
	for _, pCtx := range contexts {
		for d := 1; d <= 365; d++ {
			if pCtx.baseRet[d] > 0 {
				hasValidBenchmark = true
				break
			}
		}
		if hasValidBenchmark {
			break
		}
	}

	if !hasValidBenchmark {
		for d := maxDays + 1; d <= 365; d++ {
			cumRecharge[d] = actualRecharge
		}
		return cumRecharge
	}

	actualRoi := actualRecharge / spendVal

	// P2 模块 3：小样本强活跃大户特征识别与充值动量下界
	continuityRatio := engine.GetRechargeContinuityRatio(stat, maxDays, 7)
	isSmallCohortActive := (effectiveSubUserCount <= engine.SmallCohortMaxUsers &&
		continuityRatio >= engine.SmallCohortContinuityThreshold &&
		actualRoi >= engine.SmallCohortMinRoi && maxDays >= 10)

	recentDailyVelocity := 0.0
	if isSmallCohortActive && maxDays >= 7 {
		rNow := engine.GetRechargeForDay(stat, maxDays)
		r7Ago := engine.GetRechargeForDay(stat, maxDays-7)
		rNowVal, _ := rNow.Float64()
		r7AgoVal, _ := r7Ago.Float64()
		if rNowVal-r7AgoVal > 0 {
			recentDailyVelocity = (rNowVal - r7AgoVal) / 7.0
		}
	}

	// P2 模块 1：Cohort 自身单客充值力 (Realized ARPU) 贝叶斯动态萃取
	timeWeight := 0.0
	if maxDays > 7 && maxDays <= 14 {
		timeWeight = 0.10 + 0.40*(float64(maxDays-7)/7.0)
	} else if maxDays > 14 {
		timeWeight = 0.50 + 0.35*(math.Min(46.0, float64(maxDays-14))/46.0)
	}
	userWeight := float64(effectiveSubUserCount) / (float64(effectiveSubUserCount) + engine.ArpuShrinkageKUser)
	arpuWeight := timeWeight * userWeight

	for _, pCtx := range contexts {
		retSum := 0.0
		for d := 1; d <= maxDays; d++ {
			retSum += pCtx.baseRet[d]
		}
		defaultFallback := engine.DefaultWeeklySubPrice
		if pCtx.periodDays == 1 {
			defaultFallback = engine.DefaultDailySubPrice
		}
		baseArpuAnchor := defaultFallback
		if pCtx.configRenewUsd != nil {
			baseArpuAnchor = *pCtx.configRenewUsd
		} else if pCtx.baseArpu[maxDays] > 0 {
			baseArpuAnchor = pCtx.baseArpu[maxDays]
		}

		if retSum > 0.01 && effectiveSubUserCount > 0 && arpuWeight > 0.001 {
			realizedArpu := actualRecharge / (float64(effectiveSubUserCount) * retSum)
			boundedRealizedArpu := math.Max(0.5*baseArpuAnchor, math.Min(8.0*baseArpuAnchor, realizedArpu))
			pCtx.effectiveArpu = arpuWeight*boundedRealizedArpu + (1.0-arpuWeight)*baseArpuAnchor
		} else {
			pCtx.effectiveArpu = baseArpuAnchor
		}
	}

	baseRoiSum := 0.0
	for d := 1; d <= maxDays; d++ {
		for _, pCtx := range contexts {
			if pCtx.periodDays == 1 || (d-1)%pCtx.periodDays == 0 {
				unitPrice := pCtx.baseArpu[d]
				if d == 1 && pCtx.configFirstUsd != nil {
					unitPrice = *pCtx.configFirstUsd
				} else if d > 1 && pCtx.configRenewUsd != nil {
					unitPrice = *pCtx.configRenewUsd
				}
				baseRoiSum += (pCtx.baseRet[d] * unitPrice * float64(pCtx.userCount)) / spendVal
			}
		}
	}
	if baseRoiSum < 0.0001 {
		baseRoiSum = 0.0001
	}

	scaleFactor := engine.ComputeOptimalScaleFactor(actualRoi, baseRoiSum, maxDays, stat.Spend, isSmallCohortActive)
	scaleDecayExp := engine.ScaleDecayExponent
	if isSmallCohortActive {
		scaleDecayExp = engine.SmallCohortScaleDecayExponent
	}

	currentCum := actualRecharge
	for t := maxDays + 1; t <= 365; t++ {
		// 1. 均值回归机制
		scaleDecay := math.Pow(float64(maxDays)/float64(t), scaleDecayExp)
		effectiveScaleFactor := 1.0 + (scaleFactor-1.0)*scaleDecay

		// 2. 周期续订自然衰减校准
		cycleDecay := 1.0
		if t > 7 {
			cycleDecay = math.Pow(7.0/float64(t), engine.CycleDecayExponent)
		}

		dailyDelta := 0.0
		for _, pCtx := range contexts {
			if pCtx.periodDays == 1 || (t-1)%pCtx.periodDays == 0 {
				predRet := pCtx.baseRet[t] * effectiveScaleFactor * cycleDecay
				defaultArpu := pCtx.baseArpu[t]
				if pCtx.configRenewUsd != nil {
					defaultArpu = *pCtx.configRenewUsd
				}
				predArpu := defaultArpu
				if arpuWeight > 0.001 && pCtx.effectiveArpu > 0 {
					predArpu = pCtx.effectiveArpu
				}
				dailyDelta += predRet * predArpu * float64(pCtx.userCount)
			}
		}

		if isSmallCohortActive && recentDailyVelocity > 0.01 {
			velDecay := math.Pow(float64(maxDays)/float64(t), engine.SmallCohortScaleDecayExponent)
			if recentDailyVelocity*velDecay > dailyDelta {
				dailyDelta = recentDailyVelocity * velDecay
			}
		}

		currentCum += dailyDelta
		cumRecharge[t] = currentCum
	}

	// P2 模块 2：成熟期 (D14+) 双轨 OLS 动量动态系综融合 (Ensemble)
	if maxDays >= engine.OlsEnsembleMinDays {
		olsFit := engine.ComputeOlsFit(stat, maxDays)
		if olsFit.Valid {
			lambda := engine.ComputeOlsEnsembleWeight(olsFit.R2, maxDays)
			if lambda > 0.001 {
				baseRechargeAnchor := actualRecharge
				for t := maxDays + 1; t <= 365; t++ {
					predOlsRoi := olsFit.A*math.Log(float64(t)) + olsFit.B
					predOlsRecharge := predOlsRoi * spendVal
					effectiveOlsRecharge := math.Max(baseRechargeAnchor, predOlsRecharge)
					cumRecharge[t] = (1.0-lambda)*cumRecharge[t] + lambda*effectiveOlsRecharge
				}
			}
		}
	}

	return cumRecharge
}

func (s *PredictService) parsePeriodDistribution(distStr string, defaultPeriod, totalUsers int) map[int]int {
	res := make(map[int]int)
	clean := strings.TrimSpace(distStr)
	if clean != "" {
		clean = strings.ReplaceAll(strings.ReplaceAll(strings.ReplaceAll(clean, "{", ""), "}", ""), "\"", "")
		pairs := strings.Split(clean, ",")
		for _, p := range pairs {
			kv := strings.Split(p, ":")
			if len(kv) == 2 {
				period, e1 := strconv.Atoi(strings.TrimSpace(kv[0]))
				count, e2 := strconv.Atoi(strings.TrimSpace(kv[1]))
				if e1 == nil && e2 == nil && count > 0 {
					res[period] = count
				}
			}
		}
	}
	if len(res) == 0 {
		res[defaultPeriod] = totalUsers
	}
	return res
}

// GetBenchmarkCurve 获取 LTV 预测基准数据曲线
func (s *PredictService) GetBenchmarkCurve(
	ctx context.Context,
	platformCode, dimensionType, dimensionValue string,
	subPeriodDays int,
) ([]*model.LtvPredictBenchmark, error) {
	if subPeriodDays <= 0 {
		subPeriodDays = 1
	}
	pCode := strings.ToLower(strings.TrimSpace(platformCode))
	dimType := strings.ToUpper(strings.TrimSpace(dimensionType))
	dimVal := strings.TrimSpace(dimensionValue)
	if dimType == "" {
		dimType = "ALL"
	}
	if dimVal == "" {
		dimVal = "DEFAULT"
	}

	var list []*model.LtvPredictBenchmark

	// Level 1: (PLATFORM_USER, platformCode + ":" + userId) if platformCode is specified and dimType is USER
	if pCode != "" && !strings.EqualFold(pCode, "ALL") && dimType == "USER" {
		list, _ = s.benchmarkRepo.FindBenchmarkCurve(ctx, "PLATFORM_USER", pCode+":"+dimVal, subPeriodDays)
	}

	// Level 2: (PLATFORM, platformCode) if platformCode is specified
	if len(list) == 0 && pCode != "" && !strings.EqualFold(pCode, "ALL") {
		list, _ = s.benchmarkRepo.FindBenchmarkCurve(ctx, "PLATFORM", pCode, subPeriodDays)
	}

	// Level 3: (USER, userId)
	if len(list) == 0 && dimType == "USER" {
		list, _ = s.benchmarkRepo.FindBenchmarkCurve(ctx, "USER", dimVal, subPeriodDays)
	}

	// Level 4: (ALL, DEFAULT)
	if len(list) == 0 {
		list, _ = s.benchmarkRepo.FindBenchmarkCurve(ctx, "ALL", "DEFAULT", subPeriodDays)
	}

	// Fallback seed benchmarks if still empty
	if len(list) == 0 {
		list = s.generateSeedBenchmarks("ALL", "DEFAULT", subPeriodDays)
		_ = s.benchmarkRepo.BatchUpsertBenchmarks(ctx, list)
	}

	return list, nil
}

func (s *PredictService) generateSeedBenchmarks(dimType, dimVal string, period int) []*model.LtvPredictBenchmark {
	list := make([]*model.LtvPredictBenchmark, 0, 90)
	for d := 1; d <= 90; d++ {
		ret := 0.0
		if period > 1 {
			if (d-1)%period == 0 {
				cycleIdx := (d-1)/period + 1
				ret = math.Pow(0.55, float64(cycleIdx-1))
			}
		} else {
			ret = 1.0 / math.Pow(float64(d), 0.75)
		}

		list = append(list, &model.LtvPredictBenchmark{
			DimensionType:     dimType,
			DimensionValue:    dimVal,
			SubPeriodDays:     period,
			DayIndex:          d,
			BaseRetentionRate: decimal.NewFromFloatWithExponent(ret, -6),
			BaseArpu:          decimal.Zero,
			SampleCohortCount: 10,
			IsExtrapolated:    0,
			UpdatedAt:         time.Now(),
		})
	}
	return list
}

// RecalculateAllBenchmarks 重算预测基准库 (对齐 Java LtvBenchmarkService.recalculateAllBenchmarks)
func (s *PredictService) RecalculateAllBenchmarks(ctx context.Context) error {
	if s.orderRepo == nil {
		for _, period := range []int{1, 7} {
			_ = s.benchmarkRepo.DeleteBenchmarksByDim(ctx, "ALL", "DEFAULT", period)
			seed := s.generateSeedBenchmarks("ALL", "DEFAULT", period)
			_ = s.benchmarkRepo.BatchUpsertBenchmarks(ctx, seed)
		}
		return nil
	}

	cutoffDate := time.Now().In(timeutil.BeijingZone).AddDate(0, 0, -65).Format(timeutil.DateLayout)
	recentOrders, err := s.orderRepo.FindRecentValidOrdersByRegisterDate(ctx, cutoffDate)
	if err != nil || len(recentOrders) == 0 {
		s.populateSeedBenchmarks(ctx, "ALL", "DEFAULT", 1)
		s.populateSeedBenchmarks(ctx, "ALL", "DEFAULT", 7)
		return nil
	}

	// 提取活跃用户群体的订阅周期映射
	memberIDsMap := make(map[string]bool)
	for _, o := range recentOrders {
		mID := strings.TrimSpace(o.MemberID)
		if mID != "" {
			memberIDsMap[mID] = true
		}
	}
	memberIDs := make([]string, 0, len(memberIDsMap))
	for mID := range memberIDsMap {
		memberIDs = append(memberIDs, mID)
	}

	userPeriodMap, _ := s.orderRepo.FindSubscriptionPeriodsMap(ctx, memberIDs)

	// 按 memberID 对应的 subPeriodDays 分组处理 (1: 日订, 7: 周订 等)
	ordersByPeriod := make(map[int][]*model.RawOrder)
	for _, o := range recentOrders {
		mID := strings.TrimSpace(o.MemberID)
		period := 1
		if p, ok := userPeriodMap[mID]; ok && p > 0 {
			period = p
		}
		ordersByPeriod[period] = append(ordersByPeriod[period], o)
	}

	for subPeriod, periodOrders := range ordersByPeriod {
		s.calculateBenchmarkForGroup(ctx, "ALL", "DEFAULT", subPeriod, periodOrders, nil)
	}

	// 如果没有周订订单，为 weekly (sub_period=7) 生成基于日订衍生的基准
	if _, ok := ordersByPeriod[7]; !ok {
		s.populateSeedBenchmarks(ctx, "ALL", "DEFAULT", 7)
	}

	// 遍历每个活跃系统用户，生成专属的 USER 维度基准曲线
	if s.userRepo != nil {
		users, err := s.userRepo.FindAll(ctx)
		if err == nil {
			for _, u := range users {
				s.recalculateBenchmarksForUser(ctx, u.ID, cutoffDate)
			}
		}
	}

	return nil
}

func (s *PredictService) recalculateBenchmarksForUser(ctx context.Context, userID int64, cutoffDate string) {
	if s.userSvc == nil {
		return
	}
	_, pidList, err := s.userSvc.GetLandingPageConfigs(ctx, "ALL", userID)
	if err != nil || len(pidList) == 0 {
		return
	}

	userOrders, err := s.orderRepo.FindOrdersByLandingPageIDs(ctx, "ALL", pidList)
	if err != nil || len(userOrders) == 0 {
		return
	}

	// 过滤近 65 天
	var recentUserOrders []*model.RawOrder
	for _, o := range userOrders {
		regDate := o.RegisterDateET
		if regDate >= cutoffDate {
			recentUserOrders = append(recentUserOrders, o)
		}
	}
	if len(recentUserOrders) == 0 {
		return
	}

	memberIDsMap := make(map[string]bool)
	for _, o := range recentUserOrders {
		mID := strings.TrimSpace(o.MemberID)
		if mID != "" {
			memberIDsMap[mID] = true
		}
	}
	memberIDs := make([]string, 0, len(memberIDsMap))
	for mID := range memberIDsMap {
		memberIDs = append(memberIDs, mID)
	}
	userPeriodMap, _ := s.orderRepo.FindSubscriptionPeriodsMap(ctx, memberIDs)

	ordersByPeriod := make(map[int][]*model.RawOrder)
	for _, o := range recentUserOrders {
		mID := strings.TrimSpace(o.MemberID)
		period := 1
		if p, ok := userPeriodMap[mID]; ok && p > 0 {
			period = p
		}
		ordersByPeriod[period] = append(ordersByPeriod[period], o)
	}

	dimType := "USER"
	dimValue := strconv.FormatInt(userID, 10)

	pageConfigs, _, _ := s.userSvc.GetLandingPageConfigs(ctx, "ALL", userID)
	tzMap := make(map[string]string)
	for _, pc := range pageConfigs {
		if pc.LandingPageID != "" && pc.Timezone != "" {
			tzMap[pc.LandingPageID] = pc.Timezone
		}
	}

	for subPeriod, periodOrders := range ordersByPeriod {
		s.calculateBenchmarkForGroup(ctx, dimType, dimValue, subPeriod, periodOrders, tzMap)
	}
}

func (s *PredictService) calculateBenchmarkForGroup(
	ctx context.Context,
	dimType, dimValue string,
	subPeriodDays int,
	orders []*model.RawOrder,
	tzMap map[string]string,
) {
	if len(orders) == 0 {
		s.populateSeedBenchmarks(ctx, dimType, dimValue, subPeriodDays)
		return
	}

	// 找出所有注册日期 Cohort
	cohortMap := make(map[string][]*model.RawOrder)
	for _, o := range orders {
		regDate := GetEffectiveRegisterDate(o, tzMap)
		if regDate != "" {
			cohortMap[regDate] = append(cohortMap[regDate], o)
		}
	}

	if len(cohortMap) == 0 {
		s.populateSeedBenchmarks(ctx, dimType, dimValue, subPeriodDays)
		return
	}

	now := time.Now().In(timeutil.BeijingZone)
	maxMatureDay := 90

	// 筛选过去 60 天内注册、且注册天数 >= 30 的成熟 Cohort
	matureCohorts := make(map[string][]*model.RawOrder)
	for regDateStr, cOrders := range cohortMap {
		t, err := time.ParseInLocation(timeutil.DateLayout, regDateStr, timeutil.BeijingZone)
		if err != nil {
			continue
		}
		age := int(now.Sub(t).Hours() / 24)
		if age <= 60 && age >= 30 {
			matureCohorts[regDateStr] = cOrders
		}
	}

	if len(matureCohorts) == 0 {
		matureCohorts = cohortMap // 降级：使用所有可用 Cohort
	}

	cohortCount := len(matureCohorts)
	totalInitialSubs := 0
	totalActiveMembersCount := make(map[int]int)
	totalRechargeMap := make(map[int]decimal.Decimal)

	for regDateStr, cohortOrders := range matureCohorts {
		regDate, err := time.ParseInLocation(timeutil.DateLayout, regDateStr, timeutil.BeijingZone)
		if err != nil {
			continue
		}

		// 统计 Cohort 初始订阅人数 N1 (renewType=1 首次订阅)
		initialSubMembers := make(map[string]bool)
		for _, o := range cohortOrders {
			if o.IsSubs == 1 && (o.RenewType == 0 || o.RenewType == 1) {
				mID := strings.TrimSpace(o.MemberID)
				if mID != "" {
					initialSubMembers[mID] = true
				}
			}
		}
		totalInitialSubs += len(initialSubMembers)

		// 按 Day 统计划扣人数与金额
		dayActiveMembers := make(map[int]map[string]bool)
		for _, o := range cohortOrders {
			if o.PayState != 1 {
				continue
			}
			payDateStr := GetEffectivePayDate(o, tzMap)
			if payDateStr == "" {
				continue
			}
			payDate, err := time.ParseInLocation(timeutil.DateLayout, payDateStr, timeutil.BeijingZone)
			if err != nil {
				continue
			}
			dayIndex := int(payDate.Sub(regDate).Hours()/24) + 1
			if dayIndex >= 1 && dayIndex <= maxMatureDay {
				if dayActiveMembers[dayIndex] == nil {
					dayActiveMembers[dayIndex] = make(map[string]bool)
				}
				dayActiveMembers[dayIndex][strings.TrimSpace(o.MemberID)] = true

				cur := totalRechargeMap[dayIndex]
				totalRechargeMap[dayIndex] = cur.Add(o.OrderAmountUSD)
			}
		}

		for d, members := range dayActiveMembers {
			totalActiveMembersCount[d] += len(members)
		}
	}

	poolInitialSubs := totalInitialSubs
	if poolInitialSubs < 1 {
		poolInitialSubs = 1
	}

	var benchmarksToSave []*model.LtvPredictBenchmark
	var baseRet [91]float64
	var baseArpu [91]float64

	defaultPrice := engine.DefaultDailySubPrice
	if subPeriodDays > 1 {
		defaultPrice = engine.DefaultWeeklySubPrice
	}

	for d := 1; d <= maxMatureDay; d++ {
		activeCount := totalActiveMembersCount[d]
		avgRet := float64(activeCount) / float64(poolInitialSubs)
		recharge := totalRechargeMap[d]

		avgArpu := 0.0
		if activeCount > 0 {
			rF, _ := recharge.Float64()
			avgArpu = rF / float64(activeCount)
		} else if d == 1 {
			avgArpu = defaultPrice
		}

		baseRet[d] = avgRet
		baseArpu[d] = avgArpu

		bench := &model.LtvPredictBenchmark{
			DimensionType:     dimType,
			DimensionValue:    dimValue,
			SubPeriodDays:     subPeriodDays,
			DayIndex:          d,
			BaseRetentionRate: decimal.NewFromFloatWithExponent(avgRet, -6),
			BaseArpu:          decimal.NewFromFloatWithExponent(avgArpu, -2),
			SampleCohortCount: cohortCount,
			IsExtrapolated:    0,
			UpdatedAt:         time.Now(),
		}
		benchmarksToSave = append(benchmarksToSave, bench)
	}

	_ = s.benchmarkRepo.DeleteBenchmarksByDim(ctx, dimType, dimValue, subPeriodDays)
	_ = s.benchmarkRepo.BatchUpsertBenchmarks(ctx, benchmarksToSave)
}

func (s *PredictService) populateSeedBenchmarks(ctx context.Context, dimType, dimValue string, subPeriodDays int) {
	seed := s.generateSeedBenchmarks(dimType, dimValue, subPeriodDays)
	_ = s.benchmarkRepo.DeleteBenchmarksByDim(ctx, dimType, dimValue, subPeriodDays)
	_ = s.benchmarkRepo.BatchUpsertBenchmarks(ctx, seed)
}
