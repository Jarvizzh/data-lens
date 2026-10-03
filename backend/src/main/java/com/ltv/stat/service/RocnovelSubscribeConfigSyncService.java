package com.ltv.stat.service;

import com.ltv.stat.dto.*;
import com.ltv.stat.dto.rocnovel.*;
import com.ltv.stat.entity.SubscriptionConfigVersion;
import com.ltv.stat.entity.SystemConfig;
import com.ltv.stat.repository.SubscriptionConfigVersionRepository;
import com.ltv.stat.repository.SystemConfigRepository;
import com.ltv.stat.util.TimeUtils;
import org.slf4j.Logger;
import org.slf4j.LoggerFactory;
import org.springframework.beans.factory.annotation.Value;
import org.springframework.boot.context.event.ApplicationReadyEvent;
import org.springframework.context.event.EventListener;
import org.springframework.beans.factory.annotation.Autowired;
import org.springframework.context.annotation.Lazy;
import org.springframework.http.HttpEntity;
import org.springframework.http.HttpHeaders;
import org.springframework.http.HttpMethod;
import org.springframework.http.MediaType;
import org.springframework.http.ResponseEntity;
import org.springframework.scheduling.annotation.Scheduled;
import org.springframework.stereotype.Service;
import org.springframework.web.client.RestTemplate;

import java.math.BigDecimal;
import java.math.RoundingMode;
import java.time.LocalDateTime;
import java.util.*;
import java.util.concurrent.CompletableFuture;

@Service
public class RocnovelSubscribeConfigSyncService {

    private static final Logger log = LoggerFactory.getLogger(RocnovelSubscribeConfigSyncService.class);

    private final RestTemplate restTemplate;
    private final SubscriptionConfigVersionRepository versionRepository;
    private final SystemConfigRepository systemConfigRepository;

    @Autowired(required = false)
    @Lazy
    private LtvPredictService ltvPredictService;

    @Value("${order.api.authorization}")
    private String defaultAuthorization;

    @Value("${order.api.cookie:JSESSIONID=3f61a534-8c67-49a2-837c-4dee1ee375de}")
    private String defaultCookie;

    @Value("${order.api.client-group-id:405323222546395136}")
    private String clientGroupId;

    @Value("${order.api.client-group-name:Florastory}")
    private String clientGroupName;

    private static final String LANDING_PAGE_LIST_URL = "https://admin-api.rocnovel.com/landingPage/config/list";
    private static final String SUBSCRIBE_PRODUCT_LIST_URL = "https://admin-api.rocnovel.com/subscribe-config/product/list";

    public RocnovelSubscribeConfigSyncService(RestTemplate restTemplate,
                                              SubscriptionConfigVersionRepository versionRepository,
                                              SystemConfigRepository systemConfigRepository) {
        this.restTemplate = restTemplate;
        this.versionRepository = versionRepository;
        this.systemConfigRepository = systemConfigRepository;
    }

    public String getActiveAuthorization() {
        return systemConfigRepository.findById("API_AUTHORIZATION")
                .map(SystemConfig::getConfigValue)
                .filter(val -> !val.trim().isEmpty())
                .orElse(defaultAuthorization);
    }

    public String getActiveCookie() {
        return systemConfigRepository.findById("API_COOKIE")
                .map(SystemConfig::getConfigValue)
                .filter(val -> !val.trim().isEmpty())
                .orElse(defaultCookie);
    }

    /**
     * 服务启动完成后，异步触发一次全量落地页配置与订阅产品明细同步 (不阻塞服务启动)
     */
    @EventListener(ApplicationReadyEvent.class)
    public void onApplicationReady() {
        CompletableFuture.runAsync(() -> {
            log.info("Application ready, starting asynchronous initial sync of landing page & subscribe configs...");
            try {
                int saved = syncAllSubscribeConfigs();
                log.info("Initial asynchronous subscribe config sync finished. Saved/updated {} version records.", saved);
            } catch (Exception e) {
                log.error("Initial asynchronous subscribe config sync failed", e);
            }
        });
    }

    /**
     * 每 6 小时自动同步一次落地页与订阅配置明细 (0:00, 6:00, 12:00, 18:00)
     */
    @Scheduled(cron = "0 0 */6 * * ?")
    public void scheduledSyncSubscribeConfigs() {
        log.info("Starting scheduled 6-hour sync for landing page & subscribe configs...");
        try {
            syncAllSubscribeConfigs();
        } catch (Exception e) {
            log.error("Scheduled subscribe config sync failed", e);
        }
    }

    /**
     * 手动触发同步全量落地页配置及对应订阅产品明细
     */
    public int syncAllSubscribeConfigs() {
        List<LandingPageRecordDto> landingPages = fetchAllLandingPages();
        log.info("Fetched total {} landing page records", landingPages.size());

        int totalSavedVersions = 0;
        Set<String> processedConfigIds = new HashSet<>();

        for (LandingPageRecordDto page : landingPages) {
            String pId = page.getId();
            String configId = page.getSubscribeConfigId();

            if (pId == null || pId.trim().isEmpty() || configId == null || configId.trim().isEmpty()) {
                continue;
            }

            List<SubscribeConfigProductRecordDto> products = fetchSubscribeProductsForConfig(configId);
            for (SubscribeConfigProductRecordDto prod : products) {
                boolean saved = processAndSaveVersion(page, prod);
                if (saved) {
                    totalSavedVersions++;
                }
            }
            processedConfigIds.add(configId);
        }

        log.info("Completed subscribe config sync. Saved/updated {} version records across {} configs",
                totalSavedVersions, processedConfigIds.size());
        if (ltvPredictService != null) {
            ltvPredictService.clearVersionCache();
        }
        return totalSavedVersions;
    }

    private List<LandingPageRecordDto> fetchAllLandingPages() {
        List<LandingPageRecordDto> allPages = new ArrayList<>();
        int pageIndex = 1;
        int pageSize = 50;
        int totalPages = 1;

        HttpHeaders headers = createHeaders();

        do {
            String url = String.format("%s?clientGroupId=%s&clientGroupName=%s&contentType=4&pageIndex=%d&pageSize=%d&_=%d",
                    LANDING_PAGE_LIST_URL, clientGroupId, clientGroupName, pageIndex, pageSize, System.currentTimeMillis());

            try {
                HttpEntity<Void> requestEntity = new HttpEntity<>(headers);
                ResponseEntity<LandingPageConfigResponseDto> response = restTemplate.exchange(url, HttpMethod.GET, requestEntity, LandingPageConfigResponseDto.class);
                if (response.getStatusCode().is2xxSuccessful() && response.getBody() != null) {
                    LandingPageConfigResponseDto body = response.getBody();
                    if (body.getCode() != null && body.getCode() == 0 && body.getData() != null) {
                        totalPages = body.getData().getPages() != null ? body.getData().getPages() : 1;
                        List<LandingPageRecordDto> records = body.getData().getRecords();
                        if (records != null) {
                            allPages.addAll(records);
                        }
                    }
                }
            } catch (Exception e) {
                log.error("Failed to fetch landing page config list page: {}", pageIndex, e);
                break;
            }
            pageIndex++;
        } while (pageIndex <= totalPages);

        return allPages;
    }

    private List<SubscribeConfigProductRecordDto> fetchSubscribeProductsForConfig(String configId) {
        String url = String.format("%s?clientGroupId=%s&clientGroupName=%s&configId=%s&contentType=4&pageIndex=1&pageSize=1000&paymentMethod=3",
                SUBSCRIBE_PRODUCT_LIST_URL, clientGroupId, clientGroupName, configId);

        try {
            HttpEntity<Void> requestEntity = new HttpEntity<>(createHeaders());
            ResponseEntity<SubscribeConfigProductResponseDto> response = restTemplate.exchange(url, HttpMethod.GET, requestEntity, SubscribeConfigProductResponseDto.class);
            if (response.getStatusCode().is2xxSuccessful() && response.getBody() != null) {
                SubscribeConfigProductResponseDto body = response.getBody();
                if (body.getCode() != null && body.getCode() == 0 && body.getData() != null) {
                    List<SubscribeConfigProductRecordDto> records = body.getData().getRecords();
                    return records != null ? records : Collections.emptyList();
                }
            }
        } catch (Exception e) {
            log.error("Failed to fetch subscribe config product detail for configId: {}", configId, e);
        }
        return Collections.emptyList();
    }

    private boolean processAndSaveVersion(LandingPageRecordDto page, SubscribeConfigProductRecordDto product) {
        String landingPageId = page.getId();
        String productId = product.getId();
        if (landingPageId == null || productId == null) return false;

        int subPeriodDays = parseSubPeriodDays(product.getCycleStr(), product.getCycle());
        int firstPriceCent = parseUsdToCent(product.getPreferentialPrice());
        int renewPriceCent = parseUsdToCent(product.getPrice());

        LocalDateTime eventTime = parseTimeStr(product.getUpdateDateTime());
        if (eventTime == null) {
            eventTime = parseTimeStr(product.getCreateDateTime());
        }
        if (eventTime == null) {
            eventTime = parseTimeStr(page.getUpdateDateTime());
        }
        if (eventTime == null) {
            eventTime = LocalDateTime.now();
        }

        Optional<SubscriptionConfigVersion> latestOpt = versionRepository.findTopByLandingPageIdAndProductIdOrderByVersionNumDesc(landingPageId, productId);

        if (latestOpt.isPresent()) {
            SubscriptionConfigVersion latest = latestOpt.get();
            boolean isChanged = !Objects.equals(latest.getFirstPriceCent(), firstPriceCent)
                    || !Objects.equals(latest.getRenewPriceCent(), renewPriceCent)
                    || !Objects.equals(latest.getSubPeriodDays(), subPeriodDays);

            if (isChanged) {
                // 关闭旧版本
                latest.setEffectiveEndTime(eventTime);
                versionRepository.save(latest);

                // 创建新版本
                SubscriptionConfigVersion newVersion = createVersionEntity(page, product, subPeriodDays, firstPriceCent, renewPriceCent, latest.getVersionNum() + 1, eventTime);
                versionRepository.save(newVersion);
                return true;
            }
            return false;
        } else {
            // 首次创建版本
            SubscriptionConfigVersion newVersion = createVersionEntity(page, product, subPeriodDays, firstPriceCent, renewPriceCent, 1, eventTime);
            versionRepository.save(newVersion);
            return true;
        }
    }

    private SubscriptionConfigVersion createVersionEntity(LandingPageRecordDto page,
                                                           SubscribeConfigProductRecordDto product,
                                                           int subPeriodDays,
                                                           int firstPriceCent,
                                                           int renewPriceCent,
                                                           int versionNum,
                                                           LocalDateTime startTime) {
        SubscriptionConfigVersion v = new SubscriptionConfigVersion();
        v.setLandingPageId(page.getId());
        v.setSubscribeConfigId(page.getSubscribeConfigId());
        v.setSubscribeConfigName(page.getSubscribeConfigName());
        v.setSaleComboId(page.getSaleComboId());
        v.setSaleComboName(page.getSaleComboName());
        v.setProductId(product.getId());
        v.setProductName(product.getName());
        v.setSubPeriodDays(subPeriodDays);
        v.setFirstPriceCent(firstPriceCent);
        v.setRenewPriceCent(renewPriceCent);
        v.setVersionNum(versionNum);
        v.setEffectiveStartTime(startTime);
        v.setEffectiveEndTime(null);
        return v;
    }

    private int parseSubPeriodDays(String cycleStr, Integer cycle) {
        if (cycleStr != null) {
            String lower = cycleStr.trim().toLowerCase();
            if (lower.contains("1 day") || lower.equals("day")) return 1;
            if (lower.contains("3 day")) return 3;
            if (lower.contains("week")) return 7;
            if (lower.contains("month")) return 30;
            if (lower.contains("annual") || lower.contains("year")) return 365;
        }
        if (cycle != null && cycle > 0) {
            return cycle;
        }
        return 1;
    }

    private int parseUsdToCent(String usdStr) {
        if (usdStr == null || usdStr.trim().isEmpty()) return 0;
        try {
            BigDecimal usd = new BigDecimal(usdStr.trim());
            return usd.multiply(BigDecimal.valueOf(100)).setScale(0, RoundingMode.HALF_UP).intValue();
        } catch (Exception e) {
            return 0;
        }
    }

    private LocalDateTime parseTimeStr(String timeStr) {
        if (timeStr == null || timeStr.trim().isEmpty()) return null;
        try {
            return LocalDateTime.parse(timeStr.trim(), TimeUtils.DATETIME_FORMATTER);
        } catch (Exception e) {
            return null;
        }
    }

    private HttpHeaders createHeaders() {
        HttpHeaders headers = new HttpHeaders();
        headers.setContentType(MediaType.APPLICATION_JSON);
        headers.set("Authorization", getActiveAuthorization());
        String cookie = getActiveCookie();
        if (!cookie.isEmpty()) {
            headers.set("Cookie", cookie);
        }
        headers.set("User-Agent", "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/151.0.0.0 Safari/537.36");
        return headers;
    }
}
