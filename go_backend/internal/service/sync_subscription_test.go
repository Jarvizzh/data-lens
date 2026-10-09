package service

import (
	"testing"
	"time"

	"go_backend/internal/model"
)

func TestMatchSubscriptionVersion_TieredMatching(t *testing.T) {
	now := time.Date(2026, 1, 15, 12, 0, 0, 0, time.UTC)
	past := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	future := time.Date(2027, 1, 1, 0, 0, 0, 0, time.UTC)

	v1 := &model.SubscriptionConfigVersion{
		ID:                 1,
		LandingPageID:      "lp100",
		SubscribeConfigID:  "cfg1",
		FirstPriceCent:     199,
		RenewPriceCent:     999,
		SubPeriodDays:      7,
		VersionNum:         1,
		EffectiveStartTime: past,
		EffectiveEndTime:   nil,
	}

	v2 := &model.SubscriptionConfigVersion{
		ID:                 2,
		LandingPageID:      "lp100",
		SubscribeConfigID:  "cfg2",
		FirstPriceCent:     299,
		RenewPriceCent:     999,
		SubPeriodDays:      30,
		VersionNum:         2,
		EffectiveStartTime: past,
		EffectiveEndTime:   nil,
	}

	v3 := &model.SubscriptionConfigVersion{
		ID:                 3,
		LandingPageID:      "lp200",
		SubscribeConfigID:  "cfg3",
		FirstPriceCent:     499,
		RenewPriceCent:     1999,
		SubPeriodDays:      365,
		VersionNum:         1,
		EffectiveStartTime: past,
		EffectiveEndTime:   nil,
	}

	versions := []*model.SubscriptionConfigVersion{v1, v2, v3}

	// 1. Exact match: lp100, price 199 -> v1
	matched1 := matchSubscriptionVersion(versions, "lp100", 199, now)
	if matched1 == nil || matched1.ID != 1 {
		t.Fatalf("Expected exact match v1, got %v", matched1)
	}

	// 2. Landing page fallback: lp100, price 250 -> closest on lp100 is v2 (diff 49 vs 51)
	matched2 := matchSubscriptionVersion(versions, "lp100", 250, now)
	if matched2 == nil || matched2.ID != 2 {
		t.Fatalf("Expected page fallback v2, got %v", matched2)
	}

	// 3. Global fallback: unknown lp999, price 450 -> closest in all versions is v3 (diff 49)
	matched3 := matchSubscriptionVersion(versions, "lp999", 450, now)
	if matched3 == nil || matched3.ID != 3 {
		t.Fatalf("Expected global fallback v3, got %v", matched3)
	}

	_ = future
}
