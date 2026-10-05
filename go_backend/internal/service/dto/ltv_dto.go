package dto

import (
	"go_backend/internal/model"

	"github.com/shopspring/decimal"
)

// SingleMonthSummaryDto 单月汇总
type SingleMonthSummaryDto struct {
	Month             string           `json:"month"`
	Spend             decimal.Decimal  `json:"spend"`
	Recharge          decimal.Decimal  `json:"recharge"`
	Refund            decimal.Decimal  `json:"refund"`
	Profit            decimal.Decimal  `json:"profit"`
	Roi               decimal.Decimal  `json:"roi"`
	SubUsers          int              `json:"subUsers"`
	RetainedSubUsers  int              `json:"retainedSubUsers"`
	RetainedRate      string           `json:"retainedRate"`
	ActualPaybackDays *int             `json:"actualPaybackDays"`
	PredictedDay30Roi *decimal.Decimal `json:"predictedDay30Roi"`
	PredictedDay60Roi *decimal.Decimal `json:"predictedDay60Roi"`
	PredictedDay90Roi *decimal.Decimal `json:"predictedDay90Roi"`
}

// MonthlySummaryDto 包含近 4 个月度汇总
type MonthlySummaryDto struct {
	Months    []SingleMonthSummaryDto `json:"months"`
	ThisMonth *SingleMonthSummaryDto  `json:"thisMonth"`
	LastMonth *SingleMonthSummaryDto  `json:"lastMonth"`
}

// LtvListResponseDto 兼容 Java /api/ltv/list 强类型响应
type LtvListResponseDto struct {
	Code                          int                   `json:"code"`
	Msg                           string                `json:"msg"`
	Data                          []*model.LtvDailyStat `json:"data"`
	OverallPredictedPaybackDays   *int                  `json:"overallPredictedPaybackDays"`
	OverallPaybackCycleDays       *int                  `json:"overallPaybackCycleDays"`
	OverallPredictedDay30Roi      *decimal.Decimal      `json:"overallPredictedDay30Roi"`
	OverallPredictedDay60Roi      *decimal.Decimal      `json:"overallPredictedDay60Roi"`
	OverallPredictedDay90Roi      *decimal.Decimal      `json:"overallPredictedDay90Roi"`
	OverallPredictedDay30Recharge *decimal.Decimal      `json:"overallPredictedDay30Recharge"`
	OverallPredictedDay60Recharge *decimal.Decimal      `json:"overallPredictedDay60Recharge"`
	OverallPredictedDay90Recharge *decimal.Decimal      `json:"overallPredictedDay90Recharge"`
	MonthlySummary                *MonthlySummaryDto    `json:"monthlySummary"`
	OverallRetainedSubUsers       int                   `json:"overallRetainedSubUsers"`
	OverallRetainedRate           string                `json:"overallRetainedRate"`
	Total                         int                   `json:"total"`
	UserID                        int64                 `json:"userId"`
}

// DailyDistributionSummaryDto 每日充值分布汇总
type DailyDistributionSummaryDto struct {
	TotalRecharge     decimal.Decimal `json:"totalRecharge"`
	NewRecharge       decimal.Decimal `json:"newRecharge"`
	OldRecharge       decimal.Decimal `json:"oldRecharge"`
	ThisMonthRecharge decimal.Decimal `json:"thisMonthRecharge"`
	ThisMonthRefund   decimal.Decimal `json:"thisMonthRefund"`
	LastMonthRecharge decimal.Decimal `json:"lastMonthRecharge"`
	LastMonthRefund   decimal.Decimal `json:"lastMonthRefund"`
	ThisMonthStr      string          `json:"thisMonthStr"`
	LastMonthStr      string          `json:"lastMonthStr"`
	NewRechargeRatio  decimal.Decimal `json:"newRechargeRatio"`
	OldRechargeRatio  decimal.Decimal `json:"oldRechargeRatio"`
	TotalPaidUsers    int             `json:"totalPaidUsers"`
	NewPaidUsers      int             `json:"newPaidUsers"`
	OldPaidUsers      int             `json:"oldPaidUsers"`
	NewArpu           decimal.Decimal `json:"newArpu"`
	OldArpu           decimal.Decimal `json:"oldArpu"`
	RepeatPaidUsers   int             `json:"repeatPaidUsers"`
	RepeatRate        decimal.Decimal `json:"repeatRate"`
}

// DailyDistributionResponseDto 每日充值分布响应
type DailyDistributionResponseDto struct {
	Code    int                                `json:"code"`
	Msg     string                             `json:"msg"`
	Data    []*model.DailyRechargeDistribution `json:"data"`
	Summary *DailyDistributionSummaryDto       `json:"summary"`
	Total   int                                `json:"total"`
	UserID  int64                              `json:"userId"`
}

// VisibleAccountDto 用户视图切换列表项
type VisibleAccountDto struct {
	ID              int64  `json:"id"`
	Username        string `json:"username"`
	Role            string `json:"role"`
	IsSelf          bool   `json:"isSelf"`
	IsMaster        int    `json:"isMaster"`
	IsSettlement    int    `json:"isSettlement"`
	SubAccountCount int    `json:"subAccountCount"`
}

// LandingPageConfigItem 落地页配置项
type LandingPageConfigItem struct {
	PlatformCode  string `json:"platformCode"`
	LandingPageID string `json:"landingPageId"`
	Timezone      string `json:"timezone"`
}

// UserLandingPageConfigResponseDto 落地页查询响应
type UserLandingPageConfigResponseDto struct {
	Code           int                     `json:"code"`
	Data           []LandingPageConfigItem `json:"data"`
	LandingPageIDs []string                `json:"landingPageIds"`
}

// SaveLaunchConfigRequest 投放消耗保存入参
type SaveLaunchConfigRequest struct {
	PlatformCode string          `json:"platformCode"`
	UserID       int64           `json:"userId"`
	TargetUserID *int64          `json:"targetUserId"`
	LaunchDate   string          `json:"launchDate"`
	Spend        decimal.Decimal `json:"spend"`
	Remark       string          `json:"remark"`
}

// BatchSpendItem 批量消耗明细
type BatchSpendItem struct {
	LaunchDate string          `json:"launchDate"`
	Spend      decimal.Decimal `json:"spend"`
	Remark     string          `json:"remark"`
}

// BatchSpendRequest 批量导入消耗入参
type BatchSpendRequest struct {
	PlatformCode string           `json:"platformCode"`
	TargetUserID *int64           `json:"targetUserId"`
	Items        []BatchSpendItem `json:"items"`
}
