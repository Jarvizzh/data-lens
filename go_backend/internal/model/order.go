package model

import (
	"time"

	"github.com/shopspring/decimal"
)

// RawOrder 订单原始明细表
type RawOrder struct {
	ID              int64           `gorm:"primaryKey;autoIncrement;column:id" json:"id"`
	PlatformCode    string          `gorm:"column:platform_code;size:32;not null;default:'rocnovel'" json:"platformCode"`
	OrderID         string          `gorm:"column:order_id;size:64;not null" json:"orderId"`
	MemberID        string          `gorm:"column:member_id;size:64;not null" json:"memberId"`
	LandingPageID   string          `gorm:"column:landing_page_id;size:64" json:"landingPageId"`
	RegisterTimeBJ  time.Time       `gorm:"column:register_time_bj;not null" json:"registerTimeBj"`
	RegisterTimeET  time.Time       `gorm:"column:register_time_et;not null" json:"registerTimeEt"`
	RegisterDateET  string          `gorm:"column:register_date_et;type:date;not null" json:"registerDateEt"`
	RegisterTimeUTC *time.Time      `gorm:"column:register_time_utc" json:"registerTimeUtc"`
	RegisterDateUTC string          `gorm:"column:register_date_utc;type:date" json:"registerDateUtc"`
	PayTimeBJ       time.Time       `gorm:"column:pay_time_bj;not null" json:"payTimeBj"`
	PayTimeET       time.Time       `gorm:"column:pay_time_et;not null" json:"payTimeEt"`
	PayDateET       string          `gorm:"column:pay_date_et;type:date;not null" json:"payDateEt"`
	PayTimeUTC      *time.Time      `gorm:"column:pay_time_utc" json:"payTimeUtc"`
	PayDateUTC      string          `gorm:"column:pay_date_utc;type:date" json:"payDateUtc"`
	OrderAmountCent int             `gorm:"column:order_amount_cent;not null" json:"orderAmountCent"`
	OrderAmountUSD  decimal.Decimal `gorm:"column:order_amount_usd;type:decimal(10,2);not null" json:"orderAmountUsd"`
	IsSubs          int             `gorm:"column:is_subs;default:0" json:"isSubs"`
	RenewType       int             `gorm:"column:renew_type;default:1" json:"renewType"`
	PayState        int             `gorm:"column:pay_state;default:1" json:"payState"`
	RefundStatus    int             `gorm:"column:refund_status;default:0" json:"refundStatus"`
	RawPayload      string          `gorm:"column:raw_payload;type:text" json:"rawPayload"`
	CreatedAt       time.Time       `gorm:"column:created_at" json:"createdAt"`
}

func (RawOrder) TableName() string { return "raw_order" }
