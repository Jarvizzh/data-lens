package service

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"fmt"
	"time"

	"go_backend/internal/model"
	"go_backend/internal/repository"
)

type UserService struct {
	userRepo *repository.UserRepository
}

func NewUserService(userRepo *repository.UserRepository) *UserService {
	return &UserService{userRepo: userRepo}
}

// HashPassword 与 Java 版本加盐 SHA-256 100% 兼容
func HashPassword(rawPassword string) string {
	hash := sha256.Sum256([]byte("zw-ltv-salt-" + rawPassword))
	return base64.StdEncoding.EncodeToString(hash[:])
}

// ValidatePassword 校验密码
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
		return nil, errors.New("username already exists")
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
		return nil, fmt.Errorf("create user failed: %w", err)
	}
	return user, nil
}

func (s *UserService) UpdateUser(ctx context.Context, user *model.SysUser) error {
	user.UpdatedAt = time.Now()
	return s.userRepo.Update(ctx, user)
}

func (s *UserService) BindSubAccount(ctx context.Context, masterUserID, subUserID int64) error {
	return s.userRepo.BindSubAccount(ctx, masterUserID, subUserID)
}

func (s *UserService) UnbindSubAccount(ctx context.Context, masterUserID, subUserID int64) error {
	return s.userRepo.UnbindSubAccount(ctx, masterUserID, subUserID)
}

func (s *UserService) ReplaceLandingPages(ctx context.Context, platformCode string, userID int64, lpIDs []string) error {
	return s.userRepo.ReplaceLandingPages(ctx, platformCode, userID, lpIDs)
}

func (s *UserService) GetLandingPages(ctx context.Context, platformCode string, userID int64) ([]*model.UserLandingPage, error) {
	return s.userRepo.FindLandingPages(ctx, platformCode, userID)
}
