package engine

import (
	"time"

	"go_backend/internal/model"

	"github.com/shopspring/decimal"
)

// PredictionResult 预测结果包装
type PredictionResult struct {
	PredictedPaybackDays   *int             `json:"predictedPaybackDays"`
	PredictedDay30Roi      *decimal.Decimal `json:"predictedDay30Roi"`
	PredictedDay60Roi      *decimal.Decimal `json:"predictedDay60Roi"`
	PredictedDay90Roi      *decimal.Decimal `json:"predictedDay90Roi"`
	PredictedDay30Recharge *decimal.Decimal `json:"predictedDay30Recharge"`
	PredictedDay60Recharge *decimal.Decimal `json:"predictedDay60Recharge"`
	PredictedDay90Recharge *decimal.Decimal `json:"predictedDay90Recharge"`
}

// LtvPredictFacade 预测引擎门面 (Facade)
type LtvPredictFacade struct {
	PaybackEngine *PaybackPredictEngine
	RoiEngine     *RoiPredictEngine
}

func NewLtvPredictFacade(paybackEngine *PaybackPredictEngine, roiEngine *RoiPredictEngine) *LtvPredictFacade {
	return &LtvPredictFacade{
		PaybackEngine: paybackEngine,
		RoiEngine:     roiEngine,
	}
}

// AssembleCohortPrediction 聚合单 Cohort 的回本预测与 ROI 趋势预测
func (f *LtvPredictFacade) AssembleCohortPrediction(stat *model.LtvDailyStat, daysElapsed int, cumRechargeCurve []float64) PredictionResult {
	if stat == nil || stat.Spend.LessThanOrEqual(decimal.Zero) {
		return PredictionResult{}
	}

	roiResult := f.RoiEngine.CalculateCohortRoiTrend(stat, daysElapsed, cumRechargeCurve)
	paybackDays := f.PaybackEngine.CalculateCohortPaybackDays(stat, daysElapsed, cumRechargeCurve)

	return PredictionResult{
		PredictedPaybackDays:   paybackDays,
		PredictedDay30Roi:      roiResult.PredD30Roi,
		PredictedDay60Roi:      roiResult.PredD60Roi,
		PredictedDay90Roi:      roiResult.PredD90Roi,
		PredictedDay30Recharge: roiResult.PredD30Recharge,
		PredictedDay60Recharge: roiResult.PredD60Recharge,
		PredictedDay90Recharge: roiResult.PredD90Recharge,
	}
}

// AssembleOverallPrediction 聚合大盘整体 (Overall Cohort) 的回本预测
func (f *LtvPredictFacade) AssembleOverallPrediction(
	totalSpendAll, totalRechargeAll decimal.Decimal,
	validStats []*model.LtvDailyStat,
	cohortCurves map[*model.LtvDailyStat][]float64,
	minLaunchDate, today time.Time,
) PredictionResult {
	if totalSpendAll.LessThanOrEqual(decimal.Zero) {
		return PredictionResult{}
	}

	overallDays := f.PaybackEngine.CalculateOverallPaybackDays(totalSpendAll, totalRechargeAll, validStats, cohortCurves, minLaunchDate, today)
	return PredictionResult{
		PredictedPaybackDays: overallDays,
	}
}
