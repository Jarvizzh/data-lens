package model

import (
	"time"
)

// PlatformConfig 多平台接入配置表
type PlatformConfig struct {
	ID              int64     `gorm:"primaryKey;autoIncrement;column:id" json:"id"`
	PlatformCode    string    `gorm:"column:platform_code;size:32;not null;unique" json:"platformCode"`
	PlatformName    string    `gorm:"column:platform_name;size:64;not null" json:"platformName"`
	AuthType        string    `gorm:"column:auth_type;size:32;not null;default:'TOKEN_COOKIE'" json:"authType"`
	AuthCredentials string    `gorm:"column:auth_credentials;type:text" json:"authCredentials"`
	SyncCron        string    `gorm:"column:sync_cron;size:32" json:"syncCron"`
	LaunchStartDate string    `gorm:"column:launch_start_date;type:date" json:"launchStartDate"`
	Status          int       `gorm:"column:status;not null;default:1" json:"status"`
	CreatedAt       time.Time `gorm:"column:created_at" json:"createdAt"`
	UpdatedAt       time.Time `gorm:"column:updated_at" json:"updatedAt"`
}

func (PlatformConfig) TableName() string { return "platform_config" }

// PlatformItemDto 前端下拉选项与元数据传输对象
type PlatformItemDto struct {
	Code            string `json:"code"`
	Name            string `json:"name"`
	Enabled         bool   `json:"enabled"`
	LaunchStartDate string `json:"launchStartDate"`
}
