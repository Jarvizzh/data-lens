package model

import (
	"time"
)

// SubscriptionConfigVersion 订阅配置与落地页版本快照表
type SubscriptionConfigVersion struct {
	ID                  int64      `gorm:"primaryKey;autoIncrement;column:id" json:"id"`
	PlatformCode        string     `gorm:"column:platform_code;size:32;not null;default:'rocnovel'" json:"platformCode"`
	LandingPageID       string     `gorm:"column:landing_page_id;size:64;not null" json:"landingPageId"`
	SubscribeConfigID   string     `gorm:"column:subscribe_config_id;size:64;not null" json:"subscribeConfigId"`
	SubscribeConfigName string     `gorm:"column:subscribe_config_name;size:128;default:''" json:"subscribeConfigName"`
	SaleComboID         string     `gorm:"column:sale_combo_id;size:64" json:"saleComboId"`
	SaleComboName       string     `gorm:"column:sale_combo_name;size:128;default:''" json:"saleComboName"`
	ProductID           string     `gorm:"column:product_id;size:64;not null" json:"productId"`
	ProductName         string     `gorm:"column:product_name;size:128;default:''" json:"productName"`
	SubPeriodDays       int        `gorm:"column:sub_period_days;not null;default:1" json:"subPeriodDays"`
	FirstPriceCent      int        `gorm:"column:first_price_cent;not null" json:"firstPriceCent"`
	RenewPriceCent      int        `gorm:"column:renew_price_cent;not null" json:"renewPriceCent"`
	VersionNum          int        `gorm:"column:version_num;not null;default:1" json:"versionNum"`
	EffectiveStartTime  time.Time  `gorm:"column:effective_start_time;not null" json:"effectiveStartTime"`
	EffectiveEndTime    *time.Time `gorm:"column:effective_end_time" json:"effectiveEndTime"`
	UpdatedAt           time.Time  `gorm:"column:updated_at" json:"updatedAt"`
}

func (SubscriptionConfigVersion) TableName() string { return "subscription_config_version" }

// UserSubscriptionPeriod 用户-订阅周期与配置关联表
type UserSubscriptionPeriod struct {
	ID                int64      `gorm:"primaryKey;autoIncrement;column:id" json:"id"`
	PlatformCode      string     `gorm:"column:platform_code;size:32;not null;default:'rocnovel'" json:"platformCode"`
	MemberID          string     `gorm:"column:member_id;size:64;not null" json:"memberId"`
	LandingPageID     string     `gorm:"column:landing_page_id;size:64" json:"landingPageId"`
	SubscribeConfigID string     `gorm:"column:subscribe_config_id;size:64" json:"subscribeConfigId"`
	SubPeriodDays     int        `gorm:"column:sub_period_days;not null;default:1" json:"subPeriodDays"`
	FirstPriceCent    *int       `gorm:"column:first_price_cent" json:"firstPriceCent"`
	RenewPriceCent    *int       `gorm:"column:renew_price_cent" json:"renewPriceCent"`
	CreateTime        time.Time  `gorm:"column:create_time;default:CURRENT_TIMESTAMP" json:"createTime"`
	UpdatedAt         time.Time  `gorm:"column:updated_at" json:"updatedAt"`
}

func (UserSubscriptionPeriod) TableName() string { return "user_subscription_period" }
