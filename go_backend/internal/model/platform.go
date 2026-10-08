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

	PlatformNameAll        = "大盘汇总"
	PlatformNameRocnovel   = "中文在线"
	PlatformNameFlicknovel = "番茄司南"

	LaunchStartDateRocnovel   = "2026-07-10"
	LaunchStartDateFlicknovel = "2026-09-16"
)

// GetDisplayNameForPlatform 根据平台代码安全获取展示名称
func GetDisplayNameForPlatform(code string) string {
	c := strings.ToLower(strings.TrimSpace(code))
	if p, ok := SupportedPlatforms[c]; ok {
		return p.DisplayName
	}
	if c != "" {
		return c
	}
	return PlatformNameAll
}

// NormalizePlatform 归一化平台代码：去空格转小写；若为空或 all 则返回 PlatformAll ("ALL")
func NormalizePlatform(code string) string {
	c := strings.ToLower(strings.TrimSpace(code))
	if c == "" || c == "all" {
		return PlatformAll
	}
	return c
}

// IsRocnovel 判断平台是否为中文在线 (rocnovel)
func IsRocnovel(code string) bool {
	return strings.EqualFold(strings.TrimSpace(code), PlatformRocnovel)
}

// IsFlicknovel 判断平台是否为番茄司南 (flicknovel)
func IsFlicknovel(code string) bool {
	return strings.EqualFold(strings.TrimSpace(code), PlatformFlicknovel)
}

// IsAllPlatforms 判断是否为全平台模式 (空或 all/ALL)
func IsAllPlatforms(code string) bool {
	c := strings.ToLower(strings.TrimSpace(code))
	return c == "" || c == "all"
}


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
		DisplayName:     PlatformNameAll,
		Enabled:         true,
		LaunchStartDate: LaunchStartDateRocnovel,
		DefaultTimezone: CstDefaultTimezone,
	},
	"rocnovel": {
		Code:            PlatformRocnovel,
		DisplayName:     PlatformNameRocnovel,
		Enabled:         true,
		LaunchStartDate: LaunchStartDateRocnovel,
		DefaultTimezone: CstDefaultTimezone,
	},
	"flicknovel": {
		Code:            PlatformFlicknovel,
		DisplayName:     PlatformNameFlicknovel,
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

