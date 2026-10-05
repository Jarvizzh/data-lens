package service

import (
	"context"
	"strings"
	"time"

	"go_backend/internal/model"
	"go_backend/internal/pkg/timeutil"
	"go_backend/internal/repository"

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

// CalculateDailyDistributionForUser 统计指定用户的每日充值分布
func (s *RechargeStatService) CalculateDailyDistributionForUser(ctx context.Context, platformCode string, userID int64) error {
	pCode := strings.ToLower(strings.TrimSpace(platformCode))
	isAll := pCode == "" || pCode == "all"
	targetPlatform := "ALL"
	if !isAll {
		targetPlatform = pCode
	}

	userPages, lpIDs, err := s.userSvc.GetLandingPageConfigs(ctx, targetPlatform, userID)
	if err != nil {
		return err
	}
	tzMap := make(map[string]string, len(userPages))
	for _, lp := range userPages {
		tzMap[lp.LandingPageID] = lp.Timezone
	}

	startDate := "2026-07-10"
	if strings.EqualFold(targetPlatform, "flicknovel") {
		startDate = "2026-09-16"
	}

	var orders []*model.RawOrder
	if len(lpIDs) > 0 {
		orders, err = s.orderRepo.FindOrdersForLtvCalculation(ctx, targetPlatform, lpIDs, startDate, "")
		if err != nil {
			return err
		}
	}

	// 按支付自然日分组订单
	payDateMap := make(map[string][]*model.RawOrder)
	for _, o := range orders {
		payDate := GetEffectivePayDate(o, tzMap)
		if payDate >= startDate {
			payDateMap[payDate] = append(payDateMap[payDate], o)
		}
	}

	today := time.Now().In(timeutil.BeijingZone)
	startDateTime, _ := time.ParseInLocation(timeutil.DateLayout, startDate, timeutil.BeijingZone)
	totalDays := int(today.Sub(startDateTime).Hours()/24) + 1

	distList := make([]*model.DailyRechargeDistribution, 0, totalDays)
	for i := 0; i < totalDays; i++ {
		dateStr := startDateTime.AddDate(0, 0, i).Format(timeutil.DateLayout)
		dayOrders := payDateMap[dateStr]

		dist := s.calculateSingleDayDistribution(targetPlatform, userID, dateStr, dayOrders, tzMap)
		distList = append(distList, dist)
	}

	return s.rechargeDistRepo.BatchUpsert(ctx, distList)
}

func (s *RechargeStatService) calculateSingleDayDistribution(
	platformCode string,
	userID int64,
	dateStr string,
	orders []*model.RawOrder,
	tzMap map[string]string,
) *model.DailyRechargeDistribution {
	dist := &model.DailyRechargeDistribution{
		PlatformCode: platformCode,
		UserID:       userID,
		Date:         dateStr,
	}

	if len(orders) == 0 {
		return dist
	}

	allPaidMembers := make(map[string]int)
	singlePaidMembers := make(map[string]struct{})
	subsPaidMembers := make(map[string]struct{})

	newPaidMembers := make(map[string]struct{})
	newSingleMembers := make(map[string]struct{})
	newSubsMembers := make(map[string]struct{})

	oldPaidMembers := make(map[string]struct{})
	oldSingleMembers := make(map[string]struct{})
	oldSubsMembers := make(map[string]struct{})

	for _, o := range orders {
		amt := o.OrderAmountUSD
		dist.TotalRecharge = dist.TotalRecharge.Add(amt)

		allPaidMembers[o.MemberID]++
		isSubs := o.IsSubs == 1

		if isSubs {
			dist.SubsRecharge = dist.SubsRecharge.Add(amt)
			subsPaidMembers[o.MemberID] = struct{}{}
		} else {
			dist.SingleRecharge = dist.SingleRecharge.Add(amt)
			singlePaidMembers[o.MemberID] = struct{}{}
		}

		// 判断新老用户 (注册日期与支付日期对比)
		regDate := GetEffectiveRegisterDate(o, tzMap)

		if regDate == dateStr {
			dist.NewRecharge = dist.NewRecharge.Add(amt)
			newPaidMembers[o.MemberID] = struct{}{}
			if isSubs {
				newSubsMembers[o.MemberID] = struct{}{}
			} else {
				newSingleMembers[o.MemberID] = struct{}{}
			}
		} else {
			dist.OldRecharge = dist.OldRecharge.Add(amt)
			oldPaidMembers[o.MemberID] = struct{}{}
			if isSubs {
				oldSubsMembers[o.MemberID] = struct{}{}
			} else {
				oldSingleMembers[o.MemberID] = struct{}{}
			}
		}
	}

	dist.TotalPaidUsers = len(allPaidMembers)
	dist.SinglePaidUsers = len(singlePaidMembers)
	dist.SubsPaidUsers = len(subsPaidMembers)

	dist.NewPaidUsers = len(newPaidMembers)
	dist.NewSinglePaidUsers = len(newSingleMembers)
	dist.NewSubsPaidUsers = len(newSubsMembers)

	dist.OldPaidUsers = len(oldPaidMembers)
	dist.OldSinglePaidUsers = len(oldSingleMembers)
	dist.OldSubsPaidUsers = len(oldSubsMembers)

	repeatCount := 0
	for _, count := range allPaidMembers {
		if count > 1 {
			repeatCount++
		}
	}
	dist.RepeatPaidUsers = repeatCount

	if dist.TotalRecharge.GreaterThan(decimal.Zero) {
		dist.NewRechargeRatio = dist.NewRecharge.DivRound(dist.TotalRecharge, 4)
		dist.OldRechargeRatio = dist.OldRecharge.DivRound(dist.TotalRecharge, 4)
	}
	if dist.NewPaidUsers > 0 {
		dist.NewArpu = dist.NewRecharge.DivRound(decimal.NewFromInt(int64(dist.NewPaidUsers)), 2)
	}
	if dist.OldPaidUsers > 0 {
		dist.OldArpu = dist.OldRecharge.DivRound(decimal.NewFromInt(int64(dist.OldPaidUsers)), 2)
	}
	if dist.TotalPaidUsers > 0 {
		dist.RepeatRate = decimal.NewFromInt(int64(repeatCount)).DivRound(decimal.NewFromInt(int64(dist.TotalPaidUsers)), 4)
	}

	return dist
}

// CalculateAllDailyDistribution 计算所有用户的每日充值分布
func (s *RechargeStatService) CalculateAllDailyDistribution(ctx context.Context) error {
	users, err := s.userRepo.FindAll(ctx)
	if err != nil {
		return err
	}
	platforms := []string{"ALL", "rocnovel", "flicknovel"}
	for _, u := range users {
		for _, p := range platforms {
			_ = s.CalculateDailyDistributionForUser(ctx, p, u.ID)
		}
	}
	return nil
}
