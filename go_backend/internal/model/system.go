package model

import (
	"time"
)

// SystemConfig 系统动态 KV 配置表
type SystemConfig struct {
	ConfigKey   string    `gorm:"primaryKey;column:config_key;size:64" json:"configKey"`
	ConfigValue string    `gorm:"column:config_value;size:2000" json:"configValue"`
	UpdatedAt   time.Time `gorm:"column:updated_at" json:"updatedAt"`
}

func (SystemConfig) TableName() string { return "system_config" }
