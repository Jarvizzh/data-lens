package com.ltv.stat.config;

import com.ltv.stat.repository.PlatformConfigRepository;
import com.ltv.stat.service.UserService;
import org.slf4j.Logger;
import org.slf4j.LoggerFactory;
import org.springframework.beans.factory.annotation.Autowired;
import org.springframework.context.annotation.Lazy;
import org.springframework.jdbc.core.JdbcTemplate;
import org.springframework.stereotype.Component;

import javax.annotation.PostConstruct;
import java.time.LocalDate;

/**
 * 数据库初始化组件
 * 历史动态 DDL 及数据清洗代码已清理，最新完整数据表结构请参阅 schema.sql
 */
@Component
public class DatabasePrimaryKeysInitializer {

    private static final Logger log = LoggerFactory.getLogger(DatabasePrimaryKeysInitializer.class);

    @Autowired(required = false)
    private PlatformConfigRepository platformConfigRepository;

    @Autowired(required = false)
    private JdbcTemplate jdbcTemplate;

    @Autowired(required = false)
    @Lazy
    private UserService userService;

    @PostConstruct
    public void init() {
        log.info("DatabasePrimaryKeysInitializer: Database schema is up to date.");
        try {
            if (platformConfigRepository != null) {
                platformConfigRepository.findByPlatformCode("flicknovel").ifPresent(cfg -> {
                    if (cfg.getLaunchStartDate() == null || cfg.getLaunchStartDate().isAfter(LocalDate.of(2026, 9, 16))) {
                        cfg.setLaunchStartDate(LocalDate.of(2026, 9, 16));
                        platformConfigRepository.save(cfg);
                        log.info("Updated Flicknovel launchStartDate to 2026-09-16 in platform_config");
                    }
                });
            }
            if (jdbcTemplate != null) {
                jdbcTemplate.update("UPDATE platform_config SET launch_start_date = '2026-09-16' WHERE platform_code = 'flicknovel' AND (launch_start_date IS NULL OR launch_start_date > '2026-09-16')");
            }
        } catch (Exception e) {
            log.warn("Failed to check or update platform_config launch_start_date: {}", e.getMessage());
        }

        try {
            if (userService != null) {
                userService.autoImportFlicknovelLandingPagesForAdmins();
            }
        } catch (Exception e) {
            log.warn("Failed to auto-import flicknovel landing pages on startup: {}", e.getMessage());
        }
    }
}
