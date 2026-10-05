package repository

import (
	"context"

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

func (r *SettlementRepository) FindConfigs(ctx context.Context, settlementType string, targetUserID *int64, monthStr string) ([]*model.MonthlySettlementConfig, error) {
	var list []*model.MonthlySettlementConfig
	q := r.db.WithContext(ctx)

	if settlementType != "" {
		q = q.Where("settlement_type = ?", settlementType)
	}
	if targetUserID != nil {
		q = q.Where("target_user_id = ?", *targetUserID)
	}
	if monthStr != "" {
		q = q.Where("month_str = ?", monthStr)
	}

	err := q.Find(&list).Error
	return list, err
}

func (r *SettlementRepository) SaveConfig(ctx context.Context, cfg *model.MonthlySettlementConfig) error {
	return r.db.WithContext(ctx).Clauses(clause.OnConflict{
		Columns: []clause.Column{
			{Name: "settlement_type"},
			{Name: "target_user_id"},
			{Name: "month_str"},
		},
		UpdateAll: true,
	}).Save(cfg).Error
}
