package engine

import (
	"testing"
	"time"

	"go_backend/internal/model"

	"github.com/shopspring/decimal"
)

func TestOptimalScaleFactor(t *testing.T) {
	spend := decimal.NewFromFloat(500.0)
	alpha := ComputeOptimalScaleFactor(0.85, 0.80, 5, spend, false)
	if alpha < EarlyStageMinAlpha || alpha > EarlyStageMaxAlpha {
		t.Fatalf("early stage alpha %f out of bounds [%f, %f]", alpha, EarlyStageMinAlpha, EarlyStageMaxAlpha)
	}

	alphaMature := ComputeOptimalScaleFactor(1.2, 0.9, 20, spend, false)
	if alphaMature < MatureStageMinAlpha || alphaMature > MatureStageMaxAlpha {
		t.Fatalf("mature stage alpha %f out of bounds", alphaMature)
	}
}

func ptrDecimal(f float64) *decimal.Decimal {
	d := decimal.NewFromFloat(f)
	return &d
}

func TestOlsFit(t *testing.T) {
	stat := &model.LtvDailyStat{
		Spend:    decimal.NewFromFloat(100.0),
		Day1Roi:  ptrDecimal(0.30),
		Day2Roi:  ptrDecimal(0.40),
		Day3Roi:  ptrDecimal(0.48),
		Day4Roi:  ptrDecimal(0.55),
		Day5Roi:  ptrDecimal(0.60),
		Day6Roi:  ptrDecimal(0.65),
		Day7Roi:  ptrDecimal(0.69),
		Day14Roi: ptrDecimal(0.85),
	}

	fit := ComputeOlsFit(stat, 14)
	if !fit.Valid {
		t.Logf("OLS fit result: a=%f, b=%f, r2=%f, valid=%v", fit.A, fit.B, fit.R2, fit.Valid)
	}
	if fit.A <= 0 {
		t.Fatalf("expected positive slope, got %f", fit.A)
	}
}

func TestPaybackAndRoiEngines(t *testing.T) {
	d1R := decimal.NewFromFloat(50.0)
	d1Roi := decimal.NewFromFloat(0.50)
	d2R := decimal.NewFromFloat(70.0)
	d2Roi := decimal.NewFromFloat(0.70)
	d3R := decimal.NewFromFloat(85.0)
	d3Roi := decimal.NewFromFloat(0.85)

	stat := &model.LtvDailyStat{
		Spend:        decimal.NewFromFloat(100.0),
		Day1Recharge: &d1R,
		Day1Roi:      &d1Roi,
		Day2Recharge: &d2R,
		Day2Roi:      &d2Roi,
		Day3Recharge: &d3R,
		Day3Roi:      &d3Roi,
		SubUserCount: 10,
	}

	cumCurve := make([]float64, 367)
	for i := 1; i <= 366; i++ {
		cumCurve[i] = 85.0 + float64(i)*2.0 // crosses 100 at i = 8
	}

	paybackEngine := NewPaybackPredictEngine()
	days := paybackEngine.CalculateCohortPaybackDays(stat, 3, cumCurve)
	if days == nil || *days != 8 {
		t.Fatalf("expected payback day 8, got %v", days)
	}

	roiEngine := NewRoiPredictEngine()
	roiResult := roiEngine.CalculateCohortRoiTrend(stat, 3, cumCurve)
	if roiResult.PredD30Roi == nil || roiResult.PredD30Roi.LessThanOrEqual(decimal.NewFromFloat(0.85)) {
		t.Fatalf("expected D30 ROI > 0.85, got %v", roiResult.PredD30Roi)
	}

	facade := NewLtvPredictFacade(paybackEngine, roiEngine)
	pred := facade.AssembleCohortPrediction(stat, 3, cumCurve)
	if pred.PredictedPaybackDays == nil || *pred.PredictedPaybackDays != 8 {
		t.Fatalf("expected facade payback day 8, got %v", pred.PredictedPaybackDays)
	}

	// Overall test
	curves := map[*model.LtvDailyStat][]float64{
		stat: cumCurve,
	}
	minDate := time.Date(2026, 7, 10, 0, 0, 0, 0, time.UTC)
	today := time.Date(2026, 7, 12, 0, 0, 0, 0, time.UTC)
	overallDays := paybackEngine.CalculateOverallPaybackDays(stat.Spend, decimal.NewFromFloat(85.0), []*model.LtvDailyStat{stat}, curves, minDate, today)
	if overallDays == nil {
		t.Fatalf("expected overall days calculated, got nil")
	}
}
