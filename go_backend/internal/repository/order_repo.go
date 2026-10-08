package repository

import (
	"context"
	"errors"
	"strings"
	"time"

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

	q = applyPlatformFilter(q, platformCode)
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
	if err == nil {
		for _, o := range orders {
			if len(o.RegisterDateET) >= 10 {
				o.RegisterDateET = o.RegisterDateET[:10]
			}
			if len(o.RegisterDateUTC) >= 10 {
				o.RegisterDateUTC = o.RegisterDateUTC[:10]
			}
			if len(o.PayDateET) >= 10 {
				o.PayDateET = o.PayDateET[:10]
			}
			if len(o.PayDateUTC) >= 10 {
				o.PayDateUTC = o.PayDateUTC[:10]
			}
		}
	}
	return orders, err
}

// FindDistinctLandingPageIDs 获取指定平台所有存在订单的落地页/推广ID
func (r *OrderRepository) FindDistinctLandingPageIDs(ctx context.Context, platformCode string) ([]string, error) {
	var ids []string
	q := r.db.WithContext(ctx).Model(&model.RawOrder{}).Where("landing_page_id IS NOT NULL AND landing_page_id != ''")
	q = applyPlatformFilter(q, platformCode)
	err := q.Distinct().Pluck("landing_page_id", &ids).Error
	return ids, err
}

// FindOrdersByLandingPageIDs 查询指定落地页的所有有效订单 (不限注册时间)
func (r *OrderRepository) FindOrdersByLandingPageIDs(
	ctx context.Context,
	platformCode string,
	landingPageIDs []string,
) ([]*model.RawOrder, error) {
	if landingPageIDs != nil && len(landingPageIDs) == 0 {
		return []*model.RawOrder{}, nil
	}
	var orders []*model.RawOrder
	q := r.db.WithContext(ctx).Where("pay_state = 1")
	q = applyPlatformFilter(q, platformCode)
	if len(landingPageIDs) > 0 {
		q = q.Where("landing_page_id IN ?", landingPageIDs)
	}
	err := q.Order("register_time_bj asc, pay_time_bj asc").Find(&orders).Error
	return orders, err
}

// FindAllValidOrders 查询全量有效订单 (对应 Java rawOrderRepository.findAll())
func (r *OrderRepository) FindAllValidOrders(ctx context.Context) ([]*model.RawOrder, error) {
	var orders []*model.RawOrder
	err := r.db.WithContext(ctx).Where("pay_state = 1").
		Order("register_time_bj asc, pay_time_bj asc").Find(&orders).Error
	return orders, err
}

// FindValidOrdersByPlatform 按平台查询全量有效订单 (对应 Java rawOrderRepository.findByPlatformCode(platformCode))
func (r *OrderRepository) FindValidOrdersByPlatform(ctx context.Context, platformCode string) ([]*model.RawOrder, error) {
	var orders []*model.RawOrder
	q := r.db.WithContext(ctx).Where("pay_state = 1")
	q = applyPlatformFilter(q, platformCode)
	err := q.Order("register_time_bj asc, pay_time_bj asc").Find(&orders).Error
	return orders, err
}

// FindRecentValidOrdersByRegisterDate 查询指定生效注册日期以来的有效订单 (用于 LTV 预测基准库重算)
func (r *OrderRepository) FindRecentValidOrdersByRegisterDate(ctx context.Context, cutoffDate string) ([]*model.RawOrder, error) {
	var orders []*model.RawOrder
	err := r.db.WithContext(ctx).
		Where("pay_state = 1 AND register_date_et >= ?", cutoffDate).
		Order("register_date_et asc, pay_date_et asc").
		Find(&orders).Error
	return orders, err
}

// FindSubscriptionPeriodsMap 批量查询订阅用户的周期字典

func (r *OrderRepository) FindSubscriptionPeriodsMap(ctx context.Context, memberIDs []string) (map[string]int, error) {
	periodMap := make(map[string]int)
	if len(memberIDs) == 0 {
		return periodMap, nil
	}
	chunkSize := 500
	for i := 0; i < len(memberIDs); i += chunkSize {
		end := i + chunkSize
		if end > len(memberIDs) {
			end = len(memberIDs)
		}
		var list []*model.UserSubscriptionPeriod
		err := r.db.WithContext(ctx).Select("member_id, sub_period_days").
			Where("member_id IN ?", memberIDs[i:end]).Find(&list).Error
		if err != nil {
			return periodMap, err
		}
		for _, item := range list {
			mID := strings.TrimSpace(item.MemberID)
			if mID != "" && item.SubPeriodDays > 0 {
				periodMap[mID] = item.SubPeriodDays
			}
		}
	}
	return periodMap, nil
}

// FindHistoryOrdersByMemberIDs 批量查询指定用户的历史订单 (用于构建用户画像最早支付/注册时间)
func (r *OrderRepository) FindHistoryOrdersByMemberIDs(ctx context.Context, platformCode string, memberIDs []string) ([]*model.RawOrder, error) {
	if len(memberIDs) == 0 {
		return nil, nil
	}
	var orders []*model.RawOrder
	chunkSize := 500
	pCode := strings.ToLower(strings.TrimSpace(platformCode))

	for i := 0; i < len(memberIDs); i += chunkSize {
		end := i + chunkSize
		if end > len(memberIDs) {
			end = len(memberIDs)
		}
		var chunk []*model.RawOrder
		q := r.db.WithContext(ctx).Where("member_id IN ?", memberIDs[i:end])
		q = applyPlatformFilter(q, pCode)
		if err := q.Find(&chunk).Error; err != nil {
			return orders, err
		}
		orders = append(orders, chunk...)
	}
	return orders, nil
}

// FindUserSubscriptionPeriod 查询指定用户的订阅周期记录
func (r *OrderRepository) FindUserSubscriptionPeriod(ctx context.Context, memberID string) (*model.UserSubscriptionPeriod, error) {
	var period model.UserSubscriptionPeriod
	err := r.db.WithContext(ctx).Where("member_id = ?", memberID).First(&period).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &period, nil
}

// SaveUserSubscriptionPeriod 保存或更新用户订阅周期记录
func (r *OrderRepository) SaveUserSubscriptionPeriod(ctx context.Context, period *model.UserSubscriptionPeriod) error {
	if period == nil || strings.TrimSpace(period.MemberID) == "" {
		return nil
	}
	return r.db.WithContext(ctx).Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "member_id"}},
		DoUpdates: clause.AssignmentColumns([]string{"landing_page_id", "subscribe_config_id", "sub_period_days", "first_price_cent", "renew_price_cent", "updated_at"}),
	}).Create(period).Error
}

// FindMatchingFirstPriceVersions 按落地页、首订价格和生效时间匹配订阅版本
func (r *OrderRepository) FindMatchingFirstPriceVersions(ctx context.Context, landingPageID string, priceCent int, targetTime time.Time) ([]*model.SubscriptionConfigVersion, error) {
	var list []*model.SubscriptionConfigVersion
	err := r.db.WithContext(ctx).
		Where("landing_page_id = ? AND first_price_cent = ? AND effective_start_time <= ? AND (effective_end_time IS NULL OR effective_end_time >= ?)",
			landingPageID, priceCent, targetTime, targetTime).
		Order("version_num desc, effective_start_time desc").
		Find(&list).Error
	return list, err
}

// FindMatchingPageVersions 按落地页和生效时间匹配订阅版本
func (r *OrderRepository) FindMatchingPageVersions(ctx context.Context, landingPageID string, targetTime time.Time) ([]*model.SubscriptionConfigVersion, error) {
	var list []*model.SubscriptionConfigVersion
	err := r.db.WithContext(ctx).
		Where("landing_page_id = ? AND effective_start_time <= ? AND (effective_end_time IS NULL OR effective_end_time >= ?)",
			landingPageID, targetTime, targetTime).
		Order("version_num desc, effective_start_time desc").
		Find(&list).Error
	return list, err
}

// FindAllSubscriptionVersions 获取所有订阅配置版本
func (r *OrderRepository) FindAllSubscriptionVersions(ctx context.Context) ([]*model.SubscriptionConfigVersion, error) {
	var list []*model.SubscriptionConfigVersion
	err := r.db.WithContext(ctx).Find(&list).Error
	return list, err
}

// FindLatestVersionByPageAndProduct 查询落地页和产品ID对应的最新版本
func (r *OrderRepository) FindLatestVersionByPageAndProduct(ctx context.Context, landingPageID, productID string) (*model.SubscriptionConfigVersion, error) {
	var v model.SubscriptionConfigVersion
	err := r.db.WithContext(ctx).
		Where("landing_page_id = ? AND product_id = ?", landingPageID, productID).
		Order("version_num desc").
		First(&v).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &v, nil
}

// SaveSubscriptionVersion 保存或更新单个订阅配置版本
func (r *OrderRepository) SaveSubscriptionVersion(ctx context.Context, v *model.SubscriptionConfigVersion) error {
	if v == nil {
		return nil
	}
	v.UpdatedAt = time.Now()
	return r.db.WithContext(ctx).Save(v).Error
}

func applyPlatformFilter(q *gorm.DB, platformCode string) *gorm.DB {
	if model.IsAllPlatforms(platformCode) {
		return q
	}
	if model.IsRocnovel(platformCode) {
		return q.Where("(platform_code = ? OR platform_code IS NULL OR platform_code = '' OR platform_code = ?)", model.PlatformRocnovel, model.PlatformAll)
	}
	return q.Where("platform_code = ?", strings.ToLower(strings.TrimSpace(platformCode)))
}


