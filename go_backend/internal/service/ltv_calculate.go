package service

import (
	"context"
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

	for _, o := range cohortOrders {
		totalRecharge = totalRecharge.Add(o.OrderAmountUSD)
		if o.RefundStatus == 2 {
			totalRefund = totalRefund.Add(o.OrderAmountUSD)
		}
		if o.IsSubs == 1 && o.MemberID != "" {
			subMembersMap[o.MemberID] = struct{}{}
		}
	}

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
	}

	// 3. Day 1 ~ Day 60 充值与 ROI 计算
	for day := 1; day <= 60; day++ {
		targetDateStr := launchDate.AddDate(0, 0, day-1).Format(timeutil.DateLayout)
		if targetDateStr > maxTodayStr {
			break
		}

		dayCumRecharge := decimal.Zero
		for _, o := range cohortOrders {
			payDateStr := GetEffectivePayDate(o, tzMap)
			if payDateStr != "" && payDateStr <= targetDateStr {
				dayCumRecharge = dayCumRecharge.Add(o.OrderAmountUSD)
			}
		}

		stat.SetRechargeForDay(day, dayCumRecharge)
		if spend.GreaterThan(decimal.Zero) {
			stat.SetRoiForDay(day, dayCumRecharge.DivRound(spend, 4))
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
	switch strings.ToLower(platformCode) {
	case "flicknovel":
		return "2026-09-16"
	case "rocnovel":
		return "2026-07-10"
	default:
		return "2026-07-10"
	}
}

// GetEffectiveRegisterDate 提取订单的生效注册日期 (按落地页 ID 区分 UTC、美东与北京时间)
func GetEffectiveRegisterDate(o *model.RawOrder, tzMap map[string]string) string {
	if o == nil {
		return ""
	}
	pid := strings.TrimSpace(o.LandingPageID)
	defaultTz := "CST"
	if strings.EqualFold(o.PlatformCode, "flicknovel") {
		defaultTz = "UTC"
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

	if tz == "UTC" {
		if o.RegisterDateUTC != "" {
			return o.RegisterDateUTC
		}
		if o.RegisterTimeUTC != nil && !o.RegisterTimeUTC.IsZero() {
			return o.RegisterTimeUTC.UTC().Format(timeutil.DateLayout)
		}
		if !o.RegisterTimeBJ.IsZero() {
			return o.RegisterTimeBJ.Add(-8 * time.Hour).Format(timeutil.DateLayout)
		}
		return o.RegisterDateET
	}

	if tz == "ET" {
		if o.RegisterDateET != "" {
			return o.RegisterDateET
		}
		if !o.RegisterTimeET.IsZero() {
			return o.RegisterTimeET.In(timeutil.EasternZone).Format(timeutil.DateLayout)
		}
		return ""
	}

	// 默认 CST / BJ
	if !o.RegisterTimeBJ.IsZero() {
		return o.RegisterTimeBJ.In(timeutil.BeijingZone).Format(timeutil.DateLayout)
	}
	return o.RegisterDateET
}

// GetEffectivePayDate 提取订单的生效支付日期 (按落地页 ID 区分 UTC、美东与北京时间)
func GetEffectivePayDate(o *model.RawOrder, tzMap map[string]string) string {
	if o == nil {
		return ""
	}
	pid := strings.TrimSpace(o.LandingPageID)
	defaultTz := "CST"
	if strings.EqualFold(o.PlatformCode, "flicknovel") {
		defaultTz = "UTC"
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

	if tz == "UTC" {
		if o.PayDateUTC != "" {
			return o.PayDateUTC
		}
		if o.PayTimeUTC != nil && !o.PayTimeUTC.IsZero() {
			return o.PayTimeUTC.UTC().Format(timeutil.DateLayout)
		}
		if !o.PayTimeBJ.IsZero() {
			return o.PayTimeBJ.Add(-8 * time.Hour).Format(timeutil.DateLayout)
		}
		return o.PayDateET
	}

	if tz == "ET" {
		if o.PayDateET != "" {
			return o.PayDateET
		}
		if !o.PayTimeET.IsZero() {
			return o.PayTimeET.In(timeutil.EasternZone).Format(timeutil.DateLayout)
		}
		return ""
	}

	// 默认 CST / BJ
	if !o.PayTimeBJ.IsZero() {
		return o.PayTimeBJ.In(timeutil.BeijingZone).Format(timeutil.DateLayout)
	}
	return o.PayDateET
}
