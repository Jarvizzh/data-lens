package service

import (
	"context"
	"math"
	"strings"
	"time"

	"go_backend/internal/model"

	"go.uber.org/zap"
)

// UserProfileSnapshot 用户历史画像快照 (内存轻量级模型，用于首购与复充判定)
type UserProfileSnapshot struct {
	EarliestPayTime   *time.Time
	EarliestRegTime   *time.Time
	LandingPageID     string
	HasSubscribed     bool
	LatestSubsPayTime *time.Time
}

// getSubscriptionVersions 从内存缓存获取所有订阅配置版本 (带 10 分钟 TTL，防 N+1 数据库风暴)
func (m *SyncManager) getSubscriptionVersions(ctx context.Context) []*model.SubscriptionConfigVersion {
	m.subVersionsMu.RLock()
	if len(m.subVersionsCache) > 0 && time.Since(m.subVersionsCachedAt) < 10*time.Minute {
		cached := m.subVersionsCache
		m.subVersionsMu.RUnlock()
		return cached
	}
	m.subVersionsMu.RUnlock()

	m.subVersionsMu.Lock()
	defer m.subVersionsMu.Unlock()
	if len(m.subVersionsCache) > 0 && time.Since(m.subVersionsCachedAt) < 10*time.Minute {
		return m.subVersionsCache
	}
	if m.orderRepo != nil {
		versions, err := m.orderRepo.FindAllSubscriptionVersions(ctx)
		if err == nil {
			m.subVersionsCache = versions
			m.subVersionsCachedAt = time.Now()
			return versions
		}
	}
	return m.subVersionsCache
}

// isBetterVersion 辅助比较版本优选 (优先高版本号，次优更晚生效时间)
func isBetterVersion(candidate, currentBest *model.SubscriptionConfigVersion) bool {
	if currentBest == nil {
		return true
	}
	if candidate.VersionNum != currentBest.VersionNum {
		return candidate.VersionNum > currentBest.VersionNum
	}
	return candidate.EffectiveStartTime.After(currentBest.EffectiveStartTime)
}

// matchSubscriptionVersion 内存纯计算匹配订阅版本 (对齐 Java 三级兜底规则：精准 -> 落地页最近价格 -> 全局最近价格)
func matchSubscriptionVersion(versions []*model.SubscriptionConfigVersion, landingPageID string, priceCent int, targetTime time.Time) *model.SubscriptionConfigVersion {
	if len(versions) == 0 {
		return nil
	}

	// 1. 优先精准匹配 landingPageId + first_price_cent + 生效时间窗
	var exactMatches []*model.SubscriptionConfigVersion
	for _, v := range versions {
		if v.LandingPageID == landingPageID && v.FirstPriceCent == priceCent {
			if !v.EffectiveStartTime.After(targetTime) && (v.EffectiveEndTime == nil || !v.EffectiveEndTime.Before(targetTime)) {
				exactMatches = append(exactMatches, v)
			}
		}
	}
	if len(exactMatches) > 0 {
		best := exactMatches[0]
		for _, v := range exactMatches[1:] {
			if isBetterVersion(v, best) {
				best = v
			}
		}
		return best
	}

	// 2. 落地页内兜底：在该落地页生效版本中，选择【首订价格最接近】的套餐
	var pageMatches []*model.SubscriptionConfigVersion
	for _, v := range versions {
		if v.LandingPageID == landingPageID {
			if !v.EffectiveStartTime.After(targetTime) && (v.EffectiveEndTime == nil || !v.EffectiveEndTime.Before(targetTime)) {
				pageMatches = append(pageMatches, v)
			}
		}
	}
	if len(pageMatches) > 0 {
		var best *model.SubscriptionConfigVersion
		minDiff := math.MaxInt32
		for _, v := range pageMatches {
			diff := absInt(v.FirstPriceCent - priceCent)
			if diff < minDiff || (diff == minDiff && isBetterVersion(v, best)) {
				minDiff = diff
				best = v
			}
		}
		return best
	}

	// 3. 全局大盘兜底：查找所有配置版本中首订价格最接近的套餐
	var best *model.SubscriptionConfigVersion
	minDiff := math.MaxInt32
	for _, v := range versions {
		diff := absInt(v.FirstPriceCent - priceCent)
		if diff < minDiff || (diff == minDiff && isBetterVersion(v, best)) {
			minDiff = diff
			best = v
		}
	}
	return best
}

// BatchSaveOrUpdateSubscriptionPeriods 批量更新首次订阅用户的周期关联记录 (消除 N+1 数据库往返风暴)
func (m *SyncManager) BatchSaveOrUpdateSubscriptionPeriods(ctx context.Context, platformCode string, orders []*model.RawOrder) {
	if len(orders) == 0 || m.orderRepo == nil {
		return
	}
	pCode := strings.ToLower(strings.TrimSpace(platformCode))
	if pCode == "" {
		pCode = model.PlatformRocnovel
	}

	// 1. 过滤并提取该批次中的首次订阅订单
	var subOrders []*model.RawOrder
	memberIDSet := make(map[string]struct{})
	for _, ord := range orders {
		if ord != nil && ord.IsSubs == 1 && ord.RenewType == 1 {
			mID := strings.TrimSpace(ord.MemberID)
			if mID != "" {
				subOrders = append(subOrders, ord)
				memberIDSet[mID] = struct{}{}
			}
		}
	}
	if len(subOrders) == 0 {
		return
	}

	memberIDs := make([]string, 0, len(memberIDSet))
	for id := range memberIDSet {
		memberIDs = append(memberIDs, id)
	}

	// 2. 批量查出已有记录 (单条 SQL IN 查询，杜绝逐条单查)
	existingMap, _ := m.orderRepo.FindUserSubscriptionPeriodsByMemberIDs(ctx, memberIDs)
	if existingMap == nil {
		existingMap = make(map[string]*model.UserSubscriptionPeriod)
	}

	// 3. 内存中高效匹配订阅版本 (零 DB 往返)
	allVersions := m.getSubscriptionVersions(ctx)
	now := time.Now()
	toSaveMap := make(map[string]*model.UserSubscriptionPeriod, len(memberIDs))

	for _, ord := range subOrders {
		mID := strings.TrimSpace(ord.MemberID)
		userSub := existingMap[mID]
		if userSub == nil {
			if pending, ok := toSaveMap[mID]; ok {
				userSub = pending
			} else {
				userSub = &model.UserSubscriptionPeriod{
					PlatformCode: pCode,
					MemberID:     mID,
					CreateTime:   now,
				}
			}
		}

		matchedVer := matchSubscriptionVersion(allVersions, ord.LandingPageID, ord.OrderAmountCent, ord.RegisterTimeBJ)

		userSub.LandingPageID = ord.LandingPageID
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
			priceCent := ord.OrderAmountCent
			userSub.FirstPriceCent = &priceCent
		}
		userSub.UpdatedAt = now
		toSaveMap[mID] = userSub
	}

	// 4. 批量执行 Upsert (单条 SQL 批量落库)
	toSave := make([]*model.UserSubscriptionPeriod, 0, len(toSaveMap))
	for _, period := range toSaveMap {
		toSave = append(toSave, period)
	}
	if err := m.orderRepo.BatchSaveUserSubscriptionPeriods(ctx, toSave); err != nil {
		m.logger.Warn("Batch save user subscription periods failed", zap.Error(err), zap.Int("count", len(toSave)))
	}
}

// saveOrUpdateUserSubscriptionPeriod 单条兼容接口 (内部复用批量内存匹配与入库)
func (m *SyncManager) saveOrUpdateUserSubscriptionPeriod(ctx context.Context, platformCode, memberID, landingPageID string, priceCent int, regTime time.Time) {
	m.BatchSaveOrUpdateSubscriptionPeriods(ctx, platformCode, []*model.RawOrder{
		{
			PlatformCode:    platformCode,
			MemberID:        memberID,
			LandingPageID:   landingPageID,
			OrderAmountCent: priceCent,
			RegisterTimeBJ:  regTime,
			IsSubs:          1,
			RenewType:       1,
		},
	})
}

func absInt(n int) int {
	if n < 0 {
		return -n
	}
	return n
}
