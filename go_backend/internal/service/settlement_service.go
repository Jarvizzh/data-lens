package service

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	"go_backend/internal/model"
	"go_backend/internal/pkg/timeutil"
	"go_backend/internal/repository"
	"go_backend/internal/service/dto"

	"github.com/shopspring/decimal"
)

type SettlementService struct {
	settleRepo *repository.SettlementRepository
	orderRepo  *repository.OrderRepository
	userRepo   *repository.UserRepository
}

func NewSettlementService(
	settleRepo *repository.SettlementRepository,
	orderRepo *repository.OrderRepository,
	userRepo *repository.UserRepository,
) *SettlementService {
	return &SettlementService{
		settleRepo: settleRepo,
		orderRepo:  orderRepo,
		userRepo:   userRepo,
	}
}

func (s *SettlementService) SaveSettlementConfig(ctx context.Context, req dto.MonthlySettlementSaveRequestDto) (*model.MonthlySettlementConfig, error) {
	cfg := &model.MonthlySettlementConfig{
		SettlementType:           req.SettlementType,
		TargetUserID:             req.TargetUserID,
		MonthStr:                 req.MonthStr,
		SettledRefundAmount:      req.SettledRefundAmount,
		MonthSettledRefundAmount: req.MonthSettledRefundAmount,
		CrossPeriodRefundAmount:  req.CrossPeriodRefundAmount,
		ShareRatio:               req.ShareRatio,
		ChannelFeeRate:           req.ChannelFeeRate,
		Remark:                   req.Remark,
		UpdatedAt:                time.Now(),
	}
	if err := s.settleRepo.SaveConfig(ctx, cfg); err != nil {
		return nil, err
	}
	return cfg, nil
}

func (s *SettlementService) GetMonthlySettlementList(
	ctx context.Context,
	platformCode string,
	settlementType string,
	targetUserID *int64,
) ([]*dto.MonthlySettlementItemDto, error) {
	sType := strings.ToUpper(strings.TrimSpace(settlementType))
	if sType == "" {
		sType = "PLATFORM_ALL"
	}
	pCode := strings.ToLower(strings.TrimSpace(platformCode))
	if pCode == "" {
		pCode = "all"
	}

	targetUsername := ""
	var userPidMap map[string]bool
	if sType == "USER_ACCOUNT" && targetUserID != nil && *targetUserID > 0 {
		u, _ := s.userRepo.FindByID(ctx, *targetUserID)
		if u != nil {
			targetUsername = u.Username
		}
		userPages, _ := s.userRepo.FindLandingPages(ctx, pCode, *targetUserID)
		userPidMap = make(map[string]bool)
		for _, up := range userPages {
			userPidMap[up.LandingPageID] = true
		}
	}

	allConfigs, _ := s.settleRepo.FindConfigs(ctx, sType, targetUserID, "")
	configMap := make(map[string]*model.MonthlySettlementConfig)
	for _, c := range allConfigs {
		configMap[c.MonthStr] = c
	}

	today := time.Now().In(timeutil.BeijingZone)
	monthsSet := make(map[string]bool)
	for i := 0; i < 6; i++ {
		m := today.AddDate(0, -i, 0).Format("2006-01")
		monthsSet[m] = true
	}
	months := make([]string, 0, len(monthsSet))
	for m := range monthsSet {
		months = append(months, m)
	}
	sort.Sort(sort.Reverse(sort.StringSlice(months)))

	result := make([]*dto.MonthlySettlementItemDto, 0, len(months))
	sumHistoricalUnsettledRefund := decimal.Zero

	for _, m := range months {
		totalRecharge := decimal.Zero
		totalRefund := decimal.Zero
		totalOrders := 0
		refundOrders := 0

		startDate := m + "-01"
		endDate := m + "-31"
		orders, _ := s.orderRepo.FindOrdersForLtvCalculation(ctx, pCode, nil, startDate, endDate)
		for _, o := range orders {
			if sType == "USER_ACCOUNT" && userPidMap != nil && !userPidMap[o.LandingPageID] {
				continue
			}
			totalRecharge = totalRecharge.Add(o.OrderAmountUSD)
			totalOrders++
			if o.RefundStatus == 2 {
				totalRefund = totalRefund.Add(o.OrderAmountUSD)
				refundOrders++
			}
		}

		settledRefund := decimal.Zero
		monthSettledRefund := decimal.Zero
		crossPeriodRefund := decimal.Zero
		shareRatio := decimal.NewFromFloat(0.95)
		channelFeeRate := decimal.NewFromFloat(0.07)
		remark := ""
		var updatedAt *time.Time

		if cfg, ok := configMap[m]; ok {
			settledRefund = cfg.SettledRefundAmount
			monthSettledRefund = cfg.MonthSettledRefundAmount
			crossPeriodRefund = cfg.CrossPeriodRefundAmount
			shareRatio = cfg.ShareRatio
			channelFeeRate = cfg.ChannelFeeRate
			remark = cfg.Remark
			t := cfg.UpdatedAt
			updatedAt = &t
		}

		if m == today.Format("2006-01") {
			monthSettledRefund = totalRefund
			crossPeriodRefund = sumHistoricalUnsettledRefund
		}

		unsettledRefund := totalRefund.Sub(settledRefund)
		effectiveBase := totalRecharge.Sub(monthSettledRefund).Sub(crossPeriodRefund)
		finalSettlement := decimal.Zero
		if effectiveBase.GreaterThan(decimal.Zero) {
			netFactor := decimal.NewFromInt(1).Sub(channelFeeRate)
			finalSettlement = effectiveBase.Mul(shareRatio).Mul(netFactor).Round(2)
		} else {
			finalSettlement = effectiveBase.Mul(shareRatio).Round(2)
		}

		refundRate := "0.00%"
		if totalRecharge.GreaterThan(decimal.Zero) {
			rate := totalRefund.DivRound(totalRecharge, 4).Mul(decimal.NewFromInt(100))
			refundRate = fmt.Sprintf("%.2f%%", rate.InexactFloat64())
		}

		item := &dto.MonthlySettlementItemDto{
			MonthStr:                 m,
			SettlementType:           sType,
			TargetUserID:             targetUserID,
			TargetUsername:           targetUsername,
			TotalRecharge:            totalRecharge,
			TotalRefund:              totalRefund,
			SettledRefundAmount:      settledRefund,
			MonthSettledRefundAmount: monthSettledRefund,
			UnsettledRefundAmount:    unsettledRefund,
			CrossPeriodRefundAmount:  crossPeriodRefund,
			ShareRatio:               shareRatio,
			ChannelFeeRate:           channelFeeRate,
			EffectiveBaseAmount:      effectiveBase,
			FinalSettlementAmount:    finalSettlement,
			RefundRate:               refundRate,
			TotalOrders:              totalOrders,
			RefundOrders:             refundOrders,
			Remark:                   remark,
			UpdatedAt:                updatedAt,
		}
		result = append(result, item)
	}

	return result, nil
}
