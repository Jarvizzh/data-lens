package engine

import (
	"time"

	"go_backend/internal/model"
	"go_backend/internal/pkg/timeutil"

	"github.com/shopspring/decimal"
)

// PaybackPredictEngine 专职负责回本天数 (Payback Days) 预测与判定独立引擎
type PaybackPredictEngine struct{}

func NewPaybackPredictEngine() *PaybackPredictEngine {
	return &PaybackPredictEngine{}
}

// CalculateCohortPaybackDays 计算单 Cohort 的回本天数与状态
// 返回: nil (不可预测/首3天内), -1 (停滞), 366 (超出365天), 1~365 (已回本或预测回本天数)
func (e *PaybackPredictEngine) CalculateCohortPaybackDays(stat *model.LtvDailyStat, daysElapsed int, cumRechargeCurve []float64) *int {
	if stat == nil || stat.Spend.LessThanOrEqual(decimal.Zero) || len(cumRechargeCurve) < 366 {
		return nil
	}

	maxDays := daysElapsed
	if maxDays > 60 {
		maxDays = 60
	}
	actualRechargeVal, _ := GetRechargeForDay(stat, maxDays).Float64()

	// 1. 判断已知历史数据是否已实现回本 (ROI >= 1.0)
	for d := 1; d <= maxDays; d++ {
		r := GetRoiForDay(stat, d)
		if r.GreaterThanOrEqual(decimal.NewFromInt(1)) {
			val := d
			return &val
		}
	}

	// 2. 观察窗口不足 3 天，不具备预测条件
	if maxDays < 3 {
		return nil
	}

	// 3. 无订阅用户且充值为 0，判为停滞
	if stat.SubUserCount <= 0 && actualRechargeVal <= 0 {
		val := -1
		return &val
	}

	// 3.5 通用订阅周期平盘停滞判定：日订连续 6 天、周订连续 14 天 (2 周)、月订连续 60 天 (2 月) 无充值增长，判为回本停滞 (-1)
	if IsSubscriptionStagnant(stat, maxDays) {
		val := -1
		return &val
	}

	// 4. 从未来天数检索交叉回本点
	spendGoal, _ := stat.Spend.Float64()
	for t := maxDays + 1; t <= 365; t++ {
		if cumRechargeCurve[t] >= spendGoal-0.01 {
			val := t
			return &val
		}
	}

	// 5. 若 365 天内未能回本，判断是否充值仍在微幅增加
	if cumRechargeCurve[365] > actualRechargeVal+0.01 {
		val := 366 // >365 天回本
		return &val
	}

	val := -1 // 回本停滞
	return &val
}

// CalculateOverallPaybackDays 计算大盘整体 (Overall Cohort) 预测回本天数 (按自然日历真实 LaunchDate 动态交叠求和)
func (e *PaybackPredictEngine) CalculateOverallPaybackDays(
	totalSpendAll, totalRechargeAll decimal.Decimal,
	validStats []*model.LtvDailyStat,
	cohortCurves map[*model.LtvDailyStat][]float64,
	minLaunchDate, today time.Time,
) *int {
	if totalSpendAll.LessThanOrEqual(decimal.Zero) || len(cohortCurves) == 0 {
		return nil
	}

	spendGoal, _ := totalSpendAll.Float64()

	// 若当前大盘已知充值已实现回本
	if totalRechargeAll.GreaterThanOrEqual(totalSpendAll) {
		val := 0
		return &val
	}

	lastOverallRecharge := 0.0
	for g := 1; g <= 365; g++ {
		currentDate := minLaunchDate.AddDate(0, 0, g-1)
		overallCumRechargeAtDate := 0.0

		for _, s := range validStats {
			lDate, err := time.ParseInLocation(timeutil.DateLayout, s.LaunchDate, timeutil.BeijingZone)
			if err != nil {
				lDate = minLaunchDate
			}

			if !currentDate.Before(lDate) {
				cohortDay := int(currentDate.Sub(lDate).Hours()/24) + 1
				if cohortDay > 365 {
					cohortDay = 365
				}

				curve := cohortCurves[s]
				if curve != nil && cohortDay >= 1 && cohortDay < len(curve) {
					overallCumRechargeAtDate += curve[cohortDay]
				}
			}
		}

		if overallCumRechargeAtDate >= spendGoal-0.01 {
			if !currentDate.After(today) {
				val := 0 // 历史已回本
				return &val
			}
			remainingDays := int(currentDate.Sub(today).Hours() / 24)
			if remainingDays < 1 {
				remainingDays = 1
			}
			return &remainingDays
		}

		lastOverallRecharge = overallCumRechargeAtDate
	}

	actualRechargeVal, _ := totalRechargeAll.Float64()
	if lastOverallRecharge > actualRechargeVal+0.01 {
		val := 366 // >365 天回本
		return &val
	}

	val := -1 // 回本停滞
	return &val
}
