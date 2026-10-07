package service

import (
	"context"
	"math"
	"strings"
	"time"

	"go_backend/internal/model"
)

// UserProfileSnapshot 用户历史画像快照 (内存轻量级模型，用于首购与复充判定)
type UserProfileSnapshot struct {
	EarliestPayTime   *time.Time
	EarliestRegTime   *time.Time
	LandingPageID     string
	HasSubscribed     bool
	LatestSubsPayTime *time.Time
}

// saveOrUpdateUserSubscriptionPeriod 根据首次订阅价格反推订阅套餐，维护【用户-订阅周期关联表】(对齐 Java 实现)
func (m *SyncManager) saveOrUpdateUserSubscriptionPeriod(ctx context.Context, platformCode, memberID, landingPageID string, priceCent int, regTime time.Time) {
	mID := strings.TrimSpace(memberID)
	if mID == "" || m.orderRepo == nil {
		return
	}
	pCode := strings.ToLower(strings.TrimSpace(platformCode))
	if pCode == "" {
		pCode = model.PlatformRocnovel
	}

	userSub, _ := m.orderRepo.FindUserSubscriptionPeriod(ctx, mID)
	if userSub == nil {
		userSub = &model.UserSubscriptionPeriod{
			PlatformCode: pCode,
			MemberID:     mID,
			CreateTime:   time.Now(),
		}
	}

	var matchedVer *model.SubscriptionConfigVersion

	// 1. 优先精准匹配 landingPageId + first_price_cent + 生效时间窗
	versions, err := m.orderRepo.FindMatchingFirstPriceVersions(ctx, landingPageID, priceCent, regTime)
	if err == nil && len(versions) > 0 {
		matchedVer = versions[0]
	} else {
		// 2. 落地页内兜底：若未精准匹配，在该落地页生效版本中，选择【首订价格最接近】的套餐
		pageVersions, err2 := m.orderRepo.FindMatchingPageVersions(ctx, landingPageID, regTime)
		if err2 == nil && len(pageVersions) > 0 {
			var best *model.SubscriptionConfigVersion
			minDiff := math.MaxInt32
			for _, v := range pageVersions {
				diff := absInt(v.FirstPriceCent - priceCent)
				if diff < minDiff {
					minDiff = diff
					best = v
				}
			}
			matchedVer = best
		}
	}

	// 3. 全局大盘兜底：查找所有配置版本中首订价格最接近的套餐
	if matchedVer == nil {
		allVersions, err3 := m.orderRepo.FindAllSubscriptionVersions(ctx)
		if err3 == nil && len(allVersions) > 0 {
			var best *model.SubscriptionConfigVersion
			minDiff := math.MaxInt32
			for _, v := range allVersions {
				diff := absInt(v.FirstPriceCent - priceCent)
				if diff < minDiff {
					minDiff = diff
					best = v
				}
			}
			matchedVer = best
		}
	}

	userSub.LandingPageID = landingPageID
	if matchedVer != nil {
		userSub.SubscribeConfigID = matchedVer.SubscribeConfigID
		subPeriod := matchedVer.SubPeriodDays
		if subPeriod <= 0 {
			subPeriod = 1
		}
		userSub.SubPeriodDays = subPeriod
		firstPrice := matchedVer.FirstPriceCent
		userSub.FirstPriceCent = &firstPrice
		renewPrice := matchedVer.RenewPriceCent
		userSub.RenewPriceCent = &renewPrice
	} else {
		userSub.SubPeriodDays = 1
		userSub.FirstPriceCent = &priceCent
	}
	userSub.UpdatedAt = time.Now()
	_ = m.orderRepo.SaveUserSubscriptionPeriod(ctx, userSub)
}

func absInt(n int) int {
	if n < 0 {
		return -n
	}
	return n
}
