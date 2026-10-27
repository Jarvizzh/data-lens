package service

import (
	"context"
	"encoding/json"
	"math"
	"sort"
	"strings"
	"time"

	"go_backend/internal/model"
	"go_backend/internal/pkg/timeutil"
	"go_backend/internal/service/client/flicknovel"

	"github.com/shopspring/decimal"
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

// matchSubscriptionVersion 内存纯计算匹配订阅版本 (严格按平台隔离：精准 -> 落地页最近价格 -> 同平台最近价格)
func matchSubscriptionVersion(versions []*model.SubscriptionConfigVersion, landingPageID string, priceCent int, targetTime time.Time) *model.SubscriptionConfigVersion {
	return matchSubscriptionVersionByPlatform(versions, model.PlatformRocnovel, landingPageID, priceCent, targetTime)
}

func matchSubscriptionVersionByPlatform(versions []*model.SubscriptionConfigVersion, platformCode, landingPageID string, priceCent int, targetTime time.Time) *model.SubscriptionConfigVersion {
	if len(versions) == 0 {
		return nil
	}
	pCode := strings.ToLower(strings.TrimSpace(platformCode))
	if pCode == "" {
		pCode = model.PlatformRocnovel
	}

	var platVersions []*model.SubscriptionConfigVersion
	for _, v := range versions {
		vPlat := strings.ToLower(strings.TrimSpace(v.PlatformCode))
		if vPlat == "" {
			vPlat = model.PlatformRocnovel
		}
		if vPlat == pCode {
			platVersions = append(platVersions, v)
		}
	}
	if len(platVersions) == 0 {
		return nil
	}

	// 1. 优先精准匹配 landingPageId + first_price_cent + 生效时间窗
	var exactMatches []*model.SubscriptionConfigVersion
	for _, v := range platVersions {
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
	for _, v := range platVersions {
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

	// 3. 同平台大盘兜底：查找同平台所有配置版本中首订价格最接近的套餐
	var best *model.SubscriptionConfigVersion
	minDiff := math.MaxInt32
	for _, v := range platVersions {
		diff := absInt(v.FirstPriceCent - priceCent)
		if diff < minDiff || (diff == minDiff && isBetterVersion(v, best)) {
			minDiff = diff
			best = v
		}
	}
	return best
}

// BatchSaveOrUpdateSubscriptionPeriods 批量更新订阅用户的周期关联记录 (消除 N+1 数据库往返风暴)
func (m *SyncManager) BatchSaveOrUpdateSubscriptionPeriods(ctx context.Context, platformCode string, orders []*model.RawOrder) {
	if len(orders) == 0 || m.orderRepo == nil {
		return
	}
	pCode := strings.ToLower(strings.TrimSpace(platformCode))
	if pCode == "" {
		pCode = model.PlatformRocnovel
	}

	if model.IsFlicknovel(pCode) {
		m.batchSaveFlicknovelSubscriptionPeriods(ctx, orders)
		return
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

	sort.Slice(subOrders, func(i, j int) bool {
		if !subOrders[i].PayTimeBJ.Equal(subOrders[j].PayTimeBJ) {
			return subOrders[i].PayTimeBJ.Before(subOrders[j].PayTimeBJ)
		}
		return subOrders[i].OrderID < subOrders[j].OrderID
	})

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

		matchedVer := matchSubscriptionVersionByPlatform(allVersions, pCode, ord.LandingPageID, ord.OrderAmountCent, ord.RegisterTimeBJ)

		userSub.PlatformCode = pCode
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

// batchSaveFlicknovelSubscriptionPeriods 严格基于番茄司南订单的 period、period_count、us_price、current_product_price、product_id 维护用户订阅周期与配置版本
func (m *SyncManager) batchSaveFlicknovelSubscriptionPeriods(ctx context.Context, orders []*model.RawOrder) {
	var subOrders []*model.RawOrder
	memberIDSet := make(map[string]struct{})
	nonSubCandidateSet := make(map[string]struct{})
	for _, ord := range orders {
		if ord == nil {
			continue
		}
		mID := strings.TrimSpace(ord.MemberID)
		if mID == "" {
			continue
		}
		if ord.IsSubs == 1 {
			subOrders = append(subOrders, ord)
			memberIDSet[mID] = struct{}{}
			delete(nonSubCandidateSet, mID)
		} else if _, hasSub := memberIDSet[mID]; !hasSub {
			nonSubCandidateSet[mID] = struct{}{}
		}
	}

	if len(nonSubCandidateSet) > 0 {
		nonSubMemberIDs := make([]string, 0, len(nonSubCandidateSet))
		for id := range nonSubCandidateSet {
			nonSubMemberIDs = append(nonSubMemberIDs, id)
		}
		_ = m.orderRepo.CleanupNonSubscriberPeriods(ctx, model.PlatformFlicknovel, nonSubMemberIDs)
	}

	if len(subOrders) == 0 {
		return
	}

	sort.Slice(subOrders, func(i, j int) bool {
		if !subOrders[i].PayTimeBJ.Equal(subOrders[j].PayTimeBJ) {
			return subOrders[i].PayTimeBJ.Before(subOrders[j].PayTimeBJ)
		}
		return subOrders[i].OrderID < subOrders[j].OrderID
	})

	memberIDs := make([]string, 0, len(memberIDSet))
	for id := range memberIDSet {
		memberIDs = append(memberIDs, id)
	}

	existingMap, _ := m.orderRepo.FindUserSubscriptionPeriodsByMemberIDs(ctx, memberIDs)
	if existingMap == nil {
		existingMap = make(map[string]*model.UserSubscriptionPeriod)
	}

	type versionKey struct {
		landingPageID string
		productID     string
	}

	// 预加载库中已有的番茄司南版本价格映射，按 (landing_page_id, product_id) 精确隔离，并保留 product_id 级别兜底
	pageProductFirstCent := make(map[versionKey]int)
	pageProductRenewCent := make(map[versionKey]int)
	productFirstCent := make(map[string]int)
	productRenewCent := make(map[string]int)
	for _, v := range m.getSubscriptionVersions(ctx) {
		if v != nil && model.IsFlicknovel(v.PlatformCode) && v.ProductID != "" {
			vk := versionKey{landingPageID: v.LandingPageID, productID: v.ProductID}
			if v.FirstPriceCent > 0 {
				pageProductFirstCent[vk] = v.FirstPriceCent
				productFirstCent[v.ProductID] = v.FirstPriceCent
			}
			if v.RenewPriceCent > 0 {
				pageProductRenewCent[vk] = v.RenewPriceCent
				if v.RenewPriceCent > v.FirstPriceCent || productRenewCent[v.ProductID] == 0 {
					productRenewCent[v.ProductID] = v.RenewPriceCent
				}
			}
		}
	}

	type parsedSubOrder struct {
		ord        *model.RawOrder
		item       flicknovel.OrderItem
		prodID     string
		periodDays int
		usCent     int
		curCent    int
		isInitial  bool
	}
	parsedList := make([]parsedSubOrder, 0, len(subOrders))

	// 第一遍扫描：提取本批次中各 (landing_page_id, product_id) 的真实首订价与续订价
	for _, ord := range subOrders {
		var item flicknovel.OrderItem
		if strings.TrimSpace(ord.RawPayload) != "" {
			_ = json.Unmarshal([]byte(ord.RawPayload), &item)
		}
		prodID := strings.TrimSpace(item.ProductID)
		if prodID == "" {
			prodID = ord.LandingPageID
		}
		periodDays := item.GetSubPeriodDays()
		if periodDays <= 0 {
			periodDays = 1
		}
		usCent := ord.OrderAmountCent
		curCent := usCent
		if curUsd, err := decimal.NewFromString(strings.TrimSpace(item.CurrentProductPrice)); err == nil && curUsd.GreaterThan(decimal.Zero) {
			curCent = int(curUsd.Mul(decimal.NewFromInt(100)).IntPart())
		}
		rechType := strings.TrimSpace(item.RechargeType)
		isInitial := rechType == "1" || (rechType == "" && ord.RenewType == 1)

		if prodID != "" {
			vk := versionKey{landingPageID: ord.LandingPageID, productID: prodID}
			if isInitial {
				if usCent > 0 {
					pageProductFirstCent[vk] = usCent
					productFirstCent[prodID] = usCent
				}
				if curCent > usCent {
					pageProductRenewCent[vk] = curCent
					productRenewCent[prodID] = curCent
				}
			} else {
				rCent := curCent
				if rCent <= 0 {
					rCent = usCent
				}
				if rCent > 0 {
					pageProductRenewCent[vk] = rCent
					productRenewCent[prodID] = rCent
				}
			}
		}

		parsedList = append(parsedList, parsedSubOrder{
			ord:        ord,
			item:       item,
			prodID:     prodID,
			periodDays: periodDays,
			usCent:     usCent,
			curCent:    curCent,
			isInitial:  isInitial,
		})
	}

	now := time.Now()
	toSaveMap := make(map[string]*model.UserSubscriptionPeriod, len(memberIDs))
	defaultEffectiveStart := time.Date(2026, 9, 1, 0, 0, 0, 0, timeutil.BeijingZone)
	versionUpdates := make(map[versionKey]*model.SubscriptionConfigVersion)

	// 第二遍扫描：更新用户订阅周期画像与版本配置表
	for _, p := range parsedList {
		ord := p.ord
		item := p.item
		mID := strings.TrimSpace(ord.MemberID)

		userSub := existingMap[mID]
		if pending, ok := toSaveMap[mID]; ok {
			userSub = pending
		}

		if userSub == nil {
			userSub = &model.UserSubscriptionPeriod{
				PlatformCode: model.PlatformFlicknovel,
				MemberID:     mID,
				CreateTime:   now,
			}
		}

		// 若库中旧记录的 SubscribeConfigID 不等于当前番茄订单的 product_id（如历史串用了 rocnovel 配置 ID），则强制丢弃旧记录的脏价格
		isSameProduct := userSub.SubscribeConfigID != "" && userSub.SubscribeConfigID == p.prodID
		vk := versionKey{landingPageID: ord.LandingPageID, productID: p.prodID}

		firstCent := p.usCent
		if !p.isInitial {
			if isSameProduct && userSub.FirstPriceCent != nil && *userSub.FirstPriceCent > 0 {
				firstCent = *userSub.FirstPriceCent
			} else if fc, ok := pageProductFirstCent[vk]; ok && fc > 0 {
				firstCent = fc
			} else if fc, ok := productFirstCent[p.prodID]; ok && fc > 0 {
				firstCent = fc
			}
		}

		renewCent := p.curCent
		if p.isInitial && renewCent <= firstCent {
			if rc, ok := pageProductRenewCent[vk]; ok && rc > 0 {
				renewCent = rc
			} else if isSameProduct && userSub.RenewPriceCent != nil && *userSub.RenewPriceCent > firstCent {
				renewCent = *userSub.RenewPriceCent
			} else if rc, ok := productRenewCent[p.prodID]; ok && rc > 0 {
				renewCent = rc
			}
		}

		userSub.PlatformCode = model.PlatformFlicknovel
		if ord.LandingPageID != "" && (userSub.LandingPageID == "" || !isSameProduct || p.isInitial) {
			userSub.LandingPageID = ord.LandingPageID
		}
		if p.prodID != "" && (userSub.SubscribeConfigID == "" || !isSameProduct || p.isInitial) {
			userSub.SubscribeConfigID = p.prodID
		}
		userSub.SubPeriodDays = p.periodDays
		userSub.FirstPriceCent = &firstCent
		userSub.RenewPriceCent = &renewCent
		userSub.UpdatedAt = now
		toSaveMap[mID] = userSub

		if ord.LandingPageID != "" && p.prodID != "" {
			vFirst := firstCent
			if fc, ok := pageProductFirstCent[vk]; ok && fc > 0 {
				vFirst = fc
			} else if fc, ok := productFirstCent[p.prodID]; ok && fc > 0 {
				vFirst = fc
			}
			vRenew := renewCent
			if rc, ok := pageProductRenewCent[vk]; ok && rc > 0 {
				vRenew = rc
			} else if rc, ok := productRenewCent[p.prodID]; ok && rc > 0 {
				vRenew = rc
			}
			versionUpdates[vk] = &model.SubscriptionConfigVersion{
				PlatformCode:        model.PlatformFlicknovel,
				LandingPageID:       ord.LandingPageID,
				SubscribeConfigID:   p.prodID,
				SubscribeConfigName: strings.TrimSpace(item.AppName),
				SaleComboID:         strings.TrimSpace(item.ContentID),
				SaleComboName:       strings.TrimSpace(item.ContentName),
				ProductID:           p.prodID,
				ProductName:         strings.TrimSpace(item.ContentName),
				SubPeriodDays:       p.periodDays,
				FirstPriceCent:      vFirst,
				RenewPriceCent:      vRenew,
				VersionNum:          1,
				EffectiveStartTime:  defaultEffectiveStart,
				EffectiveEndTime:    nil,
				UpdatedAt:           now,
			}
		}
	}

	if len(toSaveMap) > 0 {
		toSave := make([]*model.UserSubscriptionPeriod, 0, len(toSaveMap))
		for _, period := range toSaveMap {
			toSave = append(toSave, period)
		}
		if err := m.orderRepo.BatchSaveUserSubscriptionPeriods(ctx, toSave); err != nil {
			m.logger.Warn("Batch save flicknovel user subscription periods failed", zap.Error(err), zap.Int("count", len(toSave)))
		}
	}

	for vk, rCent := range pageProductRenewCent {
		if rCent > 0 {
			_ = m.orderRepo.BackfillFlicknovelProductRenewPrice(ctx, vk.landingPageID, vk.productID, rCent)
		}
	}
	for prodID, rCent := range productRenewCent {
		if rCent > 0 {
			_ = m.orderRepo.BackfillFlicknovelProductRenewPrice(ctx, "", prodID, rCent)
		}
	}

	if len(versionUpdates) > 0 {
		savedAnyVersion := false
		for vk, ver := range versionUpdates {
			latest, err := m.orderRepo.FindLatestVersionByPageAndProduct(ctx, vk.landingPageID, vk.productID)
			if err != nil {
				continue
			}
			if latest == nil {
				_ = m.orderRepo.SaveSubscriptionVersion(ctx, ver)
				savedAnyVersion = true
			} else if latest.SubPeriodDays != ver.SubPeriodDays || latest.FirstPriceCent != ver.FirstPriceCent || latest.RenewPriceCent != ver.RenewPriceCent {
				latest.SubPeriodDays = ver.SubPeriodDays
				if ver.FirstPriceCent > 0 {
					latest.FirstPriceCent = ver.FirstPriceCent
				}
				if ver.RenewPriceCent > 0 {
					latest.RenewPriceCent = ver.RenewPriceCent
				}
				latest.PlatformCode = model.PlatformFlicknovel
				_ = m.orderRepo.SaveSubscriptionVersion(ctx, latest)
				savedAnyVersion = true
			}
		}
		if savedAnyVersion {
			m.InvalidateSubscriptionConfigCache()
		}
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
