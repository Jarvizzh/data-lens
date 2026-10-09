package timeutil

import (
	"testing"
)

func TestTimezoneConversion(t *testing.T) {
	bjStr := "2026-07-10 10:00:00"
	etDate := ParseBjToEtDate(bjStr)
	if etDate == "" {
		t.Fatal("etDate should not be empty")
	}
	// 2026-07-10 10:00:00 CST (UTC+8) -> 2026-07-09 22:00:00 EDT (UTC-4)
	if etDate != "2026-07-09" {
		t.Fatalf("expected 2026-07-09 in ET, got %s", etDate)
	}
}

func TestGetMaxToday(t *testing.T) {
	maxT := GetMaxToday()
	if maxT.IsZero() {
		t.Fatal("GetMaxToday returned zero time")
	}
	maxDateStr := maxT.Format(DateLayout)
	todayBj := GetTodayCst()
	todayEt := GetTodayEt()
	todayUtc := GetTodayUtc()

	if maxDateStr < todayBj || maxDateStr < todayEt || maxDateStr < todayUtc {
		t.Fatalf("GetMaxToday (%s) is smaller than one of timezone dates: Bj=%s, Et=%s, Utc=%s",
			maxDateStr, todayBj, todayEt, todayUtc)
	}
}
