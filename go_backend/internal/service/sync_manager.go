package service

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"go_backend/internal/model"
	"go_backend/internal/pkg/timeutil"
	"go_backend/internal/repository"
	"go_backend/internal/service/client/flicknovel"
	"go_backend/internal/service/client/rocnovel"

	"github.com/shopspring/decimal"
	"go.uber.org/zap"
)

type SyncManager struct {
	orderRepo      *repository.OrderRepository
	flicknovelRepo *repository.FlicknovelRepository
	platformRepo   *repository.PlatformRepository
	rocnovelClient *rocnovel.Client
	fnClient       *flicknovel.Client
	logger         *zap.Logger
}

func NewSyncManager(
	orderRepo *repository.OrderRepository,
	flicknovelRepo *repository.FlicknovelRepository,
	platformRepo *repository.PlatformRepository,
	rocnovelClient *rocnovel.Client,
	fnClient *flicknovel.Client,
	logger *zap.Logger,
) *SyncManager {
	if logger == nil {
		logger = zap.NewNop()
	}
	return &SyncManager{
		orderRepo:      orderRepo,
		flicknovelRepo: flicknovelRepo,
		platformRepo:   platformRepo,
		rocnovelClient: rocnovelClient,
		fnClient:       fnClient,
		logger:         logger,
	}
}

// syncRocnovelSingleDay 同步单日 Rocnovel 订单
func (m *SyncManager) syncRocnovelSingleDay(ctx context.Context, dayStr, auth, cookie string) (int, error) {
	dayStart := dayStr + " 00:00:00"
	dayEnd := dayStr + " 23:59:59"
	pageIndex := 1
	pageSize := 100
	savedCount := 0

	for {
		data, err := m.rocnovelClient.FetchOrdersPage(ctx, pageIndex, pageSize, dayStart, dayEnd, "", auth, cookie)
		if err != nil {
			return savedCount, err
		}

		if len(data.Records) == 0 {
			break
		}

		rawOrders := make([]*model.RawOrder, 0, len(data.Records))
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
				PlatformCode:    "rocnovel",
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
			rawOrders = append(rawOrders, order)
		}

		if len(rawOrders) > 0 {
			if err := m.orderRepo.BatchUpsert(ctx, rawOrders); err != nil {
				return savedCount, fmt.Errorf("upsert rocnovel orders for %s failed: %w", dayStr, err)
			}
			savedCount += len(rawOrders)
		}

		if int64(pageIndex*pageSize) >= data.Total || (data.Pages > 0 && pageIndex >= data.Pages) {
			break
		}
		pageIndex++
	}

	return savedCount, nil
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
		startStr = "2026-07-10"
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
		startDate, _ = time.ParseInLocation(timeutil.DateLayout, "2026-07-10", timeutil.BeijingZone)
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
		zap.String("platform", "rocnovel"),
		zap.String("start_time", startStr),
		zap.String("end_time", endStr),
		zap.Int("total_days", totalDays),
	)

	startSyncTime := time.Now()
	concurrency := 5
	sem := make(chan struct{}, concurrency)
	var wg sync.WaitGroup
	var totalSaved atomic.Int64
	var completedDays atomic.Int64
	var tokenExpired atomic.Bool
	var firstErr error
	var errOnce sync.Once

	for _, day := range targetDays {
		if tokenExpired.Load() {
			break
		}

		sem <- struct{}{}
		wg.Add(1)

		go func(dayStr string) {
			defer func() {
				<-sem
				wg.Done()
			}()

			if tokenExpired.Load() {
				return
			}

			daySaved, err := m.syncRocnovelSingleDay(ctx, dayStr, auth, cookie)
			if err != nil {
				if strings.Contains(err.Error(), "TOKEN_EXPIRED") {
					tokenExpired.Store(true)
					errOnce.Do(func() { firstErr = err })
					m.logger.Warn("Rocnovel token expired during sync", zap.String("day", dayStr))
					return
				}
				m.logger.Warn("Failed to sync Rocnovel orders for day", zap.String("day", dayStr), zap.Error(err))
			} else {
				done := completedDays.Add(1)
				accum := totalSaved.Add(int64(daySaved))
				if daySaved > 0 || done%5 == 0 || int(done) == totalDays {
					m.logger.Info("Rocnovel day sync progress",
						zap.String("day", dayStr),
						zap.Int("day_orders", daySaved),
						zap.Int64("completed_days", done),
						zap.Int("total_days", totalDays),
						zap.Int64("accumulated_orders", accum),
					)
				}
			}
		}(day)
	}

	wg.Wait()

	if tokenExpired.Load() && firstErr != nil {
		return int(totalSaved.Load()), firstErr
	}

	m.logger.Info("Finished concurrent Rocnovel order sync",
		zap.String("platform", "rocnovel"),
		zap.String("range", fmt.Sprintf("%s ~ %s", startStr, endStr)),
		zap.Int64("total_saved_orders", totalSaved.Load()),
		zap.Duration("duration", time.Since(startSyncTime)),
	)

	return int(totalSaved.Load()), nil
}

// SyncOrdersForPlatform 按平台同步订单
func (m *SyncManager) SyncOrdersForPlatform(ctx context.Context, platformCode, startTime, endTime string) (int, error) {
	pCode := strings.ToLower(strings.TrimSpace(platformCode))
	if pCode == "rocnovel" {
		return m.SyncRocnovelOrders(ctx, startTime, endTime)
	}
	if pCode == "flicknovel" {
		return m.SyncFlicknovelOrders(ctx, startTime, endTime)
	}

	// ALL 或空：全平台同步
	n1, err1 := m.SyncRocnovelOrders(ctx, startTime, endTime)
	if err1 != nil && strings.Contains(err1.Error(), "TOKEN_EXPIRED") {
		return n1, err1
	}
	n2, _ := m.SyncFlicknovelOrders(ctx, startTime, endTime)
	return n1 + n2, err1
}

// SyncOrdersAllPlatforms 同步全平台订单
func (m *SyncManager) SyncOrdersAllPlatforms(ctx context.Context, startTime, endTime string) error {
	_, err := m.SyncOrdersForPlatform(ctx, "ALL", startTime, endTime)
	return err
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
		startStr = "2026-09-16"
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
		startDateParsed, _ = time.ParseInLocation(timeutil.DateLayout, "2026-09-16", timeutil.BeijingZone)
	}
	endDateParsed, err := time.ParseInLocation(timeutil.DateLayout, endStr, timeutil.BeijingZone)
	if err != nil {
		endDateParsed, _ = time.ParseInLocation(timeutil.DateLayout, todayBj, timeutil.BeijingZone)
	}
	if startDateParsed.After(endDateParsed) {
		startDateParsed = endDateParsed
	}

	m.logger.Info("Starting Flicknovel order sync",
		zap.String("platform", "flicknovel"),
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

		pageIndex := 1
		pageSize := 200

		for {
			data, err := m.fnClient.QueryOrders(ctx, flicknovel.OrderQueryRequest{
				StartTime: segStartStr,
				EndTime:   segEndStr,
				PageIndex: pageIndex,
				PageSize:  pageSize,
			})
			if err != nil {
				m.logger.Error("Failed to query Flicknovel orders",
					zap.String("segment", fmt.Sprintf("%s ~ %s", segStartStr, segEndStr)),
					zap.Int("page", pageIndex),
					zap.Error(err),
				)
				return totalSynced, fmt.Errorf("query flicknovel orders [%s ~ %s] page %d failed: %w", segStartStr, segEndStr, pageIndex, err)
			}

			if len(data.Orders) == 0 {
				break
			}

			rawOrders := make([]*model.RawOrder, 0, len(data.Orders))
			for _, rec := range data.Orders {
				orderID := strings.TrimSpace(rec.OrderID)
				if orderID == "" {
					continue
				}

				regStr := strings.TrimSpace(rec.RegisterTime)
				payStr := strings.TrimSpace(rec.PayTime)
				if regStr == "" || payStr == "" {
					continue
				}

				regBj, err1 := time.ParseInLocation(timeutil.DateTimeLayout, regStr, timeutil.BeijingZone)
				payBj, err2 := time.ParseInLocation(timeutil.DateTimeLayout, payStr, timeutil.BeijingZone)
				if err1 != nil || err2 != nil || regBj.IsZero() || payBj.IsZero() {
					continue
				}

				regEt := regBj.In(timeutil.EasternZone)
				regUtc := regBj.UTC()
				payEt := payBj.In(timeutil.EasternZone)
				payUtc := payBj.UTC()

				amtUsd, _ := decimal.NewFromString(rec.OrderAmountUSD)
				if amtUsd.IsZero() && rec.OrderAmountCent > 0 {
					amtUsd = decimal.NewFromInt(int64(rec.OrderAmountCent)).Div(decimal.NewFromInt(100))
				}

				order := &model.RawOrder{
					PlatformCode:    "flicknovel",
					OrderID:         orderID,
					MemberID:        strings.TrimSpace(rec.UserID),
					LandingPageID:   strings.TrimSpace(rec.PromotionID),
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
				rawOrders = append(rawOrders, order)
			}

			if len(rawOrders) > 0 {
				if err := m.orderRepo.BatchUpsert(ctx, rawOrders); err != nil {
					return totalSynced, fmt.Errorf("upsert flicknovel orders failed: %w", err)
				}
				totalSynced += len(rawOrders)
			}

			m.logger.Info("Flicknovel order segment progress",
				zap.String("segment", fmt.Sprintf("%s ~ %s", segStartStr, segEndStr)),
				zap.Int("page", pageIndex),
				zap.Int("page_orders", len(rawOrders)),
				zap.Int64("segment_total", data.TotalCount),
				zap.Int("total_synced", totalSynced),
			)

			if int64(pageIndex*pageSize) >= data.TotalCount {
				break
			}
			pageIndex++
		}

		currStart = currEnd.AddDate(0, 0, 1)
	}

	m.logger.Info("Finished Flicknovel order sync",
		zap.String("platform", "flicknovel"),
		zap.String("range", fmt.Sprintf("%s ~ %s", startStr, endStr)),
		zap.Int("total_saved_orders", totalSynced),
		zap.Duration("duration", time.Since(startSyncTime)),
	)

	return totalSynced, nil
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
			PageSize:  pageSize,
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
		m.logger.Info("Flicknovel promotions sync progress",
			zap.Int("page", pageIndex),
			zap.Int("page_count", len(promotions)),
			zap.Int("accumulated", totalPromotions),
			zap.Int64("total_count", pData.TotalCount),
		)

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
	pageIndex := 1
	pageSize := 200
	totalSynced := 0

	for {
		data, err := m.fnClient.QueryRelations(ctx, flicknovel.RelationQueryRequest{
			StartTime: startDate,
			EndTime:   endDate,
			PageIndex: pageIndex,
			PageSize:  pageSize,
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
			regDateEt := ""
			if t, err := time.ParseInLocation(timeutil.DateTimeLayout, rec.RelationBeginTime, timeutil.BeijingZone); err == nil {
				regBj = &t
				et := t.In(timeutil.EasternZone)
				regEt = &et
				regDateEt = et.Format(timeutil.DateLayout)
			}

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
				CreatedAt:              time.Now(),
				UpdatedAt:              time.Now(),
			})
		}

		if err := m.flicknovelRepo.BatchUpsertRelations(ctx, relations); err != nil {
			return totalSynced, fmt.Errorf("upsert flicknovel relations failed: %w", err)
		}

		totalSynced += len(relations)
		m.logger.Info("Flicknovel relations sync progress",
			zap.Int("page", pageIndex),
			zap.Int("page_count", len(relations)),
			zap.Int("accumulated", totalSynced),
			zap.Int64("total_count", data.TotalCount),
		)

		if int64(pageIndex*pageSize) >= data.TotalCount {
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

// SyncConfigsForPlatform 根据平台同步配置快照
func (m *SyncManager) SyncConfigsForPlatform(ctx context.Context, platformCode string) (int, error) {
	if strings.EqualFold(platformCode, "flicknovel") {
		err := m.SyncFlicknovelPromotionsAndTemplates(ctx)
		if err != nil {
			return 0, err
		}
		count, _ := m.flicknovelRepo.CountPromotions(ctx)
		return int(count), nil
	}
	return 1, nil
}

func (m *SyncManager) GetFlicknovelClient() *flicknovel.Client {
	return m.fnClient
}

func (m *SyncManager) GetFlicknovelRepo() *repository.FlicknovelRepository {
	return m.flicknovelRepo
}
