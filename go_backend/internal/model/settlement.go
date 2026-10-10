package model

import (
	"time"

	"github.com/shopspring/decimal"
)

// MonthlySettlementConfig 月份结算参数配置表
type MonthlySettlementConfig struct {
	ID                       int64           `gorm:"primaryKey;autoIncrement;column:id" json:"id"`
	PlatformCode             string          `gorm:"column:platform_code;size:32;not null;default:'rocnovel'" json:"platformCode"`
	SettlementType           string          `gorm:"column:settlement_type;size:32;not null" json:"settlementType"`
	TargetUserID             *int64          `gorm:"column:target_user_id" json:"targetUserId"`
	MonthStr                 string          `gorm:"column:month_str;size:16;not null" json:"monthStr"`
	SettledRefundAmount      decimal.Decimal `gorm:"column:settled_refund_amount;type:decimal(10,2);not null;default:0.00" json:"settledRefundAmount"`
	MonthSettledRefundAmount decimal.Decimal `gorm:"column:month_settled_refund_amount;type:decimal(10,2);not null;default:0.00" json:"monthSettledRefundAmount"`
	CrossPeriodRefundAmount  decimal.Decimal `gorm:"column:cross_period_refund_amount;type:decimal(10,2);not null;default:0.00" json:"crossPeriodRefundAmount"`
	ShareRatio               decimal.Decimal `gorm:"column:share_ratio;type:decimal(6,4);not null;default:0.9500" json:"shareRatio"`
	ChannelFeeRate           decimal.Decimal `gorm:"column:channel_fee_rate;type:decimal(6,4);not null;default:0.0700" json:"channelFeeRate"`
	Remark                   string          `gorm:"column:remark;size:500;default:''" json:"remark"`
	UpdatedAt                time.Time       `gorm:"column:updated_at" json:"updatedAt"`
}

func (MonthlySettlementConfig) TableName() string { return "monthly_settlement_config" }
