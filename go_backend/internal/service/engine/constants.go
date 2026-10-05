package engine

// PredictAlgorithmConstants 预测算法超参数与熔断常量集中管理
const (
	ScaleDecayExponent = 0.35 // 均值回归放缩因子衰减指数
	CycleDecayExponent = 0.06 // 续扣脉冲自然衰减指数

	EarlyStageMinAlpha = 0.80
	EarlyStageMaxAlpha = 1.25

	MatureStageMinAlpha = 0.60
	MatureStageMaxAlpha = 2.00

	MinFlatDays           = 6
	PeriodFlatMultiplier  = 2

	Roi60ChainMultiplier = 1.25 // D30 到 D60 经验倍率
	Roi90ChainMultiplier = 1.18 // D60 到 D90 经验倍率
	RoiSampleSizeK       = 8.0  // ROI 预测用户量先验收缩常数

	ArpuShrinkageKUser = 5.0 // 用户量先验收缩常数 K_user

	OlsEnsembleMinDays   = 14   // 激活 OLS 动量融合的最小观察天数
	OlsEnsembleMinR2     = 0.85 // 激活 OLS 融合的拟合优度 R^2 门槛
	OlsEnsembleMinSlope  = 0.03 // 激活 OLS 融合的最小增长斜率 a
	OlsEnsembleMaxWeight = 0.45 // OLS 动量最大融合权重 lambda

	SmallCohortMaxUsers            = 5
	SmallCohortContinuityThreshold = 0.70 // 近7天有>=70%天数连续产生充值
	SmallCohortMinRoi              = 0.40 // 激活松绑的最小实际达成 ROI
	SmallCohortMatureMaxAlpha      = 3.50 // 小样本松绑后的最大放缩上限
	SmallCohortScaleDecayExponent  = 0.15 // 小样本松绑后的远期衰减指数

	DefaultDailySubPrice  = 9.99
	DefaultWeeklySubPrice = 19.99
)

func GetEmpiricalMultiplierTo30(day int) float64 {
	if day >= 30 {
		return 1.0
	}
	if day <= 3 {
		return 2.65
	}
	if day <= 7 {
		return 1.80
	}
	if day <= 14 {
		return 1.35
	}
	if day <= 21 {
		return 1.15
	}
	return 1.0 + (30.0-float64(day))/30.0*0.15
}
