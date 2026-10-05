package service

import (
	"context"
	"fmt"
	"time"

	"go_backend/internal/model"
	"go_backend/internal/pkg/timeutil"
	"go_backend/internal/repository"
	"go_backend/internal/service/client/flicknovel"
	"go_backend/internal/service/client/rocnovel"

	"github.com/shopspring/decimal"
)

type SyncManager struct {
	orderRepo      *repository.OrderRepository
	flicknovelRepo *repository.FlicknovelRepository
	rocnovelClient *rocnovel.Client
	fnClient       *flicknovel.Client
}

func NewSyncManager(
	orderRepo *repository.OrderRepository,
	flicknovelRepo *repository.FlicknovelRepository,
	rocnovelClient *rocnovel.Client,
	fnClient *flicknovel.Client,
) *SyncManager {
	return &SyncManager{
		orderRepo:      orderRepo,
		flicknovelRepo: flicknovelRepo,
		rocnovelClient: rocnovelClient,
		fnClient:       fnClient,
	}
}

// SyncRocnovelOrders 同步中文在线订单
func (m *SyncManager) SyncRocnovelOrders(ctx context.Context, startTime, endTime string) (int, error) {
	pageIndex := 1
	pageSize := 100
	totalSynced := 0

	for {
		data, err := m.rocnovelClient.FetchOrdersPage(ctx, pageIndex, pageSize, startTime, endTime)
		if err != nil {
			return totalSynced, fmt.Errorf("fetch rocnovel orders page %d failed: %w", pageIndex, err)
		}

		if len(data.Records) == 0 {
			break
		}

		rawOrders := make([]*model.RawOrder, 0, len(data.Records))
		for _, rec := range data.Records {
			amtUsd, _ := decimal.NewFromString(rec.OrderAmountUSD)
			if amtUsd.IsZero() && rec.OrderAmountCent > 0 {
				amtUsd = decimal.NewFromInt(int64(rec.OrderAmountCent)).Div(decimal.NewFromInt(100))
			}

			// 解析北京时间与美东时间
			regBj, _ := time.ParseInLocation(timeutil.DateTimeLayout, rec.RegisterTime, timeutil.BeijingZone)
			regEt := regBj.In(timeutil.EasternZone)
			payBj, _ := time.ParseInLocation(timeutil.DateTimeLayout, rec.PayTime, timeutil.BeijingZone)
			payEt := payBj.In(timeutil.EasternZone)

			order := &model.RawOrder{
				PlatformCode:    "rocnovel",
				OrderID:         rec.OrderNo,
				MemberID:        rec.MemberID,
				LandingPageID:   rec.LandingPageID,
				RegisterTimeBJ:  regBj,
				RegisterTimeET:  regEt,
				RegisterDateET:  regEt.Format(timeutil.DateLayout),
				PayTimeBJ:       payBj,
				PayTimeET:       payEt,
				PayDateET:       payEt.Format(timeutil.DateLayout),
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

		if err := m.orderRepo.BatchUpsert(ctx, rawOrders); err != nil {
			return totalSynced, fmt.Errorf("upsert rocnovel orders failed: %w", err)
		}

		totalSynced += len(rawOrders)
		if int64(pageIndex*pageSize) >= data.Total {
			break
		}
		pageIndex++
	}

	return totalSynced, nil
}

// SyncFlicknovelPromotionsAndTemplates 同步番茄司南推广链接与充值模板
func (m *SyncManager) SyncFlicknovelPromotionsAndTemplates(ctx context.Context) error {
	pageIndex := 1
	pageSize := 100

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
		tplIDs := make([]string, 0)
		for _, p := range pData.Promotions {
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
				CreatedAt:       time.Now(),
				UpdatedAt:       time.Now(),
			})
			if p.RechargeTplID != "" {
				tplIDs = append(tplIDs, p.RechargeTplID)
			}
		}

		if err := m.flicknovelRepo.BatchUpsertPromotions(ctx, promotions); err != nil {
			return err
		}

		// 同步关联的充值模板
		if len(tplIDs) > 0 {
			tData, err := m.fnClient.QueryRechargeTemplates(ctx, flicknovel.RechargeTemplateQueryRequest{
				TemplateIDs: tplIDs,
			})
			if err == nil && len(tData.Templates) > 0 {
				templates := make([]*model.FlicknovelRechargeTemplate, 0, len(tData.Templates))
				for _, t := range tData.Templates {
					templates = append(templates, &model.FlicknovelRechargeTemplate{
						TemplateID:      t.TemplateID,
						Name:            t.Name,
						DistAppID:       t.DistAppID,
						PriceConfigJSON: t.PriceConfigJSON,
						CreatedAt:       time.Now(),
						UpdatedAt:       time.Now(),
					})
				}
				_ = m.flicknovelRepo.BatchUpsertTemplates(ctx, templates)
			}
		}

		if int64(pageIndex*pageSize) >= pData.TotalCount {
			break
		}
		pageIndex++
	}

	return nil
}

// SyncOrdersAllPlatforms 同步全平台订单
func (m *SyncManager) SyncOrdersAllPlatforms(ctx context.Context, startTime, endTime string) error {
	_, err := m.SyncRocnovelOrders(ctx, startTime, endTime)
	return err
}
