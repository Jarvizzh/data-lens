package service

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"go_backend/internal/model"
	"go_backend/internal/repository"
	"go_backend/internal/service/client/flicknovel"
	"go_backend/internal/service/client/rocnovel"

	"go.uber.org/zap"
)

// PlatformSyncer 定义各个第三方平台的同步驱动接口
type PlatformSyncer interface {
	PlatformCode() string
	SyncOrders(ctx context.Context, startTime, endTime string) (int, error)
	SyncConfigs(ctx context.Context) (int, error)
}

type SyncManager struct {
	orderRepo      *repository.OrderRepository
	flicknovelRepo *repository.FlicknovelRepository
	platformRepo   *repository.PlatformRepository
	rocnovelClient *rocnovel.Client
	fnClient       *flicknovel.Client
	fnResolver     *FlicknovelOrderTypeResolver
	logger         *zap.Logger

	// 平台驱动注册表
	syncersMu sync.RWMutex
	syncers   map[string]PlatformSyncer

	// 番茄司南内存字典缓存 (推广链接 -> 模板ID, 模板ID -> 详情字典)
	fnCacheMu              sync.RWMutex
	promotionTemplateMap   map[string]string
	templateDetailCache    map[string]*TemplatePriceDetail
	templatePriceTypeCache map[string]map[int]int
	lastFnSyncTime         time.Time
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
	sm := &SyncManager{
		orderRepo:              orderRepo,
		flicknovelRepo:         flicknovelRepo,
		platformRepo:           platformRepo,
		rocnovelClient:         rocnovelClient,
		fnClient:               fnClient,
		fnResolver:             NewFlicknovelOrderTypeResolver(logger),
		logger:                 logger,
		syncers:                make(map[string]PlatformSyncer),
		promotionTemplateMap:   make(map[string]string),
		templateDetailCache:    make(map[string]*TemplatePriceDetail),
		templatePriceTypeCache: make(map[string]map[int]int),
	}

	// 注册内置平台驱动
	sm.RegisterSyncer(&rocnovelSyncer{manager: sm})
	sm.RegisterSyncer(&flicknovelSyncer{manager: sm})

	// 异步预热番茄司南模板价格字典缓存与中文在线落地页配置
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
		defer cancel()
		sm.initFlicknovelCache(ctx)

		// 启动异步同步一次中文在线落地页与订阅产品明细快照 (对齐 Java onApplicationReady)
		sm.logger.Info("Starting asynchronous initial sync of Rocnovel landing page & subscribe configs...")
		if saved, err := sm.SyncRocnovelSubscribeConfigs(ctx); err == nil {
			sm.logger.Info("Initial asynchronous Rocnovel subscribe config sync finished", zap.Int("savedVersions", saved))
		} else {
			sm.logger.Error("Initial asynchronous Rocnovel subscribe config sync failed", zap.Error(err))
		}
	}()

	return sm
}

// RegisterSyncer 注册平台同步驱动
func (m *SyncManager) RegisterSyncer(syncer PlatformSyncer) {
	m.syncersMu.Lock()
	defer m.syncersMu.Unlock()
	m.syncers[strings.ToLower(syncer.PlatformCode())] = syncer
}

// GetSyncer 获取指定平台的同步驱动
func (m *SyncManager) GetSyncer(platformCode string) (PlatformSyncer, bool) {
	m.syncersMu.RLock()
	defer m.syncersMu.RUnlock()
	s, ok := m.syncers[strings.ToLower(platformCode)]
	return s, ok
}

// GetAllSyncers 获取所有已注册平台的驱动列表
func (m *SyncManager) GetAllSyncers() []PlatformSyncer {
	m.syncersMu.RLock()
	defer m.syncersMu.RUnlock()
	list := make([]PlatformSyncer, 0, len(m.syncers))
	for _, s := range m.syncers {
		list = append(list, s)
	}
	return list
}

// SyncOrdersForPlatform 按平台同步订单 (全平台或多平台时通过驱动注册表并发 Fan-out 执行)
func (m *SyncManager) SyncOrdersForPlatform(ctx context.Context, platformCode, startTime, endTime string) (int, error) {
	if !model.IsAllPlatforms(platformCode) {
		syncer, ok := m.GetSyncer(platformCode)
		if !ok {
			return 0, fmt.Errorf("unsupported platform: %s", platformCode)
		}
		return syncer.SyncOrders(ctx, startTime, endTime)
	}

	// ALL 或空：全量已注册平台并行拉取
	syncers := m.GetAllSyncers()
	if len(syncers) == 0 {
		return 0, nil
	}

	type syncResult struct {
		count int
		err   error
	}

	resChan := make(chan syncResult, len(syncers))
	var wg sync.WaitGroup

	for _, s := range syncers {
		wg.Add(1)
		go func(syncer PlatformSyncer) {
			defer wg.Done()
			cnt, err := syncer.SyncOrders(ctx, startTime, endTime)
			resChan <- syncResult{count: cnt, err: err}
		}(s)
	}

	wg.Wait()
	close(resChan)

	var totalCount int
	var errs []error
	for res := range resChan {
		totalCount += res.count
		if res.err != nil {
			errs = append(errs, res.err)
		}
	}

	return totalCount, errors.Join(errs...)
}

// SyncOrdersAllPlatforms 同步全平台订单 (多个不同平台并行执行)
func (m *SyncManager) SyncOrdersAllPlatforms(ctx context.Context, startTime, endTime string) error {
	_, err := m.SyncOrdersForPlatform(ctx, model.PlatformAll, startTime, endTime)
	return err
}

// SyncConfigsForPlatform 根据平台同步配置快照
func (m *SyncManager) SyncConfigsForPlatform(ctx context.Context, platformCode string) (int, error) {
	if !model.IsAllPlatforms(platformCode) {
		syncer, ok := m.GetSyncer(platformCode)
		if !ok {
			return 0, fmt.Errorf("unsupported platform: %s", platformCode)
		}
		return syncer.SyncConfigs(ctx)
	}

	// ALL 或空：全平台并发同步配置快照
	syncers := m.GetAllSyncers()
	if len(syncers) == 0 {
		return 0, nil
	}

	type syncResult struct {
		count int
		err   error
	}

	resChan := make(chan syncResult, len(syncers))
	var wg sync.WaitGroup

	for _, s := range syncers {
		wg.Add(1)
		go func(syncer PlatformSyncer) {
			defer wg.Done()
			cnt, err := syncer.SyncConfigs(ctx)
			resChan <- syncResult{count: cnt, err: err}
		}(s)
	}

	wg.Wait()
	close(resChan)

	var totalCount int
	var errs []error
	for res := range resChan {
		totalCount += res.count
		if res.err != nil {
			errs = append(errs, res.err)
		}
	}

	return totalCount, errors.Join(errs...)
}

func (m *SyncManager) GetFlicknovelClient() *flicknovel.Client {
	return m.fnClient
}

func (m *SyncManager) GetFlicknovelRepo() *repository.FlicknovelRepository {
	return m.flicknovelRepo
}

// rocnovelSyncer 中文在线平台驱动
type rocnovelSyncer struct {
	manager *SyncManager
}

func (s *rocnovelSyncer) PlatformCode() string {
	return model.PlatformRocnovel
}

func (s *rocnovelSyncer) SyncOrders(ctx context.Context, startTime, endTime string) (int, error) {
	return s.manager.SyncRocnovelOrders(ctx, startTime, endTime)
}

func (s *rocnovelSyncer) SyncConfigs(ctx context.Context) (int, error) {
	return s.manager.SyncRocnovelSubscribeConfigs(ctx)
}

// flicknovelSyncer 番茄司南平台驱动
type flicknovelSyncer struct {
	manager *SyncManager
}

func (s *flicknovelSyncer) PlatformCode() string {
	return model.PlatformFlicknovel
}

func (s *flicknovelSyncer) SyncOrders(ctx context.Context, startTime, endTime string) (int, error) {
	return s.manager.SyncFlicknovelOrders(ctx, startTime, endTime)
}

func (s *flicknovelSyncer) SyncConfigs(ctx context.Context) (int, error) {
	if err := s.manager.SyncFlicknovelPromotionsAndTemplates(ctx); err != nil {
		return 0, err
	}
	count, _ := s.manager.flicknovelRepo.CountPromotions(ctx)
	return int(count), nil
}
