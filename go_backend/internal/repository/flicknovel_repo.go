package repository

import (
	"context"

	"go_backend/internal/model"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type FlicknovelRepository struct {
	db *gorm.DB
}

func NewFlicknovelRepository(db *gorm.DB) *FlicknovelRepository {
	return &FlicknovelRepository{db: db}
}

// BatchUpsertPromotions 批量更新推广链接
func (r *FlicknovelRepository) BatchUpsertPromotions(ctx context.Context, list []*model.FlicknovelPromotion) error {
	if len(list) == 0 {
		return nil
	}
	return r.db.WithContext(ctx).Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "promotion_id"}},
		UpdateAll: true,
	}).CreateInBatches(list, 100).Error
}

func (r *FlicknovelRepository) FindAllPromotionIDs(ctx context.Context) ([]string, error) {
	var ids []string
	err := r.db.WithContext(ctx).Model(&model.FlicknovelPromotion{}).Pluck("promotion_id", &ids).Error
	return ids, err
}

// BatchUpsertTemplates 批量更新充值模板
func (r *FlicknovelRepository) BatchUpsertTemplates(ctx context.Context, list []*model.FlicknovelRechargeTemplate) error {
	if len(list) == 0 {
		return nil
	}
	return r.db.WithContext(ctx).Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "template_id"}},
		UpdateAll: true,
	}).CreateInBatches(list, 100).Error
}

// BatchUpsertRelations 批量更新染色归因
func (r *FlicknovelRepository) BatchUpsertRelations(ctx context.Context, list []*model.FlicknovelRelation) error {
	if len(list) == 0 {
		return nil
	}
	return r.db.WithContext(ctx).Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "relation_id"}},
		UpdateAll: true,
	}).CreateInBatches(list, 200).Error
}
