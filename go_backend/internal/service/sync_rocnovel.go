package service

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"go_backend/internal/model"
	"go_backend/internal/pkg/pool"
	"go_backend/internal/pkg/timeutil"
	"go_backend/internal/service/client/rocnovel"

	"github.com/shopspring/decimal"
	"go.uber.org/zap"
)

// fetchRocnovelOrdersForDay 并行拉取单日 Rocnovel 订单，只做网络请求和解析，不写数据库
func (m *SyncManager) fetchRocnovelOrdersForDay(ctx context.Context, dayStr, auth, cookie string) ([]*model.RawOrder, error) {
	dayStart := dayStr + " 00:00:00"
	dayEnd := dayStr + " 23:59:59"
	pageIndex := 1
	pageSize := 500
	var allOrders []*model.RawOrder

	for {
		data, err := m.rocnovelClient.FetchOrdersPage(ctx, pageIndex, pageSize, dayStart, dayEnd, "", auth, cookie)
		if err != nil {
			return allOrders, err
		}

		if pageIndex == 1 && data != nil {
			m.logger.Info("Fetched Rocnovel orders page 1",
				zap.String("day", dayStr),
				zap.Int("requested_page_size", pageSize),
				zap.Int("returned_records", len(data.Records)),
				zap.Int64("total", data.Total),
				zap.Int("pages", data.Pages),
			)
		}

		if len(data.Records) == 0 {
			break
		}

		for _, rec := range data.Records {
			orderID := strings.TrimSpace(rec.OrderID)
			if orderID == "" || rec.PayState != 1 {
				continue
			}

			userCreateTime := strings.TrimSpace(rec.UserCreateTime)
			payDate := strings.TrimSpace(rec.PayDate)
			if userCreateTime == "" || payDate == "" {
				continue
			}

			regBj, err1 := time.ParseInLocation(timeutil.DateTimeLayout, userCreateTime, timeutil.BeijingZone)
			payBj, err2 := time.ParseInLocation(timeutil.DateTimeLayout, payDate, timeutil.BeijingZone)
			if err1 != nil || err2 != nil || regBj.IsZero() || payBj.IsZero() {
				continue
			}

			regEt := regBj.In(timeutil.EasternZone)
			payEt := payBj.In(timeutil.EasternZone)
			regUtc := regBj.UTC()
			payUtc := payBj.UTC()

			amtUsd, _ := decimal.NewFromString(rec.OrderAmountUSD)
			if amtUsd.IsZero() && rec.OrderAmountCent > 0 {
				amtUsd = decimal.NewFromInt(int64(rec.OrderAmountCent)).Div(decimal.NewFromInt(100))
			}

			order := &model.RawOrder{
				PlatformCode:    model.PlatformRocnovel,
				OrderID:         orderID,
				MemberID:        strings.TrimSpace(rec.MemberID),
				LandingPageID:   strings.TrimSpace(rec.LandingPageID),
				RegisterTimeBJ:  regBj,
				RegisterTimeET:  regEt,
				RegisterDateET:  regEt.Format(timeutil.DateLayout),
				RegisterTimeUTC: &regUtc,
				RegisterDateUTC: regUtc.Format(timeutil.DateLayout),
				PayTimeBJ:       payBj,
				PayTimeET:       payEt,
				PayDateET:       payEt.Format(timeutil.DateLayout),
				PayTimeUTC:      &payUtc,
				PayDateUTC:      payUtc.Format(timeutil.DateLayout),
				OrderAmountCent: rec.OrderAmountCent,
				OrderAmountUSD:  amtUsd,
				IsSubs:          rec.IsSubs,
				RenewType:       rec.RenewType,
				PayState:        rec.PayState,
				RefundStatus:    rec.RefundStatus,
				CreatedAt:       time.Now(),
			}
			allOrders = append(allOrders, order)
		}

		if int64(pageIndex*pageSize) >= data.Total || (data.Pages > 0 && pageIndex >= data.Pages) {
			break
		}
		pageIndex++
	}

	return allOrders, nil
}

// syncRocnovelSingleDay 同步单日 Rocnovel 订单（兼容单日调用）
func (m *SyncManager) syncRocnovelSingleDay(ctx context.Context, dayStr, auth, cookie string) (int, error) {
	orders, err := m.fetchRocnovelOrdersForDay(ctx, dayStr, auth, cookie)
	if err != nil {
		return 0, err
	}
	if len(orders) == 0 {
		return 0, nil
	}
	if err := m.orderRepo.BatchUpsert(ctx, orders); err != nil {
		return 0, fmt.Errorf("upsert rocnovel orders for %s failed: %w", dayStr, err)
	}
	// 维护首次订阅用户的周期配置表 (批量单条 SQL 查询与写入，杜绝 N+1 数据库风暴)
	m.BatchSaveOrUpdateSubscriptionPeriods(ctx, model.PlatformRocnovel, orders)
	return len(orders), nil
}

// SyncRocnovelOrders 同步中文在线订单 (按日并发多工拉取)
func (m *SyncManager) SyncRocnovelOrders(ctx context.Context, startTime, endTime string) (int, error) {
	auth := ""
	cookie := ""
	if m.platformRepo != nil {
		auth, _ = m.platformRepo.GetSystemConfig(ctx, "API_AUTHORIZATION")
		cookie, _ = m.platformRepo.GetSystemConfig(ctx, "API_COOKIE")
	}

	startStr := strings.TrimSpace(startTime)
	if len(startStr) >= 10 {
		startStr = startStr[:10]
	}
	if startStr == "" {
		startStr = model.LaunchStartDateRocnovel
	}

	endStr := strings.TrimSpace(endTime)
	if len(endStr) >= 10 {
		endStr = endStr[:10]
	}
	todayBj := time.Now().In(timeutil.BeijingZone).Format(timeutil.DateLayout)
	if endStr == "" {
		endStr = todayBj
	}

	startDate, err := time.ParseInLocation(timeutil.DateLayout, startStr, timeutil.BeijingZone)
	if err != nil {
		startDate, _ = time.ParseInLocation(timeutil.DateLayout, model.LaunchStartDateRocnovel, timeutil.BeijingZone)
	}
	endDate, err := time.ParseInLocation(timeutil.DateLayout, endStr, timeutil.BeijingZone)
	if err != nil {
		endDate, _ = time.ParseInLocation(timeutil.DateLayout, todayBj, timeutil.BeijingZone)
	}
	if startDate.After(endDate) {
		startDate = endDate
	}

	var targetDays []string
	for curr := startDate; !curr.After(endDate); curr = curr.AddDate(0, 0, 1) {
		targetDays = append(targetDays, curr.Format(timeutil.DateLayout))
	}

	totalDays := len(targetDays)
	m.logger.Info("Starting concurrent Rocnovel order sync",
		zap.String("platform", model.PlatformRocnovel),
		zap.String("start_time", startStr),
		zap.String("end_time", endStr),
		zap.Int("total_days", totalDays),
	)

	startSyncTime := time.Now()
	var completedDays atomic.Int64
	var tokenExpired atomic.Bool
	var firstErr error
	var errOnce sync.Once

	var failedDaysMu sync.Mutex
	var failedDays []string

	var allOrdersMu sync.Mutex
	var allOrders []*model.RawOrder
	var wg sync.WaitGroup

	// 全量日期流水线并发抓取：利用 ThirdPartyPool (容量 15) 持续工作，彻底废除 7 天串行阻塞切片
	for _, day := range targetDays {
		if tokenExpired.Load() || ctx.Err() != nil {
			break
		}

		wg.Add(1)
		dayStr := day
		task := func() {
			defer wg.Done()
			if tokenExpired.Load() || ctx.Err() != nil {
				return
			}

			// 并行拉取该天所有页的订单
			orders, err := m.fetchRocnovelOrdersForDay(ctx, dayStr, auth, cookie)
			if err != nil {
				if strings.Contains(err.Error(), "TOKEN_EXPIRED") {
					tokenExpired.Store(true)
					errOnce.Do(func() { firstErr = err })
					m.logger.Warn("Rocnovel token expired during sync", zap.String("day", dayStr))
					return
				}
				if ctx.Err() == nil {
					m.logger.Warn("Failed to fetch Rocnovel orders for day", zap.String("day", dayStr), zap.Error(err))
					failedDaysMu.Lock()
					failedDays = append(failedDays, dayStr)
					failedDaysMu.Unlock()
				}
			} else {
				completedDays.Add(1)
				if len(orders) > 0 {
					allOrdersMu.Lock()
					allOrders = append(allOrders, orders...)
					allOrdersMu.Unlock()
				}
			}
		}

		tpPool := pool.GetThirdPartyPool()
		if tpPool != nil {
			if err := tpPool.Submit(fmt.Sprintf("fetchRocnovelOrders:%s", dayStr), task); err != nil {
				pool.SafeGo(m.logger, fmt.Sprintf("fetchRocnovelOrdersFallback:%s", dayStr), task)
			}
		} else {
			pool.SafeGo(m.logger, fmt.Sprintf("fetchRocnovelOrdersFallback:%s", dayStr), task)
		}
	}

	wg.Wait()

	if tokenExpired.Load() {
		m.logger.Error("Rocnovel order sync aborted due to token expired",
			zap.Int64("completed_days", completedDays.Load()),
			zap.Int("total_days", totalDays),
		)
		if firstErr != nil {
			return 0, firstErr
		}
		return 0, fmt.Errorf("TOKEN_EXPIRED: 登录 Token 已过期，请在页面更新最新 Token")
	}

	if len(allOrders) == 0 {
		m.logger.Info("No Rocnovel orders fetched in range",
			zap.String("range", fmt.Sprintf("%s ~ %s", startStr, endStr)),
		)
		return 0, nil
	}

	// 工业级死锁防范：按主键/唯一索引 order_id 升序排序
	sort.Slice(allOrders, func(i, j int) bool {
		return allOrders[i].OrderID < allOrders[j].OrderID
	})

	// 无论外层请求上下文是否收到客户端中断，对内存中已拉取的宝贵数据执行脱钩保护写库，确保不丢失已抓取数据
	writeCtx, cancelWrite := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Minute)
	defer cancelWrite()

	batchSize := 500
	totalSaved := 0
	for i := 0; i < len(allOrders); i += batchSize {
		end := i + batchSize
		if end > len(allOrders) {
			end = len(allOrders)
		}
		batch := allOrders[i:end]

		var batchErr error
		for attempt := 0; attempt < 3; attempt++ {
			err := m.orderRepo.BatchUpsert(writeCtx, batch)
			if err == nil {
				totalSaved += len(batch)
				batchErr = nil
				// 维护首次订阅用户的周期配置表 (批量单条 SQL 查询与写入，杜绝 N+1 数据库风暴)
				m.BatchSaveOrUpdateSubscriptionPeriods(writeCtx, model.PlatformRocnovel, batch)
				break
			}
			batchErr = err
			if strings.Contains(err.Error(), "1213") || strings.Contains(err.Error(), "Deadlock") || strings.Contains(err.Error(), "1205") {
				m.logger.Warn("Deadlock/lock-wait encountered on batch upsert, retrying...",
					zap.Int("attempt", attempt+1),
					zap.Int("batch_start", i),
					zap.Error(err),
				)
				time.Sleep(time.Duration(100*(attempt+1)) * time.Millisecond)
				continue
			}
			break
		}

		if batchErr != nil {
			m.logger.Error("Failed to upsert Rocnovel order batch", zap.Int("batch_start", i), zap.Error(batchErr))
			return totalSaved, batchErr
		}
	}

	if tokenExpired.Load() {
		m.logger.Error("Rocnovel order sync aborted due to token expired",
			zap.Int64("completed_days", completedDays.Load()),
			zap.Int("total_days", totalDays),
		)
		if firstErr != nil {
			return totalSaved, firstErr
		}
		return totalSaved, fmt.Errorf("TOKEN_EXPIRED: 登录 Token 已过期，请在页面更新最新 Token")
	}

	m.logger.Info("Finished sorted batch Rocnovel order sync",
		zap.String("platform", model.PlatformRocnovel),
		zap.String("range", fmt.Sprintf("%s ~ %s", startStr, endStr)),
		zap.Int("total_saved_orders", totalSaved),
		zap.Int("failed_days_count", len(failedDays)),
		zap.Duration("duration", time.Since(startSyncTime)),
	)

	if len(failedDays) > 0 {
		m.logger.Warn("Rocnovel order sync finished with partial day failures",
			zap.Strings("failed_days", failedDays),
		)
		return totalSaved, fmt.Errorf("sync rocnovel orders partially failed for %d days: %v", len(failedDays), failedDays)
	}

	return totalSaved, nil
}

// SyncRocnovelSubscribeConfigs 同步中文在线落地页配置及对应订阅产品明细版本 (对齐 Java RocnovelSubscribeConfigSyncService)
func (m *SyncManager) SyncRocnovelSubscribeConfigs(ctx context.Context) (int, error) {
	m.logger.Info("Starting sync of Rocnovel landing page & subscribe configs...")

	auth, _ := m.platformRepo.GetSystemConfig(ctx, "API_AUTHORIZATION")
	cookie, _ := m.platformRepo.GetSystemConfig(ctx, "API_COOKIE")

	var allPages []rocnovel.LandingPageRecordDto
	pageIndex := 1
	pageSize := 50
	totalPages := 1

	for {
		resp, err := m.rocnovelClient.FetchLandingPagesPage(ctx, pageIndex, pageSize, auth, cookie)
		if err != nil {
			m.logger.Error("Failed to fetch Rocnovel landing page config list page", zap.Int("pageIndex", pageIndex), zap.Error(err))
			if len(allPages) == 0 {
				return 0, fmt.Errorf("fetch rocnovel landing page config list page %d failed: %w", pageIndex, err)
			}
			break
		}
		if resp != nil && resp.Code == 0 && resp.Data != nil {
			if resp.Data.Pages > 0 {
				totalPages = resp.Data.Pages
			}
			if len(resp.Data.Records) > 0 {
				allPages = append(allPages, resp.Data.Records...)
			}
		}
		pageIndex++
		if pageIndex > totalPages {
			break
		}
	}

	m.logger.Info("Fetched total Rocnovel landing page records", zap.Int("count", len(allPages)))

	totalSavedVersions := 0
	processedConfigIDs := make(map[string]bool)

	for _, page := range allPages {
		pID := strings.TrimSpace(page.ID)
		configID := strings.TrimSpace(page.SubscribeConfigID)
		if pID == "" || configID == "" {
			continue
		}

		products, err := m.rocnovelClient.FetchSubscribeProductsForConfig(ctx, configID, auth, cookie)
		if err != nil {
			m.logger.Warn("Failed to fetch subscribe config products", zap.String("configID", configID), zap.Error(err))
			continue
		}

		for _, prod := range products {
			saved := m.processAndSaveRocnovelVersion(ctx, page, prod)
			if saved {
				totalSavedVersions++
			}
		}
		processedConfigIDs[configID] = true
	}

	m.logger.Info("Completed Rocnovel subscribe config sync",
		zap.Int("savedVersions", totalSavedVersions),
		zap.Int("distinctConfigs", len(processedConfigIDs)),
	)

	if totalSavedVersions > 0 {
		m.InvalidateSubscriptionConfigCache()
	}
	return totalSavedVersions, nil
}

func (m *SyncManager) processAndSaveRocnovelVersion(ctx context.Context, page rocnovel.LandingPageRecordDto, product rocnovel.SubscribeConfigProductRecordDto) bool {
	landingPageID := strings.TrimSpace(page.ID)
	productID := strings.TrimSpace(product.ID)
	if landingPageID == "" || productID == "" {
		return false
	}

	subPeriodDays := parseRocnovelSubPeriodDays(product.CycleStr, product.Cycle)
	firstPriceCent := parseUsdStringToCent(product.PreferentialPrice)
	renewPriceCent := parseUsdStringToCent(product.Price)

	eventTime := parseRocnovelTimeString(product.UpdateDateTime)
	if eventTime.IsZero() {
		eventTime = parseRocnovelTimeString(product.CreateDateTime)
	}
	if eventTime.IsZero() {
		eventTime = parseRocnovelTimeString(page.UpdateDateTime)
	}
	if eventTime.IsZero() {
		eventTime = time.Now()
	}

	latest, err := m.orderRepo.FindLatestVersionByPageAndProduct(ctx, landingPageID, productID)
	if err != nil {
		m.logger.Warn("Failed to find latest version by page and product", zap.Error(err))
		return false
	}

	if latest != nil {
		isChanged := latest.FirstPriceCent != firstPriceCent ||
			latest.RenewPriceCent != renewPriceCent ||
			latest.SubPeriodDays != subPeriodDays

		if isChanged {
			// 关闭旧版本
			latest.EffectiveEndTime = &eventTime
			_ = m.orderRepo.SaveSubscriptionVersion(ctx, latest)

			// 创建新版本
			newVersion := &model.SubscriptionConfigVersion{
				PlatformCode:        model.PlatformRocnovel,
				LandingPageID:       landingPageID,
				SubscribeConfigID:   strings.TrimSpace(page.SubscribeConfigID),
				SubscribeConfigName: strings.TrimSpace(page.SubscribeConfigName),
				SaleComboID:         strings.TrimSpace(page.SaleComboID),
				SaleComboName:       strings.TrimSpace(page.SaleComboName),
				ProductID:           productID,
				ProductName:         strings.TrimSpace(product.Name),
				SubPeriodDays:       subPeriodDays,
				FirstPriceCent:      firstPriceCent,
				RenewPriceCent:      renewPriceCent,
				VersionNum:          latest.VersionNum + 1,
				EffectiveStartTime:  eventTime,
				EffectiveEndTime:    nil,
			}
			_ = m.orderRepo.SaveSubscriptionVersion(ctx, newVersion)
			return true
		}
		return false
	}

	// 首次创建版本
	newVersion := &model.SubscriptionConfigVersion{
		PlatformCode:        model.PlatformRocnovel,
		LandingPageID:       landingPageID,
		SubscribeConfigID:   strings.TrimSpace(page.SubscribeConfigID),
		SubscribeConfigName: strings.TrimSpace(page.SubscribeConfigName),
		SaleComboID:         strings.TrimSpace(page.SaleComboID),
		SaleComboName:       strings.TrimSpace(page.SaleComboName),
		ProductID:           productID,
		ProductName:         strings.TrimSpace(product.Name),
		SubPeriodDays:       subPeriodDays,
		FirstPriceCent:      firstPriceCent,
		RenewPriceCent:      renewPriceCent,
		VersionNum:          1,
		EffectiveStartTime:  eventTime,
		EffectiveEndTime:    nil,
	}
	_ = m.orderRepo.SaveSubscriptionVersion(ctx, newVersion)
	return true
}

func parseRocnovelSubPeriodDays(cycleStr string, cycle int) int {
	if strings.TrimSpace(cycleStr) != "" {
		lower := strings.ToLower(strings.TrimSpace(cycleStr))
		if strings.Contains(lower, "1 day") || lower == "day" {
			return 1
		}
		if strings.Contains(lower, "3 day") {
			return 3
		}
		if strings.Contains(lower, "week") {
			return 7
		}
		if strings.Contains(lower, "month") {
			return 30
		}
		if strings.Contains(lower, "annual") || strings.Contains(lower, "year") {
			return 365
		}
	}
	if cycle > 0 {
		return cycle
	}
	return 1
}

func parseUsdStringToCent(usdStr string) int {
	s := strings.TrimSpace(usdStr)
	if s == "" {
		return 0
	}
	d, err := decimal.NewFromString(s)
	if err != nil {
		return 0
	}
	return int(d.Mul(decimal.NewFromInt(100)).Round(0).IntPart())
}

func parseRocnovelTimeString(timeStr string) time.Time {
	s := strings.TrimSpace(timeStr)
	if s == "" {
		return time.Time{}
	}
	t, err := time.ParseInLocation("2006-01-02 15:04:05", s, timeutil.BeijingZone)
	if err == nil {
		return t
	}
	t, err = time.ParseInLocation("2006-01-02", s, timeutil.BeijingZone)
	if err == nil {
		return t
	}
	return time.Time{}
}
