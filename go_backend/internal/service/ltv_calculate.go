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
	day8Date := launchDate.AddDate(0, 0, 7)
	if !day8Date.After(maxToday) {
		retained7 := make(map[string]struct{})
		for _, o := range cohortOrders {
			if _, ok := subMembersMap[o.MemberID]; ok {
				payDate := c.getEffectivePayDate(o, tzMap)
				if !payDate.IsZero() && int(payDate.Sub(launchDate).Hours()/24) >= 7 {
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

	day16Date := launchDate.AddDate(0, 0, 15)
	if !day16Date.After(maxToday) {
		retained15 := make(map[string]struct{})
		for _, o := range cohortOrders {
			if _, ok := subMembersMap[o.MemberID]; ok {
				payDate := c.getEffectivePayDate(o, tzMap)
				if !payDate.IsZero() && int(payDate.Sub(launchDate).Hours()/24) >= 15 {
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
		targetDate := launchDate.AddDate(0, 0, day-1)
		if targetDate.After(maxToday) {
			break
		}

		dayCumRecharge := decimal.Zero
		for _, o := range cohortOrders {
			payDate := c.getEffectivePayDate(o, tzMap)
			if !payDate.IsZero() && !payDate.After(targetDate) {
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

func (c *LtvCalculator) getEffectivePayDate(o *model.RawOrder, tzMap map[string]string) time.Time {
	tz := "CST"
	if tzMap != nil && o.LandingPageID != "" {
		if t, ok := tzMap[o.LandingPageID]; ok {
			tz = t
		}
	}

	if strings.EqualFold(tz, "ET") {
		if o.PayDateET != "" {
			t, err := time.ParseInLocation(timeutil.DateLayout, o.PayDateET, timeutil.EasternZone)
			if err == nil {
				return t
			}
		}
		return o.PayTimeET
	}

	// 默认 CST
	bjDate := o.PayTimeBJ.In(timeutil.BeijingZone).Format(timeutil.DateLayout)
	t, _ := time.ParseInLocation(timeutil.DateLayout, bjDate, timeutil.BeijingZone)
	return t
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
