package dto

import (
	"go_backend/internal/model"

	"github.com/shopspring/decimal"
)

type LtvListResponse struct {
	Items       []*model.LtvDailyStat `json:"items"`
	Summary     *LtvSummaryDto        `json:"summary"`
	UserViewMap []VisibleAccountDto   `json:"userViewMap"`
}

type LtvSummaryDto struct {
	TotalSpend         decimal.Decimal  `json:"totalSpend"`
	TotalRecharge      decimal.Decimal  `json:"totalRecharge"`
	TotalRefund        decimal.Decimal  `json:"totalRefund"`
	TotalProfit        decimal.Decimal  `json:"totalProfit"`
	OverallRoi         decimal.Decimal  `json:"overallRoi"`
	SubUserCount       int              `json:"subUserCount"`
	SubUserCost        decimal.Decimal  `json:"subUserCost"`
	OverallPaybackDays *int             `json:"overallPaybackDays"`
	PredD30Roi         *decimal.Decimal `json:"predD30Roi"`
	PredD60Roi         *decimal.Decimal `json:"predD60Roi"`
	PredD90Roi         *decimal.Decimal `json:"predD90Roi"`
}

type VisibleAccountDto struct {
	UserID   int64  `json:"userId"`
	Username string `json:"username"`
	Role     string `json:"role"`
	IsMaster int    `json:"isMaster"`
}

type SaveLaunchConfigRequest struct {
	PlatformCode string          `json:"platformCode"`
	UserID       int64           `json:"userId"`
	LaunchDate   string          `json:"launchDate"`
	Spend        decimal.Decimal `json:"spend"`
	Remark       string          `json:"remark"`
}
