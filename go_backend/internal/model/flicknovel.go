package model

import (
	"time"
)

// FlicknovelPromotion 番茄司南推广链接信息表
type FlicknovelPromotion struct {
	ID              int64     `gorm:"primaryKey;autoIncrement;column:id" json:"id"`
	PromotionID     string    `gorm:"column:promotion_id;size:64;not null;unique" json:"promotionId"`
	PromotionName   string    `gorm:"column:promotion_name;size:255" json:"promotionName"`
	RechargeTplID   string    `gorm:"column:recharge_tpl_id;size:64" json:"rechargeTplId"`
	RechargeTplName string    `gorm:"column:recharge_tpl_name;size:255" json:"rechargeTplName"`
	DistAppID       int64     `gorm:"column:dist_app_id" json:"distAppId"`
	DramaID         string    `gorm:"column:drama_id;size:64" json:"dramaId"`
	DramaTitle      string    `gorm:"column:drama_title;size:255" json:"dramaTitle"`
	ChapterID       string    `gorm:"column:chapter_id;size:64" json:"chapterId"`
	ChapterTitle    string    `gorm:"column:chapter_title;size:255" json:"chapterTitle"`
	MediaChannel    int       `gorm:"column:media_channel" json:"mediaChannel"`
	RawPayload      string    `gorm:"column:raw_payload;type:text" json:"rawPayload"`
	CreatedAt       time.Time `gorm:"column:created_at" json:"createdAt"`
	UpdatedAt       time.Time `gorm:"column:updated_at" json:"updatedAt"`
}

func (FlicknovelPromotion) TableName() string { return "flicknovel_promotion" }

// FlicknovelRechargeTemplate 番茄司南充值模板表
type FlicknovelRechargeTemplate struct {
	ID              int64     `gorm:"primaryKey;autoIncrement;column:id" json:"id"`
	TemplateID      string    `gorm:"column:template_id;size:64;not null;unique" json:"templateId"`
	Name            string    `gorm:"column:name;size:255" json:"name"`
	DistAppID       int64     `gorm:"column:dist_app_id" json:"distAppId"`
	PriceConfigJSON string    `gorm:"column:price_config_json;type:text" json:"priceConfigJson"`
	RawPayload      string    `gorm:"column:raw_payload;type:mediumtext" json:"rawPayload"`
	CreatedAt       time.Time `gorm:"column:created_at" json:"createdAt"`
	UpdatedAt       time.Time `gorm:"column:updated_at" json:"updatedAt"`
}

func (FlicknovelRechargeTemplate) TableName() string { return "flicknovel_recharge_template" }

// FlicknovelRelation 番茄司南染色归因明细表
type FlicknovelRelation struct {
	ID                     int64      `gorm:"primaryKey;autoIncrement;column:id" json:"id"`
	RelationID             string     `gorm:"column:relation_id;size:64;not null;unique" json:"relationId"`
	DeviceID               string     `gorm:"column:device_id;size:128;not null" json:"deviceId"`
	PromotionID            string     `gorm:"column:promotion_id;size:64" json:"promotionId"`
	PromotionCode          string     `gorm:"column:promotion_code;size:64" json:"promotionCode"`
	AdID                   string     `gorm:"column:ad_id;size:64" json:"adId"`
	AdsetID                string     `gorm:"column:adset_id;size:64" json:"adsetId"`
	CampaignID             string     `gorm:"column:campaign_id;size:64" json:"campaignId"`
	AdAccountID            string     `gorm:"column:ad_account_id;size:64" json:"adAccountId"`
	RelationBeginTimeBJ    *time.Time `gorm:"column:relation_begin_time_bj" json:"relationBeginTimeBj"`
	RelationBeginTimeET    *time.Time `gorm:"column:relation_begin_time_et" json:"relationBeginTimeEt"`
	RelationBeginDateET    string     `gorm:"column:relation_begin_date_et;type:date" json:"relationBeginDateEt"`
	RelationBeginTimestamp int64      `gorm:"column:relation_begin_timestamp" json:"relationBeginTimestamp"`
	MediaChannel           string     `gorm:"column:media_channel;size:64" json:"mediaChannel"`
	Platform               string     `gorm:"column:platform;size:32" json:"platform"`
	AppID                  string     `gorm:"column:app_id;size:32" json:"appId"`
	RawPayload             string     `gorm:"column:raw_payload;type:text" json:"rawPayload"`
	CreatedAt              time.Time  `gorm:"column:created_at" json:"createdAt"`
	UpdatedAt              time.Time  `gorm:"column:updated_at" json:"updatedAt"`
}

func (FlicknovelRelation) TableName() string { return "flicknovel_relation" }
