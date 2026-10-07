package repository

import (
	"context"
	"strings"
	"time"

	"go_backend/internal/model"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type BenchmarkRepository struct {
	db *gorm.DB
}

func NewBenchmarkRepository(db *gorm.DB) *BenchmarkRepository {
	return &BenchmarkRepository{db: db}
}

func (r *BenchmarkRepository) FindBenchmarkCurve(
	ctx context.Context,
	dimType, dimValue string,
	periodDays int,
) ([]*model.LtvPredictBenchmark, error) {
	var list []*model.LtvPredictBenchmark
	err := r.db.WithContext(ctx).
		Where("dimension_type = ? AND dimension_value = ? AND sub_period_days = ?", dimType, dimValue, periodDays).
		Order("day_index asc").
		Find(&list).Error
	return list, err
}

// FindMatchingPagePeriodVersions 按落地页 ID 精准匹配该落地页下的生效订阅配置版本
func (r *BenchmarkRepository) FindMatchingPagePeriodVersions(
	ctx context.Context,
	landingPageID string,
	periodDays int,
	launchTime time.Time,
) ([]*model.SubscriptionConfigVersion, error) {
	if strings.TrimSpace(landingPageID) == "" {
		return nil, nil
	}
	var list []*model.SubscriptionConfigVersion
	err := r.db.WithContext(ctx).
		Where("landing_page_id = ? AND sub_period_days = ? AND effective_start_time <= ? AND (effective_end_time IS NULL OR effective_end_time >= ?)",
			landingPageID, periodDays, launchTime, launchTime).
		Order("version_num desc, effective_start_time desc, id desc").
		Find(&list).Error
	return list, err
}

// FindMatchingPlatformPeriodVersions 按平台代码匹配生效订阅配置版本
func (r *BenchmarkRepository) FindMatchingPlatformPeriodVersions(
	ctx context.Context,
	platformCode string,
	periodDays int,
	launchTime time.Time,
) ([]*model.SubscriptionConfigVersion, error) {
	pCode := strings.ToLower(strings.TrimSpace(platformCode))
	if pCode == "" || strings.EqualFold(pCode, "ALL") {
		return nil, nil
	}
	var list []*model.SubscriptionConfigVersion
	err := r.db.WithContext(ctx).
		Where("LOWER(platform_code) = ? AND sub_period_days = ? AND effective_start_time <= ? AND (effective_end_time IS NULL OR effective_end_time >= ?)",
			pCode, periodDays, launchTime, launchTime).
		Order("version_num desc, effective_start_time desc, id desc").
		Find(&list).Error
	return list, err
}

// FindMatchingPeriodVersions 全局兜底：按周期天数匹配（显式确定性排序：版本号降序 > 生效时间降序 > ID 降序）
func (r *BenchmarkRepository) FindMatchingPeriodVersions(
	ctx context.Context,
	periodDays int,
	launchTime time.Time,
) ([]*model.SubscriptionConfigVersion, error) {
	var list []*model.SubscriptionConfigVersion
	err := r.db.WithContext(ctx).
		Where("sub_period_days = ? AND effective_start_time <= ? AND (effective_end_time IS NULL OR effective_end_time >= ?)",
			periodDays, launchTime, launchTime).
		Order("version_num desc, effective_start_time desc, id desc").
		Find(&list).Error
	return list, err
}

func (r *BenchmarkRepository) BatchUpsertVersions(ctx context.Context, versions []*model.SubscriptionConfigVersion) error {
	if len(versions) == 0 {
		return nil
	}
	return r.db.WithContext(ctx).Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "id"}},
		UpdateAll: true,
	}).CreateInBatches(versions, 100).Error
}

func (r *BenchmarkRepository) BatchUpsertUserSubscriptionPeriods(ctx context.Context, periods []*model.UserSubscriptionPeriod) error {
	if len(periods) == 0 {
		return nil
	}
	return r.db.WithContext(ctx).CreateInBatches(periods, 100).Error
}

func (r *BenchmarkRepository) DeleteBenchmarksByDim(ctx context.Context, dimType, dimValue string, periodDays int) error {
	return r.db.WithContext(ctx).
		Where("dimension_type = ? AND dimension_value = ? AND sub_period_days = ?", dimType, dimValue, periodDays).
		Delete(&model.LtvPredictBenchmark{}).Error
}

func (r *BenchmarkRepository) BatchUpsertBenchmarks(ctx context.Context, list []*model.LtvPredictBenchmark) error {
	if len(list) == 0 {
		return nil
	}
	return r.db.WithContext(ctx).Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "id"}},
		UpdateAll: true,
	}).CreateInBatches(list, 100).Error
}
