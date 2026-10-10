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
	repo := &SettlementRepository{db: db}
	repo.ensureSchemaMigrated()
	return repo
}

// ensureSchemaMigrated 确保 monthly_settlement_config 具备 platform_code 维度及相应唯一索引
func (r *SettlementRepository) ensureSchemaMigrated() {
	if r.db == nil {
		return
	}
	sqlDB, err := r.db.DB()
	if err != nil {
		return
	}

	// 1. 检查是否存在 platform_code 列
	var colCount int
	err = sqlDB.QueryRow("SELECT COUNT(1) FROM information_schema.columns WHERE table_schema = DATABASE() AND table_name = 'monthly_settlement_config' AND column_name = 'platform_code'").Scan(&colCount)
	if err == nil && colCount == 0 {
		_, _ = sqlDB.Exec("ALTER TABLE monthly_settlement_config ADD COLUMN platform_code VARCHAR(32) NOT NULL DEFAULT 'rocnovel' AFTER id")
	}

	// 2. 检查旧索引 uk_settle_type_user_month 并删除
	var oldIdxCount int
	err = sqlDB.QueryRow("SELECT COUNT(1) FROM information_schema.statistics WHERE table_schema = DATABASE() AND table_name = 'monthly_settlement_config' AND index_name = 'uk_settle_type_user_month'").Scan(&oldIdxCount)
	if err == nil && oldIdxCount > 0 {
		_, _ = sqlDB.Exec("ALTER TABLE monthly_settlement_config DROP INDEX uk_settle_type_user_month")
	}

	// 3. 检查新唯一索引 uk_settle_plat_type_user_month 并创建
	var newIdxCount int
	err = sqlDB.QueryRow("SELECT COUNT(1) FROM information_schema.statistics WHERE table_schema = DATABASE() AND table_name = 'monthly_settlement_config' AND index_name = 'uk_settle_plat_type_user_month'").Scan(&newIdxCount)
	if err == nil && newIdxCount == 0 {
		_, _ = sqlDB.Exec("ALTER TABLE monthly_settlement_config ADD UNIQUE INDEX uk_settle_plat_type_user_month (platform_code, settlement_type, target_user_id, month_str)")
	}
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

