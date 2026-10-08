package service

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	"go_backend/internal/model"
	"go_backend/internal/pkg/timeutil"
	"go_backend/internal/service/client/flicknovel"

	"github.com/shopspring/decimal"
	"go.uber.org/zap"
)

const notFoundTpl = "__NOT_FOUND__"

// initFlicknovelCache 初始化从 DB 预热番茄司南缓存
func (m *SyncManager) initFlicknovelCache(ctx context.Context) {
	m.logger.Info("Initializing Flicknovel promotion & template price cache from DB...")
	m.loadFlicknovelCacheFromDB(ctx)

	m.fnCacheMu.RLock()
	pSize := len(m.promotionTemplateMap)
	tSize := len(m.templateDetailCache)
	m.fnCacheMu.RUnlock()

	if pSize == 0 || tSize == 0 {
		m.logger.Info("In-memory cache empty on startup. Triggering initial sync from OpenAPI...")
		_ = m.SyncFlicknovelPromotionsAndTemplates(ctx)
	} else {
		m.logger.Info("In-memory cache loaded successfully from DB",
			zap.Int("promotions", pSize),
			zap.Int("templates", tSize),
		)
	}
}

// loadFlicknovelCacheFromDB 从本地数据库加载数据构建内存字典
func (m *SyncManager) loadFlicknovelCacheFromDB(ctx context.Context) {
	if m.flicknovelRepo == nil {
		return
	}

	templates, err := m.flicknovelRepo.FindAllTemplates(ctx)
	if err != nil {
		m.logger.Warn("Failed to load templates from DB", zap.Error(err))
		return
	}

	newTemplateDetailCache := make(map[string]*TemplatePriceDetail)
	newTemplatePriceTypeCache := make(map[string]map[int]int)

	for _, tpl := range templates {
		tplID := strings.TrimSpace(tpl.TemplateID)
		if tplID == "" {
			continue
		}
		if strings.TrimSpace(tpl.RawPayload) != "" {
			detail := ParseTemplatePriceDetail(tpl.RawPayload)
			newTemplateDetailCache[tplID] = detail
			newTemplatePriceTypeCache[tplID] = detail.PriceMap
		} else if strings.TrimSpace(tpl.PriceConfigJSON) != "" {
			var rawMap map[string]int
			if err := json.Unmarshal([]byte(tpl.PriceConfigJSON), &rawMap); err == nil {
				priceMap := make(map[int]int)
				for k, v := range rawMap {
					if p, e := strconv.Atoi(k); e == nil {
						priceMap[p] = v
					}
				}
				newTemplatePriceTypeCache[tplID] = priceMap
				fallbackDetail := &TemplatePriceDetail{
					PriceMap:        priceMap,
					AmbiguousPrices: make(map[int]bool),
					FirstPriceMap:   priceMap,
					NoFirstPriceMap: priceMap,
				}
				newTemplateDetailCache[tplID] = fallbackDetail
			}
		}
	}

	promotions, err := m.flicknovelRepo.FindAllPromotions(ctx)
	if err != nil {
		m.logger.Warn("Failed to load promotions from DB", zap.Error(err))
	}

	newPromotionTemplateMap := make(map[string]string)
	for _, p := range promotions {
		pID := strings.TrimSpace(p.PromotionID)
		tID := strings.TrimSpace(p.RechargeTplID)
		if pID != "" && tID != "" {
			newPromotionTemplateMap[pID] = tID
		}
	}

	m.fnCacheMu.Lock()
	m.templateDetailCache = newTemplateDetailCache
	m.templatePriceTypeCache = newTemplatePriceTypeCache
	m.promotionTemplateMap = newPromotionTemplateMap
	m.fnCacheMu.Unlock()
}

// populateTemplateContext 填充模板上下文（若未命中则自愈拉取 API）
func (m *SyncManager) populateTemplateContext(ctx context.Context, promotionID string, resolveCtx *OrderResolveContext) {
	pID := strings.TrimSpace(promotionID)
	if pID == "" {
		return
	}

	m.fnCacheMu.RLock()
	tplID, exists := m.promotionTemplateMap[pID]
	m.fnCacheMu.RUnlock()

	if tplID == notFoundTpl {
		return
	}

	if !exists || tplID == "" {
		// 查 DB
		if m.flicknovelRepo != nil {
			dbPrmt, _ := m.flicknovelRepo.FindPromotionByID(ctx, pID)
			if dbPrmt != nil && strings.TrimSpace(dbPrmt.RechargeTplID) != "" {
				tplID = strings.TrimSpace(dbPrmt.RechargeTplID)
				m.fnCacheMu.Lock()
				m.promotionTemplateMap[pID] = tplID
				m.fnCacheMu.Unlock()
			} else {
				// DB 也无，触发按需自愈同步
				m.logger.Info("PromotionId not found in memory or DB, triggering on-demand sync", zap.String("promotion_id", pID))
				_ = m.syncPromotionsAndTemplatesInternal(ctx, false)

				m.fnCacheMu.RLock()
				tplID = m.promotionTemplateMap[pID]
				m.fnCacheMu.RUnlock()

				if tplID == "" {
					m.fnCacheMu.Lock()
					m.promotionTemplateMap[pID] = notFoundTpl
					m.fnCacheMu.Unlock()
					return
				}
			}
		}
	}

	if tplID != "" && tplID != notFoundTpl {
		m.fnCacheMu.RLock()
		detail, hasDetail := m.templateDetailCache[tplID]
		m.fnCacheMu.RUnlock()

		if !hasDetail || detail == nil {
			if m.flicknovelRepo != nil {
				dbTpl, _ := m.flicknovelRepo.FindTemplateByID(ctx, tplID)
				if dbTpl != nil && strings.TrimSpace(dbTpl.RawPayload) != "" {
					detail = ParseTemplatePriceDetail(dbTpl.RawPayload)
					m.fnCacheMu.Lock()
					m.templateDetailCache[tplID] = detail
					m.templatePriceTypeCache[tplID] = detail.PriceMap
					m.fnCacheMu.Unlock()
				}
			}
		}

		if detail != nil {
			resolveCtx.TemplatePriceMap = detail.PriceMap
			resolveCtx.AmbiguousPrices = detail.AmbiguousPrices
			resolveCtx.TemplateHasIntroOffer = detail.HasIntroOffer
			resolveCtx.FirstPriceMap = detail.FirstPriceMap
			resolveCtx.NoFirstPriceMap = detail.NoFirstPriceMap
			resolveCtx.FirstAmbiguousPrices = detail.FirstAmbiguousPrices
			resolveCtx.NoFirstAmbiguousPrices = detail.NoFirstAmbiguousPrices
			return
		}

		m.fnCacheMu.RLock()
		priceMap := m.templatePriceTypeCache[tplID]
		m.fnCacheMu.RUnlock()
		if priceMap != nil {
			resolveCtx.TemplatePriceMap = priceMap
		}
	}
}

// fetchRelationBeginTimes 拉取指定时间区间的染色归因记录，构建 relation_id -> relation_begin_time 字典，并自动持久化入库
func (m *SyncManager) fetchRelationBeginTimes(ctx context.Context, beginTs, endTs int64) map[string]time.Time {
	relationMap := make(map[string]time.Time)
	pageIndex := int64(1)
	pageSize := int64(1000)

	for {
		data, err := m.fnClient.QueryRelations(ctx, flicknovel.RelationQueryRequest{
			BeginTs:  beginTs,
			EndTs:    endTs,
			Page:     pageIndex,
			PageSize: pageSize,
		})
		if err != nil || data == nil || len(data.Relations) == 0 {
			break
		}

		relations := make([]*model.FlicknovelRelation, 0, len(data.Relations))
		for _, rec := range data.Relations {
			rID := strings.TrimSpace(rec.RelationID)
			if rID == "" {
				continue
			}

			var regBj *time.Time
			var regEt *time.Time
			var regTs int64
			regDateEt := ""

			timeStr := strings.TrimSpace(rec.RelationBeginTime)
			if sec, err := strconv.ParseInt(timeStr, 10, 64); err == nil && sec > 0 {
				regTs = sec
				bj := time.Unix(sec, 0).In(timeutil.BeijingZone)
				regBj = &bj
				et := bj.In(timeutil.EasternZone)
				regEt = &et
				regDateEt = et.Format(timeutil.DateLayout)
				relationMap[rID] = bj
			} else if t, err := time.ParseInLocation(timeutil.DateTimeLayout, timeStr, timeutil.BeijingZone); err == nil {
				regBj = &t
				regTs = t.Unix()
				et := t.In(timeutil.EasternZone)
				regEt = &et
				regDateEt = et.Format(timeutil.DateLayout)
				relationMap[rID] = t
			}

			rawPayload, _ := json.Marshal(rec)
			relations = append(relations, &model.FlicknovelRelation{
				RelationID:             rID,
				DeviceID:               rec.DeviceID,
				PromotionID:            rec.PromotionID,
				PromotionCode:          rec.PromotionCode,
				AdID:                   rec.AdID,
				AdsetID:                rec.AdsetID,
				CampaignID:             rec.CampaignID,
				AdAccountID:            rec.AdAccountID,
				RelationBeginTimeBJ:    regBj,
				RelationBeginTimeET:    regEt,
				RelationBeginDateET:    regDateEt,
				RelationBeginTimestamp: regTs,
				MediaChannel:           rec.MediaChannel,
				Platform:               rec.Platform,
				AppID:                  rec.AppID,
				RawPayload:             string(rawPayload),
				CreatedAt:              time.Now(),
				UpdatedAt:              time.Now(),
			})
		}

		if len(relations) > 0 && m.flicknovelRepo != nil {
			_ = m.flicknovelRepo.BatchUpsertRelations(ctx, relations)
		}

		if len(data.Relations) < int(pageSize) {
			break
		}
		pageIndex++
	}

	return relationMap
}

// extractFlicknovelMemberID 提取用户唯一标识 (优先 device_id，次选 relation_id，兜底 order_id)
func extractFlicknovelMemberID(dto *flicknovel.OrderDto) string {
	if dto == nil {
		return "UNKNOWN"
	}
	if devID := strings.TrimSpace(dto.DeviceID); devID != "" && !strings.EqualFold(devID, "null") {
		return devID
	}
	if relID := strings.TrimSpace(dto.RelationID); relID != "" && !strings.EqualFold(relID, "null") {
		return relID
	}
	if ordID := strings.TrimSpace(dto.OrderID); ordID != "" {
		return ordID
	}
	return "UNKNOWN"
}

func parseEpochSecondSafe(primaryTs, fallbackTs string) int64 {
	if sec, err := strconv.ParseInt(strings.TrimSpace(primaryTs), 10, 64); err == nil && sec > 0 {
		return sec
	}
	if sec, err := strconv.ParseInt(strings.TrimSpace(fallbackTs), 10, 64); err == nil && sec > 0 {
		return sec
	}
	return time.Now().Unix()
}

// batchCleanAndSaveFlicknovelOrders 批量清洗并持久化番茄司南订单集合 (严格对齐 Java 版业务状态机)
func (m *SyncManager) batchCleanAndSaveFlicknovelOrders(
	ctx context.Context,
	orders []flicknovel.OrderDto,
	relationTimeMap map[string]time.Time,
) (int, error) {
	if len(orders) == 0 {
		return 0, nil
	}

	// 1. 过滤非法订单并提取 memberId
	validOrders := make([]*flicknovel.OrderDto, 0, len(orders))
	memberIDSet := make(map[string]bool)
	for i := range orders {
		o := &orders[i]
		if strings.TrimSpace(o.OrderID) != "" {
			validOrders = append(validOrders, o)
			mID := extractFlicknovelMemberID(o)
			if mID != "" && mID != "UNKNOWN" {
				memberIDSet[mID] = true
			}
		}
	}

	if len(validOrders) == 0 {
		return 0, nil
	}

	var memberIDs []string
	for id := range memberIDSet {
		memberIDs = append(memberIDs, id)
	}

	// 2. 预查库中已有的历史订单，聚合各用户的最早支付时间与注册时间画像
	userHistoryMap := make(map[string]*UserProfileSnapshot)
	if len(memberIDs) > 0 {
		existingOrders, err := m.orderRepo.FindHistoryOrdersByMemberIDs(ctx, model.PlatformFlicknovel, memberIDs)
		if err == nil {
			for _, ho := range existingOrders {
				mID := strings.TrimSpace(ho.MemberID)
				if mID == "" {
					continue
				}
				isSubs := ho.IsSubs == 1
				payTime := ho.PayTimeBJ
				regTime := ho.RegisterTimeBJ

				profile, exists := userHistoryMap[mID]
				if !exists {
					p := &UserProfileSnapshot{
						EarliestPayTime: &payTime,
						EarliestRegTime: &regTime,
						LandingPageID:   ho.LandingPageID,
						HasSubscribed:   isSubs,
					}
					if isSubs {
						p.LatestSubsPayTime = &payTime
					}
					userHistoryMap[mID] = p
				} else {
					if profile.EarliestPayTime == nil || payTime.Before(*profile.EarliestPayTime) {
						profile.EarliestPayTime = &payTime
					}
					if profile.EarliestRegTime == nil || regTime.Before(*profile.EarliestRegTime) {
						profile.EarliestRegTime = &regTime
					}
					if profile.LandingPageID == "" && ho.LandingPageID != "" {
						profile.LandingPageID = ho.LandingPageID
					}
					if isSubs {
						profile.HasSubscribed = true
						if profile.LatestSubsPayTime == nil || payTime.After(*profile.LatestSubsPayTime) {
							profile.LatestSubsPayTime = &payTime
						}
					}
				}
			}
		}
	}

	// 3. 将本批次订单按用户聚类，并按支付时间升序排序
	ordersByMember := make(map[string][]*flicknovel.OrderDto)
	for _, o := range validOrders {
		mID := extractFlicknovelMemberID(o)
		ordersByMember[mID] = append(ordersByMember[mID], o)
	}

	toSave := make([]*model.RawOrder, 0, len(validOrders))

	for memberID, memberOrders := range ordersByMember {
		// 按支付时间升序排序
		sort.Slice(memberOrders, func(i, j int) bool {
			return parseEpochSecondSafe(memberOrders[i].CompletedAt, memberOrders[i].CreatedAt) <
				parseEpochSecondSafe(memberOrders[j].CompletedAt, memberOrders[j].CreatedAt)
		})

		profile, exists := userHistoryMap[memberID]
		if !exists {
			profile = &UserProfileSnapshot{}
			userHistoryMap[memberID] = profile
		}

		for _, dto := range memberOrders {
			rawOrder := m.cleanSingleFlicknovelOrder(ctx, dto, memberID, profile, relationTimeMap)
			if rawOrder != nil {
				toSave = append(toSave, rawOrder)
			}
		}
	}

	if len(toSave) == 0 {
		return 0, nil
	}

	// 按 order_id 升序排序防死锁
	sort.Slice(toSave, func(i, j int) bool {
		return toSave[i].OrderID < toSave[j].OrderID
	})

	batchSize := 200
	// 无论外层请求上下文是否中断，使用独立写上下文确保清洗后的数据安全入库
	writeCtx, cancelWrite := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Minute)
	defer cancelWrite()

	savedCount := 0
	for i := 0; i < len(toSave); i += batchSize {
		end := i + batchSize
		if end > len(toSave) {
			end = len(toSave)
		}
		batch := toSave[i:end]

		var batchErr error
		for attempt := 0; attempt < 3; attempt++ {
			err := m.orderRepo.BatchUpsert(writeCtx, batch)
			if err == nil {
				savedCount += len(batch)
				batchErr = nil
				// 维护首次订阅用户的周期配置表
				for _, ord := range batch {
					if ord.IsSubs == 1 && ord.RenewType == 1 {
						m.saveOrUpdateUserSubscriptionPeriod(writeCtx, model.PlatformFlicknovel, ord.MemberID, ord.LandingPageID, ord.OrderAmountCent, ord.RegisterTimeBJ)
					}
				}
				break
			}
			batchErr = err
			if strings.Contains(err.Error(), "1213") || strings.Contains(err.Error(), "Deadlock") || strings.Contains(err.Error(), "1205") {
				time.Sleep(time.Duration(100*(attempt+1)) * time.Millisecond)
				continue
			}
			break
		}

		if batchErr != nil {
			return savedCount, batchErr
		}
	}

	return savedCount, nil
}

// cleanSingleFlicknovelOrder 单笔番茄司南订单清洗 (对齐 Java cleanSingleOrder)
func (m *SyncManager) cleanSingleFlicknovelOrder(
	ctx context.Context,
	dto *flicknovel.OrderDto,
	memberID string,
	profile *UserProfileSnapshot,
	relationTimeMap map[string]time.Time,
) *model.RawOrder {
	if dto == nil || strings.TrimSpace(dto.OrderID) == "" {
		return nil
	}
	orderID := strings.TrimSpace(dto.OrderID)

	// 1. 落地页 / 渠道清洗
	landingPageID := strings.TrimSpace(dto.PromotionID)
	if landingPageID == "" || strings.EqualFold(landingPageID, "null") {
		landingPageID = strings.TrimSpace(dto.PromotionCode)
	}
	if (landingPageID == "" || strings.EqualFold(landingPageID, "null")) && profile != nil {
		landingPageID = profile.LandingPageID
	}

	// 2. 支付时间清洗 (番茄返回 UTC 秒级时间戳，换算为 BJ, ET, UTC)
	payTs := parseEpochSecondSafe(dto.CompletedAt, dto.CreatedAt)
	payInstant := time.Unix(payTs, 0)
	payTimeBj := payInstant.In(timeutil.BeijingZone)
	payTimeEt := payInstant.In(timeutil.EasternZone)
	payDateEt := payTimeEt.Format(timeutil.DateLayout)
	payTimeUtc := payInstant.UTC()
	payDateUtc := payTimeUtc.Format(timeutil.DateLayout)

	// 3. 染色归因时间判定
	var relationTimeBj *time.Time
	if rID := strings.TrimSpace(dto.RelationID); rID != "" {
		if t, ok := relationTimeMap[rID]; ok && !t.IsZero() {
			relationTimeBj = &t
		} else if m.flicknovelRepo != nil {
			localRel, _ := m.flicknovelRepo.FindRelationByID(ctx, rID)
			if localRel != nil && localRel.RelationBeginTimeBJ != nil {
				relationTimeBj = localRel.RelationBeginTimeBJ
			}
		}
	}

	// 4. 首充/续订判断 (renew_type) 与用户注册/归因时间 (register_time) 清洗
	var renewType int
	var regTimeBj time.Time
	var regTimeEt time.Time

	if profile.EarliestPayTime == nil {
		// 该用户在库中尚未有更早订单，本笔订单为首充！
		renewType = 1
		if relationTimeBj != nil {
			regTimeBj = *relationTimeBj
			regTimeEt = relationTimeBj.In(timeutil.EasternZone)
		} else {
			regTimeBj = payTimeBj
			regTimeEt = payTimeEt
		}
		profile.EarliestPayTime = &payTimeBj
		profile.EarliestRegTime = &regTimeBj
		if profile.LandingPageID == "" && landingPageID != "" {
			profile.LandingPageID = landingPageID
		}
	} else {
		// 库中或本批已有更早订单
		if payTimeBj.After(*profile.EarliestPayTime) {
			// 晚于首单，属于老用户复充！
			renewType = 2
			if profile.EarliestRegTime != nil {
				regTimeBj = *profile.EarliestRegTime
			} else {
				regTimeBj = *profile.EarliestPayTime
			}
			regTimeEt = regTimeBj.In(timeutil.EasternZone)
		} else {
			// 当前订单时间更早，则当前为首单
			renewType = 1
			if relationTimeBj != nil {
				regTimeBj = *relationTimeBj
				regTimeEt = relationTimeBj.In(timeutil.EasternZone)
			} else {
				regTimeBj = payTimeBj
				regTimeEt = payTimeEt
			}
			profile.EarliestPayTime = &payTimeBj
			profile.EarliestRegTime = &regTimeBj
		}
	}

	regDateEt := regTimeEt.Format(timeutil.DateLayout)
	regTimeUtc := regTimeBj.UTC()
	regDateUtc := regTimeUtc.Format(timeutil.DateLayout)

	// 5. 金额清洗 (us_price 美元字符串 -> Decimal 与 美分整数)
	amtUsd, _ := decimal.NewFromString(strings.TrimSpace(dto.USPrice))
	if amtUsd.IsZero() {
		amtUsd, _ = decimal.NewFromString(strings.TrimSpace(dto.OrderAmountUSD))
	}
	orderAmountCent := dto.OrderAmountCent
	if orderAmountCent == 0 && amtUsd.GreaterThan(decimal.Zero) {
		orderAmountCent = int(amtUsd.Mul(decimal.NewFromInt(100)).IntPart())
	}

	// 6. 充值类型清洗识别 (0=单充/代币充值, 1=时长订阅)
	resolveCtx := &OrderResolveContext{
		Dto:             dto,
		PromotionID:     landingPageID,
		OrderAmountCent: orderAmountCent,
		RenewType:       renewType,
		PayTimeBJ:       payTimeBj,
		PayTimeUTC:      payTimeUtc,
		HasSubscribed:   profile.HasSubscribed,
	}
	if profile.LatestSubsPayTime != nil {
		resolveCtx.LatestSubsPayTime = *profile.LatestSubsPayTime
	}

	m.populateTemplateContext(ctx, landingPageID, resolveCtx)
	isSubs := m.fnResolver.Resolve(resolveCtx)
	if isSubs == 1 {
		profile.HasSubscribed = true
		profile.LatestSubsPayTime = &payTimeBj
	}

	rawPayload, _ := json.Marshal(dto)

	return &model.RawOrder{
		PlatformCode:    model.PlatformFlicknovel,
		OrderID:         orderID,
		MemberID:        memberID,
		LandingPageID:   landingPageID,
		RegisterTimeBJ:  regTimeBj,
		RegisterTimeET:  regTimeEt,
		RegisterDateET:  regDateEt,
		RegisterTimeUTC: &regTimeUtc,
		RegisterDateUTC: regDateUtc,
		PayTimeBJ:       payTimeBj,
		PayTimeET:       payTimeEt,
		PayDateET:       payDateEt,
		PayTimeUTC:      &payTimeUtc,
		PayDateUTC:      payDateUtc,
		OrderAmountCent: orderAmountCent,
		OrderAmountUSD:  amtUsd,
		IsSubs:          isSubs,
		RenewType:       renewType,
		PayState:        1,
		RefundStatus:    dto.RefundStatus,
		RawPayload:      string(rawPayload),
		CreatedAt:       time.Now(),
	}
}

// SyncFlicknovelOrders 同步番茄司南订单 (按 25 天自然区间分段拉取，对齐 Java 架构)
func (m *SyncManager) SyncFlicknovelOrders(ctx context.Context, startDate, endDate string) (int, error) {
	if m.fnClient == nil {
		return 0, fmt.Errorf("flicknovel client not configured")
	}

	startStr := strings.TrimSpace(startDate)
	if len(startStr) >= 10 {
		startStr = startStr[:10]
	}
	if startStr == "" {
		startStr = model.LaunchStartDateFlicknovel
	}

	endStr := strings.TrimSpace(endDate)
	if len(endStr) >= 10 {
		endStr = endStr[:10]
	}
	todayBj := time.Now().In(timeutil.BeijingZone).Format(timeutil.DateLayout)
	if endStr == "" {
		endStr = todayBj
	}

	startDateParsed, err := time.ParseInLocation(timeutil.DateLayout, startStr, timeutil.BeijingZone)
	if err != nil {
		startDateParsed, _ = time.ParseInLocation(timeutil.DateLayout, model.LaunchStartDateFlicknovel, timeutil.BeijingZone)
	}
	endDateParsed, err := time.ParseInLocation(timeutil.DateLayout, endStr, timeutil.BeijingZone)
	if err != nil {
		endDateParsed, _ = time.ParseInLocation(timeutil.DateLayout, todayBj, timeutil.BeijingZone)
	}
	if startDateParsed.After(endDateParsed) {
		startDateParsed = endDateParsed
	}

	m.logger.Info("Starting Flicknovel order sync",
		zap.String("platform", model.PlatformFlicknovel),
		zap.String("start_time", startStr),
		zap.String("end_time", endStr),
	)

	startSyncTime := time.Now()
	totalSynced := 0

	// 25 天分段拉取，避免大时间跨度查询被三方网关限流或超时
	currStart := startDateParsed
	for !currStart.After(endDateParsed) {
		currEnd := currStart.AddDate(0, 0, 24)
		if currEnd.After(endDateParsed) {
			currEnd = endDateParsed
		}

		segStartStr := currStart.Format(timeutil.DateLayout)
		segEndStr := currEnd.Format(timeutil.DateLayout)
		beginTs := currStart.Unix()
		endTs := currEnd.AddDate(0, 0, 1).Unix()

		// 预拉取本区间内的染色归因记录，构建 relation_id -> relation_begin_time 映射
		relationTimeMap := m.fetchRelationBeginTimes(ctx, beginTs, endTs)

		pageIndex := int64(1)
		pageSize := int64(500)

		for {
			data, err := m.fnClient.QueryOrders(ctx, flicknovel.OrderQueryRequest{
				BeginTs:  beginTs,
				EndTs:    endTs,
				Page:     pageIndex,
				PageSize: pageSize,
			})
			if err != nil {
				m.logger.Error("Failed to query Flicknovel orders",
					zap.String("segment", fmt.Sprintf("%s ~ %s", segStartStr, segEndStr)),
					zap.Int64("page", pageIndex),
					zap.Error(err),
				)
				return totalSynced, fmt.Errorf("query flicknovel orders [%s ~ %s] page %d failed: %w", segStartStr, segEndStr, pageIndex, err)
			}

			if len(data.Orders) == 0 {
				break
			}

			// 执行批量清洗入库
			cleaned, cleanErr := m.batchCleanAndSaveFlicknovelOrders(ctx, data.Orders, relationTimeMap)
			if cleanErr != nil {
				return totalSynced, cleanErr
			}
			totalSynced += cleaned

			if len(data.Orders) < int(pageSize) {
				break
			}
			pageIndex++
		}

		currStart = currEnd.AddDate(0, 0, 1)
	}

	m.logger.Info("Finished Flicknovel order sync",
		zap.String("platform", model.PlatformFlicknovel),
		zap.String("range", fmt.Sprintf("%s ~ %s", startStr, endStr)),
		zap.Int("total_saved_orders", totalSynced),
		zap.Duration("duration", time.Since(startSyncTime)),
	)

	return totalSynced, nil
}

// syncPromotionsAndTemplatesInternal 内部同步推广链接和充值模板 (带防抖频控)
func (m *SyncManager) syncPromotionsAndTemplatesInternal(ctx context.Context, force bool) error {
	now := time.Now()
	m.fnCacheMu.Lock()
	if !force && now.Sub(m.lastFnSyncTime) < 30*time.Second {
		m.fnCacheMu.Unlock()
		return nil
	}
	m.fnCacheMu.Unlock()

	return m.SyncFlicknovelPromotionsAndTemplates(ctx)
}

// SyncFlicknovelPromotionsAndTemplates 同步番茄司南推广链接与充值模板
func (m *SyncManager) SyncFlicknovelPromotionsAndTemplates(ctx context.Context) error {
	if m.fnClient == nil {
		return fmt.Errorf("flicknovel client not configured")
	}
	m.logger.Info("Starting Flicknovel promotions and templates sync")
	startTime := time.Now()
	pageIndex := 1
	pageSize := 50
	distAppIDs := make(map[int64]bool)
	totalPromotions := 0

	for {
		pData, err := m.fnClient.QueryPromotions(ctx, flicknovel.PromotionQueryRequest{
			PageIndex: pageIndex,
			PageSize:  int64(pageSize),
		})
		if err != nil {
			return fmt.Errorf("query flicknovel promotions failed: %w", err)
		}

		if len(pData.Promotions) == 0 {
			break
		}

		promotions := make([]*model.FlicknovelPromotion, 0, len(pData.Promotions))
		for _, p := range pData.Promotions {
			if p.DistAppID != 0 {
				distAppIDs[p.DistAppID] = true
			}
			rawPayload, _ := json.Marshal(p)
			promotions = append(promotions, &model.FlicknovelPromotion{
				PromotionID:     p.PromotionID,
				PromotionName:   p.PromotionName,
				RechargeTplID:   p.RechargeTplID,
				RechargeTplName: p.RechargeTplName,
				DistAppID:       p.DistAppID,
				DramaID:         p.DramaID,
				DramaTitle:      p.DramaTitle,
				ChapterID:       p.ChapterID,
				ChapterTitle:    p.ChapterTitle,
				MediaChannel:    p.MediaChannel,
				RawPayload:      string(rawPayload),
				CreatedAt:       time.Now(),
				UpdatedAt:       time.Now(),
			})
		}

		if err := m.flicknovelRepo.BatchUpsertPromotions(ctx, promotions); err != nil {
			return fmt.Errorf("upsert flicknovel promotions failed: %w", err)
		}

		totalPromotions += len(promotions)

		if int64(pageIndex*pageSize) >= pData.TotalCount {
			break
		}
		pageIndex++
	}

	// 遍历 distAppIDs 拉取充值模板
	totalTemplates := 0
	for distAppID := range distAppIDs {
		tData, err := m.fnClient.QueryRechargeTemplates(ctx, flicknovel.RechargeTemplateQueryRequest{
			DistAppID: distAppID,
		})
		if err == nil && tData != nil && len(tData.Templates) > 0 {
			templates := make([]*model.FlicknovelRechargeTemplate, 0, len(tData.Templates))
			for _, t := range tData.Templates {
				rawPayload, _ := json.Marshal(t)
				templates = append(templates, &model.FlicknovelRechargeTemplate{
					TemplateID:      t.TemplateID,
					Name:            t.Name,
					DistAppID:       t.DistAppID,
					PriceConfigJSON: t.PriceConfigJSON,
					RawPayload:      string(rawPayload),
					CreatedAt:       time.Now(),
					UpdatedAt:       time.Now(),
				})
			}
			_ = m.flicknovelRepo.BatchUpsertTemplates(ctx, templates)
			totalTemplates += len(templates)
		}
	}

	// 重新从本地数据库刷新缓存字典
	m.loadFlicknovelCacheFromDB(ctx)

	m.fnCacheMu.Lock()
	m.lastFnSyncTime = time.Now()
	m.fnCacheMu.Unlock()

	m.logger.Info("Finished Flicknovel promotions and templates sync",
		zap.Int("total_promotions", totalPromotions),
		zap.Int("total_templates", totalTemplates),
		zap.Duration("duration", time.Since(startTime)),
	)

	return nil
}

// SyncFlicknovelRelations 同步番茄司南染色归因
func (m *SyncManager) SyncFlicknovelRelations(ctx context.Context, startDate, endDate string) (int, error) {
	if m.fnClient == nil {
		return 0, fmt.Errorf("flicknovel client not configured")
	}
	m.logger.Info("Starting Flicknovel relations sync", zap.String("start_date", startDate), zap.String("end_date", endDate))
	startTime := time.Now()

	startStr := strings.TrimSpace(startDate)
	if len(startStr) >= 10 {
		startStr = startStr[:10]
	}
	endStr := strings.TrimSpace(endDate)
	if len(endStr) >= 10 {
		endStr = endStr[:10]
	}
	todayBj := time.Now().In(timeutil.BeijingZone)
	if startStr == "" {
		startStr = todayBj.AddDate(0, 0, -2).Format(timeutil.DateLayout)
	}
	if endStr == "" {
		endStr = todayBj.Format(timeutil.DateLayout)
	}

	sDate, err := time.ParseInLocation(timeutil.DateLayout, startStr, timeutil.BeijingZone)
	if err != nil {
		sDate = todayBj.AddDate(0, 0, -2)
	}
	eDate, err := time.ParseInLocation(timeutil.DateLayout, endStr, timeutil.BeijingZone)
	if err != nil {
		eDate = todayBj
	}
	if sDate.After(eDate) {
		sDate, eDate = eDate, sDate
	}

	beginTs := sDate.Unix()
	endTs := eDate.AddDate(0, 0, 1).Unix()

	pageIndex := int64(1)
	pageSize := int64(1000)
	totalSynced := 0

	for {
		data, err := m.fnClient.QueryRelations(ctx, flicknovel.RelationQueryRequest{
			BeginTs:  beginTs,
			EndTs:    endTs,
			Page:     pageIndex,
			PageSize: pageSize,
		})
		if err != nil {
			return totalSynced, fmt.Errorf("query flicknovel relations page %d failed: %w", pageIndex, err)
		}

		if len(data.Relations) == 0 {
			break
		}

		relations := make([]*model.FlicknovelRelation, 0, len(data.Relations))
		for _, rec := range data.Relations {
			var regBj *time.Time
			var regEt *time.Time
			var regTs int64
			regDateEt := ""

			timeStr := strings.TrimSpace(rec.RelationBeginTime)
			if sec, err := strconv.ParseInt(timeStr, 10, 64); err == nil && sec > 0 {
				regTs = sec
				bj := time.Unix(sec, 0).In(timeutil.BeijingZone)
				regBj = &bj
				et := bj.In(timeutil.EasternZone)
				regEt = &et
				regDateEt = et.Format(timeutil.DateLayout)
			} else if t, err := time.ParseInLocation(timeutil.DateTimeLayout, timeStr, timeutil.BeijingZone); err == nil {
				regBj = &t
				regTs = t.Unix()
				et := t.In(timeutil.EasternZone)
				regEt = &et
				regDateEt = et.Format(timeutil.DateLayout)
			}

			if rec.RelationBeginTimestamp == 0 && regTs > 0 {
				rec.RelationBeginTimestamp = regTs
			}

			rawPayload, _ := json.Marshal(rec)
			relations = append(relations, &model.FlicknovelRelation{
				RelationID:             rec.RelationID,
				DeviceID:               rec.DeviceID,
				PromotionID:            rec.PromotionID,
				PromotionCode:          rec.PromotionCode,
				AdID:                   rec.AdID,
				AdsetID:                rec.AdsetID,
				CampaignID:             rec.CampaignID,
				AdAccountID:            rec.AdAccountID,
				RelationBeginTimeBJ:    regBj,
				RelationBeginTimeET:    regEt,
				RelationBeginDateET:    regDateEt,
				RelationBeginTimestamp: rec.RelationBeginTimestamp,
				MediaChannel:           rec.MediaChannel,
				Platform:               rec.Platform,
				AppID:                  rec.AppID,
				RawPayload:             string(rawPayload),
				CreatedAt:              time.Now(),
				UpdatedAt:              time.Now(),
			})
		}

		if err := m.flicknovelRepo.BatchUpsertRelations(ctx, relations); err != nil {
			return totalSynced, fmt.Errorf("upsert flicknovel relations failed: %w", err)
		}

		totalSynced += len(relations)

		if len(data.Relations) < int(pageSize) {
			break
		}
		pageIndex++
	}

	m.logger.Info("Finished Flicknovel relations sync",
		zap.Int("total_saved_relations", totalSynced),
		zap.Duration("duration", time.Since(startTime)),
	)

	return totalSynced, nil
}
