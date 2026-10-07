package model

import (
	"time"

	"github.com/shopspring/decimal"
	"gorm.io/gorm"
)

// LtvDailyStat LTV 每日统计汇总表 (包含 60 天充值 & ROI 字段)
type LtvDailyStat struct {
	PlatformCode          string           `gorm:"primaryKey;column:platform_code;size:32;not null;default:'rocnovel'" json:"platformCode"`
	UserID                int64            `gorm:"primaryKey;column:user_id;not null" json:"userId"`
	LaunchDate            string           `gorm:"primaryKey;column:launch_date;type:date;not null" json:"launchDate"`
	Spend                 decimal.Decimal  `gorm:"column:spend;type:decimal(10,2);not null;default:0.00" json:"spend"`
	Remark                string           `gorm:"column:remark;size:500;default:''" json:"remark"`
	TotalRecharge         decimal.Decimal  `gorm:"column:total_recharge;type:decimal(10,2);not null;default:0.00" json:"totalRecharge"`
	TotalRefund           decimal.Decimal  `gorm:"column:total_refund;type:decimal(10,2);not null;default:0.00" json:"totalRefund"`
	TotalProfit           decimal.Decimal  `gorm:"column:total_profit;type:decimal(10,2);not null;default:0.00" json:"totalProfit"`
	TotalRoi              decimal.Decimal  `gorm:"column:total_roi;type:decimal(10,4);not null;default:0.0000" json:"totalRoi"`
	SubUserCount          int              `gorm:"column:sub_user_count;not null;default:0" json:"subUserCount"`
	SubUserCost           decimal.Decimal  `gorm:"column:sub_user_cost;type:decimal(10,2);not null;default:0.00" json:"subUserCost"`
	Day7SubUserCount      *int             `gorm:"column:day7_sub_user_count" json:"day7SubUserCount"`
	Day7SubUserRetention  *decimal.Decimal `gorm:"column:day7_sub_user_retention;type:decimal(10,4)" json:"day7SubUserRetention"`
	Day15SubUserCount     *int             `gorm:"column:day15_sub_user_count" json:"day15SubUserCount"`
	Day15SubUserRetention *decimal.Decimal `gorm:"column:day15_sub_user_retention;type:decimal(10,4)" json:"day15SubUserRetention"`
	SubPeriodDays         int              `gorm:"column:sub_period_days;default:1" json:"subPeriodDays"`
	SubPeriodDistribution string           `gorm:"column:sub_period_distribution;size:500" json:"subPeriodDistribution"`

	// Day 1 ~ Day 60 Recharges & ROIs (未到达天数输出为 null，已到达为对应数值)
	Day1Recharge *decimal.Decimal `gorm:"column:day1_recharge;type:decimal(10,2)" json:"day1Recharge"`
	Day1Roi      *decimal.Decimal `gorm:"column:day1_roi;type:decimal(10,4)" json:"day1Roi"`
	Day2Recharge *decimal.Decimal `gorm:"column:day2_recharge;type:decimal(10,2)" json:"day2Recharge"`
	Day2Roi      *decimal.Decimal `gorm:"column:day2_roi;type:decimal(10,4)" json:"day2Roi"`
	Day3Recharge *decimal.Decimal `gorm:"column:day3_recharge;type:decimal(10,2)" json:"day3Recharge"`
	Day3Roi      *decimal.Decimal `gorm:"column:day3_roi;type:decimal(10,4)" json:"day3Roi"`
	Day4Recharge *decimal.Decimal `gorm:"column:day4_recharge;type:decimal(10,2)" json:"day4Recharge"`
	Day4Roi      *decimal.Decimal `gorm:"column:day4_roi;type:decimal(10,4)" json:"day4Roi"`
	Day5Recharge *decimal.Decimal `gorm:"column:day5_recharge;type:decimal(10,2)" json:"day5Recharge"`
	Day5Roi      *decimal.Decimal `gorm:"column:day5_roi;type:decimal(10,4)" json:"day5Roi"`
	Day6Recharge *decimal.Decimal `gorm:"column:day6_recharge;type:decimal(10,2)" json:"day6Recharge"`
	Day6Roi      *decimal.Decimal `gorm:"column:day6_roi;type:decimal(10,4)" json:"day6Roi"`
	Day7Recharge *decimal.Decimal `gorm:"column:day7_recharge;type:decimal(10,2)" json:"day7Recharge"`
	Day7Roi      *decimal.Decimal `gorm:"column:day7_roi;type:decimal(10,4)" json:"day7Roi"`
	Day8Recharge *decimal.Decimal `gorm:"column:day8_recharge;type:decimal(10,2)" json:"day8Recharge"`
	Day8Roi      *decimal.Decimal `gorm:"column:day8_roi;type:decimal(10,4)" json:"day8Roi"`
	Day9Recharge *decimal.Decimal `gorm:"column:day9_recharge;type:decimal(10,2)" json:"day9Recharge"`
	Day9Roi      *decimal.Decimal `gorm:"column:day9_roi;type:decimal(10,4)" json:"day9Roi"`
	Day10Recharge *decimal.Decimal `gorm:"column:day10_recharge;type:decimal(10,2)" json:"day10Recharge"`
	Day10Roi      *decimal.Decimal `gorm:"column:day10_roi;type:decimal(10,4)" json:"day10Roi"`
	Day11Recharge *decimal.Decimal `gorm:"column:day11_recharge;type:decimal(10,2)" json:"day11Recharge"`
	Day11Roi      *decimal.Decimal `gorm:"column:day11_roi;type:decimal(10,4)" json:"day11Roi"`
	Day12Recharge *decimal.Decimal `gorm:"column:day12_recharge;type:decimal(10,2)" json:"day12Recharge"`
	Day12Roi      *decimal.Decimal `gorm:"column:day12_roi;type:decimal(10,4)" json:"day12Roi"`
	Day13Recharge *decimal.Decimal `gorm:"column:day13_recharge;type:decimal(10,2)" json:"day13Recharge"`
	Day13Roi      *decimal.Decimal `gorm:"column:day13_roi;type:decimal(10,4)" json:"day13Roi"`
	Day14Recharge *decimal.Decimal `gorm:"column:day14_recharge;type:decimal(10,2)" json:"day14Recharge"`
	Day14Roi      *decimal.Decimal `gorm:"column:day14_roi;type:decimal(10,4)" json:"day14Roi"`
	Day15Recharge *decimal.Decimal `gorm:"column:day15_recharge;type:decimal(10,2)" json:"day15Recharge"`
	Day15Roi      *decimal.Decimal `gorm:"column:day15_roi;type:decimal(10,4)" json:"day15Roi"`
	Day16Recharge *decimal.Decimal `gorm:"column:day16_recharge;type:decimal(10,2)" json:"day16Recharge"`
	Day16Roi      *decimal.Decimal `gorm:"column:day16_roi;type:decimal(10,4)" json:"day16Roi"`
	Day17Recharge *decimal.Decimal `gorm:"column:day17_recharge;type:decimal(10,2)" json:"day17Recharge"`
	Day17Roi      *decimal.Decimal `gorm:"column:day17_roi;type:decimal(10,4)" json:"day17Roi"`
	Day18Recharge *decimal.Decimal `gorm:"column:day18_recharge;type:decimal(10,2)" json:"day18Recharge"`
	Day18Roi      *decimal.Decimal `gorm:"column:day18_roi;type:decimal(10,4)" json:"day18Roi"`
	Day19Recharge *decimal.Decimal `gorm:"column:day19_recharge;type:decimal(10,2)" json:"day19Recharge"`
	Day19Roi      *decimal.Decimal `gorm:"column:day19_roi;type:decimal(10,4)" json:"day19Roi"`
	Day20Recharge *decimal.Decimal `gorm:"column:day20_recharge;type:decimal(10,2)" json:"day20Recharge"`
	Day20Roi      *decimal.Decimal `gorm:"column:day20_roi;type:decimal(10,4)" json:"day20Roi"`
	Day21Recharge *decimal.Decimal `gorm:"column:day21_recharge;type:decimal(10,2)" json:"day21Recharge"`
	Day21Roi      *decimal.Decimal `gorm:"column:day21_roi;type:decimal(10,4)" json:"day21Roi"`
	Day22Recharge *decimal.Decimal `gorm:"column:day22_recharge;type:decimal(10,2)" json:"day22Recharge"`
	Day22Roi      *decimal.Decimal `gorm:"column:day22_roi;type:decimal(10,4)" json:"day22Roi"`
	Day23Recharge *decimal.Decimal `gorm:"column:day23_recharge;type:decimal(10,2)" json:"day23Recharge"`
	Day23Roi      *decimal.Decimal `gorm:"column:day23_roi;type:decimal(10,4)" json:"day23Roi"`
	Day24Recharge *decimal.Decimal `gorm:"column:day24_recharge;type:decimal(10,2)" json:"day24Recharge"`
	Day24Roi      *decimal.Decimal `gorm:"column:day24_roi;type:decimal(10,4)" json:"day24Roi"`
	Day25Recharge *decimal.Decimal `gorm:"column:day25_recharge;type:decimal(10,2)" json:"day25Recharge"`
	Day25Roi      *decimal.Decimal `gorm:"column:day25_roi;type:decimal(10,4)" json:"day25Roi"`
	Day26Recharge *decimal.Decimal `gorm:"column:day26_recharge;type:decimal(10,2)" json:"day26Recharge"`
	Day26Roi      *decimal.Decimal `gorm:"column:day26_roi;type:decimal(10,4)" json:"day26Roi"`
	Day27Recharge *decimal.Decimal `gorm:"column:day27_recharge;type:decimal(10,2)" json:"day27Recharge"`
	Day27Roi      *decimal.Decimal `gorm:"column:day27_roi;type:decimal(10,4)" json:"day27Roi"`
	Day28Recharge *decimal.Decimal `gorm:"column:day28_recharge;type:decimal(10,2)" json:"day28Recharge"`
	Day28Roi      *decimal.Decimal `gorm:"column:day28_roi;type:decimal(10,4)" json:"day28Roi"`
	Day29Recharge *decimal.Decimal `gorm:"column:day29_recharge;type:decimal(10,2)" json:"day29Recharge"`
	Day29Roi      *decimal.Decimal `gorm:"column:day29_roi;type:decimal(10,4)" json:"day29Roi"`
	Day30Recharge *decimal.Decimal `gorm:"column:day30_recharge;type:decimal(10,2)" json:"day30Recharge"`
	Day30Roi      *decimal.Decimal `gorm:"column:day30_roi;type:decimal(10,4)" json:"day30Roi"`
	Day31Recharge *decimal.Decimal `gorm:"column:day31_recharge;type:decimal(10,2)" json:"day31Recharge"`
	Day31Roi      *decimal.Decimal `gorm:"column:day31_roi;type:decimal(10,4)" json:"day31Roi"`
	Day32Recharge *decimal.Decimal `gorm:"column:day32_recharge;type:decimal(10,2)" json:"day32Recharge"`
	Day32Roi      *decimal.Decimal `gorm:"column:day32_roi;type:decimal(10,4)" json:"day32Roi"`
	Day33Recharge *decimal.Decimal `gorm:"column:day33_recharge;type:decimal(10,2)" json:"day33Recharge"`
	Day33Roi      *decimal.Decimal `gorm:"column:day33_roi;type:decimal(10,4)" json:"day33Roi"`
	Day34Recharge *decimal.Decimal `gorm:"column:day34_recharge;type:decimal(10,2)" json:"day34Recharge"`
	Day34Roi      *decimal.Decimal `gorm:"column:day34_roi;type:decimal(10,4)" json:"day34Roi"`
	Day35Recharge *decimal.Decimal `gorm:"column:day35_recharge;type:decimal(10,2)" json:"day35Recharge"`
	Day35Roi      *decimal.Decimal `gorm:"column:day35_roi;type:decimal(10,4)" json:"day35Roi"`
	Day36Recharge *decimal.Decimal `gorm:"column:day36_recharge;type:decimal(10,2)" json:"day36Recharge"`
	Day36Roi      *decimal.Decimal `gorm:"column:day36_roi;type:decimal(10,4)" json:"day36Roi"`
	Day37Recharge *decimal.Decimal `gorm:"column:day37_recharge;type:decimal(10,2)" json:"day37Recharge"`
	Day37Roi      *decimal.Decimal `gorm:"column:day37_roi;type:decimal(10,4)" json:"day37Roi"`
	Day38Recharge *decimal.Decimal `gorm:"column:day38_recharge;type:decimal(10,2)" json:"day38Recharge"`
	Day38Roi      *decimal.Decimal `gorm:"column:day38_roi;type:decimal(10,4)" json:"day38Roi"`
	Day39Recharge *decimal.Decimal `gorm:"column:day39_recharge;type:decimal(10,2)" json:"day39Recharge"`
	Day39Roi      *decimal.Decimal `gorm:"column:day39_roi;type:decimal(10,4)" json:"day39Roi"`
	Day40Recharge *decimal.Decimal `gorm:"column:day40_recharge;type:decimal(10,2)" json:"day40Recharge"`
	Day40Roi      *decimal.Decimal `gorm:"column:day40_roi;type:decimal(10,4)" json:"day40Roi"`
	Day41Recharge *decimal.Decimal `gorm:"column:day41_recharge;type:decimal(10,2)" json:"day41Recharge"`
	Day41Roi      *decimal.Decimal `gorm:"column:day41_roi;type:decimal(10,4)" json:"day41Roi"`
	Day42Recharge *decimal.Decimal `gorm:"column:day42_recharge;type:decimal(10,2)" json:"day42Recharge"`
	Day42Roi      *decimal.Decimal `gorm:"column:day42_roi;type:decimal(10,4)" json:"day42Roi"`
	Day43Recharge *decimal.Decimal `gorm:"column:day43_recharge;type:decimal(10,2)" json:"day43Recharge"`
	Day43Roi      *decimal.Decimal `gorm:"column:day43_roi;type:decimal(10,4)" json:"day43Roi"`
	Day44Recharge *decimal.Decimal `gorm:"column:day44_recharge;type:decimal(10,2)" json:"day44Recharge"`
	Day44Roi      *decimal.Decimal `gorm:"column:day44_roi;type:decimal(10,4)" json:"day44Roi"`
	Day45Recharge *decimal.Decimal `gorm:"column:day45_recharge;type:decimal(10,2)" json:"day45Recharge"`
	Day45Roi      *decimal.Decimal `gorm:"column:day45_roi;type:decimal(10,4)" json:"day45Roi"`
	Day46Recharge *decimal.Decimal `gorm:"column:day46_recharge;type:decimal(10,2)" json:"day46Recharge"`
	Day46Roi      *decimal.Decimal `gorm:"column:day46_roi;type:decimal(10,4)" json:"day46Roi"`
	Day47Recharge *decimal.Decimal `gorm:"column:day47_recharge;type:decimal(10,2)" json:"day47Recharge"`
	Day47Roi      *decimal.Decimal `gorm:"column:day47_roi;type:decimal(10,4)" json:"day47Roi"`
	Day48Recharge *decimal.Decimal `gorm:"column:day48_recharge;type:decimal(10,2)" json:"day48Recharge"`
	Day48Roi      *decimal.Decimal `gorm:"column:day48_roi;type:decimal(10,4)" json:"day48Roi"`
	Day49Recharge *decimal.Decimal `gorm:"column:day49_recharge;type:decimal(10,2)" json:"day49Recharge"`
	Day49Roi      *decimal.Decimal `gorm:"column:day49_roi;type:decimal(10,4)" json:"day49Roi"`
	Day50Recharge *decimal.Decimal `gorm:"column:day50_recharge;type:decimal(10,2)" json:"day50Recharge"`
	Day50Roi      *decimal.Decimal `gorm:"column:day50_roi;type:decimal(10,4)" json:"day50Roi"`
	Day51Recharge *decimal.Decimal `gorm:"column:day51_recharge;type:decimal(10,2)" json:"day51Recharge"`
	Day51Roi      *decimal.Decimal `gorm:"column:day51_roi;type:decimal(10,4)" json:"day51Roi"`
	Day52Recharge *decimal.Decimal `gorm:"column:day52_recharge;type:decimal(10,2)" json:"day52Recharge"`
	Day52Roi      *decimal.Decimal `gorm:"column:day52_roi;type:decimal(10,4)" json:"day52Roi"`
	Day53Recharge *decimal.Decimal `gorm:"column:day53_recharge;type:decimal(10,2)" json:"day53Recharge"`
	Day53Roi      *decimal.Decimal `gorm:"column:day53_roi;type:decimal(10,4)" json:"day53Roi"`
	Day54Recharge *decimal.Decimal `gorm:"column:day54_recharge;type:decimal(10,2)" json:"day54Recharge"`
	Day54Roi      *decimal.Decimal `gorm:"column:day54_roi;type:decimal(10,4)" json:"day54Roi"`
	Day55Recharge *decimal.Decimal `gorm:"column:day55_recharge;type:decimal(10,2)" json:"day55Recharge"`
	Day55Roi      *decimal.Decimal `gorm:"column:day55_roi;type:decimal(10,4)" json:"day55Roi"`
	Day56Recharge *decimal.Decimal `gorm:"column:day56_recharge;type:decimal(10,2)" json:"day56Recharge"`
	Day56Roi      *decimal.Decimal `gorm:"column:day56_roi;type:decimal(10,4)" json:"day56Roi"`
	Day57Recharge *decimal.Decimal `gorm:"column:day57_recharge;type:decimal(10,2)" json:"day57Recharge"`
	Day57Roi      *decimal.Decimal `gorm:"column:day57_roi;type:decimal(10,4)" json:"day57Roi"`
	Day58Recharge *decimal.Decimal `gorm:"column:day58_recharge;type:decimal(10,2)" json:"day58Recharge"`
	Day58Roi      *decimal.Decimal `gorm:"column:day58_roi;type:decimal(10,4)" json:"day58Roi"`
	Day59Recharge *decimal.Decimal `gorm:"column:day59_recharge;type:decimal(10,2)" json:"day59Recharge"`
	Day59Roi      *decimal.Decimal `gorm:"column:day59_roi;type:decimal(10,4)" json:"day59Roi"`
	Day60Recharge *decimal.Decimal `gorm:"column:day60_recharge;type:decimal(10,2)" json:"day60Recharge"`
	Day60Roi      *decimal.Decimal `gorm:"column:day60_roi;type:decimal(10,4)" json:"day60Roi"`

	// 预测字段
	PredictedPaybackDays   *int             `gorm:"column:predicted_payback_days" json:"predictedPaybackDays"`
	PredictedDay30Recharge *decimal.Decimal `gorm:"column:predicted_day30_recharge;type:decimal(10,2)" json:"predictedDay30Recharge"`
	PredictedDay30Roi      *decimal.Decimal `gorm:"column:predicted_day30_roi;type:decimal(10,4)" json:"predictedDay30Roi"`
	PredictedDay60Recharge *decimal.Decimal `gorm:"column:predicted_day60_recharge;type:decimal(10,2)" json:"predictedDay60Recharge"`
	PredictedDay60Roi      *decimal.Decimal `gorm:"column:predicted_day60_roi;type:decimal(10,4)" json:"predictedDay60Roi"`
	PredictedDay90Recharge *decimal.Decimal `gorm:"column:predicted_day90_recharge;type:decimal(10,2)" json:"predictedDay90Recharge"`
	PredictedDay90Roi      *decimal.Decimal `gorm:"column:predicted_day90_roi;type:decimal(10,4)" json:"predictedDay90Roi"`

	UpdatedAt time.Time `gorm:"column:updated_at" json:"updatedAt"`
	DominantLandingPageID string `gorm:"-" json:"-"`
}

func (LtvDailyStat) TableName() string { return "ltv_daily_stat" }

func (s *LtvDailyStat) AfterFind(tx *gorm.DB) error {
	if len(s.LaunchDate) >= 10 {
		s.LaunchDate = s.LaunchDate[:10]
	}
	return nil
}

// GetRechargeForDay 获取指定天的充值金额
func (s *LtvDailyStat) GetRechargeForDay(day int) decimal.Decimal {
	var p *decimal.Decimal
	switch day {
	case 1: p = s.Day1Recharge
	case 2: p = s.Day2Recharge
	case 3: p = s.Day3Recharge
	case 4: p = s.Day4Recharge
	case 5: p = s.Day5Recharge
	case 6: p = s.Day6Recharge
	case 7: p = s.Day7Recharge
	case 8: p = s.Day8Recharge
	case 9: p = s.Day9Recharge
	case 10: p = s.Day10Recharge
	case 11: p = s.Day11Recharge
	case 12: p = s.Day12Recharge
	case 13: p = s.Day13Recharge
	case 14: p = s.Day14Recharge
	case 15: p = s.Day15Recharge
	case 16: p = s.Day16Recharge
	case 17: p = s.Day17Recharge
	case 18: p = s.Day18Recharge
	case 19: p = s.Day19Recharge
	case 20: p = s.Day20Recharge
	case 21: p = s.Day21Recharge
	case 22: p = s.Day22Recharge
	case 23: p = s.Day23Recharge
	case 24: p = s.Day24Recharge
	case 25: p = s.Day25Recharge
	case 26: p = s.Day26Recharge
	case 27: p = s.Day27Recharge
	case 28: p = s.Day28Recharge
	case 29: p = s.Day29Recharge
	case 30: p = s.Day30Recharge
	case 31: p = s.Day31Recharge
	case 32: p = s.Day32Recharge
	case 33: p = s.Day33Recharge
	case 34: p = s.Day34Recharge
	case 35: p = s.Day35Recharge
	case 36: p = s.Day36Recharge
	case 37: p = s.Day37Recharge
	case 38: p = s.Day38Recharge
	case 39: p = s.Day39Recharge
	case 40: p = s.Day40Recharge
	case 41: p = s.Day41Recharge
	case 42: p = s.Day42Recharge
	case 43: p = s.Day43Recharge
	case 44: p = s.Day44Recharge
	case 45: p = s.Day45Recharge
	case 46: p = s.Day46Recharge
	case 47: p = s.Day47Recharge
	case 48: p = s.Day48Recharge
	case 49: p = s.Day49Recharge
	case 50: p = s.Day50Recharge
	case 51: p = s.Day51Recharge
	case 52: p = s.Day52Recharge
	case 53: p = s.Day53Recharge
	case 54: p = s.Day54Recharge
	case 55: p = s.Day55Recharge
	case 56: p = s.Day56Recharge
	case 57: p = s.Day57Recharge
	case 58: p = s.Day58Recharge
	case 59: p = s.Day59Recharge
	case 60: p = s.Day60Recharge
	default:
		return s.TotalRecharge
	}
	if p == nil {
		return decimal.Zero
	}
	return *p
}

// SetRechargeForDay 设置指定天的充值金额
func (s *LtvDailyStat) SetRechargeForDay(day int, val *decimal.Decimal) {
	switch day {
	case 1: s.Day1Recharge = val
	case 2: s.Day2Recharge = val
	case 3: s.Day3Recharge = val
	case 4: s.Day4Recharge = val
	case 5: s.Day5Recharge = val
	case 6: s.Day6Recharge = val
	case 7: s.Day7Recharge = val
	case 8: s.Day8Recharge = val
	case 9: s.Day9Recharge = val
	case 10: s.Day10Recharge = val
	case 11: s.Day11Recharge = val
	case 12: s.Day12Recharge = val
	case 13: s.Day13Recharge = val
	case 14: s.Day14Recharge = val
	case 15: s.Day15Recharge = val
	case 16: s.Day16Recharge = val
	case 17: s.Day17Recharge = val
	case 18: s.Day18Recharge = val
	case 19: s.Day19Recharge = val
	case 20: s.Day20Recharge = val
	case 21: s.Day21Recharge = val
	case 22: s.Day22Recharge = val
	case 23: s.Day23Recharge = val
	case 24: s.Day24Recharge = val
	case 25: s.Day25Recharge = val
	case 26: s.Day26Recharge = val
	case 27: s.Day27Recharge = val
	case 28: s.Day28Recharge = val
	case 29: s.Day29Recharge = val
	case 30: s.Day30Recharge = val
	case 31: s.Day31Recharge = val
	case 32: s.Day32Recharge = val
	case 33: s.Day33Recharge = val
	case 34: s.Day34Recharge = val
	case 35: s.Day35Recharge = val
	case 36: s.Day36Recharge = val
	case 37: s.Day37Recharge = val
	case 38: s.Day38Recharge = val
	case 39: s.Day39Recharge = val
	case 40: s.Day40Recharge = val
	case 41: s.Day41Recharge = val
	case 42: s.Day42Recharge = val
	case 43: s.Day43Recharge = val
	case 44: s.Day44Recharge = val
	case 45: s.Day45Recharge = val
	case 46: s.Day46Recharge = val
	case 47: s.Day47Recharge = val
	case 48: s.Day48Recharge = val
	case 49: s.Day49Recharge = val
	case 50: s.Day50Recharge = val
	case 51: s.Day51Recharge = val
	case 52: s.Day52Recharge = val
	case 53: s.Day53Recharge = val
	case 54: s.Day54Recharge = val
	case 55: s.Day55Recharge = val
	case 56: s.Day56Recharge = val
	case 57: s.Day57Recharge = val
	case 58: s.Day58Recharge = val
	case 59: s.Day59Recharge = val
	case 60: s.Day60Recharge = val
	}
}

// GetRoiForDay 获取指定天的 ROI
func (s *LtvDailyStat) GetRoiForDay(day int) decimal.Decimal {
	var p *decimal.Decimal
	switch day {
	case 1: p = s.Day1Roi
	case 2: p = s.Day2Roi
	case 3: p = s.Day3Roi
	case 4: p = s.Day4Roi
	case 5: p = s.Day5Roi
	case 6: p = s.Day6Roi
	case 7: p = s.Day7Roi
	case 8: p = s.Day8Roi
	case 9: p = s.Day9Roi
	case 10: p = s.Day10Roi
	case 11: p = s.Day11Roi
	case 12: p = s.Day12Roi
	case 13: p = s.Day13Roi
	case 14: p = s.Day14Roi
	case 15: p = s.Day15Roi
	case 16: p = s.Day16Roi
	case 17: p = s.Day17Roi
	case 18: p = s.Day18Roi
	case 19: p = s.Day19Roi
	case 20: p = s.Day20Roi
	case 21: p = s.Day21Roi
	case 22: p = s.Day22Roi
	case 23: p = s.Day23Roi
	case 24: p = s.Day24Roi
	case 25: p = s.Day25Roi
	case 26: p = s.Day26Roi
	case 27: p = s.Day27Roi
	case 28: p = s.Day28Roi
	case 29: p = s.Day29Roi
	case 30: p = s.Day30Roi
	case 31: p = s.Day31Roi
	case 32: p = s.Day32Roi
	case 33: p = s.Day33Roi
	case 34: p = s.Day34Roi
	case 35: p = s.Day35Roi
	case 36: p = s.Day36Roi
	case 37: p = s.Day37Roi
	case 38: p = s.Day38Roi
	case 39: p = s.Day39Roi
	case 40: p = s.Day40Roi
	case 41: p = s.Day41Roi
	case 42: p = s.Day42Roi
	case 43: p = s.Day43Roi
	case 44: p = s.Day44Roi
	case 45: p = s.Day45Roi
	case 46: p = s.Day46Roi
	case 47: p = s.Day47Roi
	case 48: p = s.Day48Roi
	case 49: p = s.Day49Roi
	case 50: p = s.Day50Roi
	case 51: p = s.Day51Roi
	case 52: p = s.Day52Roi
	case 53: p = s.Day53Roi
	case 54: p = s.Day54Roi
	case 55: p = s.Day55Roi
	case 56: p = s.Day56Roi
	case 57: p = s.Day57Roi
	case 58: p = s.Day58Roi
	case 59: p = s.Day59Roi
	case 60: p = s.Day60Roi
	default:
		return s.TotalRoi
	}
	if p == nil {
		return decimal.Zero
	}
	return *p
}

// SetRoiForDay 设置指定天的 ROI
func (s *LtvDailyStat) SetRoiForDay(day int, val *decimal.Decimal) {
	switch day {
	case 1: s.Day1Roi = val
	case 2: s.Day2Roi = val
	case 3: s.Day3Roi = val
	case 4: s.Day4Roi = val
	case 5: s.Day5Roi = val
	case 6: s.Day6Roi = val
	case 7: s.Day7Roi = val
	case 8: s.Day8Roi = val
	case 9: s.Day9Roi = val
	case 10: s.Day10Roi = val
	case 11: s.Day11Roi = val
	case 12: s.Day12Roi = val
	case 13: s.Day13Roi = val
	case 14: s.Day14Roi = val
	case 15: s.Day15Roi = val
	case 16: s.Day16Roi = val
	case 17: s.Day17Roi = val
	case 18: s.Day18Roi = val
	case 19: s.Day19Roi = val
	case 20: s.Day20Roi = val
	case 21: s.Day21Roi = val
	case 22: s.Day22Roi = val
	case 23: s.Day23Roi = val
	case 24: s.Day24Roi = val
	case 25: s.Day25Roi = val
	case 26: s.Day26Roi = val
	case 27: s.Day27Roi = val
	case 28: s.Day28Roi = val
	case 29: s.Day29Roi = val
	case 30: s.Day30Roi = val
	case 31: s.Day31Roi = val
	case 32: s.Day32Roi = val
	case 33: s.Day33Roi = val
	case 34: s.Day34Roi = val
	case 35: s.Day35Roi = val
	case 36: s.Day36Roi = val
	case 37: s.Day37Roi = val
	case 38: s.Day38Roi = val
	case 39: s.Day39Roi = val
	case 40: s.Day40Roi = val
	case 41: s.Day41Roi = val
	case 42: s.Day42Roi = val
	case 43: s.Day43Roi = val
	case 44: s.Day44Roi = val
	case 45: s.Day45Roi = val
	case 46: s.Day46Roi = val
	case 47: s.Day47Roi = val
	case 48: s.Day48Roi = val
	case 49: s.Day49Roi = val
	case 50: s.Day50Roi = val
	case 51: s.Day51Roi = val
	case 52: s.Day52Roi = val
	case 53: s.Day53Roi = val
	case 54: s.Day54Roi = val
	case 55: s.Day55Roi = val
	case 56: s.Day56Roi = val
	case 57: s.Day57Roi = val
	case 58: s.Day58Roi = val
	case 59: s.Day59Roi = val
	case 60: s.Day60Roi = val
	}
}
