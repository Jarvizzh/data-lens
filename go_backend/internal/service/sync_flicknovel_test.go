package service

import (
	"context"
	"testing"
	"time"

	"go_backend/internal/pkg/timeutil"
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
