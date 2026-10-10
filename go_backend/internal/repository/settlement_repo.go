package repository

import (
	"context"
	"strings"

	"go_backend/internal/model"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type SettlementRepository struct {
	db *gorm.DB
}

func NewSettlementRepository(db *gorm.DB) *SettlementRepository {
	return &SettlementRepository{db: db}
}

func (r *SettlementRepository) FindConfigs(ctx context.Context, platformCode, settlementType string, targetUserID *int64, monthStr string) ([]*model.MonthlySettlementConfig, error) {
	var list []*model.MonthlySettlementConfig
	q := r.db.WithContext(ctx)

	if platformCode != "" {
		pCode := strings.ToLower(strings.TrimSpace(platformCode))
		if pCode == "" || pCode == "all" {
			pCode = "ALL"
		}
		q = q.Where("platform_code = ?", pCode)
	}
	if settlementType != "" {
		q = q.Where("settlement_type = ?", settlementType)
	}
	if targetUserID != nil && *targetUserID > 0 {
		q = q.Where("target_user_id = ?", *targetUserID)
	} else if strings.EqualFold(settlementType, "PLATFORM_ALL") || strings.EqualFold(settlementType, "UNLINKED_PID") {
		q = q.Where("target_user_id IS NULL OR target_user_id = 0")
	}
	if monthStr != "" {
		q = q.Where("month_str = ?", monthStr)
	}

	err := q.Find(&list).Error
	return list, err
}

func (r *SettlementRepository) FindOneConfig(ctx context.Context, platformCode, settlementType string, targetUserID *int64, monthStr string) (*model.MonthlySettlementConfig, error) {
	var cfg model.MonthlySettlementConfig
	pCode := strings.ToLower(strings.TrimSpace(platformCode))
	if pCode == "" || pCode == "all" {
		pCode = "ALL"
	}

	q := r.db.WithContext(ctx).Where("platform_code = ? AND settlement_type = ? AND month_str = ?", pCode, settlementType, monthStr)
	if targetUserID != nil && *targetUserID > 0 {
		q = q.Where("target_user_id = ?", *targetUserID)
	} else {
		q = q.Where("target_user_id IS NULL OR target_user_id = 0")
	}
	err := q.First(&cfg).Error
	if err != nil {
		return nil, err
	}
	return &cfg, nil
}

func (r *SettlementRepository) SaveConfig(ctx context.Context, cfg *model.MonthlySettlementConfig) error {
	if cfg.PlatformCode == "" {
		cfg.PlatformCode = "ALL"
	}
	return r.db.WithContext(ctx).Clauses(clause.OnConflict{
		Columns: []clause.Column{
			{Name: "platform_code"},
			{Name: "settlement_type"},
			{Name: "target_user_id"},
			{Name: "month_str"},
		},
		DoUpdates: clause.AssignmentColumns([]string{
			"settled_refund_amount",
			"month_settled_refund_amount",
			"cross_period_refund_amount",
			"share_ratio",
			"channel_fee_rate",
			"remark",
			"updated_at",
		}),
	}).Save(cfg).Error
}

