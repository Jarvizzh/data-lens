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

func (r *FlicknovelRepository) CountPromotions(ctx context.Context) (int64, error) {
	var count int64
	err := r.db.WithContext(ctx).Model(&model.FlicknovelPromotion{}).Count(&count).Error
	return count, err
}

func (r *FlicknovelRepository) CountTemplates(ctx context.Context) (int64, error) {
	var count int64
	err := r.db.WithContext(ctx).Model(&model.FlicknovelRechargeTemplate{}).Count(&count).Error
	return count, err
}

func (r *FlicknovelRepository) FindAllPromotions(ctx context.Context) ([]*model.FlicknovelPromotion, error) {
	var list []*model.FlicknovelPromotion
	err := r.db.WithContext(ctx).Find(&list).Error
	return list, err
}

func (r *FlicknovelRepository) FindAllTemplates(ctx context.Context) ([]*model.FlicknovelRechargeTemplate, error) {
	var list []*model.FlicknovelRechargeTemplate
	err := r.db.WithContext(ctx).Find(&list).Error
	return list, err
}

func (r *FlicknovelRepository) FindPromotionByID(ctx context.Context, promotionID string) (*model.FlicknovelPromotion, error) {
	var p model.FlicknovelPromotion
	err := r.db.WithContext(ctx).Where("promotion_id = ?", promotionID).First(&p).Error
	if err != nil {
		return nil, err
	}
	return &p, nil
}

func (r *FlicknovelRepository) FindTemplateByID(ctx context.Context, templateID string) (*model.FlicknovelRechargeTemplate, error) {
	var t model.FlicknovelRechargeTemplate
	err := r.db.WithContext(ctx).Where("template_id = ?", templateID).First(&t).Error
	if err != nil {
		return nil, err
	}
	return &t, nil
}

func (r *FlicknovelRepository) FindRelationByID(ctx context.Context, relationID string) (*model.FlicknovelRelation, error) {
	var rel model.FlicknovelRelation
	err := r.db.WithContext(ctx).Where("relation_id = ?", relationID).First(&rel).Error
	if err != nil {
		return nil, err
	}
	return &rel, nil
}

func (r *FlicknovelRepository) FindRelationsByIDs(ctx context.Context, relationIDs []string) (map[string]*model.FlicknovelRelation, error) {
	res := make(map[string]*model.FlicknovelRelation)
	if len(relationIDs) == 0 {
		return res, nil
	}
	chunkSize := 500
	for i := 0; i < len(relationIDs); i += chunkSize {
		end := i + chunkSize
		if end > len(relationIDs) {
			end = len(relationIDs)
		}
		var list []*model.FlicknovelRelation
		err := r.db.WithContext(ctx).Where("relation_id IN ?", relationIDs[i:end]).Find(&list).Error
		if err != nil {
			return res, err
		}
		for _, item := range list {
			res[item.RelationID] = item
		}
	}
	return res, nil
}
