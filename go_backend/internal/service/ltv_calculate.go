package service

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	"go_backend/internal/model"
	"go_backend/internal/pkg/timeutil"
	"go_backend/internal/repository"

	"github.com/shopspring/decimal"
)

type LtvCalculator struct {
	orderRepo     *repository.OrderRepository
	ltvStatRepo   *repository.LtvStatRepository
	userRepo      *repository.UserRepository
	predictSvc    *PredictService
}

func NewLtvCalculator(
	orderRepo *repository.OrderRepository,
	ltvStatRepo *repository.LtvStatRepository,
	userRepo *repository.UserRepository,
	predictSvc *PredictService,
) *LtvCalculator {
	return &LtvCalculator{
		orderRepo:   orderRepo,
		ltvStatRepo: ltvStatRepo,
		userRepo:    userRepo,
		predictSvc:  predictSvc,
	}
}

// CalculateSingleCohort 计算单个 Cohort 的完整指标与 60 天 LTV
func (c *LtvCalculator) CalculateSingleCohort(
	ctx context.Context,
	platformCode string,
	userID int64,
	launchDateStr string,
	cohortOrders []*model.RawOrder,
	spend decimal.Decimal,
	remark string,
	maxToday time.Time,
	tzMap map[string]string,
	periodMap map[string]int,
) *model.LtvDailyStat {
	stat := &model.LtvDailyStat{
		PlatformCode: platformCode,
		UserID:       userID,
		LaunchDate:   launchDateStr,
		Spend:        spend,
		Remark:       remark,
	}

	launchDate, err := time.ParseInLocation(timeutil.DateLayout, launchDateStr, timeutil.BeijingZone)
	if err != nil {
		launchDate = time.Now().In(timeutil.BeijingZone)
	}

	// 1. 累计充值与退款
	totalRecharge := decimal.Zero
	totalRefund := decimal.Zero
	subMembersMap := make(map[string]struct{})

	lpCountMap := make(map[string]int)
	for _, o := range cohortOrders {
		totalRecharge = totalRecharge.Add(o.OrderAmountUSD)
		if o.RefundStatus == 2 {
			totalRefund = totalRefund.Add(o.OrderAmountUSD)
		}
		if o.IsSubs == 1 && o.MemberID != "" {
			subMembersMap[o.MemberID] = struct{}{}
		}
		pid := strings.TrimSpace(o.LandingPageID)
		if pid != "" {
			lpCountMap[pid]++
		}
	}

	dominantLp := ""
	maxLpCount := 0
	for pid, cnt := range lpCountMap {
		if cnt > maxLpCount {
			maxLpCount = cnt
			dominantLp = pid
		}
	}
	stat.DominantLandingPageID = dominantLp

	stat.TotalRecharge = totalRecharge
	stat.TotalRefund = totalRefund
	stat.TotalProfit = totalRecharge.Sub(totalRefund).Sub(spend)

	if spend.GreaterThan(decimal.Zero) {
		stat.TotalRoi = totalRecharge.Sub(totalRefund).DivRound(spend, 4)
	} else {
		stat.TotalRoi = decimal.Zero
	}

	subUserCount := len(subMembersMap)
	stat.SubUserCount = subUserCount
	if subUserCount > 0 && spend.GreaterThan(decimal.Zero) {
		stat.SubUserCost = spend.DivRound(decimal.NewFromInt(int64(subUserCount)), 2)
	} else {
		stat.SubUserCost = decimal.Zero
	}

	// 3.0 订阅周期分布与主导周期计算 (对齐 Java 算法)
	detectedPeriod := 1
	if len(subMembersMap) > 0 {
		periodCountMap := make(map[int]int)
		for mID := range subMembersMap {
			if periodMap != nil {
				if p, ok := periodMap[mID]; ok && p > 0 {
					periodCountMap[p]++
				}
			}
		}
		if len(periodCountMap) > 0 {
			var periods []int
			for p := range periodCountMap {
				periods = append(periods, p)
			}
			sort.Ints(periods)
			maxCount := -1
			for _, p := range periods {
				cnt := periodCountMap[p]
				if cnt > maxCount {
					maxCount = cnt
					detectedPeriod = p
				}
			}
			var parts []string
			for _, p := range periods {
				parts = append(parts, fmt.Sprintf("\"%d\":%d", p, periodCountMap[p]))
			}
			stat.SubPeriodDistribution = "{" + strings.Join(parts, ",") + "}"
		} else {
			stat.SubPeriodDistribution = ""
		}
	} else {
		stat.SubPeriodDistribution = ""
	}
	stat.SubPeriodDays = detectedPeriod

	// 2. 7日与15日留存
	day8DateStr := launchDate.AddDate(0, 0, 7).Format(timeutil.DateLayout)
	maxTodayStr := maxToday.Format(timeutil.DateLayout)
	if day8DateStr <= maxTodayStr {
		retained7 := make(map[string]struct{})
		for _, o := range cohortOrders {
			if _, ok := subMembersMap[o.MemberID]; ok {
				payDateStr := GetEffectivePayDate(o, tzMap)
				if payDateStr != "" && payDateStr >= day8DateStr {
					retained7[o.MemberID] = struct{}{}
				}
			}
		}
		c7 := len(retained7)
		stat.Day7SubUserCount = &c7
		if subUserCount > 0 {
			r7 := decimal.NewFromInt(int64(c7)).DivRound(decimal.NewFromInt(int64(subUserCount)), 4)
			stat.Day7SubUserRetention = &r7
		}
	} else {
		stat.Day7SubUserCount = nil
		stat.Day7SubUserRetention = nil
	}

	day16DateStr := launchDate.AddDate(0, 0, 15).Format(timeutil.DateLayout)
	if day16DateStr <= maxTodayStr {
		retained15 := make(map[string]struct{})
		for _, o := range cohortOrders {
			if _, ok := subMembersMap[o.MemberID]; ok {
				payDateStr := GetEffectivePayDate(o, tzMap)
				if payDateStr != "" && payDateStr >= day16DateStr {
					retained15[o.MemberID] = struct{}{}
				}
			}
		}
		c15 := len(retained15)
		stat.Day15SubUserCount = &c15
		if subUserCount > 0 {
			r15 := decimal.NewFromInt(int64(c15)).DivRound(decimal.NewFromInt(int64(subUserCount)), 4)
			stat.Day15SubUserRetention = &r15
		}
	} else {
		stat.Day15SubUserCount = nil
		stat.Day15SubUserRetention = nil
	}

	// 3. Day 1 ~ Day 60 充值与 ROI 计算 (未到达天数置为 nil，序列化输出为 null)
	for day := 1; day <= 60; day++ {
		targetDateStr := launchDate.AddDate(0, 0, day-1).Format(timeutil.DateLayout)
		if targetDateStr > maxTodayStr {
			stat.SetRechargeForDay(day, nil)
			stat.SetRoiForDay(day, nil)
			continue
		}

		dayCumRecharge := decimal.Zero
		for _, o := range cohortOrders {
			payDateStr := GetEffectivePayDate(o, tzMap)
			if payDateStr != "" && payDateStr <= targetDateStr {
				dayCumRecharge = dayCumRecharge.Add(o.OrderAmountUSD)
			}
		}

		stat.SetRechargeForDay(day, &dayCumRecharge)
		if spend.GreaterThan(decimal.Zero) {
			roi := dayCumRecharge.DivRound(spend, 4)
			stat.SetRoiForDay(day, &roi)
		} else {
			stat.SetRoiForDay(day, nil)
		}
	}

	// 4. 回本与 ROI 预测
	daysElapsed := int(maxToday.Sub(launchDate).Hours()/24) + 1
	pred := c.predictSvc.PredictCohort(ctx, stat, daysElapsed)
	stat.PredictedPaybackDays = pred.PredictedPaybackDays
	stat.PredictedDay30Roi = pred.PredictedDay30Roi
	stat.PredictedDay60Roi = pred.PredictedDay60Roi
	stat.PredictedDay90Roi = pred.PredictedDay90Roi
	stat.PredictedDay30Recharge = pred.PredictedDay30Recharge
	stat.PredictedDay60Recharge = pred.PredictedDay60Recharge
	stat.PredictedDay90Recharge = pred.PredictedDay90Recharge

	return stat
}

func (c *LtvCalculator) GetLaunchStartDateForPlatform(platformCode string) string {
	return model.GetLaunchStartDateForPlatform(platformCode)
}

func formatDate10(s string) string {
	s = strings.TrimSpace(s)
	if len(s) >= 10 {
		return s[:10]
	}
	return s
}

// GetEffectiveRegisterDate 提取订单的生效注册日期 (按落地页 ID 区分 UTC、美东与北京时间)
func GetEffectiveRegisterDate(o *model.RawOrder, tzMap map[string]string) string {
	if o == nil {
		return ""
	}
	pid := strings.TrimSpace(o.LandingPageID)
	defaultTz := model.CstDefaultTimezone
	if model.IsFlicknovel(o.PlatformCode) {
		defaultTz = model.UtcDefaultTimezone
	}
	tz := defaultTz
	if tzMap != nil && pid != "" {
		if t, ok := tzMap[pid]; ok && strings.TrimSpace(t) != "" {
			tz = strings.ToUpper(strings.TrimSpace(t))
		}
	}
	if tz == "" || tz == "BJ" {
		tz = "CST"
	}

	var res string
	if tz == "UTC" {
		if o.RegisterDateUTC != "" {
			res = o.RegisterDateUTC
		} else if o.RegisterTimeUTC != nil && !o.RegisterTimeUTC.IsZero() {
			res = o.RegisterTimeUTC.UTC().Format(timeutil.DateLayout)
		} else if !o.RegisterTimeBJ.IsZero() {
			res = o.RegisterTimeBJ.Add(-8 * time.Hour).Format(timeutil.DateLayout)
		} else {
			res = o.RegisterDateET
		}
	} else if tz == "ET" {
		if o.RegisterDateET != "" {
			res = o.RegisterDateET
		} else if !o.RegisterTimeET.IsZero() {
			res = o.RegisterTimeET.In(timeutil.EasternZone).Format(timeutil.DateLayout)
		}
	} else {
		// 默认 CST / BJ
		if !o.RegisterTimeBJ.IsZero() {
			res = o.RegisterTimeBJ.In(timeutil.BeijingZone).Format(timeutil.DateLayout)
		} else {
			res = o.RegisterDateET
		}
	}
	return formatDate10(res)
}

// GetEffectivePayDate 提取订单的生效支付日期 (按落地页 ID 区分 UTC、美东与北京时间)
func GetEffectivePayDate(o *model.RawOrder, tzMap map[string]string) string {
	if o == nil {
		return ""
	}
	pid := strings.TrimSpace(o.LandingPageID)
	defaultTz := model.CstDefaultTimezone
	if model.IsFlicknovel(o.PlatformCode) {
		defaultTz = model.UtcDefaultTimezone
	}
	tz := defaultTz
	if tzMap != nil && pid != "" {
		if t, ok := tzMap[pid]; ok && strings.TrimSpace(t) != "" {
			tz = strings.ToUpper(strings.TrimSpace(t))
		}
	}
	if tz == "" || tz == "BJ" {
		tz = "CST"
	}

	var res string
	if tz == "UTC" {
		if o.PayDateUTC != "" {
			res = o.PayDateUTC
		} else if o.PayTimeUTC != nil && !o.PayTimeUTC.IsZero() {
			res = o.PayTimeUTC.UTC().Format(timeutil.DateLayout)
		} else if !o.PayTimeBJ.IsZero() {
			res = o.PayTimeBJ.Add(-8 * time.Hour).Format(timeutil.DateLayout)
		} else {
			res = o.PayDateET
		}
	} else if tz == "ET" {
		if o.PayDateET != "" {
			res = o.PayDateET
		} else if !o.PayTimeET.IsZero() {
			res = o.PayTimeET.In(timeutil.EasternZone).Format(timeutil.DateLayout)
		}
	} else {
		// 默认 CST / BJ
		if !o.PayTimeBJ.IsZero() {
			res = o.PayTimeBJ.In(timeutil.BeijingZone).Format(timeutil.DateLayout)
		} else {
			res = o.PayDateET
		}
	}
	return formatDate10(res)
}

// GetUtcPayDate 提取订单的 UTC 支付日期 (对应 Java LtvStatService.getUtcPayDate)
func GetUtcPayDate(o *model.RawOrder) string {
	if o == nil {
		return ""
	}
	if o.PayDateUTC != "" {
		return formatDate10(o.PayDateUTC)
	}
	if o.PayTimeUTC != nil && !o.PayTimeUTC.IsZero() {
		return o.PayTimeUTC.UTC().Format(timeutil.DateLayout)
	}
	if !o.PayTimeBJ.IsZero() {
		return o.PayTimeBJ.Add(-8 * time.Hour).Format(timeutil.DateLayout)
	}
	return formatDate10(o.PayDateET)
}

// GetUtcRegisterDate 提取订单的 UTC 注册日期 (对应 Java LtvStatService.getUtcRegisterDate)
func GetUtcRegisterDate(o *model.RawOrder) string {
	if o == nil {
		return ""
	}
	if o.RegisterDateUTC != "" {
		return formatDate10(o.RegisterDateUTC)
	}
	if o.RegisterTimeUTC != nil && !o.RegisterTimeUTC.IsZero() {
		return o.RegisterTimeUTC.UTC().Format(timeutil.DateLayout)
	}
	if !o.RegisterTimeBJ.IsZero() {
		return o.RegisterTimeBJ.Add(-8 * time.Hour).Format(timeutil.DateLayout)
	}
	return formatDate10(o.RegisterDateET)
}

// GetBjPayDate 提取订单的北京时间支付日期 (每日充值分析使用，对应 Java LtvStatService.getBjPayDate)
func GetBjPayDate(o *model.RawOrder) string {
	if o == nil {
		return ""
	}
	if !o.PayTimeBJ.IsZero() {
		return o.PayTimeBJ.In(timeutil.BeijingZone).Format(timeutil.DateLayout)
	}
	return formatDate10(o.PayDateET)
}

// GetBjRegisterDate 提取订单的北京时间注册日期 (每日充值分析使用，对应 Java LtvStatService.getBjRegisterDate)
func GetBjRegisterDate(o *model.RawOrder) string {
	if o == nil {
		return ""
	}
	if !o.RegisterTimeBJ.IsZero() {
		return o.RegisterTimeBJ.In(timeutil.BeijingZone).Format(timeutil.DateLayout)
	}
	return formatDate10(o.RegisterDateET)
}

// GetOrderPayDateForPlatform 专用于每日充值分析：flicknovel 用 UTC，其他平台用北京时间 (对应 Java DailyRechargeStatService.getOrderPayDateForPlatform)
func GetOrderPayDateForPlatform(platformCode string, o *model.RawOrder) string {
	if o == nil {
		return ""
	}
	if model.IsFlicknovel(platformCode) || model.IsFlicknovel(o.PlatformCode) {
		return GetUtcPayDate(o)
	}
	return GetBjPayDate(o)
}

// GetOrderRegisterDateForPlatform 专用于每日充值分析：flicknovel 用 UTC，其他平台用北京时间 (对应 Java DailyRechargeStatService.getOrderRegisterDateForPlatform)
func GetOrderRegisterDateForPlatform(platformCode string, o *model.RawOrder) string {
	if o == nil {
		return ""
	}
	if model.IsFlicknovel(platformCode) || model.IsFlicknovel(o.PlatformCode) {
		return GetUtcRegisterDate(o)
	}
	return GetBjRegisterDate(o)
}

