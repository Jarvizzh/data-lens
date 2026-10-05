package repository

import (
	"context"
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

func (r *BenchmarkRepository) FindMatchingPeriodVersions(
	ctx context.Context,
	periodDays int,
	launchTime time.Time,
) ([]*model.SubscriptionConfigVersion, error) {
	var list []*model.SubscriptionConfigVersion
	err := r.db.WithContext(ctx).
		Where("sub_period_days = ? AND effective_start_time <= ? AND (effective_end_time IS NULL OR effective_end_time >= ?)",
			periodDays, launchTime, launchTime).
		Order("effective_start_time desc").
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
