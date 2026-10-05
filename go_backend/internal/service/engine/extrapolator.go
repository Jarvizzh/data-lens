package engine

import (
	"math"

	"go_backend/internal/model"

	"github.com/shopspring/decimal"
)

// OlsFitResult OLS 对数回归拟合结果结构体
type OlsFitResult struct {
	A     float64 `json:"a"`
	B     float64 `json:"b"`
	R2    float64 `json:"r2"`
	Valid bool    `json:"valid"`
}

// ComputeOptimalScaleFactor 算法 1~3 综合放缩因子计算：
// 1. 极早期 (maxDays <= 7) 贝叶斯先验收缩 (Empirical Bayes Shrinkage)
// 2. 分段阶跃 Sigmoid 贝叶斯权重函数 w(t)
// 3. 离群点 Cohort 动态平滑与熔断 (Outlier Clamping)
func ComputeOptimalScaleFactor(actualRoi, baseRoiSum float64, maxDays int, spend decimal.Decimal, isSmallCohortActive bool) float64 {
	if baseRoiSum < 0.0001 {
		baseRoiSum = 0.0001
	}
	rawAlpha := actualRoi / baseRoiSum

	var alpha float64
	var w float64

	spendVal, _ := spend.Float64()
	if spendVal <= 0 {
		spendVal = 1000.0
	}

	maxAlpha := MatureStageMaxAlpha
	if isSmallCohortActive {
		maxAlpha = SmallCohortMatureMaxAlpha
	}

	if maxDays <= 7 {
		// 算法 1：样本消耗加权贝叶斯先验收缩
		spendFloor := spendVal
		if spendFloor < 100.0 {
			spendFloor = 100.0
		}
		priorWeight := math.Min(0.20, math.Max(0.05, 1000.0/spendFloor*0.05))
		alpha = (actualRoi + priorWeight) / (baseRoiSum + priorWeight)

		// 算法 3：极早期离群点截断
		if alpha < EarlyStageMinAlpha {
			alpha = EarlyStageMinAlpha
		} else if alpha > EarlyStageMaxAlpha {
			alpha = EarlyStageMaxAlpha
		}

		// 算法 2：未发生周划扣期平滑权重 (w: 0.15 ~ 0.25)
		w = 0.15 + 0.10*(float64(maxDays)/7.0)
	} else if maxDays <= 14 {
		// 算法 2：首个周划扣周期内 (7~14天) 平滑渐进权重 (w: 0.25 ~ 0.85)
		if rawAlpha < MatureStageMinAlpha {
			alpha = MatureStageMinAlpha
		} else if rawAlpha > maxAlpha {
			alpha = maxAlpha
		} else {
			alpha = rawAlpha
		}

		w = 0.25 + 0.60*(float64(maxDays-7)/7.0)
	} else {
		// 算法 3：跨过双周划扣节点后的离群点熔断
		if rawAlpha < MatureStageMinAlpha {
			alpha = MatureStageMinAlpha
		} else if rawAlpha > maxAlpha {
			alpha = maxAlpha
		} else {
			alpha = rawAlpha
		}

		// 算法 2：成熟期高信任真实划扣数据 (w: 0.85 ~ 0.95)
		matureSpan := float64(maxDays - 14)
		if matureSpan > 46.0 {
			matureSpan = 46.0
		}
		w = 0.85 + 0.10*(matureSpan/46.0)
	}

	return w*alpha + (1.0-w)*1.0
}

// ComputeOlsFit 计算历史实际数据的对数拟合 ROI(t) = a * ln(t) + b 以及判定系数 R^2
func ComputeOlsFit(stat *model.LtvDailyStat, maxDays int) OlsFitResult {
	limit := maxDays
	if limit > 60 {
		limit = 60
	}

	var xList []float64
	var yList []float64

	for d := 1; d <= limit; d++ {
		roi := GetRoiForDay(stat, d)
		if roi.GreaterThan(decimal.Zero) {
			rVal, _ := roi.Float64()
			xList = append(xList, math.Log(float64(d)))
			yList = append(yList, rVal)
		}
	}

	n := len(xList)
	if n < 5 {
		return OlsFitResult{A: 0, B: 0, R2: 0, Valid: false}
	}

	var sumX, sumY, sumXY, sumXX float64
	for i := 0; i < n; i++ {
		x := xList[i]
		y := yList[i]
		sumX += x
		sumY += y
		sumXY += x * y
		sumXX += x * x
	}

	denominator := float64(n)*sumXX - sumX*sumX
	var a, b float64
	if denominator != 0 {
		a = (float64(n)*sumXY - sumX*sumY) / denominator
	}
	b = (sumY - a*sumX) / float64(n)

	meanY := sumY / float64(n)
	var ssTot, ssRes float64
	for i := 0; i < n; i++ {
		y := yList[i]
		yPred := a*xList[i] + b
		ssTot += (y - meanY) * (y - meanY)
		ssRes += (y - yPred) * (y - yPred)
	}

	var r2 float64
	if ssTot > 0.0001 {
		r2 = math.Max(0.0, 1.0-ssRes/ssTot)
	}
	valid := (a >= OlsEnsembleMinSlope && r2 >= OlsEnsembleMinR2)

	return OlsFitResult{A: a, B: b, R2: r2, Valid: valid}
}

// ComputeOlsEnsembleWeight 计算成熟期 (D14+) 双轨 OLS 动量动态系综融合权重 lambda
func ComputeOlsEnsembleWeight(r2 float64, maxDays int) float64 {
	if maxDays < OlsEnsembleMinDays || r2 < OlsEnsembleMinR2 {
		return 0.0
	}
	r2Bonus := math.Min(1.0, (r2-OlsEnsembleMinR2)/(1.0-OlsEnsembleMinR2))
	timeBonus := math.Min(1.0, float64(maxDays-OlsEnsembleMinDays)/20.0)
	weight := OlsEnsembleMaxWeight * (0.50 + 0.50*r2Bonus) * (0.30 + 0.70*timeBonus)
	return math.Max(0.0, math.Min(OlsEnsembleMaxWeight, weight))
}
