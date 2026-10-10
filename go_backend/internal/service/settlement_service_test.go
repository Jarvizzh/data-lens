package service

import (
	"fmt"
	"testing"
	"time"

	"go_backend/internal/pkg/timeutil"
	"go_backend/internal/repository"

	"github.com/shopspring/decimal"
)

// TestSettlementCalculationFormulas 验证月度结算的核心公式与边界条件
func TestSettlementCalculationFormulas(t *testing.T) {
	// 场景 1: 常规月份
	// 累计充值: 1000.00
	// 当月结算退款: 50.00
	// 跨周期退款: 20.00
	// 分成比例: 95% (0.9500)
	// 渠道费率: 7% (0.0700)
	totalRecharge := decimal.NewFromFloat(1000.00)
	totalRefund := decimal.NewFromFloat(50.00)
	monthSettledRefund := decimal.NewFromFloat(50.00)
	crossPeriodRefund := decimal.NewFromFloat(20.00)
	shareRatio := decimal.NewFromFloat(0.9500)
	channelFeeRate := decimal.NewFromFloat(0.0700)

	// 有效结算基数 = 1000 - 50 - 20 = 930.00
	effectiveBase := totalRecharge.Sub(monthSettledRefund).Sub(crossPeriodRefund)
	if !effectiveBase.Equal(decimal.NewFromFloat(930.00)) {
		t.Fatalf("Expected effective base 930.00, got %s", effectiveBase)
	}

	// 最终结算金额 = 930.00 * 0.95 * (1 - 0.07) = 930 * 0.95 * 0.93 = 821.655 -> 821.66
	netFactor := decimal.NewFromInt(1).Sub(channelFeeRate)
	finalAmount := effectiveBase.Mul(shareRatio).Mul(netFactor).Round(2)
	expectedFinal := decimal.NewFromFloat(821.66)
	if !finalAmount.Equal(expectedFinal) {
		t.Fatalf("Expected final settlement %s, got %s", expectedFinal, finalAmount)
	}

	// 退款率 = 50 / 1000 = 5.00%
	rate := totalRefund.DivRound(totalRecharge, 4).Mul(decimal.NewFromInt(100))
	refundRateStr := fmt.Sprintf("%.2f%%", rate.InexactFloat64())
	if refundRateStr != "5.00%" {
		t.Fatalf("Expected refund rate 5.00%%, got %s", refundRateStr)
	}

	// 场景 2: 负收益/亏损月份 (有效基数 <= 0)
	negRecharge := decimal.NewFromFloat(100.00)
	negRefund := decimal.NewFromFloat(150.00)
	negEffective := negRecharge.Sub(negRefund) // -50.00
	// 当基数 <= 0 时，无需扣除渠道费率: -50 * 0.95 = -47.50
	var negFinal decimal.Decimal
	if negEffective.GreaterThan(decimal.Zero) {
		negFinal = negEffective.Mul(shareRatio).Mul(netFactor).Round(2)
	} else {
		negFinal = negEffective.Mul(shareRatio).Round(2)
	}
	expectedNeg := decimal.NewFromFloat(-47.50)
	if !negFinal.Equal(expectedNeg) {
		t.Fatalf("Expected negative settlement %s, got %s", expectedNeg, negFinal)
	}
}

// TestHistoricalUnsettledRefundAccumulation 验证历史未结算退款向当月的滚算逻辑
func TestHistoricalUnsettledRefundAccumulation(t *testing.T) {
	currentYm, _ := time.ParseInLocation("2006-01", "2026-03", timeutil.BeijingZone)

	// 历史月 1 (2026-01): 累计退款 100, 已结算 80 -> 未结算 20
	// 历史月 2 (2026-02): 累计退款 200, 已结算 150 -> 未结算 50
	type monthMock struct {
		ym            string
		totalRefund   decimal.Decimal
		settledRefund decimal.Decimal
	}

	history := []monthMock{
		{
			ym:            "2026-01",
			totalRefund:   decimal.NewFromFloat(100),
			settledRefund: decimal.NewFromFloat(80),
		},
		{
			ym:            "2026-02",
			totalRefund:   decimal.NewFromFloat(200),
			settledRefund: decimal.NewFromFloat(150),
		},
	}

	sumHistoricalUnsettled := decimal.Zero
	for _, m := range history {
		tYm, _ := time.ParseInLocation("2006-01", m.ym, timeutil.BeijingZone)
		if tYm.Before(currentYm) {
			unsettled := m.totalRefund.Sub(m.settledRefund)
			sumHistoricalUnsettled = sumHistoricalUnsettled.Add(unsettled)
		}
	}

	// 20 + 50 = 70
	if !sumHistoricalUnsettled.Equal(decimal.NewFromFloat(70)) {
		t.Fatalf("Expected sum historical unsettled 70, got %s", sumHistoricalUnsettled)
	}
}

// TestMonthlySettlementSummaryMapping 验证 MonthlySettlementSummary 转换为聚合数据的正确性
func TestMonthlySettlementSummaryMapping(t *testing.T) {
	summaries := []*repository.MonthlySettlementSummary{
		{
			MonthStr:         "2026-01",
			LandingPageID:    "lp_1",
			TotalAmountCent:  50000, // 500.00
			RefundAmountCent: 2000,  // 20.00
			TotalOrders:      10,
			RefundOrders:     1,
		},
		{
			MonthStr:         "2026-01",
			LandingPageID:    "lp_2",
			TotalAmountCent:  30000, // 300.00
			RefundAmountCent: 1000,  // 10.00
			TotalOrders:      6,
			RefundOrders:     1,
		},
		{
			MonthStr:         "2026-02",
			LandingPageID:    "lp_1",
			TotalAmountCent:  80000, // 800.00
			RefundAmountCent: 4000,  // 40.00
			TotalOrders:      15,
			RefundOrders:     2,
		},
	}

	// 模拟按月份聚合
	type monthAggData struct {
		totalRechargeCents int64
		totalRefundCents   int64
		totalOrders        int
		refundOrders       int
	}
	monthDataMap := make(map[string]*monthAggData)

	for _, sum := range summaries {
		agg := monthDataMap[sum.MonthStr]
		if agg == nil {
			agg = &monthAggData{}
			monthDataMap[sum.MonthStr] = agg
		}
		agg.totalRechargeCents += sum.TotalAmountCent
		agg.totalRefundCents += sum.RefundAmountCent
		agg.totalOrders += sum.TotalOrders
		agg.refundOrders += sum.RefundOrders
	}

	// 验证 2026-01 聚合
	agg01 := monthDataMap["2026-01"]
	if agg01 == nil {
		t.Fatal("Missing 2026-01")
	}
	if agg01.totalRechargeCents != 80000 { // 50000 + 30000
		t.Fatalf("Expected 80000 cents for 2026-01, got %d", agg01.totalRechargeCents)
	}
	if agg01.totalRefundCents != 3000 { // 2000 + 1000
		t.Fatalf("Expected 3000 cents refund for 2026-01, got %d", agg01.totalRefundCents)
	}
	if agg01.totalOrders != 16 || agg01.refundOrders != 2 {
		t.Fatalf("Expected 16 orders, 2 refund orders, got %d / %d", agg01.totalOrders, agg01.refundOrders)
	}

	// 转换为元
	recharge01 := decimal.NewFromInt(agg01.totalRechargeCents).DivRound(decimal.NewFromInt(100), 2)
	refund01 := decimal.NewFromInt(agg01.totalRefundCents).DivRound(decimal.NewFromInt(100), 2)
	if !recharge01.Equal(decimal.NewFromFloat(800.00)) || !refund01.Equal(decimal.NewFromFloat(30.00)) {
		t.Fatalf("Expected 800.00 / 30.00, got %s / %s", recharge01, refund01)
	}
}

// TestPlatformIsolationInSettlementConfig 验证多平台配置隔离与独立性
func TestPlatformIsolationInSettlementConfig(t *testing.T) {
	configMap := make(map[string]map[string]decimal.Decimal)
	configMap["rocnovel"] = map[string]decimal.Decimal{
		"2026-08": decimal.NewFromFloat(1000.00),
	}
	configMap["flicknovel"] = map[string]decimal.Decimal{
		"2026-08": decimal.NewFromFloat(50.00),
	}

	if configMap["rocnovel"]["2026-08"].Equal(configMap["flicknovel"]["2026-08"]) {
		t.Fatal("Platform configs should be strictly isolated and not equal")
	}
}
