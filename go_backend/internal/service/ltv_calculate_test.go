package service

import (
	"context"
	"testing"
	"time"

	"go_backend/internal/model"
	"go_backend/internal/pkg/timeutil"

	"github.com/shopspring/decimal"
)

func TestCalculateSingleCohort_PrefixSumAndRetention(t *testing.T) {
	predictSvc := NewPredictService(nil)
	calc := NewLtvCalculator(nil, nil, nil, predictSvc)

	launchDateStr := "2026-01-01"
	launchDate, _ := time.ParseInLocation(timeutil.DateLayout, launchDateStr, timeutil.BeijingZone)
	maxToday, _ := time.ParseInLocation(timeutil.DateLayout, "2026-03-01", timeutil.BeijingZone)

	// Cohort of 3 users:
	// User A: sub user, paid 10 on Day 1, paid 5 on Day 8, paid 5 on Day 16
	// User B: sub user, paid 20 on Day 2
	// User C: non-sub user, paid 30 on Day 1
	cohortOrders := []*model.RawOrder{
		{
			PlatformCode:   model.PlatformRocnovel,
			MemberID:       "userA",
			LandingPageID:  "lp1",
			PayDateET:      "2026-01-01",
			OrderAmountUSD: decimal.NewFromInt(10),
			IsSubs:         1,
		},
		{
			PlatformCode:   model.PlatformRocnovel,
			MemberID:       "userA",
			LandingPageID:  "lp1",
			PayDateET:      "2026-01-08", // Day 8 (Day 7 retention: payDate >= day8DateStr)
			OrderAmountUSD: decimal.NewFromInt(5),
			IsSubs:         1,
		},
		{
			PlatformCode:   model.PlatformRocnovel,
			MemberID:       "userA",
			LandingPageID:  "lp1",
			PayDateET:      "2026-01-16", // Day 16 (Day 15 retention: payDate >= day16DateStr)
			OrderAmountUSD: decimal.NewFromInt(5),
			IsSubs:         1,
		},
		{
			PlatformCode:   model.PlatformRocnovel,
			MemberID:       "userB",
			LandingPageID:  "lp1",
			PayDateET:      "2026-01-02", // Day 2
			OrderAmountUSD: decimal.NewFromInt(20),
			IsSubs:         1,
		},
		{
			PlatformCode:   model.PlatformRocnovel,
			MemberID:       "userC",
			LandingPageID:  "lp1",
			PayDateET:      "2026-01-01", // Day 1
			OrderAmountUSD: decimal.NewFromInt(30),
			IsSubs:         0,
		},
	}

	spend := decimal.NewFromInt(100)
	tzMap := map[string]string{"lp1": "ET"}
	periodMap := map[string]int{"userA": 7, "userB": 7}

	stat := calc.CalculateSingleCohort(
		context.Background(),
		model.PlatformRocnovel,
		1,
		launchDateStr,
		cohortOrders,
		spend,
		"test",
		maxToday,
		tzMap,
		periodMap,
	)

	if stat == nil {
		t.Fatal("Expected non-nil stat")
	}

	// 1. Check TotalRecharge: 10 + 5 + 5 + 20 + 30 = 70
	if !stat.TotalRecharge.Equal(decimal.NewFromInt(70)) {
		t.Fatalf("Expected TotalRecharge 70, got %s", stat.TotalRecharge)
	}

	// 2. Check SubUserCount: userA, userB = 2
	if stat.SubUserCount != 2 {
		t.Fatalf("Expected SubUserCount 2, got %d", stat.SubUserCount)
	}

	// 3. Check Retention:
	// Day 7: userA paid on Day 8 >= Day 8 -> 1 retained / 2 = 0.5000
	if stat.Day7SubUserCount == nil || *stat.Day7SubUserCount != 1 {
		t.Fatalf("Expected Day7SubUserCount 1, got %v", stat.Day7SubUserCount)
	}
	expectedR7 := decimal.NewFromFloat(0.5)
	if stat.Day7SubUserRetention == nil || !stat.Day7SubUserRetention.Equal(expectedR7) {
		t.Fatalf("Expected Day7SubUserRetention 0.5, got %v", stat.Day7SubUserRetention)
	}

	// Day 15: userA paid on Day 16 >= Day 16 -> 1 retained / 2 = 0.5000
	if stat.Day15SubUserCount == nil || *stat.Day15SubUserCount != 1 {
		t.Fatalf("Expected Day15SubUserCount 1, got %v", stat.Day15SubUserCount)
	}

	// 4. Check Day 1 ~ Day 60 cumulative recharge prefix sum:
	// Day 1: UserA (10) + UserC (30) = 40
	d1 := stat.GetRechargeForDay(1)
	if !d1.Equal(decimal.NewFromInt(40)) {
		t.Fatalf("Expected Day 1 recharge 40, got %v", d1)
	}

	// Day 2: Day 1 (40) + UserB (20) = 60
	d2 := stat.GetRechargeForDay(2)
	if !d2.Equal(decimal.NewFromInt(60)) {
		t.Fatalf("Expected Day 2 recharge 60, got %v", d2)
	}

	// Day 7: Still 60 (no purchases on Day 3..7)
	d7 := stat.GetRechargeForDay(7)
	if !d7.Equal(decimal.NewFromInt(60)) {
		t.Fatalf("Expected Day 7 recharge 60, got %v", d7)
	}

	// Day 8: Day 7 (60) + UserA (5) = 65
	d8 := stat.GetRechargeForDay(8)
	if !d8.Equal(decimal.NewFromInt(65)) {
		t.Fatalf("Expected Day 8 recharge 65, got %v", d8)
	}

	// Day 16: Day 15 (65) + UserA (5) = 70
	d16 := stat.GetRechargeForDay(16)
	if !d16.Equal(decimal.NewFromInt(70)) {
		t.Fatalf("Expected Day 16 recharge 70, got %v", d16)
	}

	// Day 60: 70
	d60 := stat.GetRechargeForDay(60)
	if !d60.Equal(decimal.NewFromInt(70)) {
		t.Fatalf("Expected Day 60 recharge 70, got %v", d60)
	}

	_ = launchDate
}
