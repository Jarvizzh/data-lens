package dto

import (
	"time"

	"github.com/shopspring/decimal"
)

type MonthlySettlementItemDto struct {
	MonthStr                 string          `json:"monthStr"`
	SettlementType           string          `json:"settlementType"`
	TargetUserID             *int64          `json:"targetUserId"`
	TargetUsername           string          `json:"targetUsername"`
	TotalRecharge            decimal.Decimal `json:"totalRecharge"`
	TotalRefund              decimal.Decimal `json:"totalRefund"`
	SettledRefundAmount      decimal.Decimal `json:"settledRefundAmount"`
	MonthSettledRefundAmount decimal.Decimal `json:"monthSettledRefundAmount"`
	UnsettledRefundAmount    decimal.Decimal `json:"unsettledRefundAmount"`
	CrossPeriodRefundAmount  decimal.Decimal `json:"crossPeriodRefundAmount"`
	ShareRatio               decimal.Decimal `json:"shareRatio"`
	ChannelFeeRate           decimal.Decimal `json:"channelFeeRate"`
	EffectiveBaseAmount      decimal.Decimal `json:"effectiveBaseAmount"`
	FinalSettlementAmount    decimal.Decimal `json:"finalSettlementAmount"`
	RefundRate               string          `json:"refundRate"`
	TotalOrders              int             `json:"totalOrders"`
	RefundOrders             int             `json:"refundOrders"`
	Remark                   string          `json:"remark"`
	UpdatedAt                *time.Time      `json:"updatedAt"`
}

type MonthlySettlementSaveRequestDto struct {
	SettlementType           string          `json:"settlementType"`
	TargetUserID             *int64          `json:"targetUserId"`
	MonthStr                 string          `json:"monthStr"`
	SettledRefundAmount      decimal.Decimal `json:"settledRefundAmount"`
	MonthSettledRefundAmount decimal.Decimal `json:"monthSettledRefundAmount"`
	CrossPeriodRefundAmount  decimal.Decimal `json:"crossPeriodRefundAmount"`
	ShareRatio               decimal.Decimal `json:"shareRatio"`
	ChannelFeeRate           decimal.Decimal `json:"channelFeeRate"`
	Remark                   string          `json:"remark"`
}
