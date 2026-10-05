package model

import (
	"time"

	"github.com/shopspring/decimal"
)

// LtvLaunchConfig 投放消耗与备注配置表
type LtvLaunchConfig struct {
	PlatformCode string          `gorm:"primaryKey;column:platform_code;size:32;not null;default:'rocnovel'" json:"platformCode"`
	UserID       int64           `gorm:"primaryKey;column:user_id;not null" json:"userId"`
	LaunchDate   string          `gorm:"primaryKey;column:launch_date;type:date;not null" json:"launchDate"`
	Spend        decimal.Decimal `gorm:"column:spend;type:decimal(10,2);not null;default:0.00" json:"spend"`
	Remark       string          `gorm:"column:remark;size:500;default:''" json:"remark"`
	UpdatedAt    time.Time       `gorm:"column:updated_at" json:"updatedAt"`
}

func (LtvLaunchConfig) TableName() string { return "ltv_launch_config" }

// DailyRechargeDistribution 每日充值分布统计汇总表 (自然日)
type DailyRechargeDistribution struct {
	PlatformCode       string          `gorm:"primaryKey;column:platform_code;size:32;not null;default:'rocnovel'" json:"platformCode"`
	UserID             int64           `gorm:"primaryKey;column:user_id;not null" json:"userId"`
	Date               string          `gorm:"primaryKey;column:date;type:date;not null" json:"date"`
	TotalRecharge      decimal.Decimal `gorm:"column:total_recharge;type:decimal(10,2);not null;default:0.00" json:"totalRecharge"`
	SingleRecharge     decimal.Decimal `gorm:"column:single_recharge;type:decimal(10,2);not null;default:0.00" json:"singleRecharge"`
	SubsRecharge       decimal.Decimal `gorm:"column:subs_recharge;type:decimal(10,2);not null;default:0.00" json:"subsRecharge"`
	TotalPaidUsers     int             `gorm:"column:total_paid_users;not null;default:0" json:"totalPaidUsers"`
	SinglePaidUsers    int             `gorm:"column:single_paid_users;not null;default:0" json:"singlePaidUsers"`
	SubsPaidUsers      int             `gorm:"column:subs_paid_users;not null;default:0" json:"subsPaidUsers"`
	NewRecharge        decimal.Decimal `gorm:"column:new_recharge;type:decimal(10,2);not null;default:0.00" json:"newRecharge"`
	NewRechargeRatio   decimal.Decimal `gorm:"column:new_recharge_ratio;type:decimal(10,4);not null;default:0.0000" json:"newRechargeRatio"`
	NewArpu            decimal.Decimal `gorm:"column:new_arpu;type:decimal(10,2);not null;default:0.00" json:"newArpu"`
	NewPaidUsers       int             `gorm:"column:new_paid_users;not null;default:0" json:"newPaidUsers"`
	NewSinglePaidUsers int             `gorm:"column:new_single_paid_users;not null;default:0" json:"newSinglePaidUsers"`
	NewSubsPaidUsers   int             `gorm:"column:new_subs_paid_users;not null;default:0" json:"newSubsPaidUsers"`
	OldRecharge        decimal.Decimal `gorm:"column:old_recharge;type:decimal(10,2);not null;default:0.00" json:"oldRecharge"`
	OldRechargeRatio   decimal.Decimal `gorm:"column:old_recharge_ratio;type:decimal(10,4);not null;default:0.0000" json:"oldRechargeRatio"`
	OldArpu            decimal.Decimal `gorm:"column:old_arpu;type:decimal(10,2);not null;default:0.00" json:"oldArpu"`
	OldPaidUsers       int             `gorm:"column:old_paid_users;not null;default:0" json:"oldPaidUsers"`
	OldSinglePaidUsers int             `gorm:"column:old_single_paid_users;not null;default:0" json:"oldSinglePaidUsers"`
	OldSubsPaidUsers   int             `gorm:"column:old_subs_paid_users;not null;default:0" json:"oldSubsPaidUsers"`
	RepeatPaidUsers    int             `gorm:"column:repeat_paid_users;not null;default:0" json:"repeatPaidUsers"`
	RepeatRate         decimal.Decimal `gorm:"column:repeat_rate;type:decimal(10,4);not null;default:0.0000" json:"repeatRate"`
	UpdatedAt          time.Time       `gorm:"column:updated_at" json:"updatedAt"`
}

func (DailyRechargeDistribution) TableName() string { return "daily_recharge_distribution" }

// LtvPredictBenchmark LTV 预测基准数据表
type LtvPredictBenchmark struct {
	ID                int64           `gorm:"primaryKey;autoIncrement;column:id" json:"id"`
	DimensionType     string          `gorm:"column:dimension_type;size:32;not null;default:'ALL'" json:"dimensionType"`
	DimensionValue    string          `gorm:"column:dimension_value;size:64;not null;default:'DEFAULT'" json:"dimensionValue"`
	SubPeriodDays     int             `gorm:"column:sub_period_days;not null;default:1" json:"subPeriodDays"`
	DayIndex          int             `gorm:"column:day_index;not null" json:"dayIndex"`
	BaseRetentionRate decimal.Decimal `gorm:"column:base_retention_rate;type:decimal(10,6);not null;default:0.000000" json:"baseRetentionRate"`
	BaseArpu          decimal.Decimal `gorm:"column:base_arpu;type:decimal(10,2);not null;default:0.00" json:"baseArpu"`
	SampleCohortCount int             `gorm:"column:sample_cohort_count;not null;default:0" json:"sampleCohortCount"`
	IsExtrapolated    int             `gorm:"column:is_extrapolated;not null;default:0" json:"isExtrapolated"`
	UpdatedAt         time.Time       `gorm:"column:updated_at" json:"updatedAt"`
}

func (LtvPredictBenchmark) TableName() string { return "ltv_predict_benchmark" }
