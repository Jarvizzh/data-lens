package repository

import (
	"context"
	"strings"

	"go_backend/internal/model"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type RechargeDistributionRepository struct {
	db *gorm.DB
}

func NewRechargeDistributionRepository(db *gorm.DB) *RechargeDistributionRepository {
	return &RechargeDistributionRepository{db: db}
}

func (r *RechargeDistributionRepository) BatchUpsert(ctx context.Context, list []*model.DailyRechargeDistribution) error {
	if len(list) == 0 {
		return nil
	}
	return r.db.WithContext(ctx).Clauses(clause.OnConflict{
		Columns: []clause.Column{
			{Name: "platform_code"},
			{Name: "user_id"},
			{Name: "date"},
		},
		UpdateAll: true,
	}).CreateInBatches(list, 100).Error
}

func (r *RechargeDistributionRepository) FindByFilter(
	ctx context.Context,
	platformCode string,
	userIDs []int64,
	startDate, endDate string,
) ([]*model.DailyRechargeDistribution, error) {
	var list []*model.DailyRechargeDistribution
	q := r.db.WithContext(ctx)

	targetPlatform := "ALL"
	if platformCode != "" && !strings.EqualFold(platformCode, "ALL") {
		targetPlatform = strings.ToLower(platformCode)
	}
	q = q.Where("platform_code = ?", targetPlatform)

	if len(userIDs) > 0 {
		q = q.Where("user_id IN ?", userIDs)
	}
	if startDate != "" {
		q = q.Where("date >= ?", startDate)
	}
	if endDate != "" {
		q = q.Where("date <= ?", endDate)
	}

	err := q.Order("date desc, user_id asc").Find(&list).Error
	if err == nil {
		for _, d := range list {
			if len(d.Date) >= 10 {
				d.Date = d.Date[:10]
			}
		}
	}
	return list, err
}
