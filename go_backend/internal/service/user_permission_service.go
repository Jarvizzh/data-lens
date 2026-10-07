package service

import (
	"context"
	"fmt"
	"strings"
	"time"

	"go_backend/internal/model"
	"go_backend/internal/repository"
	"go_backend/internal/service/dto"
)

type UserPermissionService struct {
	userRepo *repository.UserRepository
}

func NewUserPermissionService(userRepo *repository.UserRepository) *UserPermissionService {
	return &UserPermissionService{userRepo: userRepo}
}

// GetVisibleAccountsForUser 返回当前登录用户可见的账户切换列表
func (s *UserPermissionService) GetVisibleAccountsForUser(ctx context.Context, userID int64) ([]dto.VisibleAccountDto, error) {
	user, err := s.userRepo.FindByID(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("用户不存在: %w", err)
	}

	allUsers, err := s.userRepo.FindAll(ctx)
	if err != nil {
		return nil, err
	}

	// 1. 超级管理员：可查看所有活跃账户
	if user.IsSuperAdmin() {
		result := make([]dto.VisibleAccountDto, 0, len(allUsers))
		for _, u := range allUsers {
			if u.IsActive() {
				subCount := 0
				if u.IsMaster == 1 {
					subIDs, _ := s.userRepo.FindSubAccountIDs(ctx, u.ID)
					subCount = len(subIDs)
				}
				result = append(result, dto.VisibleAccountDto{
					ID:              u.ID,
					Username:        u.Username,
					Role:            u.Role,
					IsSelf:          u.ID == userID,
					IsMaster:        u.IsMaster,
					IsSettlement:    u.IsSettlement,
					SubAccountCount: subCount,
				})
			}
		}
		return result, nil
	}

	// 2. 普通管理员 / 普通用户：自身主账户 + 被分配允许查看的目标账户 + 若为主账号则自动包含名下子账号
	grantedTargetIDs, _ := s.userRepo.FindViewPermissionTargetIDs(ctx, userID)
	visibleSet := make(map[int64]bool)
	for _, id := range grantedTargetIDs {
		visibleSet[id] = true
	}
	visibleSet[userID] = true

	if user.IsMaster == 1 {
		subIDs, _ := s.userRepo.FindSubAccountIDs(ctx, userID)
		for _, id := range subIDs {
			visibleSet[id] = true
		}
	}

	result := make([]dto.VisibleAccountDto, 0)
	// 本人账户置顶
	selfSubCount := 0
	if user.IsMaster == 1 {
		subIDs, _ := s.userRepo.FindSubAccountIDs(ctx, user.ID)
		selfSubCount = len(subIDs)
	}
	result = append(result, dto.VisibleAccountDto{
		ID:              user.ID,
		Username:        user.Username,
		Role:            user.Role,
		IsSelf:          true,
		IsMaster:        user.IsMaster,
		IsSettlement:    user.IsSettlement,
		SubAccountCount: selfSubCount,
	})

	for _, u := range allUsers {
		if u.ID != userID && visibleSet[u.ID] && u.Status == 1 {
			subCount := 0
			if u.IsMaster == 1 {
				subIDs, _ := s.userRepo.FindSubAccountIDs(ctx, u.ID)
				subCount = len(subIDs)
			}
			result = append(result, dto.VisibleAccountDto{
				ID:              u.ID,
				Username:        u.Username,
				Role:            u.Role,
				IsSelf:          false,
				IsMaster:        u.IsMaster,
				IsSettlement:    u.IsSettlement,
				SubAccountCount: subCount,
			})
		}
	}

	return result, nil
}

// CanUserViewTarget 判断当前用户是否可以查看目标用户
func (s *UserPermissionService) CanUserViewTarget(ctx context.Context, currentUserID int64, currentUserRole string, targetUserID int64) bool {
	if targetUserID <= 0 || targetUserID == currentUserID {
		return true
	}
	if strings.EqualFold(currentUserRole, model.RoleSuperAdmin) {
		return true
	}
	if s.userRepo.ExistsSubAccount(ctx, currentUserID, targetUserID) {
		return true
	}
	return s.userRepo.ExistsViewPermission(ctx, currentUserID, targetUserID)
}

// CanUserModifyTarget 判断当前用户是否可以修改目标用户的配置（只读视图不能修改）
func (s *UserPermissionService) CanUserModifyTarget(ctx context.Context, currentUserID int64, currentUserRole string, targetUserID int64) bool {
	if targetUserID <= 0 || targetUserID == currentUserID {
		return true
	}
	if strings.EqualFold(currentUserRole, model.RoleSuperAdmin) {
		return true
	}
	return s.userRepo.ExistsSubAccount(ctx, currentUserID, targetUserID)
}

// HasPermGlobalDistribution 判断用户是否拥有平台汇总权限 (对应 Java user.hasPermGlobalDistribution())
func (s *UserPermissionService) HasPermGlobalDistribution(ctx context.Context, userID int64) bool {
	if userID <= 0 {
		return false
	}
	user, err := s.userRepo.FindByID(ctx, userID)
	if err != nil || user == nil {
		return false
	}
	return user.IsSuperAdmin() || user.PermGlobalDistribution == 1
}


// GetSettlementAccountsForUser 获取结算账号列表
func (s *UserPermissionService) GetSettlementAccountsForUser(ctx context.Context, userID int64) ([]dto.VisibleAccountDto, error) {
	user, err := s.userRepo.FindByID(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("用户不存在: %w", err)
	}

	allUsers, err := s.userRepo.FindAll(ctx)
	if err != nil {
		return nil, err
	}

	isAdmin := user.IsAdmin()
	if isAdmin {
		result := make([]dto.VisibleAccountDto, 0)
		if user.IsSettlement == 1 || isAdmin {
			selfSubCount := 0
			if user.IsMaster == 1 {
				subIDs, _ := s.userRepo.FindSubAccountIDs(ctx, user.ID)
				selfSubCount = len(subIDs)
			}
			result = append(result, dto.VisibleAccountDto{
				ID:              user.ID,
				Username:        user.Username,
				Role:            user.Role,
				IsSelf:          true,
				IsMaster:        user.IsMaster,
				IsSettlement:    user.IsSettlement,
				SubAccountCount: selfSubCount,
			})
		}

		for _, u := range allUsers {
			if u.ID != userID && u.Status == 1 && u.IsSettlement == 1 {
				subCount := 0
				if u.IsMaster == 1 {
					subIDs, _ := s.userRepo.FindSubAccountIDs(ctx, u.ID)
					subCount = len(subIDs)
				}
				result = append(result, dto.VisibleAccountDto{
					ID:              u.ID,
					Username:        u.Username,
					Role:            u.Role,
					IsSelf:          false,
					IsMaster:        u.IsMaster,
					IsSettlement:    u.IsSettlement,
					SubAccountCount: subCount,
				})
			}
		}
		return result, nil
	}

	// 普通用户
	result := make([]dto.VisibleAccountDto, 0)
	selfSubCount := 0
	if user.IsMaster == 1 {
		subIDs, _ := s.userRepo.FindSubAccountIDs(ctx, user.ID)
		selfSubCount = len(subIDs)
	}
	result = append(result, dto.VisibleAccountDto{
		ID:              user.ID,
		Username:        user.Username,
		Role:            user.Role,
		IsSelf:          true,
		IsMaster:        user.IsMaster,
		IsSettlement:    user.IsSettlement,
		SubAccountCount: selfSubCount,
	})
	return result, nil
}

func (s *UserPermissionService) UpdateMasterStatus(ctx context.Context, userID int64, isMaster int) error {
	user, err := s.userRepo.FindByID(ctx, userID)
	if err != nil {
		return fmt.Errorf("用户不存在: %w", err)
	}
	user.IsMaster = isMaster
	user.UpdatedAt = time.Now()
	if isMaster != 1 {
		_ = s.userRepo.ReplaceMasterSubAccounts(ctx, userID, nil)
	}
	return s.userRepo.Update(ctx, user)
}

func (s *UserPermissionService) UpdateSettlementStatus(ctx context.Context, userID int64, isSettlement int) error {
	user, err := s.userRepo.FindByID(ctx, userID)
	if err != nil {
		return fmt.Errorf("用户不存在: %w", err)
	}
	user.IsSettlement = isSettlement
	user.UpdatedAt = time.Now()
	return s.userRepo.Update(ctx, user)
}

func (s *UserPermissionService) UpdateMasterSubAccounts(ctx context.Context, masterUserID int64, subUserIDs []int64) error {
	return s.userRepo.ReplaceMasterSubAccounts(ctx, masterUserID, subUserIDs)
}

func (s *UserPermissionService) GetSubUserIDsForMaster(ctx context.Context, masterUserID int64) ([]int64, error) {
	return s.userRepo.FindSubAccountIDs(ctx, masterUserID)
}

func (s *UserPermissionService) UpdateUserViewPermissions(ctx context.Context, userID int64, targetUserIDs []int64) error {
	return s.userRepo.ReplaceViewPermissions(ctx, userID, targetUserIDs)
}

func (s *UserPermissionService) GetUserViewPermissionTargetIDs(ctx context.Context, userID int64) ([]int64, error) {
	return s.userRepo.FindViewPermissionTargetIDs(ctx, userID)
}

type UserPermissionsParam struct {
	PermPredictPayback     int
	PermRoiPredict         int
	PermGlobalDistribution int
	PermExport             int
	PermSettlement         int
	PermVideoGen           int
	AllowedPlatforms       string
}

func (s *UserPermissionService) UpdateUserPermissions(ctx context.Context, userID int64, param UserPermissionsParam) error {
	user, err := s.userRepo.FindByID(ctx, userID)
	if err != nil {
		return fmt.Errorf("用户不存在: %w", err)
	}
	user.PermPredictPayback = param.PermPredictPayback
	user.PermRoiPredict = param.PermRoiPredict
	user.PermGlobalDistribution = param.PermGlobalDistribution
	user.PermExport = param.PermExport
	user.PermSettlement = param.PermSettlement
	user.PermVideoGen = param.PermVideoGen
	if !user.IsSuperAdmin() && param.AllowedPlatforms != "" {
		user.AllowedPlatforms = param.AllowedPlatforms
	}
	user.UpdatedAt = time.Now()
	return s.userRepo.Update(ctx, user)
}
