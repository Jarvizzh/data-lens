package repository

import (
	"context"
	"strings"

	"go_backend/internal/model"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type OrderRepository struct {
	db *gorm.DB
}

func NewOrderRepository(db *gorm.DB) *OrderRepository {
	return &OrderRepository{db: db}
}

// BatchUpsert 批量插入或更新订单明细
func (r *OrderRepository) BatchUpsert(ctx context.Context, orders []*model.RawOrder) error {
	if len(orders) == 0 {
		return nil
	}

	// 500 条一批进行插入，基于唯一键 uk_platform_order 更新
	return r.db.WithContext(ctx).Clauses(clause.OnConflict{
		Columns: []clause.Column{{Name: "platform_code"}, {Name: "order_id"}},
		DoUpdates: clause.AssignmentColumns([]string{
			"member_id", "landing_page_id", "register_time_bj", "register_time_et",
			"register_date_et", "register_time_utc", "register_date_utc",
			"pay_time_bj", "pay_time_et", "pay_date_et", "pay_time_utc", "pay_date_utc",
			"order_amount_cent", "order_amount_usd", "is_subs", "renew_type",
			"pay_state", "refund_status", "raw_payload",
		}),
	}).CreateInBatches(orders, 500).Error
}

// FindOrdersForLtvCalculation 按落地页与注册时间范围查询订单
func (r *OrderRepository) FindOrdersForLtvCalculation(
	ctx context.Context,
	platformCode string,
	landingPageIDs []string,
	startDate, endDate string,
) ([]*model.RawOrder, error) {
	if landingPageIDs != nil && len(landingPageIDs) == 0 {
		return []*model.RawOrder{}, nil
	}
	var orders []*model.RawOrder
	q := r.db.WithContext(ctx).Where("pay_state = 1")

	if platformCode != "" && !strings.EqualFold(platformCode, "ALL") {
		q = q.Where("platform_code = ?", strings.ToLower(platformCode))
	}
	if len(landingPageIDs) > 0 {
		q = q.Where("landing_page_id IN ?", landingPageIDs)
	}
	if startDate != "" {
		q = q.Where("register_date_et >= ?", startDate)
	}
	if endDate != "" {
		q = q.Where("register_date_et <= ?", endDate)
	}

	err := q.Order("register_time_et asc, pay_time_et asc").Find(&orders).Error
	return orders, err
}

// FindDistinctLandingPageIDs 获取指定平台所有存在订单的落地页/推广ID
func (r *OrderRepository) FindDistinctLandingPageIDs(ctx context.Context, platformCode string) ([]string, error) {
	var ids []string
	q := r.db.WithContext(ctx).Model(&model.RawOrder{}).Where("landing_page_id IS NOT NULL AND landing_page_id != ''")
	if platformCode != "" && !strings.EqualFold(platformCode, "ALL") {
		q = q.Where("platform_code = ?", strings.ToLower(platformCode))
	}
	err := q.Distinct().Pluck("landing_page_id", &ids).Error
	return ids, err
}
