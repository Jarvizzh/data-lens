package service

import (
	"context"
	"fmt"
	"strings"
	"time"

	"go_backend/internal/model"
	"go_backend/internal/pkg/timeutil"
	"go_backend/internal/service/dto"

	"github.com/shopspring/decimal"
)

type MonthlySummaryService struct{}

func NewMonthlySummaryService() *MonthlySummaryService {
	return &MonthlySummaryService{}
}

type RetainedSubscribersInfo struct {
	SubUsers         int
	RetainedSubUsers int
	RetainedRate     string
}

// CalculateRetainedSubscribers 计算订阅用户留存 (对齐 Java 核心算法)
func CalculateRetainedSubscribers(
	orders []*model.RawOrder,
	tzMap map[string]string,
	periodMap map[string]int,
) RetainedSubscribersInfo {
	if len(orders) == 0 {
		return RetainedSubscribersInfo{SubUsers: 0, RetainedSubUsers: 0, RetainedRate: "0.00%"}
	}

	subMemberIdsMap := make(map[string]bool)
	for _, o := range orders {
		if o.IsSubs == 1 && strings.TrimSpace(o.MemberID) != "" {
			subMemberIdsMap[strings.TrimSpace(o.MemberID)] = true
		}
	}

	totalSubCount := len(subMemberIdsMap)
	if totalSubCount == 0 {
		return RetainedSubscribersInfo{SubUsers: 0, RetainedSubUsers: 0, RetainedRate: "0.00%"}
	}

	todayBj := time.Now().In(timeutil.BeijingZone).Format(timeutil.DateLayout)
	todayBjDate, _ := time.ParseInLocation(timeutil.DateLayout, todayBj, timeutil.BeijingZone)

	// 找出每个订阅会员的最后有效支付日期
	lastPayDateMap := make(map[string]time.Time)
	for _, o := range orders {
		mID := strings.TrimSpace(o.MemberID)
		if !subMemberIdsMap[mID] {
			continue
		}
		payDateStr := GetEffectivePayDate(o, tzMap)
		if len(payDateStr) >= 10 {
			payDateStr = payDateStr[:10]
		}
		pDate, err := time.ParseInLocation(timeutil.DateLayout, payDateStr, timeutil.BeijingZone)
		if err != nil {
			continue
		}
		if prev, ok := lastPayDateMap[mID]; !ok || pDate.After(prev) {
			lastPayDateMap[mID] = pDate
		}
	}

	retainedCount := 0
	for mID := range subMemberIdsMap {
		lastPay, ok := lastPayDateMap[mID]
		if !ok {
			continue
		}
		periodDays := 1
		if p, ok := periodMap[mID]; ok && p > 0 {
			periodDays = p
		}
		expireDate := lastPay.AddDate(0, 0, periodDays)
		if !todayBjDate.After(expireDate) {
			retainedCount++
		}
	}

	rate := decimal.Zero
	if totalSubCount > 0 {
		rate = decimal.NewFromInt(int64(retainedCount)).
			DivRound(decimal.NewFromInt(int64(totalSubCount)), 4).
			Mul(decimal.NewFromInt(100))
	}

	return RetainedSubscribersInfo{
		SubUsers:         totalSubCount,
		RetainedSubUsers: retainedCount,
		RetainedRate:     fmt.Sprintf("%.2f%%", rate.InexactFloat64()),
	}
}

// CalculateActualPaybackDaysForMonth 计算历史月份的实际回本天数
func CalculateActualPaybackDaysForMonth(
	monthStats []*model.LtvDailyStat,
	monthOrders []*model.RawOrder,
	totalSpend decimal.Decimal,
	totalRecharge decimal.Decimal,
	tzMap map[string]string,
	firstDayOfMonth time.Time,
) *int {
	todayBj := time.Now().In(timeutil.BeijingZone).Format(timeutil.DateLayout)
	todayBjDate, _ := time.ParseInLocation(timeutil.DateLayout, todayBj, timeutil.BeijingZone)

	// 1. 若总消耗 <= 0 或 截至目前的总充值 < 总消耗 (未回本)，绝对不推测，直接返回 nil
	if totalSpend.LessThanOrEqual(decimal.Zero) || totalRecharge.LessThan(totalSpend) {
		return nil
	}

	// 2. 找出该月首个有消耗的自然日 (firstSpendDate) 作为基准
	var firstSpendDate time.Time
	hasSpend := false
	for _, s := range monthStats {
		if s.Spend.GreaterThan(decimal.Zero) && len(s.LaunchDate) >= 10 {
			lDate, err := time.ParseInLocation(timeutil.DateLayout, s.LaunchDate[:10], timeutil.BeijingZone)
			if err == nil {
				if !hasSpend || lDate.Before(firstSpendDate) {
					firstSpendDate = lDate
					hasSpend = true
				}
			}
		}
	}
	if !hasSpend {
		firstSpendDate = firstDayOfMonth
	}
	if firstSpendDate.After(todayBjDate) {
		return nil
	}

	// 3. 按订单生效支付日期 (payDate) 归集每日充值额 (早于首个消耗日的归计到首个消耗日)
	rechargeByPayDate := make(map[string]decimal.Decimal)
	for _, o := range monthOrders {
		payDateStr := GetEffectivePayDate(o, tzMap)
		if len(payDateStr) >= 10 {
			payDateStr = payDateStr[:10]
		}
		pDate, err := time.ParseInLocation(timeutil.DateLayout, payDateStr, timeutil.BeijingZone)
		if err != nil || pDate.After(todayBjDate) {
			continue
		}
		effectivePay := payDateStr
		if pDate.Before(firstSpendDate) {
			effectivePay = firstSpendDate.Format(timeutil.DateLayout)
		}
		rechargeByPayDate[effectivePay] = rechargeByPayDate[effectivePay].Add(o.OrderAmountUSD)
	}

	// 4. 从首个消耗日 firstSpendDate 开始按自然日推进累加充值额，截至今天
	cumulative := decimal.Zero
	targetThreshold := totalSpend.Sub(decimal.NewFromFloat(0.01))
	currDate := firstSpendDate

	for !currDate.After(todayBjDate) {
		dayStr := currDate.Format(timeutil.DateLayout)
		dayAmount := rechargeByPayDate[dayStr]
		cumulative = cumulative.Add(dayAmount)

		if cumulative.GreaterThanOrEqual(targetThreshold) {
			days := int(currDate.Sub(firstSpendDate).Hours()/24) + 1
			return &days
		}
		currDate = currDate.AddDate(0, 0, 1)
	}

	return nil
}

// BuildMonthlySummary 从每日统计与订单中聚合近 6 个自然月的月度汇总指标
func (s *MonthlySummaryService) BuildMonthlySummary(
	ctx context.Context,
	platformCode string,
	stats []*model.LtvDailyStat,
	orders []*model.RawOrder,
	tzMap map[string]string,
	periodMap map[string]int,
) *dto.MonthlySummaryDto {
	today := time.Now().In(timeutil.BeijingZone)
	currentYM := today.Format("2006-01")

	platStartDate := model.GetLaunchStartDateForPlatform(platformCode)
	minYM := ""
	if len(platStartDate) >= 7 {
		minYM = platStartDate[:7]
	}

	// 按月份 (yyyy-MM) 分组每日统计
	monthStatMap := make(map[string][]*model.LtvDailyStat)
	for _, stat := range stats {
		if len(stat.LaunchDate) >= 7 {
			ym := stat.LaunchDate[:7]
			monthStatMap[ym] = append(monthStatMap[ym], stat)
		}
	}

	// 按用户生效注册月份 (yyyy-MM) 分组订单
	monthOrderMap := make(map[string][]*model.RawOrder)
	for _, o := range orders {
		regDate := GetEffectiveRegisterDate(o, tzMap)
		if len(regDate) >= 7 {
			ym := regDate[:7]
			monthOrderMap[ym] = append(monthOrderMap[ym], o)
		}
	}

	months := make([]dto.SingleMonthSummaryDto, 0, 6)
	for i := 0; i < 6; i++ {
		targetDate := today.AddDate(0, -i, 0)
		ym := targetDate.Format("2006-01")
		if minYM != "" && ym < minYM {
			break
		}
		groupStats := monthStatMap[ym]
		groupOrders := monthOrderMap[ym]
		isPastMonth := ym != currentYM

		monthSummary := dto.SingleMonthSummaryDto{
			Month:    ym,
			Spend:    decimal.Zero,
			Recharge: decimal.Zero,
			Refund:   decimal.Zero,
			Profit:   decimal.Zero,
			Roi:      decimal.Zero,
		}

		for _, st := range groupStats {
			monthSummary.Spend = monthSummary.Spend.Add(st.Spend)
			monthSummary.Recharge = monthSummary.Recharge.Add(st.TotalRecharge)
			monthSummary.Refund = monthSummary.Refund.Add(st.TotalRefund)
		}

		monthSummary.Profit = monthSummary.Recharge.Sub(monthSummary.Refund).Sub(monthSummary.Spend)

		// 1. 月度 ROI 计算：按百分比数值返回 (如 92.34 代表 92.34%)，对齐前端展示
		if monthSummary.Spend.GreaterThan(decimal.Zero) {
			monthSummary.Roi = monthSummary.Recharge.Sub(monthSummary.Refund).
				DivRound(monthSummary.Spend, 4).
				Mul(decimal.NewFromInt(100)).
				Round(2)
		}

		// 2. 月度订阅留存计算：精准基于该月注册订单的用户集合及其订阅周期和最后支付时间
		retention := CalculateRetainedSubscribers(groupOrders, tzMap, periodMap)
		monthSummary.SubUsers = retention.SubUsers
		monthSummary.RetainedSubUsers = retention.RetainedSubUsers
		monthSummary.RetainedRate = retention.RetainedRate

		// 3. 历史月份计算实际回本天数与预测 ROI
		firstDayOfMonth, _ := time.ParseInLocation("2006-01-02", ym+"-01", timeutil.BeijingZone)
		if isPastMonth {
			monthSummary.ActualPaybackDays = CalculateActualPaybackDaysForMonth(
				groupStats, groupOrders, monthSummary.Spend, monthSummary.Recharge, tzMap, firstDayOfMonth,
			)

			if monthSummary.Spend.GreaterThan(decimal.Zero) && len(groupStats) > 0 {
				sumD30 := decimal.Zero
				sumD60 := decimal.Zero
				sumD90 := decimal.Zero
				for _, st := range groupStats {
					if st.Spend.GreaterThan(decimal.Zero) {
						d30 := st.TotalRecharge
						if st.PredictedDay30Recharge != nil {
							d30 = *st.PredictedDay30Recharge
						}
						d60 := d30
						if st.PredictedDay60Recharge != nil {
							d60 = *st.PredictedDay60Recharge
						}
						d90 := d60
						if st.PredictedDay90Recharge != nil {
							d90 = *st.PredictedDay90Recharge
						}
						sumD30 = sumD30.Add(d30)
						sumD60 = sumD60.Add(d60)
						sumD90 = sumD90.Add(d90)
					}
				}
				d30Roi := sumD30.DivRound(monthSummary.Spend, 4)
				d60Roi := sumD60.DivRound(monthSummary.Spend, 4)
				d90Roi := sumD90.DivRound(monthSummary.Spend, 4)
				monthSummary.PredictedDay30Roi = &d30Roi
				monthSummary.PredictedDay60Roi = &d60Roi
				monthSummary.PredictedDay90Roi = &d90Roi
			}
		}

		months = append(months, monthSummary)
	}

	res := &dto.MonthlySummaryDto{
		Months: months,
	}
	if len(months) > 0 {
		res.ThisMonth = &months[0]
	}
	if len(months) > 1 {
		res.LastMonth = &months[1]
	}
	return res
}
