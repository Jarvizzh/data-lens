package service

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"fmt"
	"strings"
	"time"

	"go_backend/internal/model"
	"go_backend/internal/repository"
	"go_backend/internal/service/dto"
)

type UserService struct {
	userRepo       *repository.UserRepository
	orderRepo      *repository.OrderRepository
	flicknovelRepo *repository.FlicknovelRepository
}

func NewUserService(
	userRepo *repository.UserRepository,
	orderRepo *repository.OrderRepository,
	flicknovelRepo *repository.FlicknovelRepository,
) *UserService {
	return &UserService{
		userRepo:       userRepo,
		orderRepo:      orderRepo,
		flicknovelRepo: flicknovelRepo,
	}
}

// HashPassword 与 Java 版本加盐 SHA-256 100% 兼容: Base64(SHA-256("zw-ltv-salt-" + rawPassword))
func HashPassword(rawPassword string) string {
	hash := sha256.Sum256([]byte("zw-ltv-salt-" + rawPassword))
	return base64.StdEncoding.EncodeToString(hash[:])
}

func (s *UserService) ValidatePassword(user *model.SysUser, rawPassword string) bool {
	if user == nil || rawPassword == "" {
		return false
	}
	return user.PasswordHash == HashPassword(rawPassword)
}

func (s *UserService) FindByUsername(ctx context.Context, username string) (*model.SysUser, error) {
	return s.userRepo.FindByUsername(ctx, username)
}

func (s *UserService) FindByID(ctx context.Context, id int64) (*model.SysUser, error) {
	return s.userRepo.FindByID(ctx, id)
}

func (s *UserService) ListAllUsers(ctx context.Context) ([]*model.SysUser, error) {
	return s.userRepo.FindAll(ctx)
}

// InitDefaultUsers 初始化默认超级管理员和管理员 (读取配置中的超级管理员账号与密码)
func (s *UserService) InitDefaultUsers(ctx context.Context, defaultSuperAdminUsername, defaultSuperAdminPassword string) error {
	if defaultSuperAdminUsername == "" {
		defaultSuperAdminUsername = "super"
	}
	if defaultSuperAdminPassword == "" {
		defaultSuperAdminPassword = "@super"
	}

	// 1. 初始化或升级配置中的超级管理员 (SUPER_ADMIN)
	superAdmin, err := s.userRepo.FindByUsername(ctx, defaultSuperAdminUsername)
	if err != nil || superAdmin == nil {
		newSuper := &model.SysUser{
			Username:         defaultSuperAdminUsername,
			PasswordHash:     HashPassword(defaultSuperAdminPassword),
			Role:             model.RoleSuperAdmin,
			Status:           model.UserStatusActive,
			AllowedPlatforms: model.PlatformAll,
			CreatedAt:        time.Now(),
			UpdatedAt:        time.Now(),
		}
		_ = s.userRepo.Create(ctx, newSuper)
	} else {
		if !superAdmin.IsSuperAdmin() {
			superAdmin.Role = model.RoleSuperAdmin
			_ = s.userRepo.Update(ctx, superAdmin)
		}
	}

	// 自动清理此前代码硬编码历史遗留且密码未变更的冗余 superadmin 账号
	if defaultSuperAdminUsername != "superadmin" {
		if oldSuper, err := s.userRepo.FindByUsername(ctx, "superadmin"); err == nil && oldSuper != nil {
			if oldSuper.PasswordHash == HashPassword("@superadmin666") {
				_ = s.userRepo.Delete(ctx, oldSuper.ID)
			}
		}
	}

	// 2. 确保 admin 账号角色归位为普通管理员 ADMIN
	admin, err := s.userRepo.FindByUsername(ctx, "admin")
	if err == nil && admin != nil {
		if admin.IsSuperAdmin() {
			admin.Role = model.RoleAdmin
			_ = s.userRepo.Update(ctx, admin)
		}
	} else if admin == nil {
		newAdmin := &model.SysUser{
			Username:         "admin",
			PasswordHash:     HashPassword("admin666"),
			Role:             model.RoleAdmin,
			Status:           model.UserStatusActive,
			AllowedPlatforms: model.PlatformAll,
			CreatedAt:        time.Now(),
			UpdatedAt:        time.Now(),
		}
		_ = s.userRepo.Create(ctx, newAdmin)
	}

	return nil
}

type CreateUserParam struct {
	Username               string
	RawPassword            string
	Role                   string
	IsMaster               int
	IsSettlement           int
	PermPredictPayback     int
	PermRoiPredict         int
	PermGlobalDistribution int
	PermExport             int
	PermSettlement         int
	PermVideoGen           int
	AllowedPlatforms       string
}

func (s *UserService) CreateUser(ctx context.Context, param CreateUserParam) (*model.SysUser, error) {
	existing, _ := s.userRepo.FindByUsername(ctx, param.Username)
	if existing != nil {
		return nil, errors.New("用户名已存在")
	}

	role := param.Role
	if role == "" {
		role = "USER"
	}
	platforms := param.AllowedPlatforms
	if platforms == "" {
		platforms = "ALL"
	}

	user := &model.SysUser{
		Username:               param.Username,
		PasswordHash:           HashPassword(param.RawPassword),
		Role:                   role,
		Status:                 1,
		IsMaster:               param.IsMaster,
		IsSettlement:           param.IsSettlement,
		PermPredictPayback:     param.PermPredictPayback,
		PermRoiPredict:         param.PermRoiPredict,
		PermGlobalDistribution: param.PermGlobalDistribution,
		PermExport:             param.PermExport,
		PermSettlement:         param.PermSettlement,
		PermVideoGen:           param.PermVideoGen,
		AllowedPlatforms:       platforms,
		CreatedAt:              time.Now(),
		UpdatedAt:              time.Now(),
	}

	if err := s.userRepo.Create(ctx, user); err != nil {
		return nil, fmt.Errorf("创建用户失败: %w", err)
	}
	return user, nil
}

func (s *UserService) ResetPassword(ctx context.Context, userID int64, newPassword string) error {
	user, err := s.userRepo.FindByID(ctx, userID)
	if err != nil {
		return fmt.Errorf("用户不存在: %w", err)
	}
	user.PasswordHash = HashPassword(newPassword)
	user.UpdatedAt = time.Now()
	return s.userRepo.Update(ctx, user)
}

func (s *UserService) UpdateUserRole(ctx context.Context, userID int64, newRole string) error {
	user, err := s.userRepo.FindByID(ctx, userID)
	if err != nil {
		return fmt.Errorf("用户不存在: %w", err)
	}
	user.Role = newRole
	user.UpdatedAt = time.Now()
	return s.userRepo.Update(ctx, user)
}

func (s *UserService) DeleteUser(ctx context.Context, userID int64) error {
	_ = s.userRepo.DeleteSubAccountRelationsForUser(ctx, userID)
	return s.userRepo.Delete(ctx, userID)
}

func (s *UserService) GetLandingPageConfigs(ctx context.Context, platformCode string, userID int64) ([]dto.LandingPageConfigItem, []string, error) {
	if userID <= 0 {
		return nil, nil, nil
	}
	user, err := s.userRepo.FindByID(ctx, userID)
	if err != nil || user == nil {
		return nil, nil, err
	}

	pCode := strings.ToLower(strings.TrimSpace(platformCode))
	if pCode == "" {
		pCode = "all"
	}

	// 1. 若为主账号，自动聚合所有子账号配置的落地页（去重）
	if user.IsMaster == 1 {
		subUserIDs, err := s.userRepo.FindSubAccountIDs(ctx, userID)
		if err != nil {
			return nil, nil, err
		}
		uniquePids := make(map[string]bool)
		var aggregated []dto.LandingPageConfigItem
		var aggregatedIDs []string
		for _, subID := range subUserIDs {
			subConfigs, _, err := s.GetLandingPageConfigs(ctx, platformCode, subID)
			if err != nil {
				continue
			}
			for _, item := range subConfigs {
				pid := strings.TrimSpace(item.LandingPageID)
				if pid != "" && !strings.EqualFold(pid, "__EMPTY__") {
					if !uniquePids[pid] {
						uniquePids[pid] = true
						aggregated = append(aggregated, item)
						aggregatedIDs = append(aggregatedIDs, pid)
					}
				}
			}
		}
		return aggregated, aggregatedIDs, nil
	}

	// 2. 普通账号或管理员账号
	pages, err := s.userRepo.FindLandingPages(ctx, pCode, userID)
	if err != nil {
		return nil, nil, err
	}

	isAdmin := user.IsAdmin()

	// 番茄司南 (flicknovel) 初始配置特殊处理：
	// 仅管理员初始落地页默认填充系统已知的所有推广ID（时区默认 UTC）；普通用户初始默认为空
	if model.IsFlicknovel(pCode) && len(pages) == 0 {
		if isAdmin {
			allPids, _ := s.GetAllPlatformLandingPageIds(ctx, model.PlatformFlicknovel)
			if len(allPids) > 0 {
				items := make([]dto.LandingPageConfigItem, 0, len(allPids))
				ids := make([]string, 0, len(allPids))
				for _, pid := range allPids {
					clean := strings.TrimSpace(pid)
					if clean != "" && !strings.EqualFold(clean, "__EMPTY__") {
						items = append(items, dto.LandingPageConfigItem{
							PlatformCode:  model.PlatformFlicknovel,
							LandingPageID: clean,
							Timezone:      model.UtcDefaultTimezone,
						})
						ids = append(ids, clean)
					}
				}
				return items, ids, nil
			}
		} else {
			return nil, nil, nil
		}
	}

	// 过滤掉用于标记已主动清空的占位记录 __EMPTY__
	filtered := make([]*model.UserLandingPage, 0, len(pages))
	for _, p := range pages {
		clean := strings.TrimSpace(p.LandingPageID)
		if clean != "" && !strings.EqualFold(clean, "__EMPTY__") {
			filtered = append(filtered, p)
		}
	}

	// 如果是普通用户 (USER)，剔除已被管理员配置的隔离落地页 ID
	if strings.EqualFold(user.Role, "USER") {
		adminPids, _ := s.userRepo.FindAdminLandingPageIDs(ctx, userID)
		adminPidSet := make(map[string]bool, len(adminPids))
		for _, apid := range adminPids {
			adminPidSet[strings.TrimSpace(apid)] = true
		}
		nonAdmin := make([]*model.UserLandingPage, 0, len(filtered))
		for _, p := range filtered {
			if !adminPidSet[strings.TrimSpace(p.LandingPageID)] {
				nonAdmin = append(nonAdmin, p)
			}
		}
		filtered = nonAdmin
	}

	items := make([]dto.LandingPageConfigItem, 0, len(filtered))
	ids := make([]string, 0, len(filtered))
	for _, p := range filtered {
		items = append(items, dto.LandingPageConfigItem{
			PlatformCode:  p.PlatformCode,
			LandingPageID: p.LandingPageID,
			Timezone:      p.Timezone,
		})
		ids = append(ids, p.LandingPageID)
	}
	return items, ids, nil
}

func (s *UserService) UpdateLandingPageConfigs(ctx context.Context, platformCode string, userID int64, items []dto.LandingPageConfigItem) error {
	user, err := s.userRepo.FindByID(ctx, userID)
	if err != nil || user == nil {
		return errors.New("用户不存在")
	}
	if user.IsMaster == 1 {
		return errors.New("主账号为数据汇总账号，落地页由关联子账号自动聚合，不可直接编辑！")
	}

	// 如果是普通用户 (USER)，拦截校验：不允许配置已被管理员配置的独占隔离落地页
	if strings.EqualFold(user.Role, "USER") {
		adminPids, _ := s.userRepo.FindAdminLandingPageIDs(ctx, userID)
		adminPidSet := make(map[string]bool, len(adminPids))
		for _, apid := range adminPids {
			adminPidSet[strings.TrimSpace(apid)] = true
		}
		for _, item := range items {
			pid := strings.TrimSpace(item.LandingPageID)
			if adminPidSet[pid] {
				return fmt.Errorf("落地页 ID [%s] 为管理员独占/隔离落地页，普通用户无法配置！", pid)
			}
		}
	}

	pCode := model.NormalizePlatform(platformCode)
	if pCode == model.PlatformAll {
		pCode = model.PlatformRocnovel
	}

	pages := make([]*model.UserLandingPage, 0, len(items))
	seen := make(map[string]bool)
	for _, item := range items {
		pid := strings.TrimSpace(item.LandingPageID)
		if pid == "" || pid == "__EMPTY__" {
			continue
		}
		itemPCode := strings.ToLower(strings.TrimSpace(item.PlatformCode))
		if itemPCode == "" {
			itemPCode = pCode
		}
		key := itemPCode + "_" + pid
		if seen[key] {
			continue
		}
		seen[key] = true

		tz := strings.ToUpper(strings.TrimSpace(item.Timezone))
		if tz == "" || tz == "BJ" {
			if model.IsFlicknovel(itemPCode) {
				tz = model.UtcDefaultTimezone
			} else {
				tz = model.CstDefaultTimezone
			}
		}

		pages = append(pages, &model.UserLandingPage{
			PlatformCode:  itemPCode,
			UserID:        userID,
			LandingPageID: pid,
			Timezone:      tz,
		})
	}

	if len(pages) == 0 {
		pages = append(pages, &model.UserLandingPage{
			PlatformCode:  pCode,
			UserID:        userID,
			LandingPageID: "__EMPTY__",
			Timezone:      "CST",
		})
	}

	return s.userRepo.ReplaceLandingPageConfigs(ctx, pCode, userID, pages)
}

func (s *UserService) GetAllPlatformLandingPageIds(ctx context.Context, platformCode string) ([]string, error) {
	pCode := model.NormalizePlatform(platformCode)
	if pCode == model.PlatformAll {
		pCode = model.PlatformRocnovel
	}

	seen := make(map[string]bool)
	var result []string

	if model.IsFlicknovel(pCode) && s.flicknovelRepo != nil {
		promos, err := s.flicknovelRepo.FindAllPromotionIDs(ctx)
		if err == nil {
			for _, pid := range promos {
				clean := strings.TrimSpace(pid)
				if clean != "" && !seen[clean] {
					seen[clean] = true
					result = append(result, clean)
				}
			}
		}
	}

	if s.orderRepo != nil {
		orderPids, err := s.orderRepo.FindDistinctLandingPageIDs(ctx, pCode)
		if err == nil {
			for _, pid := range orderPids {
				clean := strings.TrimSpace(pid)
				if clean != "" && !seen[clean] {
					seen[clean] = true
					result = append(result, clean)
				}
			}
		}
	}

	return result, nil
}

// AutoImportFlicknovelLandingPagesForAdmins 自动为所有超级管理员/管理员补齐番茄司南全量推广ID (对齐 Java UserService.autoImportFlicknovelLandingPagesForAdmins)
func (s *UserService) AutoImportFlicknovelLandingPagesForAdmins(ctx context.Context) (int, error) {
	allPids, err := s.GetAllPlatformLandingPageIds(ctx, model.PlatformFlicknovel)
	if err != nil || len(allPids) == 0 {
		return 0, nil
	}

	users, err := s.userRepo.FindAll(ctx)
	if err != nil {
		return 0, err
	}

	var admins []*model.SysUser
	for _, u := range users {
		if u.IsActive() && u.IsMaster != 1 && u.IsAdmin() {
			admins = append(admins, u)
		}
	}
	if len(admins) == 0 {
		return 0, nil
	}

	totalImported := 0
	for _, admin := range admins {
		existingPages, err := s.userRepo.FindLandingPages(ctx, model.PlatformFlicknovel, admin.ID)
		if err != nil {
			continue
		}

		existingPids := make(map[string]bool)
		for _, ep := range existingPages {
			pid := strings.TrimSpace(ep.LandingPageID)
			if pid != "" && pid != "__EMPTY__" {
				existingPids[pid] = true
			}
		}

		var toAdd []*model.UserLandingPage
		for _, pid := range allPids {
			cleanPid := strings.TrimSpace(pid)
			if cleanPid != "" && cleanPid != "__EMPTY__" && !existingPids[cleanPid] {
				toAdd = append(toAdd, &model.UserLandingPage{
					PlatformCode:  model.PlatformFlicknovel,
					UserID:        admin.ID,
					LandingPageID: cleanPid,
					Timezone:      model.UtcDefaultTimezone,
				})
				existingPids[cleanPid] = true
			}
		}

		if len(toAdd) > 0 {
			if err := s.userRepo.AddLandingPagesBatch(ctx, toAdd); err == nil {
				totalImported += len(toAdd)
			}
		}
	}

	return totalImported, nil
}
