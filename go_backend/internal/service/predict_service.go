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

type PredictService struct {
	benchmarkRepo *repository.BenchmarkRepository
	facade        *engine.LtvPredictFacade
}

func NewPredictService(benchmarkRepo *repository.BenchmarkRepository) *PredictService {
	paybackEngine := engine.NewPaybackPredictEngine()
	roiEngine := engine.NewRoiPredictEngine()
	facade := engine.NewLtvPredictFacade(paybackEngine, roiEngine)
	return &PredictService{
		benchmarkRepo: benchmarkRepo,
		facade:        facade,
	}
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
		periodDays    int
		userCount     int
		baseRet       [367]float64
		baseArpu      [367]float64
		effectiveArpu float64
	}

	contexts := make([]*periodCtx, 0, len(periodDistMap))
	hasValidBenchmark := false

	for period, count := range periodDistMap {
		pCtx := &periodCtx{
			periodDays: period,
			userCount:  count,
		}

		// 查询基准线
		benchmarks, _ := s.benchmarkRepo.FindBenchmarkCurve(ctx, dimType, dimValue, period)
		if len(benchmarks) == 0 && dimType != "ALL" {
			benchmarks, _ = s.benchmarkRepo.FindBenchmarkCurve(ctx, "ALL", "DEFAULT", period)
		}

		if len(benchmarks) > 0 {
			for _, b := range benchmarks {
				idx := b.DayIndex
				if idx >= 1 && idx <= 365 {
					retVal, _ := b.BaseRetentionRate.Float64()
					arpuVal, _ := b.BaseArpu.Float64()
					pCtx.baseRet[idx] = retVal
					pCtx.baseArpu[idx] = arpuVal
					hasValidBenchmark = true
				}
			}
		} else {
			// 合成基准线兜底
			unitPrice := engine.DefaultWeeklySubPrice
			if period == 1 {
				unitPrice = engine.DefaultDailySubPrice
			}
			for d := 1; d <= 90; d++ {
				if period > 1 {
					if (d-1)%period == 0 {
						cycleIdx := (d-1)/period + 1
						pCtx.baseRet[d] = math.Pow(0.55, float64(cycleIdx-1))
					}
				} else {
					pCtx.baseRet[d] = 1.0 / math.Pow(float64(d), 0.75)
				}
				pCtx.baseArpu[d] = unitPrice
				hasValidBenchmark = true
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

		// 匹配价格版本
		versions, _ := s.benchmarkRepo.FindMatchingPeriodVersions(ctx, period, launchTime)
		if len(versions) > 0 {
			v := versions[0]
			if v.RenewPriceCent > 0 {
				pCtx.effectiveArpu = float64(v.RenewPriceCent) / 100.0
			}
		}
		if pCtx.effectiveArpu <= 0 {
			if period == 1 {
				pCtx.effectiveArpu = engine.DefaultDailySubPrice
			} else {
				pCtx.effectiveArpu = engine.DefaultWeeklySubPrice
			}
		}

		contexts = append(contexts, pCtx)
	}

	if !hasValidBenchmark {
		for d := maxDays + 1; d <= 365; d++ {
			cumRecharge[d] = actualRecharge
		}
		return cumRecharge
	}

	actualRoi := actualRecharge / spendVal
	continuityRatio := engine.GetRechargeContinuityRatio(stat, maxDays, 7)
	isSmallCohortActive := (effectiveSubUserCount <= engine.SmallCohortMaxUsers &&
		continuityRatio >= engine.SmallCohortContinuityThreshold &&
		actualRoi >= engine.SmallCohortMinRoi && maxDays >= 10)

	baseRoiSum := 0.0
	for d := 1; d <= maxDays; d++ {
		for _, pCtx := range contexts {
			if pCtx.periodDays == 1 || (d-1)%pCtx.periodDays == 0 {
				baseRoiSum += (pCtx.baseRet[d] * pCtx.effectiveArpu * float64(pCtx.userCount)) / spendVal
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
		scaleDecay := math.Pow(float64(maxDays)/float64(t), scaleDecayExp)
		effectiveScaleFactor := 1.0 + (scaleFactor-1.0)*scaleDecay
		cycleDecay := 1.0
		if t > 7 {
			cycleDecay = math.Pow(7.0/float64(t), engine.CycleDecayExponent)
		}

		dailyDelta := 0.0
		for _, pCtx := range contexts {
			if pCtx.periodDays == 1 || (t-1)%pCtx.periodDays == 0 {
				predRet := pCtx.baseRet[t] * effectiveScaleFactor * cycleDecay
				dailyDelta += predRet * pCtx.effectiveArpu * float64(pCtx.userCount)
			}
		}
		currentCum += dailyDelta
		cumRecharge[t] = currentCum
	}

	// 成熟期 OLS 系综融合
	if maxDays >= engine.OlsEnsembleMinDays {
		olsFit := engine.ComputeOlsFit(stat, maxDays)
		if olsFit.Valid {
			lambda := engine.ComputeOlsEnsembleWeight(olsFit.R2, maxDays)
			for t := maxDays + 1; t <= 365; t++ {
				predOlsRoi := olsFit.A*math.Log(float64(t)) + olsFit.B
				predOlsRecharge := predOlsRoi * spendVal
				if predOlsRecharge > cumRecharge[t] {
					cumRecharge[t] = (1.0-lambda)*cumRecharge[t] + lambda*predOlsRecharge
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
	pCode := strings.TrimSpace(platformCode)
	dimType := strings.ToUpper(strings.TrimSpace(dimensionType))
	dimVal := strings.TrimSpace(dimensionValue)
	if dimType == "" {
		dimType = "ALL"
	}
	if dimVal == "" {
		dimVal = "DEFAULT"
	}

	// Level 1: (PLATFORM, platformCode, period)
	var list []*model.LtvPredictBenchmark
	if pCode != "" && !strings.EqualFold(pCode, "ALL") {
		list, _ = s.benchmarkRepo.FindBenchmarkCurve(ctx, "PLATFORM", pCode, subPeriodDays)
	}

	// Level 2: (USER, userId, period)
	if len(list) == 0 && dimType == "USER" {
		list, _ = s.benchmarkRepo.FindBenchmarkCurve(ctx, "USER", dimVal, subPeriodDays)
	}

	// Level 3: (ALL, DEFAULT, period)
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

// RecalculateAllBenchmarks 手动重算预测基准库
func (s *PredictService) RecalculateAllBenchmarks(ctx context.Context) error {
	for _, period := range []int{1, 7} {
		_ = s.benchmarkRepo.DeleteBenchmarksByDim(ctx, "ALL", "DEFAULT", period)
		seed := s.generateSeedBenchmarks("ALL", "DEFAULT", period)
		_ = s.benchmarkRepo.BatchUpsertBenchmarks(ctx, seed)
	}
	return nil
}
