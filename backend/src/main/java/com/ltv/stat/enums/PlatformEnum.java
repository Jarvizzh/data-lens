package com.ltv.stat.enums;

import java.time.LocalDate;
import java.util.Arrays;
import java.util.List;
import java.util.Optional;
import java.util.stream.Collectors;

/**
 * 平台枚举定义 (强类型)
 * 严禁在业务逻辑与数据接入层使用魔术字符串
 */
public enum PlatformEnum {

    ALL("ALL", "大盘汇总", true, LocalDate.of(2026, 7, 10), "CST"),
    ROCNOVEL("rocnovel", "中文在线", true, LocalDate.of(2026, 7, 10), "CST"),
    FLICKNOVEL("flicknovel", "番茄司南", true, LocalDate.of(2026, 9, 16), "UTC");

    private final String code;
    private final String displayName;
    private final boolean enabled;
    private final LocalDate launchStartDate;
    private final String defaultTimezone;

    PlatformEnum(String code, String displayName, boolean enabled, LocalDate launchStartDate, String defaultTimezone) {
        this.code = code;
        this.displayName = displayName;
        this.enabled = enabled;
        this.launchStartDate = launchStartDate;
        this.defaultTimezone = defaultTimezone;
    }

    public String getDefaultTimezone() {
        return defaultTimezone;
    }

    public String getCode() {
        return code;
    }

    public String getDisplayName() {
        return displayName;
    }

    public boolean isEnabled() {
        return enabled;
    }

    public LocalDate getLaunchStartDate() {
        return launchStartDate;
    }

    public String getLaunchStartDateStr() {
        return launchStartDate != null ? launchStartDate.toString() : "2026-07-10";
    }

    public boolean isAll() {
        return this == ALL;
    }

    /**
     * 根据 code 安全解析枚举，若为空默认返回 ALL，若无法识别返回 empty
     */
    public static Optional<PlatformEnum> fromCode(String code) {
        if (code == null || code.trim().isEmpty()) {
            return Optional.of(ALL);
        }
        for (PlatformEnum p : values()) {
            if (p.code.equalsIgnoreCase(code.trim())) {
                return Optional.of(p);
            }
        }
        return Optional.empty();
    }

    /**
     * 获取所有可独立接入业务的实际平台（排除 ALL 综合大盘）
     */
    public static List<PlatformEnum> getActualPlatforms() {
        return Arrays.stream(values())
                .filter(p -> p != ALL && p.isEnabled())
                .collect(Collectors.toList());
    }

    /**
     * 根据平台代码安全获取投放起始日期，若未指定或无法匹配默认返回 ROCNOVEL (2026-07-10)
     */
    public static LocalDate getLaunchStartDateForPlatform(String code) {
        return fromCode(code).map(PlatformEnum::getLaunchStartDate).orElse(ROCNOVEL.getLaunchStartDate());
    }

    /**
     * 根据平台代码安全获取默认时区，番茄司南默认返回 UTC，中文在线或未指定默认返回 CST
     */
    public static String getDefaultTimezoneForPlatform(String code) {
        return fromCode(code).map(PlatformEnum::getDefaultTimezone).orElse(CST_DEFAULT_TIMEZONE);
    }

    public static final String CST_DEFAULT_TIMEZONE = "CST";
}
