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
	pCode := strings.ToLower(platformCode)
	if pCode == "" {
		pCode = "all"
	}
	pages, err := s.userRepo.FindLandingPages(ctx, pCode, userID)
	if err != nil {
		return nil, nil, err
	}

	items := make([]dto.LandingPageConfigItem, 0, len(pages))
	ids := make([]string, 0, len(pages))
	for _, p := range pages {
		if p.LandingPageID == "__EMPTY__" {
			continue
		}
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
	pCode := strings.ToLower(platformCode)
	if pCode == "" || pCode == "all" {
		pCode = "rocnovel"
	}

	pages := make([]*model.UserLandingPage, 0, len(items))
	seen := make(map[string]bool)
	for _, item := range items {
		pid := strings.TrimSpace(item.LandingPageID)
		if pid == "" || pid == "__EMPTY__" {
			continue
		}
		itemPCode := strings.ToLower(item.PlatformCode)
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
			if itemPCode == "flicknovel" {
				tz = "UTC"
			} else {
				tz = "CST"
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
	pCode := strings.ToLower(platformCode)
	if pCode == "" {
		pCode = "rocnovel"
	}

	seen := make(map[string]bool)
	var result []string

	if pCode == "flicknovel" && s.flicknovelRepo != nil {
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
