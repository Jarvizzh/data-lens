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
