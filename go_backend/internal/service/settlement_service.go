package service

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	"go_backend/internal/model"
	"go_backend/internal/pkg/timeutil"
	"go_backend/internal/repository"
	"go_backend/internal/service/dto"

	"github.com/shopspring/decimal"
)

type SettlementService struct {
	settleRepo *repository.SettlementRepository
	orderRepo  *repository.OrderRepository
	userRepo   *repository.UserRepository
	userSvc    *UserService
}

func NewSettlementService(
	settleRepo *repository.SettlementRepository,
	orderRepo *repository.OrderRepository,
	userRepo *repository.UserRepository,
	userSvc *UserService,
) *SettlementService {
	return &SettlementService{
		settleRepo: settleRepo,
		orderRepo:  orderRepo,
		userRepo:   userRepo,
		userSvc:    userSvc,
	}
}

// SaveSettlementConfig 保存或更新月度结算配置 (对应 Java saveSettlementConfig 严格对齐)
func (s *SettlementService) SaveSettlementConfig(ctx context.Context, req dto.MonthlySettlementSaveRequestDto) (*model.MonthlySettlementConfig, error) {
	if req.SettlementType == "" || req.MonthStr == "" {
		return nil, fmt.Errorf("结算类型与月份不能为空")
	}

	sType := strings.ToUpper(strings.TrimSpace(req.SettlementType))
	monthStr := strings.TrimSpace(req.MonthStr)
	var targetUserID *int64
	if sType == "USER_ACCOUNT" {
		targetUserID = req.TargetUserID
	}

	// 查出已有记录，若存在则更新字段，不存在则新建
	cfg, err := s.settleRepo.FindOneConfig(ctx, sType, targetUserID, monthStr)
	if err != nil || cfg == nil {
		cfg = &model.MonthlySettlementConfig{
			SettlementType:           sType,
			TargetUserID:             targetUserID,
			MonthStr:                 monthStr,
			SettledRefundAmount:      decimal.Zero,
			MonthSettledRefundAmount: decimal.Zero,
			CrossPeriodRefundAmount:  decimal.Zero,
			ShareRatio:               decimal.NewFromFloat(0.95),
			ChannelFeeRate:           decimal.NewFromFloat(0.07),
		}
	}

	if req.SettledRefundAmount.GreaterThanOrEqual(decimal.Zero) {
		cfg.SettledRefundAmount = req.SettledRefundAmount
	}
	if req.MonthSettledRefundAmount.GreaterThanOrEqual(decimal.Zero) {
		cfg.MonthSettledRefundAmount = req.MonthSettledRefundAmount
	}
	if req.CrossPeriodRefundAmount.GreaterThanOrEqual(decimal.Zero) {
		cfg.CrossPeriodRefundAmount = req.CrossPeriodRefundAmount
	}
	if req.ShareRatio.GreaterThan(decimal.Zero) {
		cfg.ShareRatio = req.ShareRatio
	}
	if req.ChannelFeeRate.GreaterThanOrEqual(decimal.Zero) {
		cfg.ChannelFeeRate = req.ChannelFeeRate
	}
	if req.Remark != "" {
		cfg.Remark = strings.TrimSpace(req.Remark)
	}
	cfg.UpdatedAt = time.Now()

	if err := s.settleRepo.SaveConfig(ctx, cfg); err != nil {
		return nil, err
	}
	return cfg, nil
}

type monthCalculationData struct {
	monthStr      string
	yearMonthTime time.Time
	totalRecharge decimal.Decimal
	totalRefund   decimal.Decimal
	totalOrders   int
	refundOrders  int
	cfg           *model.MonthlySettlementConfig
}

// GetMonthlySettlementList 获取月度结算列表 (对应 Java getMonthlySettlementList 严格100%对齐)
func (s *SettlementService) GetMonthlySettlementList(
	ctx context.Context,
	platformCode string,
	settlementType string,
	targetUserID *int64,
) ([]*dto.MonthlySettlementItemDto, error) {
	sType := strings.ToUpper(strings.TrimSpace(settlementType))
	if sType == "" {
		sType = "PLATFORM_ALL"
	}
	pCode := "ALL"
	if platformCode != "" && !strings.EqualFold(platformCode, "ALL") {
		pCode = strings.ToLower(strings.TrimSpace(platformCode))
	}

	// 1. 获取聚合统计明细 (按月度与落地页分组下推到数据库，彻底避免全表原始订单加载到堆内存引发 OOM)
	summaries, err := s.orderRepo.FindMonthlySettlementSummaries(ctx, pCode)
	if err != nil {
		return nil, err
	}

	// 2. 获取系统中已配置的落地页 ID 集合 (对应 Java allConfiguredPids)
	landingPages, _ := s.userRepo.FindLandingPages(ctx, pCode, 0)
	allConfiguredPids := make(map[string]bool)
	for _, lp := range landingPages {
		clean := strings.TrimSpace(lp.LandingPageID)
		if clean != "" && !strings.EqualFold(clean, "__EMPTY__") {
			allConfiguredPids[clean] = true
		}
	}

	// 3. 针对 USER_ACCOUNT 获取该用户的落地页集合（主账号自动聚合子账号，对应 Java userPids）
	userPids := make(map[string]bool)
	targetUsername := ""
	if sType == "USER_ACCOUNT" && targetUserID != nil && *targetUserID > 0 {
		if u, err := s.userRepo.FindByID(ctx, *targetUserID); err == nil && u != nil {
			targetUsername = u.Username
		} else {
			targetUsername = fmt.Sprintf("用户#%d", *targetUserID)
		}
		_, pids, err := s.userSvc.GetLandingPageConfigs(ctx, pCode, *targetUserID)
		if err == nil {
			for _, p := range pids {
				clean := strings.TrimSpace(p)
				if clean != "" {
					userPids[clean] = true
				}
			}
		}
	}

	// 4. 获取现有月份列表（该平台投放起始月份 至今所有月份，外加订单聚合中出现的月份）
	platformStartDate := model.GetLaunchStartDateForPlatform(pCode)
	startYm, _ := time.ParseInLocation("2006-01", platformStartDate[:7], timeutil.BeijingZone)
	today := time.Now().In(timeutil.BeijingZone)
	currentYm, _ := time.ParseInLocation("2006-01", today.Format("2006-01"), timeutil.BeijingZone)

	monthsSet := make(map[string]bool)
	cur := startYm
	for !cur.After(currentYm) {
		monthsSet[cur.Format("2006-01")] = true
		cur = cur.AddDate(0, 1, 0)
	}

	// 基于数据库聚合结果在内存做 O(Summaries) 单趟归并，彻底杜绝 O(Months * Orders) 双重循环
	type monthAggData struct {
		totalRechargeCents int64
		totalRefundCents   int64
		totalOrders        int
		refundOrders       int
	}
	monthDataMap := make(map[string]*monthAggData)

	for _, sum := range summaries {
		if sum == nil || sum.MonthStr == "" {
			continue
		}
		monthsSet[sum.MonthStr] = true

		pid := strings.TrimSpace(sum.LandingPageID)
		if sType == "PLATFORM_ALL" {
			// 全部通过
		} else if sType == "USER_ACCOUNT" {
			if pid == "" || !userPids[pid] {
				continue
			}
		} else if sType == "UNLINKED_PID" {
			if pid != "" && allConfiguredPids[pid] {
				continue
			}
		}

		agg := monthDataMap[sum.MonthStr]
		if agg == nil {
			agg = &monthAggData{}
			monthDataMap[sum.MonthStr] = agg
		}
		agg.totalRechargeCents += sum.TotalAmountCent
		agg.totalRefundCents += sum.RefundAmountCent
		agg.totalOrders += sum.TotalOrders
		agg.refundOrders += sum.RefundOrders
	}

	monthsList := make([]string, 0, len(monthsSet))
	for m := range monthsSet {
		monthsList = append(monthsList, m)
	}
	// 月份从新到旧倒序排序
	sort.Sort(sort.Reverse(sort.StringSlice(monthsList)))

	// 查询所有历史配置
	allConfigs, _ := s.settleRepo.FindConfigs(ctx, sType, targetUserID, "")
	configMap := make(map[string]*model.MonthlySettlementConfig)
	for _, c := range allConfigs {
		configMap[c.MonthStr] = c
	}

	// 第一阶段：统计每个月的充值与退款，并累加早于当前月份（ym < currentYm）的历史未结算退款之和
	sumHistoricalUnsettledRefund := decimal.Zero
	calcList := make([]*monthCalculationData, 0, len(monthsList))

	for _, monthStr := range monthsList {
		ym, _ := time.ParseInLocation("2006-01", monthStr, timeutil.BeijingZone)

		var totalRechargeCents int64
		var totalRefundCents int64
		totalOrders := 0
		refundOrders := 0

		if agg, ok := monthDataMap[monthStr]; ok && agg != nil {
			totalRechargeCents = agg.totalRechargeCents
			totalRefundCents = agg.totalRefundCents
			totalOrders = agg.totalOrders
			refundOrders = agg.refundOrders
		}

		totalRecharge := decimal.NewFromInt(totalRechargeCents).DivRound(decimal.NewFromInt(100), 2)
		totalRefund := decimal.NewFromInt(totalRefundCents).DivRound(decimal.NewFromInt(100), 2)
		cfg := configMap[monthStr]

		mcd := &monthCalculationData{
			monthStr:      monthStr,
			yearMonthTime: ym,
			totalRecharge: totalRecharge,
			totalRefund:   totalRefund,
			totalOrders:   totalOrders,
			refundOrders:  refundOrders,
			cfg:           cfg,
		}
		calcList = append(calcList, mcd)

		// 累加历史月份（早于当前月）的未结算退款：未结算退款 = 累计退款 - 已结算退款
		if ym.Before(currentYm) {
			settledRefund := decimal.Zero
			if cfg != nil {
				settledRefund = cfg.SettledRefundAmount
			}
			unsettledRefund := totalRefund.Sub(settledRefund)
			sumHistoricalUnsettledRefund = sumHistoricalUnsettledRefund.Add(unsettledRefund)
		}
	}

	// 第二阶段：组装月度结算结果 DTO
	resultList := make([]*dto.MonthlySettlementItemDto, 0, len(calcList))

	for _, mcd := range calcList {
		monthStr := mcd.monthStr
		ym := mcd.yearMonthTime
		totalRecharge := mcd.totalRecharge
		totalRefund := mcd.totalRefund
		cfg := mcd.cfg

		settledRefundAmount := decimal.Zero
		monthSettledRefundAmount := decimal.Zero
		crossPeriodRefundAmount := decimal.Zero
		shareRatio := decimal.NewFromFloat(0.9500)
		channelFeeRate := decimal.NewFromFloat(0.0700)
		remark := ""
		var updatedAt *time.Time

		if cfg != nil {
			settledRefundAmount = cfg.SettledRefundAmount
			monthSettledRefundAmount = cfg.MonthSettledRefundAmount
			crossPeriodRefundAmount = cfg.CrossPeriodRefundAmount
			shareRatio = cfg.ShareRatio
			channelFeeRate = cfg.ChannelFeeRate
			remark = cfg.Remark
			t := cfg.UpdatedAt
			updatedAt = &t
		}

		// 当月份自动填写逻辑 (对应 Java)：
		// 当月结算退款 = 累计退款
		// 跨周期退款 = 历史月份未结算退款之和
		if ym.Equal(currentYm) {
			monthSettledRefundAmount = totalRefund
			crossPeriodRefundAmount = sumHistoricalUnsettledRefund.Round(2)
		}

		// 计算未结算退款 = 累计退款 - 已结算退款
		unsettledRefundAmount := totalRefund.Sub(settledRefundAmount)

		// 计算有效结算基数 = 累计充值 - 当月结算退款 - 跨周期退款
		effectiveBaseAmount := totalRecharge.Sub(monthSettledRefundAmount).Sub(crossPeriodRefundAmount)

		// 计算最终结算金额 = 有效结算基数 * 分成比例 * (1 - 渠道费率)
		finalSettlementAmount := decimal.Zero
		if effectiveBaseAmount.GreaterThan(decimal.Zero) {
			netFactor := decimal.NewFromInt(1).Sub(channelFeeRate)
			finalSettlementAmount = effectiveBaseAmount.Mul(shareRatio).Mul(netFactor).Round(2)
		} else {
			finalSettlementAmount = effectiveBaseAmount.Mul(shareRatio).Round(2)
		}

		// 退款率
		refundRate := "0.00%"
		if totalRecharge.GreaterThan(decimal.Zero) {
			rate := totalRefund.DivRound(totalRecharge, 4).Mul(decimal.NewFromInt(100))
			refundRate = fmt.Sprintf("%.2f%%", rate.InexactFloat64())
		}

		item := &dto.MonthlySettlementItemDto{
			MonthStr:                 monthStr,
			SettlementType:           sType,
			TargetUserID:             targetUserID,
			TargetUsername:           targetUsername,
			TotalRecharge:            totalRecharge,
			TotalRefund:              totalRefund,
			SettledRefundAmount:      settledRefundAmount,
			MonthSettledRefundAmount: monthSettledRefundAmount,
			UnsettledRefundAmount:    unsettledRefundAmount,
			CrossPeriodRefundAmount:  crossPeriodRefundAmount,
			ShareRatio:               shareRatio,
			ChannelFeeRate:           channelFeeRate,
			EffectiveBaseAmount:      effectiveBaseAmount,
			FinalSettlementAmount:    finalSettlementAmount,
			RefundRate:               refundRate,
			TotalOrders:              mcd.totalOrders,
			RefundOrders:             mcd.refundOrders,
			Remark:                   remark,
			UpdatedAt:                updatedAt,
		}

		resultList = append(resultList, item)
	}

	return resultList, nil
}

