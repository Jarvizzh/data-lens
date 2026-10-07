package service

import (
	"context"
	"sort"
	"strings"
	"time"

	"go_backend/internal/model"
	"go_backend/internal/pkg/timeutil"
	"go_backend/internal/repository"
	"go_backend/internal/service/dto"

	"github.com/shopspring/decimal"
)

type RechargeStatService struct {
	orderRepo        *repository.OrderRepository
	userRepo         *repository.UserRepository
	userSvc          *UserService
	rechargeDistRepo *repository.RechargeDistributionRepository
}

func NewRechargeStatService(
	orderRepo *repository.OrderRepository,
	userRepo *repository.UserRepository,
	userSvc *UserService,
	rechargeDistRepo *repository.RechargeDistributionRepository,
) *RechargeStatService {
	return &RechargeStatService{
		orderRepo:        orderRepo,
		userRepo:         userRepo,
		userSvc:          userSvc,
		rechargeDistRepo: rechargeDistRepo,
	}
}

// GetTodayForPlatform 对应 Java DailyRechargeStatService.getTodayForPlatform
func GetTodayForPlatform(platformCode string) string {
	if strings.EqualFold(platformCode, "flicknovel") {
		return timeutil.GetTodayUtc()
	}
	return timeutil.GetTodayCst()
}

// GetOrdersFilteredForUser 对应 Java LtvStatService.getOrdersFilteredForUser
func (s *RechargeStatService) GetOrdersFilteredForUser(ctx context.Context, platformCode string, userID int64) ([]*model.RawOrder, error) {
	if userID <= 0 {
		userID = 1
	}
	pCode := strings.ToLower(strings.TrimSpace(platformCode))
	if pCode == "" {
		pCode = "all"
	}

	_, lpIDs, err := s.userSvc.GetLandingPageConfigs(ctx, pCode, userID)
	if err != nil {
		return nil, err
	}
	if len(lpIDs) == 0 {
		return []*model.RawOrder{}, nil
	}

	// 去重并清除空白
	cleanIDs := make([]string, 0, len(lpIDs))
	seen := make(map[string]bool)
	for _, id := range lpIDs {
		c := strings.TrimSpace(id)
		if c != "" && !seen[c] {
			seen[c] = true
			cleanIDs = append(cleanIDs, c)
		}
	}
	if len(cleanIDs) == 0 {
		return []*model.RawOrder{}, nil
	}

	return s.orderRepo.FindOrdersByLandingPageIDs(ctx, pCode, cleanIDs)
}

// CalculateDailyDistributionForUserDirect 计算并持久化指定用户和平台的每日充值分布表 (仅针对指定用户本身，不触发父级主账号，对应 Java calculateDailyDistributionStatsForUserDirect)
func (s *RechargeStatService) CalculateDailyDistributionForUserDirect(ctx context.Context, platformCode string, userID int64) error {
	if userID <= 0 {
		userID = 1
	}
	pCode := strings.ToLower(strings.TrimSpace(platformCode))
	if pCode == "" {
		pCode = "all"
	}
	targetPlatform := "ALL"
	if pCode != "all" {
		targetPlatform = pCode
	}

	today := GetTodayForPlatform(targetPlatform)
	platformStartDate := model.GetLaunchStartDateForPlatform(targetPlatform)

	orders, err := s.GetOrdersFilteredForUser(ctx, targetPlatform, userID)
	if err != nil {
		return err
	}

	ordersByPayDate := make(map[string][]*model.RawOrder)
	for _, o := range orders {
		payDate := GetOrderPayDateForPlatform(targetPlatform, o)
		if payDate != "" && payDate >= platformStartDate {
			ordersByPayDate[payDate] = append(ordersByPayDate[payDate], o)
		}
	}

	startDateTime, _ := time.ParseInLocation(timeutil.DateLayout, platformStartDate, timeutil.BeijingZone)
	endDateTime, _ := time.ParseInLocation(timeutil.DateLayout, today, timeutil.BeijingZone)

	statList := make([]*model.DailyRechargeDistribution, 0)
	currDate := startDateTime
	for !currDate.After(endDateTime) {
		dateStr := currDate.Format(timeutil.DateLayout)
		dayOrders := ordersByPayDate[dateStr]
		stat := s.CalculateSingleDayDistribution(targetPlatform, userID, dateStr, dayOrders)
		statList = append(statList, stat)
		currDate = currDate.AddDate(0, 0, 1)
	}

	// 与 Java 一致：先按 platformCode 和 userId 清空旧记录，再重新写入
	return s.rechargeDistRepo.ReplaceAllForUser(ctx, targetPlatform, userID, statList)
}

// CalculateDailyDistributionForUser 统计指定用户的每日充值分布 (兼容调用)
func (s *RechargeStatService) CalculateDailyDistributionForUser(ctx context.Context, platformCode string, userID int64) error {
	return s.CalculateDailyDistributionForUserDirect(ctx, platformCode, userID)
}

// CalculateSingleDayDistribution 计算单日充值分布数据 (对应 Java calculateSingleDayDistribution 严格对齐)
func (s *RechargeStatService) CalculateSingleDayDistribution(
	platformCode string,
	userID int64,
	payDate string,
	dayOrders []*model.RawOrder,
) *model.DailyRechargeDistribution {
	pCode := "ALL"
	if platformCode != "" && !strings.EqualFold(platformCode, "ALL") {
		pCode = strings.ToLower(platformCode)
	}

	stat := &model.DailyRechargeDistribution{
		PlatformCode:       pCode,
		UserID:             userID,
		Date:               payDate,
		TotalRecharge:      decimal.Zero,
		SingleRecharge:     decimal.Zero,
		SubsRecharge:       decimal.Zero,
		TotalPaidUsers:     0,
		SinglePaidUsers:    0,
		SubsPaidUsers:      0,
		NewRecharge:        decimal.Zero,
		NewRechargeRatio:   decimal.Zero,
		NewArpu:            decimal.Zero,
		NewPaidUsers:       0,
		NewSinglePaidUsers: 0,
		NewSubsPaidUsers:   0,
		OldRecharge:        decimal.Zero,
		OldRechargeRatio:   decimal.Zero,
		OldArpu:            decimal.Zero,
		OldPaidUsers:       0,
		OldSinglePaidUsers: 0,
		OldSubsPaidUsers:   0,
		RepeatPaidUsers:    0,
		RepeatRate:         decimal.Zero,
	}

	if len(dayOrders) == 0 {
		return stat
	}

	totalRecharge := decimal.Zero
	singleRecharge := decimal.Zero
	subsRecharge := decimal.Zero

	totalPaidUserSet := make(map[string]struct{})
	singlePaidUserSet := make(map[string]struct{})
	subsPaidUserSet := make(map[string]struct{})

	dayUserOrderCounts := make(map[string]int)

	var newOrders []*model.RawOrder
	var oldOrders []*model.RawOrder

	for _, o := range dayOrders {
		amt := o.OrderAmountUSD
		totalRecharge = totalRecharge.Add(amt)

		mID := strings.TrimSpace(o.MemberID)
		if mID != "" {
			totalPaidUserSet[mID] = struct{}{}
			dayUserOrderCounts[mID]++
		}

		if o.IsSubs == 1 {
			subsRecharge = subsRecharge.Add(amt)
			if mID != "" {
				subsPaidUserSet[mID] = struct{}{}
			}
		} else {
			singleRecharge = singleRecharge.Add(amt)
			if mID != "" {
				singlePaidUserSet[mID] = struct{}{}
			}
		}

		// 新老客判定 (与 Java 严格一致：优先 renewType，其次对比 regDate 与 payDate)
		if o.RenewType == 1 {
			newOrders = append(newOrders, o)
		} else if o.RenewType == 2 {
			oldOrders = append(oldOrders, o)
		} else {
			regDate := GetOrderRegisterDateForPlatform(pCode, o)
			orderPayDate := GetOrderPayDateForPlatform(pCode, o)
			if regDate != "" && regDate == orderPayDate {
				newOrders = append(newOrders, o)
			} else if regDate != "" && regDate < orderPayDate {
				oldOrders = append(oldOrders, o)
			}
		}
	}

	newRecharge := decimal.Zero
	newPaidUserSet := make(map[string]struct{})
	newSinglePaidUserSet := make(map[string]struct{})
	newSubsPaidUserSet := make(map[string]struct{})

	for _, o := range newOrders {
		newRecharge = newRecharge.Add(o.OrderAmountUSD)
		mID := strings.TrimSpace(o.MemberID)
		if mID != "" {
			newPaidUserSet[mID] = struct{}{}
			if o.IsSubs == 1 {
				newSubsPaidUserSet[mID] = struct{}{}
			} else {
				newSinglePaidUserSet[mID] = struct{}{}
			}
		}
	}

	oldRecharge := decimal.Zero
	oldPaidUserSet := make(map[string]struct{})
	oldSinglePaidUserSet := make(map[string]struct{})
	oldSubsPaidUserSet := make(map[string]struct{})

	for _, o := range oldOrders {
		oldRecharge = oldRecharge.Add(o.OrderAmountUSD)
		mID := strings.TrimSpace(o.MemberID)
		if mID != "" {
			oldPaidUserSet[mID] = struct{}{}
			if o.IsSubs == 1 {
				oldSubsPaidUserSet[mID] = struct{}{}
			} else {
				oldSinglePaidUserSet[mID] = struct{}{}
			}
		}
	}

	newPaidCount := len(newPaidUserSet)
	newArpu := decimal.Zero
	if newPaidCount > 0 {
		newArpu = newRecharge.DivRound(decimal.NewFromInt(int64(newPaidCount)), 2)
	}

	oldPaidCount := len(oldPaidUserSet)
	oldArpu := decimal.Zero
	if oldPaidCount > 0 {
		oldArpu = oldRecharge.DivRound(decimal.NewFromInt(int64(oldPaidCount)), 2)
	}

	newRechargeRatio := decimal.Zero
	oldRechargeRatio := decimal.Zero
	if totalRecharge.GreaterThan(decimal.Zero) {
		newRechargeRatio = newRecharge.DivRound(totalRecharge, 4)
		oldRechargeRatio = oldRecharge.DivRound(totalRecharge, 4)
	}

	dayRepeatPaidUsers := 0
	for _, count := range dayUserOrderCounts {
		if count >= 2 {
			dayRepeatPaidUsers++
		}
	}
	totalPaidCount := len(totalPaidUserSet)
	repeatRate := decimal.Zero
	if totalPaidCount > 0 {
		repeatRate = decimal.NewFromInt(int64(dayRepeatPaidUsers)).DivRound(decimal.NewFromInt(int64(totalPaidCount)), 4)
	}

	stat.TotalRecharge = totalRecharge
	stat.SingleRecharge = singleRecharge
	stat.SubsRecharge = subsRecharge
	stat.TotalPaidUsers = totalPaidCount
	stat.SinglePaidUsers = len(singlePaidUserSet)
	stat.SubsPaidUsers = len(subsPaidUserSet)

	stat.NewRecharge = newRecharge
	stat.NewRechargeRatio = newRechargeRatio
	stat.NewArpu = newArpu
	stat.NewPaidUsers = newPaidCount
	stat.NewSinglePaidUsers = len(newSinglePaidUserSet)
	stat.NewSubsPaidUsers = len(newSubsPaidUserSet)

	stat.OldRecharge = oldRecharge
	stat.OldRechargeRatio = oldRechargeRatio
	stat.OldArpu = oldArpu
	stat.OldPaidUsers = oldPaidCount
	stat.OldSinglePaidUsers = len(oldSinglePaidUserSet)
	stat.OldSubsPaidUsers = len(oldSubsPaidUserSet)

	stat.RepeatPaidUsers = dayRepeatPaidUsers
	stat.RepeatRate = repeatRate

	return stat
}

// CalculateDistributionSummaryFromOrders 从原始订单集合精确计算充值汇总 (对应 Java calculateDistributionSummaryFromOrders 严格对齐)
func (s *RechargeStatService) CalculateDistributionSummaryFromOrders(platformCode string, orders []*model.RawOrder) *dto.DailyDistributionSummaryDto {
	pCode := "ALL"
	if platformCode != "" && !strings.EqualFold(platformCode, "ALL") {
		pCode = strings.ToLower(platformCode)
	}

	totalRecharge := decimal.Zero
	for _, o := range orders {
		totalRecharge = totalRecharge.Add(o.OrderAmountUSD)
	}

	var newOrders []*model.RawOrder
	var oldOrders []*model.RawOrder

	for _, o := range orders {
		if o.RenewType == 1 {
			newOrders = append(newOrders, o)
		} else if o.RenewType == 2 {
			oldOrders = append(oldOrders, o)
		} else {
			regDate := GetOrderRegisterDateForPlatform(pCode, o)
			payDate := GetOrderPayDateForPlatform(pCode, o)
			if regDate != "" && regDate == payDate {
				newOrders = append(newOrders, o)
			} else if regDate != "" && regDate < payDate {
				oldOrders = append(oldOrders, o)
			}
		}
	}

	newRecharge := decimal.Zero
	for _, o := range newOrders {
		newRecharge = newRecharge.Add(o.OrderAmountUSD)
	}

	oldRecharge := decimal.Zero
	for _, o := range oldOrders {
		oldRecharge = oldRecharge.Add(o.OrderAmountUSD)
	}

	totalPaidUsersSet := make(map[string]struct{})
	for _, o := range orders {
		if strings.TrimSpace(o.MemberID) != "" {
			totalPaidUsersSet[strings.TrimSpace(o.MemberID)] = struct{}{}
		}
	}

	newPaidUsersSet := make(map[string]struct{})
	for _, o := range newOrders {
		if strings.TrimSpace(o.MemberID) != "" {
			newPaidUsersSet[strings.TrimSpace(o.MemberID)] = struct{}{}
		}
	}

	oldPaidUsersSet := make(map[string]struct{})
	for _, o := range oldOrders {
		if strings.TrimSpace(o.MemberID) != "" {
			oldPaidUsersSet[strings.TrimSpace(o.MemberID)] = struct{}{}
		}
	}

	totalPaidUsers := len(totalPaidUsersSet)
	newPaidUsers := len(newPaidUsersSet)
	oldPaidUsers := len(oldPaidUsersSet)

	newArpu := decimal.Zero
	if newPaidUsers > 0 {
		newArpu = newRecharge.DivRound(decimal.NewFromInt(int64(newPaidUsers)), 2)
	}

	oldArpu := decimal.Zero
	if oldPaidUsers > 0 {
		oldArpu = oldRecharge.DivRound(decimal.NewFromInt(int64(oldPaidUsers)), 2)
	}

	newRechargeRatio := decimal.Zero
	oldRechargeRatio := decimal.Zero
	if totalRecharge.GreaterThan(decimal.Zero) {
		newRechargeRatio = newRecharge.DivRound(totalRecharge, 4)
		oldRechargeRatio = oldRecharge.DivRound(totalRecharge, 4)
	}

	userOrderCounts := make(map[string]int)
	for _, o := range orders {
		if strings.TrimSpace(o.MemberID) != "" {
			userOrderCounts[strings.TrimSpace(o.MemberID)]++
		}
	}

	repeatPaidUsers := 0
	for _, count := range userOrderCounts {
		if count >= 2 {
			repeatPaidUsers++
		}
	}

	repeatRate := decimal.Zero
	if totalPaidUsers > 0 {
		repeatRate = decimal.NewFromInt(int64(repeatPaidUsers)).DivRound(decimal.NewFromInt(int64(totalPaidUsers)), 4)
	}

	todayStr := GetTodayForPlatform(pCode)
	thisMonthStr := ""
	lastMonthStr := ""
	if todayTime, err := time.ParseInLocation(timeutil.DateLayout, todayStr, timeutil.BeijingZone); err == nil {
		thisMonthStr = todayTime.Format("2006-01")
		lastMonthStr = todayTime.AddDate(0, -1, 0).Format("2006-01")
	}

	thisMonthRecharge := decimal.Zero
	thisMonthRefund := decimal.Zero
	lastMonthRecharge := decimal.Zero
	lastMonthRefund := decimal.Zero

	for _, o := range orders {
		payDate := GetOrderPayDateForPlatform(pCode, o)
		if len(payDate) >= 7 {
			ym := payDate[:7]
			if ym == thisMonthStr {
				thisMonthRecharge = thisMonthRecharge.Add(o.OrderAmountUSD)
				if o.RefundStatus == 2 {
					thisMonthRefund = thisMonthRefund.Add(o.OrderAmountUSD)
				}
			} else if ym == lastMonthStr {
				lastMonthRecharge = lastMonthRecharge.Add(o.OrderAmountUSD)
				if o.RefundStatus == 2 {
					lastMonthRefund = lastMonthRefund.Add(o.OrderAmountUSD)
				}
			}
		}
	}

	return &dto.DailyDistributionSummaryDto{
		TotalRecharge:     totalRecharge,
		NewRecharge:       newRecharge,
		OldRecharge:       oldRecharge,
		ThisMonthRecharge: thisMonthRecharge,
		ThisMonthRefund:   thisMonthRefund,
		LastMonthRecharge: lastMonthRecharge,
		LastMonthRefund:   lastMonthRefund,
		ThisMonthStr:      thisMonthStr,
		LastMonthStr:      lastMonthStr,
		NewRechargeRatio:  newRechargeRatio,
		OldRechargeRatio:  oldRechargeRatio,
		TotalPaidUsers:    totalPaidUsers,
		NewPaidUsers:      newPaidUsers,
		OldPaidUsers:      oldPaidUsers,
		NewArpu:           newArpu,
		OldArpu:           oldArpu,
		RepeatPaidUsers:   repeatPaidUsers,
		RepeatRate:        repeatRate,
	}
}

// GetGlobalDailyDistributionStats 对应 Java DailyRechargeStatService.getGlobalDailyDistributionStats
func (s *RechargeStatService) GetGlobalDailyDistributionStats(ctx context.Context, platformCode string) ([]*model.DailyRechargeDistribution, error) {
	pCode := "ALL"
	if platformCode != "" && !strings.EqualFold(platformCode, "ALL") {
		pCode = strings.ToLower(strings.TrimSpace(platformCode))
	}
	today := GetTodayForPlatform(pCode)
	platformStartDate := model.GetLaunchStartDateForPlatform(pCode)

	var allOrders []*model.RawOrder
	var err error
	if pCode == "ALL" {
		allOrders, err = s.orderRepo.FindAllValidOrders(ctx)
	} else {
		allOrders, err = s.orderRepo.FindValidOrdersByPlatform(ctx, pCode)
	}
	if err != nil {
		return nil, err
	}

	ordersByPayDate := make(map[string][]*model.RawOrder)
	for _, o := range allOrders {
		payDate := GetOrderPayDateForPlatform(pCode, o)
		if payDate != "" && payDate >= platformStartDate {
			ordersByPayDate[payDate] = append(ordersByPayDate[payDate], o)
		}
	}

	startDateTime, _ := time.ParseInLocation(timeutil.DateLayout, platformStartDate, timeutil.BeijingZone)
	endDateTime, _ := time.ParseInLocation(timeutil.DateLayout, today, timeutil.BeijingZone)

	statList := make([]*model.DailyRechargeDistribution, 0)
	currDate := startDateTime
	for !currDate.After(endDateTime) {
		dateStr := currDate.Format(timeutil.DateLayout)
		dayOrders := ordersByPayDate[dateStr]
		stat := s.CalculateSingleDayDistribution(pCode, 0, dateStr, dayOrders)
		statList = append(statList, stat)
		currDate = currDate.AddDate(0, 0, 1)
	}

	sort.Slice(statList, func(i, j int) bool {
		return statList[i].Date > statList[j].Date
	})

	return statList, nil
}

// GetGlobalDailyDistributionSummary 对应 Java DailyRechargeStatService.getGlobalDailyDistributionSummary
func (s *RechargeStatService) GetGlobalDailyDistributionSummary(ctx context.Context, platformCode string) (*dto.DailyDistributionSummaryDto, error) {
	pCode := "ALL"
	if platformCode != "" && !strings.EqualFold(platformCode, "ALL") {
		pCode = strings.ToLower(strings.TrimSpace(platformCode))
	}
	platformStartDate := model.GetLaunchStartDateForPlatform(pCode)

	var allOrders []*model.RawOrder
	var err error
	if pCode == "ALL" {
		allOrders, err = s.orderRepo.FindAllValidOrders(ctx)
	} else {
		allOrders, err = s.orderRepo.FindValidOrdersByPlatform(ctx, pCode)
	}
	if err != nil {
		return nil, err
	}

	filtered := make([]*model.RawOrder, 0, len(allOrders))
	for _, o := range allOrders {
		payDate := GetOrderPayDateForPlatform(pCode, o)
		if payDate != "" && payDate >= platformStartDate {
			filtered = append(filtered, o)
		}
	}

	return s.CalculateDistributionSummaryFromOrders(pCode, filtered), nil
}

// GetDailyDistributionSummary 对应 Java DailyRechargeStatService.getDailyDistributionSummary
func (s *RechargeStatService) GetDailyDistributionSummary(ctx context.Context, platformCode string, userID int64) (*dto.DailyDistributionSummaryDto, error) {
	if userID <= 0 {
		userID = 1
	}
	pCode := "ALL"
	if platformCode != "" && !strings.EqualFold(platformCode, "ALL") {
		pCode = strings.ToLower(strings.TrimSpace(platformCode))
	}
	platformStartDate := model.GetLaunchStartDateForPlatform(pCode)

	orders, err := s.GetOrdersFilteredForUser(ctx, pCode, userID)
	if err != nil {
		return nil, err
	}

	filtered := make([]*model.RawOrder, 0, len(orders))
	for _, o := range orders {
		payDate := GetOrderPayDateForPlatform(pCode, o)
		if payDate != "" && payDate >= platformStartDate {
			filtered = append(filtered, o)
		}
	}

	return s.CalculateDistributionSummaryFromOrders(pCode, filtered), nil
}

// CalculateAllDailyDistribution 计算所有用户的每日充值分布 (对应 Java calculateAllDailyDistributionStats)
func (s *RechargeStatService) CalculateAllDailyDistribution(ctx context.Context) error {
	users, err := s.userRepo.FindAll(ctx)
	if err != nil {
		return err
	}
	platforms := []string{"ALL", "rocnovel", "flicknovel"}
	if len(users) == 0 {
		for _, p := range platforms {
			_ = s.CalculateDailyDistributionForUserDirect(ctx, p, 1)
		}
	} else {
		for _, u := range users {
			for _, p := range platforms {
				_ = s.CalculateDailyDistributionForUserDirect(ctx, p, u.ID)
			}
		}
	}
	return nil
}

