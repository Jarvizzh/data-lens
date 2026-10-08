package engine

import (
	"math"

	"go_backend/internal/model"

	"github.com/shopspring/decimal"
)

// RoiTrendResult 封装 D30, D60, D90 预测结果
type RoiTrendResult struct {
	PredD30Roi      *decimal.Decimal `json:"predD30Roi"`
	PredD60Roi      *decimal.Decimal `json:"predD60Roi"`
	PredD90Roi      *decimal.Decimal `json:"predD90Roi"`
	PredD30Recharge *decimal.Decimal `json:"predD30Recharge"`
	PredD60Recharge *decimal.Decimal `json:"predD60Recharge"`
	PredD90Recharge *decimal.Decimal `json:"predD90Recharge"`
}

// RoiPredictEngine 专职负责 ROI 趋势与里程碑节点 (D30, D60, D90) 外推独立引擎
type RoiPredictEngine struct{}

func NewRoiPredictEngine() *RoiPredictEngine {
	return &RoiPredictEngine{}
}

// CalculateCohortRoiTrend 计算单 Cohort 的 D30, D60, D90 预测 ROI 与预估充值金额
func (e *RoiPredictEngine) CalculateCohortRoiTrend(stat *model.LtvDailyStat, daysElapsed int, cumRechargeCurve []float64) RoiTrendResult {
	if stat == nil || stat.Spend.LessThanOrEqual(decimal.Zero) || len(cumRechargeCurve) <= 90 {
		return RoiTrendResult{}
	}

	maxDays := daysElapsed
	if maxDays > 60 {
		maxDays = 60
	}
	spend := stat.Spend
	spendVal, _ := spend.Float64()

	actualRechargeVal, _ := GetRechargeForDay(stat, maxDays).Float64()
	actualRoiVal := actualRechargeVal / spendVal

	// 提取 Day 30、Day 60 与 Day 90 原始外推充值与 ROI
	rawD30Recharge := cumRechargeCurve[30]
	rawD60Recharge := cumRechargeCurve[60]
	rawD90Recharge := cumRechargeCurve[90]

	rawD30Roi := rawD30Recharge / spendVal
	rawD60Roi := rawD60Recharge / spendVal
	rawD90Roi := rawD90Recharge / spendVal

	// 若已判定进入订阅平盘停滞期，未来不再产生任何充值，直接返回当前实际 ROI
	if IsSubscriptionStagnant(stat, maxDays) {
		pRoi := decimal.NewFromFloat(actualRoiVal).Round(4)
		pRec := decimal.NewFromFloat(actualRechargeVal).Round(2)
		return RoiTrendResult{
			PredD30Roi:      &pRoi,
			PredD60Roi:      &pRoi,
			PredD90Roi:      &pRoi,
			PredD30Recharge: &pRec,
			PredD60Recharge: &pRec,
			PredD90Recharge: &pRec,
		}
	}

	// 1. D30 ROI:
	var finalD30Roi float64
	if daysElapsed >= 30 {
		actualD30Roi := GetRoiForDay(stat, 30)
		if actualD30Roi.GreaterThan(decimal.Zero) {
			finalD30Roi, _ = actualD30Roi.Float64()
		} else {
			actualD30Recharge, _ := GetRechargeForDay(stat, 30).Float64()
			finalD30Roi = actualD30Recharge / spendVal
		}
	} else if daysElapsed >= 14 {
		finalD30Roi = math.Max(actualRoiVal, rawD30Roi)
	} else {
		subUserCount := stat.SubUserCount
		if subUserCount < 1 {
			subUserCount = 1
		}
		userWeight := float64(subUserCount) / (float64(subUserCount) + RoiSampleSizeK)
		beta30 := 0.15 + 0.35*(float64(maxDays)/14.0)*(0.40+0.60*userWeight)

		// 检查 D1 冲动充值与 D2/D3 停滞特征
		impulseDiscount := 1.0
		if maxDays <= 5 {
			r1 := stat.GetRechargeForDay(1)
			rNow := GetRechargeForDay(stat, maxDays)
			if r1.GreaterThan(decimal.Zero) && rNow.GreaterThan(decimal.Zero) {
				r1Val, _ := r1.Float64()
				rNowVal, _ := rNow.Float64()
				if rNowVal <= r1Val*1.05 {
					impulseDiscount = 0.70
				}
			}
		}

		priorMult30 := GetEmpiricalMultiplierTo30(maxDays) * impulseDiscount
		priorD30Roi := actualRoiVal * priorMult30
		blendedD30Roi := beta30*(rawD30Roi*impulseDiscount) + (1.0-beta30)*priorD30Roi

		remainFraction30 := math.Max(0.0, float64(30-maxDays)/30.0)
		maxGrowth30 := 1.0 + 2.2*math.Sqrt(remainFraction30)*impulseDiscount
		maxAllowedRoi30 := actualRoiVal*maxGrowth30 + 0.08*remainFraction30

		finalD30Roi = math.Max(actualRoiVal, math.Min(blendedD30Roi, maxAllowedRoi30))
	}

	// 2. D60 ROI:
	var finalD60Roi float64
	if daysElapsed >= 60 {
		actualD60Roi := GetRoiForDay(stat, 60)
		if actualD60Roi.GreaterThan(decimal.Zero) {
			finalD60Roi, _ = actualD60Roi.Float64()
		} else {
			actualD60Recharge, _ := GetRechargeForDay(stat, 60).Float64()
			finalD60Roi = actualD60Recharge / spendVal
		}
	} else if daysElapsed >= 30 {
		finalD60Roi = math.Max(actualRoiVal, rawD60Roi)
	} else {
		beta60 := 0.30 + 0.50*(float64(maxDays)/30.0)
		priorD60Roi := finalD30Roi * Roi60ChainMultiplier
		blendedD60Roi := beta60*rawD60Roi + (1.0-beta60)*priorD60Roi

		boundMaxDays := maxDays
		if boundMaxDays > 30 {
			boundMaxDays = 30
		}
		remainFraction60 := math.Max(0.0, float64(60-boundMaxDays)/60.0)
		maxAllowedRoi60 := finalD30Roi * (1.0 + 1.6*math.Sqrt(remainFraction60))

		lowerBound60 := finalD30Roi
		if daysElapsed >= 30 {
			lowerBound60 = actualRoiVal
		}
		finalD60Roi = math.Max(lowerBound60, math.Min(blendedD60Roi, maxAllowedRoi60))
	}

	// 3. D90 ROI:
	var finalD90Roi float64
	if daysElapsed >= 90 {
		actualD90Roi := GetRoiForDay(stat, 90)
		if actualD90Roi.GreaterThan(decimal.Zero) {
			finalD90Roi, _ = actualD90Roi.Float64()
		} else {
			actualD90Recharge, _ := GetRechargeForDay(stat, 90).Float64()
			finalD90Roi = actualD90Recharge / spendVal
		}
	} else if daysElapsed >= 60 {
		finalD90Roi = math.Max(actualRoiVal, rawD90Roi)
	} else {
		beta90 := 0.30 + 0.50*(float64(maxDays)/60.0)
		priorD90Roi := finalD60Roi * Roi90ChainMultiplier
		blendedD90Roi := beta90*rawD90Roi + (1.0-beta90)*priorD90Roi

		boundMaxDays := maxDays
		if boundMaxDays > 60 {
			boundMaxDays = 60
		}
		remainFraction90 := math.Max(0.0, float64(90-boundMaxDays)/90.0)
		maxAllowedRoi90 := finalD60Roi * (1.0 + 1.4*math.Sqrt(remainFraction90))

		lowerBound90 := finalD60Roi
		if daysElapsed >= 60 {
			lowerBound90 = actualRoiVal
		}
		finalD90Roi = math.Max(lowerBound90, math.Min(blendedD90Roi, maxAllowedRoi90))
	}

	predD30Roi := decimal.NewFromFloat(finalD30Roi).Round(4)
	predD60Roi := decimal.NewFromFloat(finalD60Roi).Round(4)
	predD90Roi := decimal.NewFromFloat(finalD90Roi).Round(4)

	var predD30Recharge, predD60Recharge, predD90Recharge decimal.Decimal
	if daysElapsed >= 30 {
		predD30Recharge = GetRechargeForDay(stat, 30).Round(2)
	} else {
		predD30Recharge = predD30Roi.Mul(spend).Round(2)
	}

	if daysElapsed >= 60 {
		predD60Recharge = GetRechargeForDay(stat, 60).Round(2)
	} else {
		predD60Recharge = predD60Roi.Mul(spend).Round(2)
	}

	if daysElapsed >= 90 {
		predD90Recharge = GetRechargeForDay(stat, 90).Round(2)
	} else {
		predD90Recharge = predD90Roi.Mul(spend).Round(2)
	}

	return RoiTrendResult{
		PredD30Roi:      &predD30Roi,
		PredD60Roi:      &predD60Roi,
		PredD90Roi:      &predD90Roi,
		PredD30Recharge: &predD30Recharge,
		PredD60Recharge: &predD60Recharge,
		PredD90Recharge: &predD90Recharge,
	}
}
