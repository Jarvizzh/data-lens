package service

import (
	"context"
	"time"

	"go_backend/internal/model"
	"go_backend/internal/repository"

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

type SaveSettlementParam struct {
	SettlementType           string
	TargetUserID             *int64
	MonthStr                 string
	SettledRefundAmount      decimal.Decimal
	MonthSettledRefundAmount decimal.Decimal
	CrossPeriodRefundAmount  decimal.Decimal
	ShareRatio               decimal.Decimal
	ChannelFeeRate           decimal.Decimal
	Remark                   string
}

func (s *SettlementService) SaveSettlementConfig(ctx context.Context, param SaveSettlementParam) error {
	cfg := &model.MonthlySettlementConfig{
		SettlementType:           param.SettlementType,
		TargetUserID:             param.TargetUserID,
		MonthStr:                 param.MonthStr,
		SettledRefundAmount:      param.SettledRefundAmount,
		MonthSettledRefundAmount: param.MonthSettledRefundAmount,
		CrossPeriodRefundAmount:  param.CrossPeriodRefundAmount,
		ShareRatio:               param.ShareRatio,
		ChannelFeeRate:           param.ChannelFeeRate,
		Remark:                   param.Remark,
		UpdatedAt:                time.Now(),
	}
	return s.settleRepo.SaveConfig(ctx, cfg)
}

func (s *SettlementService) GetSettlementConfigs(ctx context.Context, settlementType string, targetUserID *int64, monthStr string) ([]*model.MonthlySettlementConfig, error) {
	return s.settleRepo.FindConfigs(ctx, settlementType, targetUserID, monthStr)
}
