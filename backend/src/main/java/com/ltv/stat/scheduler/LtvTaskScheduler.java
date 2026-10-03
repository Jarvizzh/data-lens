package com.ltv.stat.scheduler;

import com.ltv.stat.service.DailyRechargeStatService;
import com.ltv.stat.service.LtvStatService;
import com.ltv.stat.service.PlatformSyncManager;
import com.ltv.stat.service.UserService;
import com.ltv.stat.service.flicknovel.FlicknovelApiService;
import org.slf4j.Logger;
import org.slf4j.LoggerFactory;
import org.springframework.context.annotation.Lazy;
import org.springframework.scheduling.annotation.EnableScheduling;
import org.springframework.scheduling.annotation.Scheduled;
import org.springframework.stereotype.Component;

import java.time.LocalDate;
import java.time.ZoneId;
import java.time.format.DateTimeFormatter;

@Component
@EnableScheduling
public class LtvTaskScheduler {

    private static final Logger log = LoggerFactory.getLogger(LtvTaskScheduler.class);
    private static final DateTimeFormatter DATE_FORMATTER = DateTimeFormatter.ofPattern("yyyy-MM-dd");

    private final PlatformSyncManager platformSyncManager;
    private final LtvStatService ltvStatService;
    private final DailyRechargeStatService dailyRechargeStatService;
    private final FlicknovelApiService flicknovelApiService;
    private final UserService userService;

    public LtvTaskScheduler(PlatformSyncManager platformSyncManager,
                            LtvStatService ltvStatService,
                            DailyRechargeStatService dailyRechargeStatService,
                            FlicknovelApiService flicknovelApiService,
                            @Lazy UserService userService) {
        this.platformSyncManager = platformSyncManager;
        this.ltvStatService = ltvStatService;
        this.dailyRechargeStatService = dailyRechargeStatService;
        this.flicknovelApiService = flicknovelApiService;
        this.userService = userService;
    }

    /**
     * 北京时间每 4 小时整点: 定时拉取番茄司南所有推广链接与充值模板入库，并自动导入全量推广ID给超级管理员/管理员
     */
    @Scheduled(cron = "0 0 */4 * * ?", zone = "Asia/Shanghai")
    public void scheduledFlicknovelPromotionAndTemplateSync() {
        log.info("Starting scheduled Flicknovel promotions & recharge templates sync (every 4h)...");
        try {
            flicknovelApiService.syncPromotionsAndTemplates(true);
            // 同步完番茄司南推广链接后，自动为所有超级管理员/管理员补齐全量推广ID
            if (userService != null) {
                int imported = userService.autoImportFlicknovelLandingPagesForAdmins();
                log.info("Scheduled Flicknovel admin landing page auto-import completed, imported: {}", imported);
            }
        } catch (Exception e) {
            log.error("Scheduled Flicknovel promotions & recharge templates sync failed", e);
        }
        log.info("Finished scheduled Flicknovel promotions & recharge templates sync.");
    }

    /**
     * 北京时间每小时 05 分 (例如 00:05, 01:05, ..., 23:05): 定时拉取过去 2 天的全量增量订单与染色信息
     */
    @Scheduled(cron = "0 5 * * * ?", zone = "Asia/Shanghai")
    public void scheduledOrderFetch() {
        log.info("Starting scheduled order fetch across all platforms at xx:05 BJ Time (past 2 days)");
        LocalDate todayBj = LocalDate.now(ZoneId.of("Asia/Shanghai"));
        LocalDate startBj = todayBj.minusDays(2);

        String startTimeStr = startBj.format(DATE_FORMATTER);
        String endTimeStr = todayBj.format(DATE_FORMATTER);

        try {
            platformSyncManager.syncOrdersAllPlatforms(startTimeStr, endTimeStr);
        } catch (Exception e) {
            log.error("Scheduled multi-platform order fetch failed", e);
        }

        // 定时拉取番茄司南近 2 天染色归因记录独立落库
        try {
            flicknovelApiService.syncRelations(startBj, todayBj);
        } catch (Exception e) {
            log.error("Scheduled Flicknovel relations fetch failed", e);
        }

        log.info("Scheduled order & relation fetch finished.");
    }

    /**
     * 北京时间每天凌晨 00:40 分: 定时拉取全量订单与染色归因
     */
    @Scheduled(cron = "0 40 0 * * ?", zone = "Asia/Shanghai")
    public void scheduledFullOrderFetch() {
        log.info("Starting scheduled full order fetch across all platforms at 00:40 BJ Time (from 2026-07-10 to today)");
        LocalDate todayBj = LocalDate.now(ZoneId.of("Asia/Shanghai"));
        String startTimeStr = "2026-07-10";
        String endTimeStr = todayBj.format(DATE_FORMATTER);

        try {
            platformSyncManager.syncOrdersAllPlatforms(startTimeStr, endTimeStr);
        } catch (Exception e) {
            log.error("Scheduled multi-platform full order fetch failed", e);
        }

        // 全量同步染色信息 (近 30 天)
        try {
            flicknovelApiService.syncRelations(todayBj.minusDays(30), todayBj);
        } catch (Exception e) {
            log.error("Scheduled Flicknovel full relations fetch failed", e);
        }

        // 每日全量同步后兜底自动补齐一次管理员落地页
        try {
            if (userService != null) {
                userService.autoImportFlicknovelLandingPagesForAdmins();
            }
        } catch (Exception e) {
            log.error("Scheduled full auto-import flicknovel landing pages for admins failed", e);
        }

        log.info("Scheduled full order & relation fetch finished.");
    }

    /**
     * 北京时间每小时 30 分 (例如 00:30, 01:30, ..., 23:30): 定时全量统计 LTV 数据
     */
    @Scheduled(cron = "0 30 * * * ?", zone = "Asia/Shanghai")
    public void scheduledLtvCalculation() {
        log.info("Starting hourly scheduled LTV calculation at xx:30 BJ Time");
        try {
            ltvStatService.calculateAllLtvStats();
        } catch (Exception e) {
            log.error("Scheduled LTV calculation failed", e);
        }
        log.info("Scheduled LTV calculation finished.");
    }

    /**
     * 北京时间每小时 20 分 (例如 00:20, 01:20, ..., 23:20): 定时统计【每日充值分布】数据并落库
     */
    @Scheduled(cron = "0 20 * * * ?", zone = "Asia/Shanghai")
    public void scheduledDailyDistributionCalculation() {
        log.info("Starting hourly scheduled daily distribution calculation at xx:20 BJ Time");
        try {
            dailyRechargeStatService.calculateAllDailyDistributionStats();
        } catch (Exception e) {
            log.error("Scheduled daily distribution calculation failed", e);
        }
        log.info("Scheduled daily distribution calculation finished.");
    }

    /**
     * 北京时间每 30 分钟: 定期清理过期的内存报表缓存，释放空闲堆内存
     */
    @Scheduled(cron = "0 0/30 * * * ?", zone = "Asia/Shanghai")
    public void scheduledCacheCleanup() {
        try {
            ltvStatService.cleanExpiredResponseCache();
        } catch (Exception e) {
            log.error("Scheduled cache cleanup failed", e);
        }
    }
}
