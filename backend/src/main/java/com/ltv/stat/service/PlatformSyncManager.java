package com.ltv.stat.service;

import com.ltv.stat.adapter.PlatformAdapterRegistry;
import com.ltv.stat.adapter.PlatformSyncAdapter;
import com.ltv.stat.entity.PlatformConfig;
import com.ltv.stat.enums.PlatformEnum;
import com.ltv.stat.repository.PlatformConfigRepository;
import org.slf4j.Logger;
import org.slf4j.LoggerFactory;
import org.springframework.beans.factory.annotation.Autowired;
import org.springframework.beans.factory.annotation.Qualifier;
import org.springframework.context.annotation.Lazy;
import org.springframework.stereotype.Service;

import java.time.LocalDate;
import java.util.*;
import java.util.concurrent.CompletableFuture;
import java.util.concurrent.ConcurrentHashMap;
import java.util.concurrent.Executor;

/**
 * 平台多源同步统一编排管理器
 * 负责各平台适配器的任务调度、线程池并发分发与故障强隔离
 */
@Service
public class PlatformSyncManager {

    private static final Logger log = LoggerFactory.getLogger(PlatformSyncManager.class);

    private final PlatformAdapterRegistry adapterRegistry;
    private final PlatformConfigRepository platformConfigRepository;
    private final Executor syncExecutor;

    @Autowired(required = false)
    @Lazy
    private LtvPredictService ltvPredictService;

    @Autowired(required = false)
    @Lazy
    private LtvStatService ltvStatService;

    public PlatformSyncManager(PlatformAdapterRegistry adapterRegistry,
                               PlatformConfigRepository platformConfigRepository,
                               @Qualifier("orderSyncExecutor") Executor syncExecutor) {
        this.adapterRegistry = adapterRegistry;
        this.platformConfigRepository = platformConfigRepository;
        this.syncExecutor = syncExecutor;
    }

    /**
     * 针对所有启用的平台，并发拉取增量订单 (startDate ~ endDate)
     * 异常强隔离：单个平台拉取失败绝不影响其他平台
     */
    public Map<String, Integer> syncOrdersAllPlatforms(LocalDate startDate, LocalDate endDate) {
        List<PlatformConfig> activeConfigs = platformConfigRepository.findByStatusOrderByCreatedAtAsc(1);
        Map<String, Integer> results = new ConcurrentHashMap<>();
        List<CompletableFuture<Void>> futures = new ArrayList<>();

        for (PlatformConfig config : activeConfigs) {
            String pCode = config.getPlatformCode();
            Optional<PlatformEnum> platformOpt = PlatformEnum.fromCode(pCode);
            if (!platformOpt.isPresent() || platformOpt.get().isAll()) {
                continue;
            }
            PlatformEnum platform = platformOpt.get();
            Optional<PlatformSyncAdapter> adapterOpt = adapterRegistry.getAdapter(platform);
            if (!adapterOpt.isPresent()) {
                log.warn("[PlatformSyncManager] No adapter found for enabled platform: {}", platform);
                continue;
            }

            PlatformSyncAdapter adapter = adapterOpt.get();
            CompletableFuture<Void> future = CompletableFuture.runAsync(() -> {
                try {
                    LocalDate effectiveStartDate = (startDate != null && startDate.isAfter(platform.getLaunchStartDate()))
                            ? startDate
                            : platform.getLaunchStartDate();
                    log.info("[PlatformSyncManager] Starting sync for platform: {} (from {} to {})", platform, effectiveStartDate, endDate);
                    int count = adapter.syncOrders(effectiveStartDate, endDate, config);
                    results.put(platform.getCode(), count);
                    log.info("[PlatformSyncManager] Finished sync for platform: {}, saved/updated orders: {}", platform, count);
                } catch (Exception e) {
                    log.error("[PlatformSyncManager] Error syncing platform: {}, error: {}", platform, e.getMessage(), e);
                    results.put(platform.getCode(), -1);
                }
            }, syncExecutor);

            futures.add(future);
        }

        try {
            CompletableFuture.allOf(futures.toArray(new CompletableFuture[0])).join();
        } catch (Exception e) {
            log.error("[PlatformSyncManager] One or more platform sync tasks encountered unexpected error", e);
        }

        return results;
    }

    /**
     * 按指定平台代码同步订单
     */
    public int syncOrdersForSinglePlatform(PlatformEnum platform, LocalDate startDate, LocalDate endDate) {
        if (platform == null || platform.isAll()) {
            Map<String, Integer> map = syncOrdersAllPlatforms(startDate, endDate);
            return map.values().stream().filter(v -> v > 0).mapToInt(Integer::intValue).sum();
        }

        PlatformConfig config = platformConfigRepository.findByPlatformCode(platform.getCode()).orElse(null);
        Optional<PlatformSyncAdapter> adapterOpt = adapterRegistry.getAdapter(platform);
        if (!adapterOpt.isPresent()) {
            log.warn("[PlatformSyncManager] No adapter registered for platform: {}", platform);
            return 0;
        }

        try {
            LocalDate effectiveStartDate = (startDate != null && startDate.isAfter(platform.getLaunchStartDate()))
                    ? startDate
                    : platform.getLaunchStartDate();
            return adapterOpt.get().syncOrders(effectiveStartDate, endDate, config);
        } catch (Exception e) {
            log.error("[PlatformSyncManager] Error syncing single platform: {}", platform, e);
            throw new RuntimeException("平台 [" + platform.getDisplayName() + "] 同步失败: " + e.getMessage(), e);
        }
    }

    /**
     * 按指定平台代码和落地页同步订单
     */
    public int syncOrdersForLandingPage(PlatformEnum platform, String landingPageId) {
        if (platform == null || platform.isAll()) {
            platform = PlatformEnum.ROCNOVEL; // 默认回退为中文在线
        }
        PlatformConfig config = platformConfigRepository.findByPlatformCode(platform.getCode()).orElse(null);
        Optional<PlatformSyncAdapter> adapterOpt = adapterRegistry.getAdapter(platform);
        if (adapterOpt.isPresent()) {
            return adapterOpt.get().syncOrdersForLandingPage(landingPageId, config);
        }
        return 0;
    }

    /**
     * 全平台渠道与订阅配置快照同步
     */
    public void syncAllConfigs() {
        List<PlatformConfig> activeConfigs = platformConfigRepository.findByStatusOrderByCreatedAtAsc(1);
        for (PlatformConfig config : activeConfigs) {
            PlatformEnum.fromCode(config.getPlatformCode())
                    .filter(p -> !p.isAll())
                    .flatMap(adapterRegistry::getAdapter)
                    .ifPresent(adapter -> {
                        try {
                            adapter.syncConfigs(config);
                        } catch (Exception e) {
                            log.error("[PlatformSyncManager] Error syncing configs for platform: {}", adapter.getPlatform(), e);
                        }
                    });
        }
        if (ltvPredictService != null) {
            ltvPredictService.clearVersionCache();
        }
        if (ltvStatService != null) {
            ltvStatService.clearResponseCache();
        }
    }

    public int syncOrdersAllPlatforms(String startTimeStr, String endTimeStr) {
        LocalDate startDate = LocalDate.parse(startTimeStr);
        LocalDate endDate = LocalDate.parse(endTimeStr);
        Map<String, Integer> map = syncOrdersAllPlatforms(startDate, endDate);
        return map.values().stream().filter(v -> v > 0).mapToInt(Integer::intValue).sum();
    }

    public int syncOrdersForPlatform(PlatformEnum platform, String startTimeStr, String endTimeStr) {
        LocalDate startDate = LocalDate.parse(startTimeStr);
        LocalDate endDate = LocalDate.parse(endTimeStr);
        return syncOrdersForSinglePlatform(platform, startDate, endDate);
    }

    public int syncConfigsForPlatform(PlatformEnum platform) {
        if (platform == null || platform.isAll()) {
            syncAllConfigs();
            return 1;
        }
        PlatformConfig config = platformConfigRepository.findByPlatformCode(platform.getCode()).orElse(null);
        int res = adapterRegistry.getAdapter(platform)
                .map(adapter -> adapter.syncConfigs(config))
                .orElse(0);
        if (ltvPredictService != null) {
            ltvPredictService.clearVersionCache();
        }
        if (ltvStatService != null) {
            ltvStatService.clearResponseCache();
        }
        return res;
    }
}
