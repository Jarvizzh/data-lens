package service

import (
	"context"
	"strings"
	"time"

	"go_backend/internal/model"
	"go_backend/internal/pkg/timeutil"
	"go_backend/internal/repository"
	"go_backend/internal/service/dto"

	"github.com/shopspring/decimal"
)

type DailyDistributionService struct {
	rechargeDistRepo *repository.RechargeDistributionRepository
	orderRepo        *repository.OrderRepository
	userRepo         *repository.UserRepository
	calcSvc          *RechargeStatService
}

func NewDailyDistributionService(
	rechargeDistRepo *repository.RechargeDistributionRepository,
	orderRepo *repository.OrderRepository,
	userRepo *repository.UserRepository,
	calcSvc *RechargeStatService,
) *DailyDistributionService {
	return &DailyDistributionService{
		rechargeDistRepo: rechargeDistRepo,
		orderRepo:        orderRepo,
		userRepo:         userRepo,
		calcSvc:          calcSvc,
	}
}

// GetDailyDistributionResponse 获取指定用户/平台的每日充值分布与汇总
func (s *DailyDistributionService) GetDailyDistributionResponse(
	ctx context.Context,
	platformCode string,
	targetUserID int64,
) (*dto.DailyDistributionResponseDto, error) {
	pCode := strings.ToLower(platformCode)
	if pCode == "" {
		pCode = "all"
	}
	if targetUserID <= 0 {
		targetUserID = 1
	}

	startDate := "2026-07-10"
	if pCode == "flicknovel" {
		startDate = "2026-09-16"
	}

	userIDs := []int64{targetUserID}
	list, err := s.rechargeDistRepo.FindByFilter(ctx, pCode, userIDs, startDate, "")
	if err != nil || len(list) == 0 {
		_ = s.calcSvc.CalculateDailyDistributionForUser(ctx, pCode, targetUserID)
		list, _ = s.rechargeDistRepo.FindByFilter(ctx, pCode, userIDs, startDate, "")
	}

	summary := s.calculateSummaryFromDistribution(list)

	return &dto.DailyDistributionResponseDto{
		Code:    0,
		Msg:     "success",
		Data:    list,
		Summary: summary,
		Total:   len(list),
		UserID:  targetUserID,
	}, nil
}

// RecalculateDailyDistribution 重新计算指定用户/平台的每日充值分布统计
func (s *DailyDistributionService) RecalculateDailyDistribution(
	ctx context.Context,
	platformCode string,
	targetUserID int64,
) (*dto.DailyDistributionResponseDto, error) {
	pCode := strings.ToLower(platformCode)
	if pCode == "" {
		pCode = "all"
	}
	if targetUserID <= 0 {
		targetUserID = 1
	}

	if s.calcSvc != nil {
		_ = s.calcSvc.CalculateDailyDistributionForUser(ctx, pCode, targetUserID)
	}

	return s.GetDailyDistributionResponse(ctx, platformCode, targetUserID)
}

// GetGlobalDailyDistributionResponse 获取全盘每日充值分布
func (s *DailyDistributionService) GetGlobalDailyDistributionResponse(
	ctx context.Context,
	platformCode string,
) (*dto.DailyDistributionResponseDto, error) {
	pCode := strings.ToLower(platformCode)
	if pCode == "" {
		pCode = "all"
	}

	startDate := "2026-07-10"
	if pCode == "flicknovel" {
		startDate = "2026-09-16"
	}

	list, _ := s.rechargeDistRepo.FindByFilter(ctx, pCode, nil, startDate, "")
	summary := s.calculateSummaryFromDistribution(list)

	return &dto.DailyDistributionResponseDto{
		Code:    0,
		Msg:     "success",
		Data:    list,
		Summary: summary,
		Total:   len(list),
		UserID:  0,
	}, nil
}

func (s *DailyDistributionService) calculateSummaryFromDistribution(list []*model.DailyRechargeDistribution) *dto.DailyDistributionSummaryDto {
	today := time.Now().In(timeutil.BeijingZone)
	thisMonthStr := today.Format("2006-01")
	lastMonthStr := today.AddDate(0, -1, 0).Format("2006-01")

	sum := &dto.DailyDistributionSummaryDto{
		TotalRecharge:     decimal.Zero,
		NewRecharge:       decimal.Zero,
		OldRecharge:       decimal.Zero,
		ThisMonthRecharge: decimal.Zero,
		ThisMonthRefund:   decimal.Zero,
		LastMonthRecharge: decimal.Zero,
		LastMonthRefund:   decimal.Zero,
		ThisMonthStr:      thisMonthStr,
		LastMonthStr:      lastMonthStr,
		NewRechargeRatio:  decimal.Zero,
		OldRechargeRatio:  decimal.Zero,
		NewArpu:           decimal.Zero,
		OldArpu:           decimal.Zero,
		RepeatRate:        decimal.Zero,
	}

	totalPaidUsers := 0
	newPaidUsers := 0
	oldPaidUsers := 0
	repeatPaidUsers := 0

	for _, item := range list {
		sum.TotalRecharge = sum.TotalRecharge.Add(item.TotalRecharge)
		sum.NewRecharge = sum.NewRecharge.Add(item.NewRecharge)
		sum.OldRecharge = sum.OldRecharge.Add(item.OldRecharge)
		totalPaidUsers += item.TotalPaidUsers
		newPaidUsers += item.NewPaidUsers
		oldPaidUsers += item.OldPaidUsers
		repeatPaidUsers += item.RepeatPaidUsers

		if len(item.Date) >= 7 {
			ym := item.Date[:7]
			if ym == thisMonthStr {
				sum.ThisMonthRecharge = sum.ThisMonthRecharge.Add(item.TotalRecharge)
			} else if ym == lastMonthStr {
				sum.LastMonthRecharge = sum.LastMonthRecharge.Add(item.TotalRecharge)
			}
		}
	}

	sum.TotalPaidUsers = totalPaidUsers
	sum.NewPaidUsers = newPaidUsers
	sum.OldPaidUsers = oldPaidUsers
	sum.RepeatPaidUsers = repeatPaidUsers

	if sum.TotalRecharge.GreaterThan(decimal.Zero) {
		sum.NewRechargeRatio = sum.NewRecharge.DivRound(sum.TotalRecharge, 4)
		sum.OldRechargeRatio = sum.OldRecharge.DivRound(sum.TotalRecharge, 4)
	}
	if newPaidUsers > 0 {
		sum.NewArpu = sum.NewRecharge.DivRound(decimal.NewFromInt(int64(newPaidUsers)), 2)
	}
	if oldPaidUsers > 0 {
		sum.OldArpu = sum.OldRecharge.DivRound(decimal.NewFromInt(int64(oldPaidUsers)), 2)
	}
	if totalPaidUsers > 0 {
		sum.RepeatRate = decimal.NewFromInt(int64(repeatPaidUsers)).DivRound(decimal.NewFromInt(int64(totalPaidUsers)), 4)
	}

	return sum
}
