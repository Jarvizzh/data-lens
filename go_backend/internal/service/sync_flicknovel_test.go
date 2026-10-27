package service

import (
	"context"
	"testing"
	"time"

	"go_backend/internal/model"
	"go_backend/internal/pkg/timeutil"
	"go_backend/internal/service/client/flicknovel"
)

func TestSplitFlicknovelSegments_ShortRange(t *testing.T) {
	start, _ := time.ParseInLocation(timeutil.DateLayout, "2025-01-01", timeutil.BeijingZone)
	end, _ := time.ParseInLocation(timeutil.DateLayout, "2025-01-03", timeutil.BeijingZone)

	segments := splitFlicknovelSegments(start, end)
	if len(segments) != 1 {
		t.Fatalf("expected 1 segment for 3 days, got %d", len(segments))
	}
	if segments[0].StartStr != "2025-01-01" || segments[0].EndStr != "2025-01-03" {
		t.Fatalf("unexpected segment bounds: %s ~ %s", segments[0].StartStr, segments[0].EndStr)
	}
	if segments[0].Index != 0 {
		t.Fatalf("expected index 0, got %d", segments[0].Index)
	}
}

func TestSplitFlicknovelSegments_MultiSegments(t *testing.T) {
	// 60 天测试：2025-01-01 ~ 2025-03-01
	start, _ := time.ParseInLocation(timeutil.DateLayout, "2025-01-01", timeutil.BeijingZone)
	end, _ := time.ParseInLocation(timeutil.DateLayout, "2025-03-01", timeutil.BeijingZone)

	segments := splitFlicknovelSegments(start, end)
	if len(segments) < 2 {
		t.Fatalf("expected multiple segments, got %d", len(segments))
	}

	for i, seg := range segments {
		if seg.Index != i {
			t.Errorf("segment %d index mismatch: %d", i, seg.Index)
		}
		sDate, err1 := time.ParseInLocation(timeutil.DateLayout, seg.StartStr, timeutil.BeijingZone)
		eDate, err2 := time.ParseInLocation(timeutil.DateLayout, seg.EndStr, timeutil.BeijingZone)
		if err1 != nil || err2 != nil {
			t.Fatalf("parse segment date failed: %v, %v", err1, err2)
		}
		// 验证每段跨度不能超过 25 天（天数差 <= 24）
		diffDays := int(eDate.Sub(sDate).Hours() / 24)
		if diffDays > 24 {
			t.Errorf("segment %d exceeds 25 days limit: %d days", i, diffDays+1)
		}
	}

	// 验证第一段从 2025-01-01 开始，最后一段到 2025-03-01 结束
	if segments[0].StartStr != "2025-01-01" {
		t.Errorf("expected first segment to start at 2025-01-01, got %s", segments[0].StartStr)
	}
	lastIdx := len(segments) - 1
	if segments[lastIdx].EndStr != "2025-03-01" {
		t.Errorf("expected last segment to end at 2025-03-01, got %s", segments[lastIdx].EndStr)
	}
}

func TestSyncFlicknovelOrders_ClientNotConfigured(t *testing.T) {
	mgr := &SyncManager{
		fnClient: nil,
	}

	_, err := mgr.SyncFlicknovelOrders(context.Background(), "2025-01-01", "2025-01-05")
	if err == nil {
		t.Fatal("expected error when fnClient is nil, got nil")
	}
	if err.Error() != "flicknovel client not configured" {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestSyncFlicknovelRelations_ClientNotConfigured(t *testing.T) {
	mgr := &SyncManager{
		fnClient: nil,
	}

	_, err := mgr.SyncFlicknovelRelations(context.Background(), "2025-01-01", "2025-01-05")
	if err == nil {
		t.Fatal("expected error when fnClient is nil, got nil")
	}
	if err.Error() != "flicknovel client not configured" {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestFlicknovelOrderItem_EnumsAndPeriod(t *testing.T) {
	resolver := NewFlicknovelOrderTypeResolver(nil)

	// right_type: 2=订阅会员 -> 1, 其余(1,3,4,5) -> 0
	for _, rt := range []string{"1", "3", "4", "5"} {
		item := &flicknovel.OrderItem{RightType: rt}
		if item.IsSubscription() {
			t.Fatalf("expected right_type=%s to not be subscription", rt)
		}
		if resolver.Resolve(&OrderResolveContext{Dto: item}) != 0 {
			t.Fatalf("expected Resolve=0 for right_type=%s", rt)
		}
	}

	subItem := &flicknovel.OrderItem{RightType: "2", Period: "2", PeriodCount: "3", RefundStatus: "4"}
	if !subItem.IsSubscription() {
		t.Fatal("expected right_type=2 to be subscription")
	}
	if resolver.Resolve(&OrderResolveContext{Dto: subItem}) != 1 {
		t.Fatal("expected Resolve=1 for right_type=2")
	}
	if days := subItem.GetSubPeriodDays(); days != 90 {
		t.Fatalf("expected 90 days for period=2, period_count=3 (quarterly card), got %d", days)
	}
	if subItem.GetRefundStatusInt() != 4 {
		t.Fatalf("expected refund_status=4, got %d", subItem.GetRefundStatusInt())
	}

	// 验证 RawOrder.IsRefunded 严格区分 flicknovel(4) 与 rocnovel(2)
	fnOrder4 := &model.RawOrder{PlatformCode: model.PlatformFlicknovel, RefundStatus: 4}
	fnOrder2 := &model.RawOrder{PlatformCode: model.PlatformFlicknovel, RefundStatus: 2}
	rocOrder2 := &model.RawOrder{PlatformCode: model.PlatformRocnovel, RefundStatus: 2}
	rocOrder4 := &model.RawOrder{PlatformCode: model.PlatformRocnovel, RefundStatus: 4}

	if !fnOrder4.IsRefunded() {
		t.Fatal("expected flicknovel refund_status=4 to be refunded")
	}
	if fnOrder2.IsRefunded() {
		t.Fatal("expected flicknovel refund_status=2 to NOT be refunded (strict enum)")
	}
	if !rocOrder2.IsRefunded() {
		t.Fatal("expected rocnovel refund_status=2 to be refunded")
	}
	if rocOrder4.IsRefunded() {
		t.Fatal("expected rocnovel refund_status=4 to NOT be refunded")
	}
}
