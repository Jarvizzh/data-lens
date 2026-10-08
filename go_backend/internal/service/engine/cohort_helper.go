package engine

import (
	"encoding/json"
	"strconv"
	"strings"

	"go_backend/internal/model"

	"github.com/shopspring/decimal"
)

// GetRechargeForDay 获取指定 Cohort 第 day 天的累计充值金额（向上兜底查找最新有效值）
func GetRechargeForDay(stat *model.LtvDailyStat, day int) decimal.Decimal {
	if stat == nil || day <= 0 {
		return decimal.Zero
	}
	for d := day; d >= 1; d-- {
		val := stat.GetRechargeForDay(d)
		if val.GreaterThan(decimal.Zero) {
			return val
		}
	}
	return decimal.Zero
}

// GetRoiForDay 读取指定 Cohort 第 day 天的 ROI 属性
func GetRoiForDay(stat *model.LtvDailyStat, day int) decimal.Decimal {
	if stat == nil || day <= 0 {
		return decimal.Zero
	}
	return stat.GetRoiForDay(day)
}

// GetMaxPeriodInDistribution 解析 Cohort 订阅分布 JSON，获取存在的最长订阅周期天数 (日订=1, 周订=7, 月订=30等)
func GetMaxPeriodInDistribution(stat *model.LtvDailyStat) int {
	if stat == nil {
		return 1
	}
	distStr := strings.TrimSpace(stat.SubPeriodDistribution)
	if distStr != "" {
		// 尝试解析 JSON 如 {"1": 10, "7": 5}
		var m map[string]int
		if err := json.Unmarshal([]byte(distStr), &m); err == nil && len(m) > 0 {
			maxP := 1
			for k, count := range m {
				if count > 0 {
					if p, err := strconv.Atoi(k); err == nil && p > maxP {
						maxP = p
					}
				}
			}
			return maxP
		}

		// 容错: 兼容无标准 JSON 格式的字符串解析
		clean := strings.ReplaceAll(strings.ReplaceAll(strings.ReplaceAll(distStr, "{", ""), "}", ""), "\"", "")
		if clean != "" {
			pairs := strings.Split(clean, ",")
			maxP := 1
			for _, p := range pairs {
				kv := strings.Split(p, ":")
				if len(kv) == 2 {
					period, err1 := strconv.Atoi(strings.TrimSpace(kv[0]))
					count, err2 := strconv.Atoi(strings.TrimSpace(kv[1]))
					if err1 == nil && err2 == nil && count > 0 && period > maxP {
						maxP = period
					}
				}
			}
			return maxP
		}
	}

	if stat.SubPeriodDays > 1 {
		return stat.SubPeriodDays
	}
	return 1
}

// IsSubscriptionStagnant 根据订阅/追更周期与实际充值增长情况，通用判定 ROI 是否已进入平盘停滞期
func IsSubscriptionStagnant(stat *model.LtvDailyStat, maxDays int) bool {
	if stat == nil || maxDays < MinFlatDays {
		return false
	}
	maxPeriod := GetMaxPeriodInDistribution(stat)
	requiredFlatDays := MinFlatDays
	if maxPeriod*PeriodFlatMultiplier > requiredFlatDays {
		requiredFlatDays = maxPeriod * PeriodFlatMultiplier
	}

	if maxDays >= requiredFlatDays {
		rNow := GetRechargeForDay(stat, maxDays)
		rPrev := GetRechargeForDay(stat, maxDays-requiredFlatDays)
		if rNow.Sub(rPrev).LessThanOrEqual(decimal.NewFromFloat(0.01)) {
			return true
		}
	}
	return false
}

// GetRechargeContinuityRatio 计算最近 window 天内产生新充值增量（正充值天数）的比例
func GetRechargeContinuityRatio(stat *model.LtvDailyStat, maxDays int, window int) float64 {
	if stat == nil || maxDays <= 1 || window <= 0 {
		return 0.0
	}
	startDay := maxDays - window + 1
	if startDay < 2 {
		startDay = 2
	}
	totalDays := maxDays - startDay + 1
	if totalDays <= 0 {
		return 0.0
	}
	activeDays := 0
	threshold := decimal.NewFromFloat(0.01)
	for d := startDay; d <= maxDays; d++ {
		curr := GetRechargeForDay(stat, d)
		prev := GetRechargeForDay(stat, d-1)
		if curr.Sub(prev).GreaterThan(threshold) {
			activeDays++
		}
	}
	return float64(activeDays) / float64(totalDays)
}
