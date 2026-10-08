package repository

import (
	"context"
	"strings"

	"go_backend/internal/model"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type LtvStatRepository struct {
	db *gorm.DB
}

func NewLtvStatRepository(db *gorm.DB) *LtvStatRepository {
	return &LtvStatRepository{db: db}
}

// BatchUpsertLtvDailyStat 批量插入或根据复合主键更新 LtvDailyStat
func (r *LtvStatRepository) BatchUpsertLtvDailyStat(ctx context.Context, stats []*model.LtvDailyStat) error {
	if len(stats) == 0 {
		return nil
	}

	return r.db.WithContext(ctx).Clauses(clause.OnConflict{
		Columns: []clause.Column{
			{Name: "platform_code"},
			{Name: "user_id"},
			{Name: "launch_date"},
		},
		UpdateAll: true,
	}).CreateInBatches(stats, 100).Error
}

// DeleteAndBatchInsert 原子删除指定平台和用户的数据并批量插入新数据
func (r *LtvStatRepository) DeleteAndBatchInsert(ctx context.Context, platformCode string, userID int64, stats []*model.LtvDailyStat) error {
	targetPlatform := "ALL"
	if platformCode != "" && !strings.EqualFold(platformCode, "ALL") {
		targetPlatform = strings.ToLower(platformCode)
	}

	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("platform_code = ? AND user_id = ?", targetPlatform, userID).Delete(&model.LtvDailyStat{}).Error; err != nil {
			return err
		}
		if len(stats) == 0 {
			return nil
		}
		return tx.CreateInBatches(stats, 100).Error
	})
}

// FindStatsByFilter 多条件查询 LtvDailyStat (按日期正序)
func (r *LtvStatRepository) FindStatsByFilter(
	ctx context.Context,
	platformCode string,
	userIDs []int64,
	startDate, endDate string,
) ([]*model.LtvDailyStat, error) {
	var stats []*model.LtvDailyStat
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
		q = q.Where("launch_date >= ?", startDate)
	}
	if endDate != "" {
		q = q.Where("launch_date <= ?", endDate)
	}

	err := q.Order("launch_date asc, user_id asc").Find(&stats).Error
	if err == nil {
		for _, s := range stats {
			if len(s.LaunchDate) >= 10 {
				s.LaunchDate = s.LaunchDate[:10]
			}
		}
	}
	return stats, err
}

// SaveLaunchConfig 保存或更新投放消耗与备注
func (r *LtvStatRepository) SaveLaunchConfig(ctx context.Context, cfg *model.LtvLaunchConfig) error {
	return r.db.WithContext(ctx).Clauses(clause.OnConflict{
		Columns: []clause.Column{
			{Name: "platform_code"},
			{Name: "user_id"},
			{Name: "launch_date"},
		},
		DoUpdates: clause.AssignmentColumns([]string{"spend", "remark", "updated_at"}),
	}).Save(cfg).Error
}

// FindLaunchConfigs 查询投放消耗配置
func (r *LtvStatRepository) FindLaunchConfigs(
	ctx context.Context,
	platformCode string,
	userIDs []int64,
	startDate, endDate string,
) ([]*model.LtvLaunchConfig, error) {
	var configs []*model.LtvLaunchConfig
	q := r.db.WithContext(ctx)

	if platformCode != "" && !strings.EqualFold(platformCode, "ALL") {
		q = q.Where("platform_code = ?", strings.ToLower(platformCode))
	}
	if len(userIDs) > 0 {
		q = q.Where("user_id IN ?", userIDs)
	}
	if startDate != "" {
		q = q.Where("launch_date >= ?", startDate)
	}
	if endDate != "" {
		q = q.Where("launch_date <= ?", endDate)
	}

	err := q.Order("launch_date asc, user_id asc").Find(&configs).Error
	if err == nil {
		for _, c := range configs {
			if len(c.LaunchDate) >= 10 {
				c.LaunchDate = c.LaunchDate[:10]
			}
		}
	}
	return configs, err
}
