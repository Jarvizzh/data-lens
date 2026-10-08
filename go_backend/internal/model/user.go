package model

import (
	"strings"
	"time"
)

// 用户角色与状态常量 (严禁在鉴权与业务层使用魔术字符串/数字)
const (
	RoleSuperAdmin = "SUPER_ADMIN"
	RoleAdmin      = "ADMIN"
	RoleUser       = "USER"

	UserStatusActive   = 1
	UserStatusDisabled = 0

	UserIsMasterYes = 1
	UserIsMasterNo  = 0
)


// SysUser 系统用户表
type SysUser struct {
	ID                     int64     `gorm:"primaryKey;autoIncrement;column:id" json:"id"`
	Username               string    `gorm:"column:username;size:50;not null;unique" json:"username"`
	PasswordHash           string    `gorm:"column:password_hash;size:100;not null" json:"-"`
	Role                   string    `gorm:"column:role;size:20;not null" json:"role"` // SUPER_ADMIN, ADMIN, USER
	Status                 int       `gorm:"column:status;not null;default:1" json:"status"`
	IsMaster               int       `gorm:"column:is_master;not null;default:0" json:"isMaster"`
	IsSettlement           int       `gorm:"column:is_settlement;not null;default:0" json:"isSettlement"`
	PermPredictPayback     int       `gorm:"column:perm_predict_payback;not null;default:0" json:"permPredictPayback"`
	PermRoiPredict         int       `gorm:"column:perm_roi_predict;not null;default:0" json:"permRoiPredict"`
	PermGlobalDistribution int       `gorm:"column:perm_global_distribution;not null;default:0" json:"permGlobalDistribution"`
	PermExport             int       `gorm:"column:perm_export;not null;default:0" json:"permExport"`
	PermSettlement         int       `gorm:"column:perm_settlement;not null;default:0" json:"permSettlement"`
	PermVideoGen           int       `gorm:"column:perm_video_gen;not null;default:0" json:"permVideoGen"`
	AllowedPlatforms       string    `gorm:"column:allowed_platforms;size:255;not null;default:'ALL'" json:"allowedPlatforms"`
	CreatedAt              time.Time `gorm:"column:created_at" json:"createdAt"`
	UpdatedAt              time.Time `gorm:"column:updated_at" json:"updatedAt"`
}

func (SysUser) TableName() string { return "sys_user" }

// IsAdmin 判断用户是否具有管理员或超级管理员权限
func (u *SysUser) IsAdmin() bool {
	if u == nil {
		return false
	}
	return strings.EqualFold(u.Role, RoleAdmin) || strings.EqualFold(u.Role, RoleSuperAdmin)
}

// IsSuperAdmin 判断用户是否为超级管理员
func (u *SysUser) IsSuperAdmin() bool {
	if u == nil {
		return false
	}
	return strings.EqualFold(u.Role, RoleSuperAdmin)
}

// IsActive 判断用户是否处于启用状态
func (u *SysUser) IsActive() bool {
	if u == nil {
		return false
	}
	return u.Status == UserStatusActive
}


// UserSubAccount 主账号与子账号绑定关联表
type UserSubAccount struct {
	ID           int64     `gorm:"primaryKey;autoIncrement;column:id" json:"id"`
	MasterUserID int64     `gorm:"column:master_user_id;not null" json:"masterUserId"`
	SubUserID    int64     `gorm:"column:sub_user_id;not null" json:"subUserId"`
	CreatedAt    time.Time `gorm:"column:created_at" json:"createdAt"`
}

func (UserSubAccount) TableName() string { return "user_sub_account" }

// UserViewPermission 用户-账户视图只读分配关联表
type UserViewPermission struct {
	ID           int64     `gorm:"primaryKey;autoIncrement;column:id" json:"id"`
	UserID       int64     `gorm:"column:user_id;not null" json:"userId"`
	TargetUserID int64     `gorm:"column:target_user_id;not null" json:"targetUserId"`
	CreatedAt    time.Time `gorm:"column:created_at" json:"createdAt"`
}

func (UserViewPermission) TableName() string { return "user_view_permission" }

// UserLandingPage 用户-落地页/推广ID关联配置表
type UserLandingPage struct {
	ID            int64     `gorm:"primaryKey;autoIncrement;column:id" json:"id"`
	PlatformCode  string    `gorm:"column:platform_code;size:32;not null;default:'rocnovel'" json:"platformCode"`
	UserID        int64     `gorm:"column:user_id;not null" json:"userId"`
	LandingPageID string    `gorm:"column:landing_page_id;size:64;not null" json:"landingPageId"`
	Timezone      string    `gorm:"column:timezone;size:32;not null;default:'CST'" json:"timezone"`
	CreatedAt     time.Time `gorm:"column:created_at" json:"createdAt"`
}

func (UserLandingPage) TableName() string { return "user_landing_page" }
