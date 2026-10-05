package service

import (
	"context"
	"fmt"
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

// BuildMonthlySummary 从每日统计表中聚合近 4 个自然月的月度汇总指标
func (s *MonthlySummaryService) BuildMonthlySummary(
	ctx context.Context,
	stats []*model.LtvDailyStat,
) *dto.MonthlySummaryDto {
	today := time.Now().In(timeutil.BeijingZone)
	currentYM := today.Format("2006-01")

	// 按月份 (yyyy-MM) 分组
	monthMap := make(map[string][]*model.LtvDailyStat)
	for _, stat := range stats {
		if len(stat.LaunchDate) >= 7 {
			ym := stat.LaunchDate[:7]
			monthMap[ym] = append(monthMap[ym], stat)
		}
	}

	months := make([]dto.SingleMonthSummaryDto, 0, 4)
	for i := 0; i < 4; i++ {
		targetDate := today.AddDate(0, -i, 0)
		ym := targetDate.Format("2006-01")
		group := monthMap[ym]

		monthSummary := dto.SingleMonthSummaryDto{
			Month:    ym,
			Spend:    decimal.Zero,
			Recharge: decimal.Zero,
			Refund:   decimal.Zero,
			Profit:   decimal.Zero,
			Roi:      decimal.Zero,
		}

		totalSubCount := 0
		totalRetainedCount := 0

		for _, st := range group {
			monthSummary.Spend = monthSummary.Spend.Add(st.Spend)
			monthSummary.Recharge = monthSummary.Recharge.Add(st.TotalRecharge)
			monthSummary.Refund = monthSummary.Refund.Add(st.TotalRefund)
			totalSubCount += st.SubUserCount
			if st.Day7SubUserCount != nil {
				totalRetainedCount += *st.Day7SubUserCount
			}
		}

		monthSummary.SubUsers = totalSubCount
		monthSummary.RetainedSubUsers = totalRetainedCount
		if totalSubCount > 0 {
			rate := decimal.NewFromInt(int64(totalRetainedCount)).
				DivRound(decimal.NewFromInt(int64(totalSubCount)), 4).
				Mul(decimal.NewFromInt(100))
			monthSummary.RetainedRate = fmt.Sprintf("%.2f%%", rate.InexactFloat64())
		} else {
			monthSummary.RetainedRate = "0.00%"
		}

		monthSummary.Profit = monthSummary.Recharge.Sub(monthSummary.Refund).Sub(monthSummary.Spend)
		if monthSummary.Spend.GreaterThan(decimal.Zero) {
			monthSummary.Roi = monthSummary.Recharge.Sub(monthSummary.Refund).DivRound(monthSummary.Spend, 4)
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
	_ = currentYM
	return res
}
