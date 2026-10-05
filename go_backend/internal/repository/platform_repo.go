package repository

import (
	"context"

	"go_backend/internal/model"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type PlatformRepository struct {
	db *gorm.DB
}

func NewPlatformRepository(db *gorm.DB) *PlatformRepository {
	return &PlatformRepository{db: db}
}

func (r *PlatformRepository) FindAll(ctx context.Context) ([]*model.PlatformConfig, error) {
	var list []*model.PlatformConfig
	err := r.db.WithContext(ctx).Find(&list).Error
	return list, err
}

func (r *PlatformRepository) FindByCode(ctx context.Context, platformCode string) (*model.PlatformConfig, error) {
	var cfg model.PlatformConfig
	err := r.db.WithContext(ctx).Where("platform_code = ?", platformCode).First(&cfg).Error
	if err != nil {
		return nil, err
	}
	return &cfg, nil
}

func (r *PlatformRepository) SavePlatform(ctx context.Context, cfg *model.PlatformConfig) error {
	return r.db.WithContext(ctx).Save(cfg).Error
}

// SystemConfig
func (r *PlatformRepository) GetSystemConfig(ctx context.Context, key string) (string, error) {
	var cfg model.SystemConfig
	err := r.db.WithContext(ctx).Where("config_key = ?", key).First(&cfg).Error
	if err != nil {
		return "", err
	}
	return cfg.ConfigValue, nil
}

func (r *PlatformRepository) SetSystemConfig(ctx context.Context, key, val string) error {
	cfg := model.SystemConfig{
		ConfigKey:   key,
		ConfigValue: val,
	}
	return r.db.WithContext(ctx).Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "config_key"}},
		DoUpdates: clause.AssignmentColumns([]string{"config_value", "updated_at"}),
	}).Save(&cfg).Error
}
