package model

import (
	"strings"
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

// Platform 常量与枚举定义 (强类型，严禁在业务逻辑与数据接入层使用魔术字符串)
const (
	PlatformAll        = "ALL"
	PlatformRocnovel   = "rocnovel"
	PlatformFlicknovel = "flicknovel"

	CstDefaultTimezone = "CST"
	UtcDefaultTimezone = "UTC"

	LaunchStartDateRocnovel   = "2026-07-10"
	LaunchStartDateFlicknovel = "2026-09-16"
)

// PlatformInfo 平台元数据
type PlatformInfo struct {
	Code            string
	DisplayName     string
	Enabled         bool
	LaunchStartDate string
	DefaultTimezone string
}

var SupportedPlatforms = map[string]PlatformInfo{
	"all": {
		Code:            PlatformAll,
		DisplayName:     "大盘汇总",
		Enabled:         true,
		LaunchStartDate: LaunchStartDateRocnovel,
		DefaultTimezone: CstDefaultTimezone,
	},
	"rocnovel": {
		Code:            PlatformRocnovel,
		DisplayName:     "中文在线",
		Enabled:         true,
		LaunchStartDate: LaunchStartDateRocnovel,
		DefaultTimezone: CstDefaultTimezone,
	},
	"flicknovel": {
		Code:            PlatformFlicknovel,
		DisplayName:     "番茄司南",
		Enabled:         true,
		LaunchStartDate: LaunchStartDateFlicknovel,
		DefaultTimezone: UtcDefaultTimezone,
	},
}

// GetLaunchStartDateForPlatform 根据平台代码安全获取投放起始日期，若未指定或无法匹配默认返回 ROCNOVEL (2026-07-10)
func GetLaunchStartDateForPlatform(code string) string {
	c := strings.ToLower(strings.TrimSpace(code))
	if p, ok := SupportedPlatforms[c]; ok {
		return p.LaunchStartDate
	}
	return LaunchStartDateRocnovel
}

// GetDefaultTimezoneForPlatform 根据平台代码安全获取默认时区
func GetDefaultTimezoneForPlatform(code string) string {
	c := strings.ToLower(strings.TrimSpace(code))
	if p, ok := SupportedPlatforms[c]; ok {
		return p.DefaultTimezone
	}
	return CstDefaultTimezone
}

// PlatformItemDto 前端下拉选项与元数据传输对象
type PlatformItemDto struct {
	Code            string `json:"code"`
	Name            string `json:"name"`
	Enabled         bool   `json:"enabled"`
	LaunchStartDate string `json:"launchStartDate"`
}

