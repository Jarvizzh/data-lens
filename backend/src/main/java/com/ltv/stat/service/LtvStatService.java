package com.ltv.stat.service;

import com.ltv.stat.dto.LandingPageConfigItem;
import com.ltv.stat.dto.LtvListResponseDto;
import com.ltv.stat.dto.MonthlySummaryDto;
import com.ltv.stat.dto.PredictionResult;
import com.ltv.stat.dto.RetainedSubscribersDto;
import com.ltv.stat.dto.SingleMonthSummaryDto;
import com.ltv.stat.entity.*;
import com.ltv.stat.enums.PlatformEnum;
import com.ltv.stat.repository.*;
import com.ltv.stat.util.TimeUtils;
import org.slf4j.Logger;
import org.slf4j.LoggerFactory;
import org.springframework.beans.factory.annotation.Autowired;
import org.springframework.context.annotation.Lazy;
import org.springframework.stereotype.Service;
import org.springframework.transaction.annotation.Transactional;

import java.math.BigDecimal;
import java.math.RoundingMode;
import java.time.LocalDate;
import java.time.YearMonth;
import java.time.ZoneId;
import java.time.ZonedDateTime;
import java.time.temporal.ChronoUnit;
import java.util.*;
import java.util.concurrent.ConcurrentHashMap;
import java.util.stream.Collectors;

@Service
public class LtvStatService {

    private static final Logger log = LoggerFactory.getLogger(LtvStatService.class);
    public static final LocalDate START_DATE = LocalDate.of(2026, 7, 10);
    public static final LocalDate MULTI_PLATFORM_START_DATE = LocalDate.of(2026, 9, 17);

    /**
     * 根据平台代码安全获取投放起始日期，中文在线为 2026-07-10，番茄司南为 2026-09-16，ALL 为 2026-07-10
     */
    public static LocalDate getLaunchStartDateForPlatform(String platformCode) {
        return PlatformEnum.getLaunchStartDateForPlatform(platformCode);
    }

    // =========================================================================
    // 方案一：高性能内存缓存结构与主动失效机制 (Memory Cache & Active Invalidation)
    // =========================================================================
    private static class CachedLtvResponse {
        final LtvListResponseDto data;
        final long timestamp;

        CachedLtvResponse(LtvListResponseDto data) {
            this.data = data;
            this.timestamp = System.currentTimeMillis();
        }

        boolean isExpired(long ttlMs) {
            return System.currentTimeMillis() - timestamp > ttlMs;
        }
    }

    private final RawOrderRepository rawOrderRepository;
    private final LtvLaunchConfigRepository ltvLaunchConfigRepository;
    private final LtvDailyStatRepository ltvDailyStatRepository;
    private final DailyRechargeDistributionRepository dailyRechargeDistributionRepository;
    private final UserSubscriptionPeriodRepository userSubscriptionPeriodRepository;
    private final UserService userService;
    private final LtvPredictService ltvPredictService;
    private final LtvBenchmarkService ltvBenchmarkService;
    private final AsyncRecalculateService asyncRecalculateService;

    public LtvStatService(RawOrderRepository rawOrderRepository,
                          LtvLaunchConfigRepository ltvLaunchConfigRepository,
                          LtvDailyStatRepository ltvDailyStatRepository,
                          DailyRechargeDistributionRepository dailyRechargeDistributionRepository,
                          UserSubscriptionPeriodRepository userSubscriptionPeriodRepository,
                          UserService userService,
                          LtvPredictService ltvPredictService,
                          @Autowired(required = false) LtvBenchmarkService ltvBenchmarkService,
                          @Lazy AsyncRecalculateService asyncRecalculateService) {
        this.rawOrderRepository = rawOrderRepository;
        this.ltvLaunchConfigRepository = ltvLaunchConfigRepository;
        this.ltvDailyStatRepository = ltvDailyStatRepository;
        this.dailyRechargeDistributionRepository = dailyRechargeDistributionRepository;
        this.userSubscriptionPeriodRepository = userSubscriptionPeriodRepository;
        this.userService = userService;
        this.ltvPredictService = ltvPredictService;
        this.ltvBenchmarkService = ltvBenchmarkService;
        this.asyncRecalculateService = asyncRecalculateService;
    }

    @Transactional
    public LtvLaunchConfig saveLaunchConfig(String platformCode, Long userId, LocalDate launchDate, BigDecimal spend, String remark) {
        if (userId == null) userId = 1L;
        if (platformCode == null || "ALL".equalsIgnoreCase(platformCode.trim())) {
            throw new IllegalArgumentException("大盘数据不可直接编辑，请先切换至具体平台");
        }
        String pCode = platformCode.trim().toLowerCase();
        if (userService.isMasterAccount(userId)) {
            throw new IllegalArgumentException("主账号为数据汇总账号，消耗由关联子账号自动计算，不可直接编辑！");
        }
        final Long uid = userId;
        final String finalPCode = pCode;
        LtvLaunchConfig config = ltvLaunchConfigRepository.findByPlatformCodeAndUserIdAndLaunchDate(finalPCode, uid, launchDate).orElseGet(() -> {
            LtvLaunchConfig c = new LtvLaunchConfig();
            c.setPlatformCode(finalPCode);
            c.setUserId(uid);
            c.setLaunchDate(launchDate);
            return c;
        });
        config.setPlatformCode(finalPCode);
        config.setUserId(uid);
        config.setLaunchDate(launchDate);
        if (spend != null) config.setSpend(spend);
        if (remark != null) config.setRemark(remark);
        LtvLaunchConfig saved = ltvLaunchConfigRepository.saveAndFlush(config);
        invalidateUserCache(finalPCode, uid);
        return saved;
    }

    public LtvLaunchConfig saveLaunchConfig(Long userId, LocalDate launchDate, BigDecimal spend, String remark) {
        return saveLaunchConfig("rocnovel", userId, launchDate, spend, remark);
    }

    public LtvLaunchConfig saveLaunchConfig(LocalDate launchDate, BigDecimal spend, String remark) {
        return saveLaunchConfig("rocnovel", 1L, launchDate, spend, remark);
    }

    @Transactional
    public int batchSaveLaunchConfig(String platformCode, Long userId, List<Map<String, Object>> items) {
        if (userId == null) userId = 1L;
        if (platformCode == null || "ALL".equalsIgnoreCase(platformCode.trim())) {
            throw new IllegalArgumentException("大盘数据不可直接导入，请先切换至具体平台");
        }
        String pCode = platformCode.trim().toLowerCase();
        if (userService.isMasterAccount(userId)) {
            throw new IllegalArgumentException("主账号为数据汇总账号，消耗由关联子账号自动计算，不可直接导入！");
        }
        int count = 0;
        for (Map<String, Object> item : items) {
            String dateStr = (String) item.get("launchDate");
            Object spendObj = item.get("spend");
            String remark = (String) item.get("remark");

            if (dateStr == null || dateStr.trim().isEmpty()) continue;
            LocalDate launchDate = parseFlexDate(dateStr);
            if (launchDate == null) continue;
            BigDecimal spend = BigDecimal.ZERO;
            if (spendObj != null) {
                try {
                    spend = new BigDecimal(spendObj.toString().trim());
                } catch (Exception ignored) {}
            }

            final Long uid = userId;
            final LocalDate fDate = launchDate;
            final BigDecimal fSpend = spend;
            final String fRemark = remark;
            final String finalPCode = pCode;
            LtvLaunchConfig config = ltvLaunchConfigRepository.findByPlatformCodeAndUserIdAndLaunchDate(finalPCode, uid, fDate).orElseGet(() -> {
                LtvLaunchConfig c = new LtvLaunchConfig();
                c.setPlatformCode(finalPCode);
                c.setUserId(uid);
                c.setLaunchDate(fDate);
                return c;
            });
            config.setPlatformCode(finalPCode);
            config.setUserId(uid);
            config.setLaunchDate(fDate);
            if (fSpend != null) config.setSpend(fSpend);
            if (fRemark != null) config.setRemark(fRemark);
            ltvLaunchConfigRepository.save(config);
            count++;
        }
        ltvLaunchConfigRepository.flush();
        invalidateUserCache(pCode, userId);
        calculateLtvStatsForUser(pCode, userId);
        return count;
    }

    @Transactional
    public int batchSaveLaunchConfig(Long userId, List<Map<String, Object>> items) {
        return batchSaveLaunchConfig("rocnovel", userId, items);
    }

    /**
     * 提取订单的生效注册日期 (按落地页 ID 对应的时区配置区分 UTC、美东与北京时间，未标注根据平台默认判定，兼容历史 BJ)
     */
    public static LocalDate getEffectiveRegisterDate(RawOrder order, Map<String, String> tzMap) {
        if (order == null) return null;
        String pid = order.getLandingPageId() != null ? order.getLandingPageId().trim() : "";
        String defaultTz = (order.getPlatformCode() != null && "flicknovel".equalsIgnoreCase(order.getPlatformCode().trim())) ? "UTC" : "CST";
        String tz = (tzMap != null && tzMap.containsKey(pid)) ? tzMap.get(pid) : defaultTz;
        if (tz == null || tz.trim().isEmpty() || "BJ".equalsIgnoreCase(tz.trim())) {
            tz = "CST";
        }
        if ("UTC".equalsIgnoreCase(tz)) {
            return getUtcRegisterDate(order);
        }
        if ("ET".equalsIgnoreCase(tz)) {
            return order.getRegisterDateEt();
        }
        // CST / BJ
        return order.getRegisterTimeBj() != null ? order.getRegisterTimeBj().toLocalDate() : order.getRegisterDateEt();
    }

    public static LocalDate parseFlexDate(String dateStr) {
        if (dateStr == null || dateStr.trim().isEmpty()) return null;
        try {
            String s = dateStr.trim().replace('.', '-').replace('/', '-');
            String[] parts = s.split("-");
            if (parts.length == 3) {
                int year = Integer.parseInt(parts[0]);
                int month = Integer.parseInt(parts[1]);
                int day = Integer.parseInt(parts[2]);
                return LocalDate.of(year, month, day);
            }
            return LocalDate.parse(s);
        } catch (Exception e) {
            return null;
        }
    }

    public static LocalDate getEffectiveRegisterDate(RawOrder order) {
        return getEffectiveRegisterDate(order, null);
    }

    /**
     * 提取订单的生效支付日期 (按落地页 ID 对应的时区配置区分 UTC、美东与北京时间，未标注根据平台默认判定，兼容历史 BJ)
     */
    public static LocalDate getEffectivePayDate(RawOrder order, Map<String, String> tzMap) {
        if (order == null) return null;
        String pid = order.getLandingPageId() != null ? order.getLandingPageId().trim() : "";
        String defaultTz = (order.getPlatformCode() != null && "flicknovel".equalsIgnoreCase(order.getPlatformCode().trim())) ? "UTC" : "CST";
        String tz = (tzMap != null && tzMap.containsKey(pid)) ? tzMap.get(pid) : defaultTz;
        if (tz == null || tz.trim().isEmpty() || "BJ".equalsIgnoreCase(tz.trim())) {
            tz = "CST";
        }
        if ("UTC".equalsIgnoreCase(tz)) {
            return getUtcPayDate(order);
        }
        if ("ET".equalsIgnoreCase(tz)) {
            return order.getPayDateEt();
        }
        // CST / BJ
        return order.getPayTimeBj() != null ? order.getPayTimeBj().toLocalDate() : order.getPayDateEt();
    }

    public static LocalDate getEffectivePayDate(RawOrder order) {
        return getEffectivePayDate(order, null);
    }

    /**
     * 提取订单的 UTC 注册日期
     */
    public static LocalDate getUtcRegisterDate(RawOrder order) {
        if (order == null) return null;
        if (order.getRegisterDateUtc() != null) return order.getRegisterDateUtc();
        if (order.getRegisterTimeUtc() != null) return order.getRegisterTimeUtc().toLocalDate();
        if (order.getRegisterTimeBj() != null) return TimeUtils.convertBjToUtc(order.getRegisterTimeBj()).toLocalDate();
        return order.getRegisterDateEt();
    }

    /**
     * 提取订单的 UTC 支付日期
     */
    public static LocalDate getUtcPayDate(RawOrder order) {
        if (order == null) return null;
        if (order.getPayDateUtc() != null) return order.getPayDateUtc();
        if (order.getPayTimeUtc() != null) return order.getPayTimeUtc().toLocalDate();
        if (order.getPayTimeBj() != null) return TimeUtils.convertBjToUtc(order.getPayTimeBj()).toLocalDate();
        return order.getPayDateEt();
    }

    /**
     * 提取订单的北京时间注册日期 (每日充值分析全部使用北京时间)
     */
    public static LocalDate getBjRegisterDate(RawOrder order) {
        if (order == null) return null;
        return order.getRegisterTimeBj() != null ? order.getRegisterTimeBj().toLocalDate() : order.getRegisterDateEt();
    }

    /**
     * 提取订单的北京时间支付日期 (每日充值分析全部使用北京时间)
     */
    public static LocalDate getBjPayDate(RawOrder order) {
        if (order == null) return null;
        return order.getPayTimeBj() != null ? order.getPayTimeBj().toLocalDate() : order.getPayDateEt();
    }



    /**
     * 根据特定用户和平台配置的落地页 ID 过滤原始订单
     */
    public List<RawOrder> getOrdersFilteredForUser(String platformCode, Long userId) {
        if (userId == null) userId = 1L;
        List<String> userPIds = userService.getUserLandingPageIds(platformCode, userId);

        if (userPIds == null || userPIds.isEmpty()) {
            return Collections.emptyList();
        }

        List<String> trimmedPIds = userPIds.stream()
                .filter(p -> p != null && !p.trim().isEmpty())
                .map(String::trim)
                .distinct()
                .collect(Collectors.toList());

        if (trimmedPIds.isEmpty()) {
            return Collections.emptyList();
        }

        if (platformCode == null || platformCode.trim().isEmpty() || "ALL".equalsIgnoreCase(platformCode.trim())) {
            return rawOrderRepository.findByLandingPageIdIn(trimmedPIds);
        } else {
            return rawOrderRepository.findByPlatformCodeAndLandingPageIdIn(platformCode.trim().toLowerCase(), trimmedPIds);
        }
    }

    public List<RawOrder> getOrdersFilteredForUser(Long userId) {
        return getOrdersFilteredForUser("ALL", userId);
    }

    /**
     * 计算并持久化指定用户和平台的 LTV 统计表 (仅针对指定用户本身，不触发父级主账号)
     */
    @Transactional
    public void calculateLtvStatsForUserDirect(String platformCode, Long userId) {
        if (userId == null) userId = 1L;
        String pCode = (platformCode != null && !platformCode.trim().isEmpty()) ? platformCode.trim().toLowerCase() : "all";
        boolean isAll = "all".equalsIgnoreCase(pCode);
        String targetPlatform = isAll ? "ALL" : pCode;

        invalidateUserCache(pCode, userId);
        LocalDate todayBj = LocalDate.now(TimeUtils.BEIJING_ZONE);
        LocalDate todayEt = LocalDate.now(TimeUtils.EASTERN_ZONE);
        LocalDate todayUtc = LocalDate.now(TimeUtils.UTC_ZONE);
        LocalDate maxToday = todayBj;
        if (todayEt.isAfter(maxToday)) maxToday = todayEt;
        if (todayUtc.isAfter(maxToday)) maxToday = todayUtc;

        List<LandingPageConfigItem> userPages = userService.getUserLandingPageConfigs(pCode, userId);
        Map<String, String> tzMap = userPages.stream()
                .filter(p -> p.getLandingPageId() != null)
                .collect(Collectors.toMap(p -> p.getLandingPageId().trim(), LandingPageConfigItem::getTimezone, (a, b) -> a));

        List<RawOrder> orders = getOrdersFilteredForUser(pCode, userId);

        if (ltvBenchmarkService != null) {
            ltvBenchmarkService.recalculateBenchmarksForUser(userId);
        }

        LocalDate platformStartDate = getLaunchStartDateForPlatform(targetPlatform);

        Map<LocalDate, List<RawOrder>> ordersByDate = orders.stream()
                .filter(o -> {
                    LocalDate regDate = getEffectiveRegisterDate(o, tzMap);
                    return regDate != null && !regDate.isBefore(platformStartDate);
                })
                .collect(Collectors.groupingBy(o -> getEffectiveRegisterDate(o, tzMap)));

        Map<LocalDate, LtvLaunchConfig> configsByDate;
        String masterRemark = "";
        boolean isMasterAcc = userService.isMasterAccount(userId);
        if (isMasterAcc) {
            List<Long> subUserIds = userService.getSubUserIdsForMaster(userId);
            List<String> subUsernames = new ArrayList<>();
            for (Long subId : subUserIds) {
                userService.findById(subId).ifPresent(u -> subUsernames.add(u.getUsername()));
            }
            String subNamesStr = String.join("、", subUsernames);
            masterRemark = subNamesStr.isEmpty() ? "汇总数据" : "汇总数据（子账号：" + subNamesStr + "）";

            Map<LocalDate, BigDecimal> sumSpendMap = new HashMap<>();
            Map<LocalDate, List<String>> remarkMap = new HashMap<>();
            for (Long subId : subUserIds) {
                List<LtvLaunchConfig> subConfigs = isAll ? ltvLaunchConfigRepository.findByUserId(subId)
                        : ltvLaunchConfigRepository.findByPlatformCodeAndUserId(pCode, subId);
                for (LtvLaunchConfig sc : subConfigs) {
                    if ("ALL".equalsIgnoreCase(sc.getPlatformCode())) continue;
                    if (sc.getLaunchDate() != null) {
                        if (sc.getSpend() != null) {
                            sumSpendMap.merge(sc.getLaunchDate(), sc.getSpend(), BigDecimal::add);
                        }
                        if (sc.getRemark() != null && !sc.getRemark().trim().isEmpty()) {
                            String cleanRemark = sc.getRemark().trim();
                            String formatted = !sc.getLaunchDate().isBefore(MULTI_PLATFORM_START_DATE)
                                    ? formatPlatformRemark(sc.getPlatformCode(), cleanRemark)
                                    : cleanRemark;
                            List<String> rList = remarkMap.computeIfAbsent(sc.getLaunchDate(), k -> new ArrayList<>());
                            if (!rList.contains(formatted)) {
                                rList.add(formatted);
                            }
                        }
                    }
                }
            }
            configsByDate = new HashMap<>();
            Set<LocalDate> allDates = new HashSet<>(sumSpendMap.keySet());
            allDates.addAll(remarkMap.keySet());
            for (LocalDate d : allDates) {
                LtvLaunchConfig mc = new LtvLaunchConfig();
                mc.setPlatformCode(targetPlatform);
                mc.setUserId(userId);
                mc.setLaunchDate(d);
                mc.setSpend(sumSpendMap.getOrDefault(d, BigDecimal.ZERO));
                if (isAll) {
                    List<String> remarks = remarkMap.get(d);
                    mc.setRemark(remarks != null && !remarks.isEmpty() ? String.join(" | ", remarks) : masterRemark);
                } else {
                    mc.setRemark(masterRemark);
                }
                configsByDate.put(d, mc);
            }
        } else {
            List<LtvLaunchConfig> list = isAll ? ltvLaunchConfigRepository.findByUserId(userId)
                    : ltvLaunchConfigRepository.findByPlatformCodeAndUserId(pCode, userId);
            if (isAll) {
                Map<LocalDate, BigDecimal> sumSpendMap = new HashMap<>();
                Map<LocalDate, List<String>> remarkMap = new HashMap<>();
                List<LtvLaunchConfig> sortedList = list.stream()
                        .sorted(Comparator.comparing(c -> c.getPlatformCode() == null ? "" : c.getPlatformCode()))
                        .collect(Collectors.toList());
                for (LtvLaunchConfig c : sortedList) {
                    if ("ALL".equalsIgnoreCase(c.getPlatformCode())) {
                        continue;
                    }
                    if (c.getLaunchDate() != null) {
                        if (c.getSpend() != null) {
                            sumSpendMap.merge(c.getLaunchDate(), c.getSpend(), BigDecimal::add);
                        }
                        if (c.getRemark() != null && !c.getRemark().trim().isEmpty()) {
                            String cleanRemark = c.getRemark().trim();
                            String formatted = !c.getLaunchDate().isBefore(MULTI_PLATFORM_START_DATE)
                                    ? formatPlatformRemark(c.getPlatformCode(), cleanRemark)
                                    : cleanRemark;
                            List<String> rList = remarkMap.computeIfAbsent(c.getLaunchDate(), k -> new ArrayList<>());
                            if (!rList.contains(formatted)) {
                                rList.add(formatted);
                            }
                        }
                    }
                }
                configsByDate = new HashMap<>();
                Set<LocalDate> allDates = new HashSet<>(sumSpendMap.keySet());
                allDates.addAll(remarkMap.keySet());
                for (LocalDate d : allDates) {
                    LtvLaunchConfig ac = new LtvLaunchConfig();
                    ac.setPlatformCode("ALL");
                    ac.setUserId(userId);
                    ac.setLaunchDate(d);
                    ac.setSpend(sumSpendMap.getOrDefault(d, BigDecimal.ZERO));
                    List<String> remarks = remarkMap.get(d);
                    ac.setRemark(remarks != null && !remarks.isEmpty() ? String.join(" | ", remarks) : "");
                    configsByDate.put(d, ac);
                }
            } else {
                configsByDate = list.stream()
                        .collect(Collectors.toMap(LtvLaunchConfig::getLaunchDate, c -> c, (a, b) -> a));
            }
        }

        LocalDate currDate = platformStartDate;
        List<LtvDailyStat> statList = new ArrayList<>();

        Set<String> allSubMemberIds = orders.stream()
                .filter(o -> o.getIsSubs() != null && o.getIsSubs() == 1)
                .map(RawOrder::getMemberId)
                .filter(id -> id != null && !id.trim().isEmpty())
                .collect(Collectors.toSet());

        Map<String, UserSubscriptionPeriod> subPeriodMap = allSubMemberIds.isEmpty() ? Collections.emptyMap() :
                userSubscriptionPeriodRepository.findByMemberIdIn(allSubMemberIds).stream()
                        .filter(p -> p.getMemberId() != null)
                        .collect(Collectors.toMap(p -> p.getMemberId().trim(), p -> p, (a, b) -> a));

        while (!currDate.isAfter(maxToday)) {
            List<RawOrder> cohortOrders = ordersByDate.getOrDefault(currDate, Collections.emptyList());
            LtvLaunchConfig launchConfig = configsByDate.get(currDate);

            BigDecimal spend = launchConfig != null ? launchConfig.getSpend() : BigDecimal.ZERO;
            String remark = (launchConfig != null && launchConfig.getRemark() != null && !launchConfig.getRemark().trim().isEmpty())
                    ? launchConfig.getRemark()
                    : (isMasterAcc ? masterRemark : "");

            LtvDailyStat stat = calculateSingleCohort(userId, currDate, cohortOrders, spend, remark, maxToday, tzMap, subPeriodMap);
            stat.setPlatformCode(targetPlatform);
            statList.add(stat);
            currDate = currDate.plusDays(1);
        }

        ltvDailyStatRepository.deleteByPlatformCodeAndUserId(targetPlatform, userId);
        ltvDailyStatRepository.flush();
        ltvDailyStatRepository.saveAll(statList);
        ltvDailyStatRepository.flush();

        invalidateUserCache(pCode, userId);
    }

    @Transactional
    public void calculateLtvStatsForUserDirect(Long userId) {
        calculateLtvStatsForUserDirect("ALL", userId);
    }

    /**
     * 计算并持久化指定用户的 LTV 统计表，并异步触发关联主账号数据重算
     */
    @Transactional
    public void calculateLtvStatsForUser(String platformCode, Long userId) {
        calculateLtvStatsForUserDirect(platformCode, userId);

        // 异步触发所属主账号的报表重算
        if (asyncRecalculateService != null) {
            asyncRecalculateService.asyncRecalculateMastersForSubUser(userId);
        }
    }

    @Transactional
    public void calculateLtvStatsForUser(Long userId) {
        calculateLtvStatsForUser("ALL", userId);
    }

    /**
     * 重新计算所有用户的 LTV 统计表 (定时任务调用，无外部大事务，循环内部逐用户独立小事务)
     */
    public void calculateAllLtvStatsOnly() {
        List<SysUser> users = userService.listAllUsers();
        String[] platforms = new String[]{"ALL", "rocnovel", "flicknovel"};
        if (users.isEmpty()) {
            for (String p : platforms) {
                calculateLtvStatsForUserDirect(p, 1L);
            }
        } else {
            for (SysUser user : users) {
                for (String p : platforms) {
                    calculateLtvStatsForUserDirect(p, user.getId());
                }
            }
        }
        log.info("LTV calculation completed for all active users across platforms.");
    }

    public void calculateAllLtvStats() {
        calculateAllLtvStatsOnly();
    }

    private LtvDailyStat calculateSingleCohort(Long userId, LocalDate launchDate, List<RawOrder> cohortOrders, BigDecimal spend, String remark, LocalDate maxToday, Map<String, String> tzMap) {
        return calculateSingleCohort(userId, launchDate, cohortOrders, spend, remark, maxToday, tzMap, null);
    }

    private LtvDailyStat calculateSingleCohort(Long userId, LocalDate launchDate, List<RawOrder> cohortOrders, BigDecimal spend, String remark, LocalDate maxToday, Map<String, String> tzMap, Map<String, UserSubscriptionPeriod> preloadedSubPeriodMap) {
        LtvDailyStat stat = new LtvDailyStat();
        stat.setUserId(userId);
        stat.setLaunchDate(launchDate);
        stat.setSpend(spend != null ? spend : BigDecimal.ZERO);
        stat.setRemark(remark != null ? remark : "");

        // 1. 总充值
        BigDecimal totalRecharge = cohortOrders.stream()
                .map(o -> o.getOrderAmountUsd() != null ? o.getOrderAmountUsd() : BigDecimal.ZERO)
                .reduce(BigDecimal.ZERO, BigDecimal::add);

        // 1.1 已退款 (refundStatus == 2)
        BigDecimal totalRefund = cohortOrders.stream()
                .filter(o -> o.getRefundStatus() != null && o.getRefundStatus() == 2)
                .map(o -> o.getOrderAmountUsd() != null ? o.getOrderAmountUsd() : BigDecimal.ZERO)
                .reduce(BigDecimal.ZERO, BigDecimal::add);

        // 2. 总盈亏 = 累计充值 - 已退款 - 消耗
        BigDecimal totalProfit = totalRecharge.subtract(totalRefund).subtract(stat.getSpend());

        // 2.1 累计 ROI = (累计充值 - 已退款) / 消耗
        BigDecimal totalRoi = BigDecimal.ZERO;
        if (stat.getSpend().compareTo(BigDecimal.ZERO) > 0) {
            totalRoi = totalRecharge.subtract(totalRefund).divide(stat.getSpend(), 4, RoundingMode.HALF_UP);
        }

        // 3. 累计订阅用户数
        List<RawOrder> subOrders = cohortOrders.stream()
                .filter(o -> o.getIsSubs() != null && o.getIsSubs() == 1)
                .collect(Collectors.toList());

        Set<String> subMemberIds = subOrders.stream()
                .map(RawOrder::getMemberId)
                .filter(id -> id != null && !id.trim().isEmpty())
                .collect(Collectors.toSet());

        long subUserCount = subMemberIds.size();

        // 3.0 从 user_subscription_period 优先从预查 Map 关联该 Cohort 订阅用户的订阅周期天数，避免循环逐天查库
        Integer detectedPeriod = 1;
        if (!subMemberIds.isEmpty()) {
            List<UserSubscriptionPeriod> userSubPeriods;
            if (preloadedSubPeriodMap != null) {
                userSubPeriods = subMemberIds.stream()
                        .map(preloadedSubPeriodMap::get)
                        .filter(Objects::nonNull)
                        .collect(Collectors.toList());
            } else {
                userSubPeriods = userSubscriptionPeriodRepository.findByMemberIdIn(subMemberIds);
            }
            if (!userSubPeriods.isEmpty()) {
                Map<Integer, Long> periodCountMap = userSubPeriods.stream()
                        .filter(p -> p.getSubPeriodDays() != null)
                        .collect(Collectors.groupingBy(UserSubscriptionPeriod::getSubPeriodDays, Collectors.counting()));
                detectedPeriod = periodCountMap.entrySet().stream()
                        .max(Map.Entry.comparingByValue())
                        .map(Map.Entry::getKey)
                        .orElse(1);

                StringBuilder sb = new StringBuilder("{");
                boolean first = true;
                for (Map.Entry<Integer, Long> entry : periodCountMap.entrySet()) {
                    if (!first) sb.append(",");
                    sb.append("\"").append(entry.getKey()).append("\":").append(entry.getValue());
                    first = false;
                }
                sb.append("}");
                stat.setSubPeriodDistribution(sb.toString());
            } else {
                stat.setSubPeriodDistribution(null);
            }
        } else {
            stat.setSubPeriodDistribution(null);
        }
        stat.setSubPeriodDays(detectedPeriod);

        // 3.1 订阅用户 7 日留存（> 7天后仍有充值的去重人数）
        LocalDate day8Date = launchDate.plusDays(7);
        if (!day8Date.isAfter(maxToday)) {
            Set<String> retainedMemberIds = cohortOrders.stream()
                    .filter(o -> o.getMemberId() != null && subMemberIds.contains(o.getMemberId()))
                    .filter(o -> {
                        LocalDate payDate = getEffectivePayDate(o, tzMap);
                        if (payDate == null) return false;
                        long diff = ChronoUnit.DAYS.between(launchDate, payDate);
                        return diff >= 7;
                    })
                    .map(RawOrder::getMemberId)
                    .collect(Collectors.toSet());

            long day7SubCount = retainedMemberIds.size();
            BigDecimal retention = BigDecimal.ZERO;
            if (subUserCount > 0) {
                retention = BigDecimal.valueOf(day7SubCount)
                        .divide(BigDecimal.valueOf(subUserCount), 4, RoundingMode.HALF_UP);
            }
            stat.setDay7SubUserCount((int) day7SubCount);
            stat.setDay7SubUserRetention(retention);
        } else {
            stat.setDay7SubUserCount(null);
            stat.setDay7SubUserRetention(null);
        }

        // 3.2 订阅用户 15 日留存（> 15天后仍有充值的去重人数）
        LocalDate day16Date = launchDate.plusDays(15);
        if (!day16Date.isAfter(maxToday)) {
            Set<String> retainedMemberIds15 = cohortOrders.stream()
                    .filter(o -> o.getMemberId() != null && subMemberIds.contains(o.getMemberId()))
                    .filter(o -> {
                        LocalDate payDate = getEffectivePayDate(o, tzMap);
                        if (payDate == null) return false;
                        long diff = ChronoUnit.DAYS.between(launchDate, payDate);
                        return diff >= 15;
                    })
                    .map(RawOrder::getMemberId)
                    .collect(Collectors.toSet());

            long day15SubCount = retainedMemberIds15.size();
            BigDecimal retention15 = BigDecimal.ZERO;
            if (subUserCount > 0) {
                retention15 = BigDecimal.valueOf(day15SubCount)
                        .divide(BigDecimal.valueOf(subUserCount), 4, RoundingMode.HALF_UP);
            }
            stat.setDay15SubUserCount((int) day15SubCount);
            stat.setDay15SubUserRetention(retention15);
        } else {
            stat.setDay15SubUserCount(null);
            stat.setDay15SubUserRetention(null);
        }

        // 4. 订阅用户成本
        BigDecimal subUserCost = BigDecimal.ZERO;
        if (subUserCount > 0 && stat.getSpend().compareTo(BigDecimal.ZERO) > 0) {
            subUserCost = stat.getSpend().divide(BigDecimal.valueOf(subUserCount), 2, RoundingMode.HALF_UP);
        }

        stat.setTotalRecharge(totalRecharge);
        stat.setTotalRefund(totalRefund);
        stat.setTotalProfit(totalProfit);
        stat.setTotalRoi(totalRoi);
        stat.setSubUserCount((int) subUserCount);
        stat.setSubUserCost(subUserCost);

        // 5. Day 1 ~ Day 60 充值与 ROI 计算
        for (int day = 1; day <= 60; day++) {
            LocalDate dayTargetDate = launchDate.plusDays(day - 1);
            if (dayTargetDate.isAfter(maxToday)) {
                stat.setDayData(day, null, null);
                continue;
            }

            BigDecimal dayCumRecharge = cohortOrders.stream()
                    .filter(o -> {
                        LocalDate payDate = getEffectivePayDate(o, tzMap);
                        return payDate != null && !payDate.isAfter(dayTargetDate);
                    })
                    .map(o -> o.getOrderAmountUsd() != null ? o.getOrderAmountUsd() : BigDecimal.ZERO)
                    .reduce(BigDecimal.ZERO, BigDecimal::add);

            BigDecimal dayCumRoi = null;
            if (stat.getSpend().compareTo(BigDecimal.ZERO) > 0) {
                dayCumRoi = dayCumRecharge.divide(stat.getSpend(), 4, RoundingMode.HALF_UP);
            }

            stat.setDayData(day, dayCumRecharge, dayCumRoi);
        }

        // 6. 回本预测计算 (最小二乘法对数拟合)
        long daysElapsed = ChronoUnit.DAYS.between(launchDate, maxToday) + 1;
        PredictionResult pred = ltvPredictService.predictCohort(stat, (int) daysElapsed);
        stat.setPredictedPaybackDays(pred.getPredictedPaybackDays());
        stat.setPredictedDay30Roi(pred.getPredictedDay30Roi());
        stat.setPredictedDay90Roi(pred.getPredictedDay90Roi());
        stat.setPredictedDay60Roi(pred.getPredictedDay60Roi());
        stat.setPredictedDay30Recharge(pred.getPredictedDay30Recharge());
        stat.setPredictedDay60Recharge(pred.getPredictedDay60Recharge());
        stat.setPredictedDay90Recharge(pred.getPredictedDay90Recharge());

        return stat;
    }

    @Transactional
    public List<LtvDailyStat> getLtvDailyStats(String platformCode, Long userId) {
        if (userId == null) userId = 1L;
        String pCode = (platformCode != null && !platformCode.trim().isEmpty()) ? platformCode.trim().toUpperCase() : "ALL";
        String targetPlatform = "ALL".equalsIgnoreCase(pCode) ? "ALL" : pCode.toLowerCase();
        LocalDate platformStartDate = getLaunchStartDateForPlatform(targetPlatform);
        List<LtvDailyStat> list = ltvDailyStatRepository.findByPlatformCodeAndUserIdAndLaunchDateGreaterThanEqualOrderByLaunchDateAsc(targetPlatform, userId, platformStartDate);
        boolean allZeros = !list.isEmpty() && list.stream().allMatch(s ->
                (s.getSpend() == null || s.getSpend().compareTo(BigDecimal.ZERO) == 0) &&
                (s.getTotalRecharge() == null || s.getTotalRecharge().compareTo(BigDecimal.ZERO) == 0)
        );

        LocalDate todayBj = LocalDate.now(TimeUtils.BEIJING_ZONE);
        LocalDate todayEt = LocalDate.now(TimeUtils.EASTERN_ZONE);
        LocalDate todayUtc = LocalDate.now(TimeUtils.UTC_ZONE);
        LocalDate maxToday = todayBj;
        if (todayEt.isAfter(maxToday)) maxToday = todayEt;
        if (todayUtc.isAfter(maxToday)) maxToday = todayUtc;

        boolean isMissingToday = !list.isEmpty() && list.get(list.size() - 1).getLaunchDate().isBefore(maxToday);
        boolean isMissingStartDate = !list.isEmpty() && list.get(0).getLaunchDate().isAfter(platformStartDate);

        if (list.isEmpty() || allZeros || isMissingStartDate || isMissingToday) {
            calculateLtvStatsForUser(targetPlatform, userId);
            list = ltvDailyStatRepository.findByPlatformCodeAndUserIdAndLaunchDateGreaterThanEqualOrderByLaunchDateAsc(targetPlatform, userId, platformStartDate);
        }
        return list;
    }

    @Transactional
    public List<LtvDailyStat> getLtvDailyStats(Long userId) {
        return getLtvDailyStats("ALL", userId);
    }

    public List<LtvDailyStat> getLtvDailyStats() {
        return getLtvDailyStats("ALL", 1L);
    }

    /**
     * 计算指定用户和平台的整体全盘预测回本天数
     */
    public Integer getOverallPredictedPaybackDays(String platformCode, Long userId) {
        List<LtvDailyStat> list = getLtvDailyStats(platformCode, userId);
        PredictionResult pred = ltvPredictService.predictOverallCohort(list);
        return pred != null ? pred.getPredictedPaybackDays() : null;
    }

    public Integer getOverallPredictedPaybackDays(Long userId) {
        return getOverallPredictedPaybackDays("ALL", userId);
    }

    public PredictionResult getOverallPredictionResult(String platformCode, Long userId) {
        List<LtvDailyStat> list = getLtvDailyStats(platformCode, userId);
        return ltvPredictService.predictOverallCohort(list);
    }

    public PredictionResult getOverallPredictionResult(Long userId) {
        return getOverallPredictionResult("ALL", userId);
    }

    private static final int MAX_RESPONSE_CACHE_SIZE = 100;
    private final Map<String, CachedLtvResponse> ltvListResponseCache = Collections.synchronizedMap(
            new LinkedHashMap<String, CachedLtvResponse>(64, 0.75f, true) {
                @Override
                protected boolean removeEldestEntry(Map.Entry<String, CachedLtvResponse> eldest) {
                    return size() > MAX_RESPONSE_CACHE_SIZE;
                }
            }
    );

    public void clearResponseCache() {
        ltvListResponseCache.clear();
    }

    public void cleanExpiredResponseCache() {
        long ttlMs = 10 * 60 * 1000L;
        synchronized (ltvListResponseCache) {
            ltvListResponseCache.entrySet().removeIf(entry -> entry.getValue() == null || entry.getValue().isExpired(ttlMs));
        }
    }

    public void invalidateUserCache(String platformCode, Long userId) {
        if (userId != null) {
            String p = (platformCode != null && !platformCode.trim().isEmpty()) ? platformCode.trim().toLowerCase() : "all";
            ltvListResponseCache.remove(p + "_" + userId);
            ltvListResponseCache.remove("all_" + userId);
        } else {
            ltvListResponseCache.clear();
        }
    }

    public void invalidateUserCache(Long userId) {
        if (userId != null) {
            synchronized (ltvListResponseCache) {
                ltvListResponseCache.keySet().removeIf(k -> k.endsWith("_" + userId));
            }
        } else {
            ltvListResponseCache.clear();
        }
    }

    /**
     * 方案一 + 方案二入口：带内存缓存与 Context 共享的极速 LTV 响应获取方法 (支持平台切片，带最大容量约束与主动过期剔除)
     */
    public LtvListResponseDto getLtvListResponse(String platformCode, Long userId) {
        if (userId == null) userId = 1L;
        String pCode = (platformCode != null && !platformCode.trim().isEmpty()) ? platformCode.trim().toLowerCase() : "all";
        String cacheKey = pCode + "_" + userId;
        CachedLtvResponse cached = ltvListResponseCache.get(cacheKey);
        if (cached != null) {
            if (!cached.isExpired(10 * 60 * 1000L)) {
                return cached.data;
            } else {
                ltvListResponseCache.remove(cacheKey);
            }
        }

        LtvListResponseDto freshResponse = computeLtvListResponse(pCode, userId);
        ltvListResponseCache.put(cacheKey, new CachedLtvResponse(freshResponse));
        return freshResponse;
    }

    public LtvListResponseDto getLtvListResponse(Long userId) {
        return getLtvListResponse("ALL", userId);
    }

    public LtvListResponseDto computeLtvListResponse(String platformCode, Long userId) {
        if (userId == null) userId = 1L;
        String pCode = (platformCode != null && !platformCode.trim().isEmpty()) ? platformCode.trim().toLowerCase() : "all";
        List<LtvDailyStat> list = getLtvDailyStats(pCode, userId);
        if (list.isEmpty()) {
            calculateLtvStatsForUser(pCode, userId);
            list = getLtvDailyStats(pCode, userId);
        }

        List<RawOrder> userOrders = getOrdersFilteredForUser(pCode, userId);
        List<LandingPageConfigItem> userPages = userService.getUserLandingPageConfigs(pCode, userId);
        Map<String, String> tzMap = (userPages != null) ? userPages.stream()
                .filter(p -> p.getLandingPageId() != null)
                .collect(Collectors.toMap(p -> p.getLandingPageId().trim(), LandingPageConfigItem::getTimezone, (a, b) -> a)) : Collections.emptyMap();

        UserCalculationContext ctx = new UserCalculationContext(pCode, userId, list, userOrders, tzMap);

        PredictionResult overallPred = ltvPredictService.predictOverallCohort(list);
        Integer overallPayback = overallPred != null ? overallPred.getPredictedPaybackDays() : null;
        Integer overallPaybackCycle = overallPred != null ? overallPred.getPaybackCycleDays() : null;

        MonthlySummaryDto monthlySummary = getMonthlySummaryForUser(userId, ctx);
        RetainedSubscribersDto overallRetention = calculateRetainedSubscribers(userId, userOrders, ctx);

        LtvListResponseDto response = new LtvListResponseDto();
        response.setCode(0);
        response.setMsg("success");
        response.setData(list);
        response.setOverallPredictedPaybackDays(overallPayback);
        response.setOverallPaybackCycleDays(overallPaybackCycle);
        if (overallPred != null) {
            response.setOverallPredictedDay30Roi(overallPred.getPredictedDay30Roi());
            response.setOverallPredictedDay60Roi(overallPred.getPredictedDay60Roi());
            response.setOverallPredictedDay90Roi(overallPred.getPredictedDay90Roi());
            response.setOverallPredictedDay30Recharge(overallPred.getPredictedDay30Recharge());
            response.setOverallPredictedDay60Recharge(overallPred.getPredictedDay60Recharge());
            response.setOverallPredictedDay90Recharge(overallPred.getPredictedDay90Recharge());
        }
        response.setMonthlySummary(monthlySummary);
        response.setOverallRetainedSubUsers(overallRetention != null ? overallRetention.getRetainedSubUsers() : 0);
        response.setOverallRetainedRate(overallRetention != null ? overallRetention.getRetainedRate() : "0.00%");
        response.setTotal(list.size());
        response.setUserId(userId);
        return response;
    }

    public MonthlySummaryDto getMonthlySummaryForUser(String platformCode, Long userId) {
        if (userId == null) userId = 1L;
        String pCode = (platformCode != null && !platformCode.trim().isEmpty()) ? platformCode.trim() : "ALL";
        List<LtvDailyStat> allStats = getLtvDailyStats(pCode, userId);
        List<RawOrder> userOrders = getOrdersFilteredForUser(pCode, userId);
        List<LandingPageConfigItem> userPages = userService.getUserLandingPageConfigs(pCode, userId);
        Map<String, String> tzMap = (userPages != null) ? userPages.stream()
                .filter(p -> p.getLandingPageId() != null)
                .collect(Collectors.toMap(p -> p.getLandingPageId().trim(), LandingPageConfigItem::getTimezone, (a, b) -> a)) : Collections.emptyMap();

        UserCalculationContext ctx = new UserCalculationContext(pCode, userId, allStats, userOrders, tzMap);
        return getMonthlySummaryForUser(userId, ctx);
    }

    public MonthlySummaryDto getMonthlySummaryForUser(Long userId) {
        return getMonthlySummaryForUser("ALL", userId);
    }

    public MonthlySummaryDto getMonthlySummaryForUser(Long userId, UserCalculationContext ctx) {
        LocalDate todayBj = LocalDate.now(ZoneId.of("Asia/Shanghai"));
        YearMonth currentMonth = YearMonth.from(todayBj);
        LocalDate platStartDate = getLaunchStartDateForPlatform(ctx != null ? ctx.platformCode : "ALL");
        YearMonth minMonth = YearMonth.from(platStartDate);

        List<SingleMonthSummaryDto> months = new ArrayList<>();
        for (int i = 0; i < 4; i++) {
            YearMonth ym = currentMonth.minusMonths(i);
            if (ym.isBefore(minMonth)) {
                break;
            }
            boolean isPastMonth = (i > 0);
            SingleMonthSummaryDto summary = getSingleMonthSummary(userId, ym, isPastMonth, ctx);
            months.add(summary);
        }
        return new MonthlySummaryDto(months);
    }

    public RetainedSubscribersDto calculateRetainedSubscribers(Long userId, List<RawOrder> orders, UserCalculationContext ctx) {
        return calculateRetainedSubscribers(userId, orders, ctx != null ? ctx.tzMap : null);
    }

    public RetainedSubscribersDto calculateRetainedSubscribers(Long userId, List<RawOrder> orders) {
        return calculateRetainedSubscribers(userId, orders, (Map<String, String>) null);
    }

    public RetainedSubscribersDto calculateRetainedSubscribers(Long userId, List<RawOrder> orders, Map<String, String> passedTzMap) {
        if (orders == null || orders.isEmpty()) {
            return new RetainedSubscribersDto(0, 0, "0.00%");
        }

        List<RawOrder> subOrders = orders.stream()
                .filter(o -> o.getIsSubs() != null && o.getIsSubs() == 1)
                .collect(Collectors.toList());

        Set<String> subMemberIds = subOrders.stream()
                .map(RawOrder::getMemberId)
                .filter(id -> id != null && !id.trim().isEmpty())
                .collect(Collectors.toSet());

        int totalSubCount = subMemberIds.size();
        if (totalSubCount == 0) {
            return new RetainedSubscribersDto(0, 0, "0.00%");
        }

        List<UserSubscriptionPeriod> subPeriods = userSubscriptionPeriodRepository.findByMemberIdIn(subMemberIds);
        Map<String, Integer> periodMap = subPeriods.stream()
                .filter(p -> p.getMemberId() != null && p.getSubPeriodDays() != null)
                .collect(Collectors.toMap(p -> p.getMemberId().trim(), UserSubscriptionPeriod::getSubPeriodDays, (a, b) -> a));

        Map<String, String> tzMap = passedTzMap;
        if (tzMap == null) {
            List<LandingPageConfigItem> userPages = userService.getUserLandingPageConfigs(userId);
            tzMap = (userPages != null) ? userPages.stream()
                    .filter(p -> p.getLandingPageId() != null)
                    .collect(Collectors.toMap(p -> p.getLandingPageId().trim(), LandingPageConfigItem::getTimezone, (a, b) -> a)) : Collections.emptyMap();
        }

        LocalDate todayBj = LocalDate.now(ZoneId.of("Asia/Shanghai"));

        Map<String, LocalDate> lastPayDateMap = new HashMap<>();
        for (RawOrder o : orders) {
            if (o.getMemberId() == null || !subMemberIds.contains(o.getMemberId())) continue;
            LocalDate payDate = getEffectivePayDate(o, tzMap);
            if (payDate == null) continue;

            LocalDate prev = lastPayDateMap.get(o.getMemberId());
            if (prev == null || payDate.isAfter(prev)) {
                lastPayDateMap.put(o.getMemberId(), payDate);
            }
        }

        int retainedCount = 0;
        for (String memberId : subMemberIds) {
            LocalDate lastPay = lastPayDateMap.get(memberId);
            if (lastPay == null) continue;

            int periodDays = periodMap.getOrDefault(memberId, 1);
            LocalDate expireDate = lastPay.plusDays(periodDays);
            if (!todayBj.isAfter(expireDate)) {
                retainedCount++;
            }
        }

        BigDecimal rate = totalSubCount > 0
                ? BigDecimal.valueOf(retainedCount).divide(BigDecimal.valueOf(totalSubCount), 4, RoundingMode.HALF_UP).multiply(new BigDecimal("100"))
                : BigDecimal.ZERO;

        return new RetainedSubscribersDto(totalSubCount, retainedCount, rate.setScale(2, RoundingMode.HALF_UP).toString() + "%");
    }

    public SingleMonthSummaryDto getSingleMonthSummary(Long userId, YearMonth yearMonth, boolean isPastMonth, List<LtvDailyStat> allStats, List<RawOrder> userOrders) {
        List<LandingPageConfigItem> userPages = userService.getUserLandingPageConfigs(userId);
        Map<String, String> tzMap = (userPages != null) ? userPages.stream()
                .filter(p -> p.getLandingPageId() != null)
                .collect(Collectors.toMap(p -> p.getLandingPageId().trim(), LandingPageConfigItem::getTimezone, (a, b) -> a)) : Collections.emptyMap();
        UserCalculationContext ctx = new UserCalculationContext(userId, allStats, userOrders, tzMap);
        return getSingleMonthSummary(userId, yearMonth, isPastMonth, ctx);
    }

    public SingleMonthSummaryDto getSingleMonthSummary(Long userId, YearMonth yearMonth, boolean isPastMonth, UserCalculationContext ctx) {
        String monthStr = yearMonth.toString();

        List<LtvDailyStat> monthStats = ctx.allStats.stream()
                .filter(s -> s.getLaunchDate() != null && YearMonth.from(s.getLaunchDate()).equals(yearMonth))
                .collect(Collectors.toList());

        BigDecimal totalSpend = monthStats.stream()
                .map(LtvDailyStat::getSpend)
                .filter(Objects::nonNull)
                .reduce(BigDecimal.ZERO, BigDecimal::add);

        BigDecimal totalRecharge = monthStats.stream()
                .map(LtvDailyStat::getTotalRecharge)
                .filter(Objects::nonNull)
                .reduce(BigDecimal.ZERO, BigDecimal::add);

        BigDecimal totalRefund = monthStats.stream()
                .map(LtvDailyStat::getTotalRefund)
                .filter(Objects::nonNull)
                .reduce(BigDecimal.ZERO, BigDecimal::add);

        BigDecimal profit = totalRecharge.subtract(totalRefund).subtract(totalSpend);

        BigDecimal roi = totalSpend.compareTo(BigDecimal.ZERO) > 0
                ? totalRecharge.subtract(totalRefund).divide(totalSpend, 4, RoundingMode.HALF_UP).multiply(new BigDecimal("100"))
                : BigDecimal.ZERO;

        List<RawOrder> monthOrders = ctx.userOrders.stream()
                .filter(o -> {
                    LocalDate regDate = getEffectiveRegisterDate(o, ctx.tzMap);
                    return regDate != null && YearMonth.from(regDate).equals(yearMonth);
                })
                .collect(Collectors.toList());

        RetainedSubscribersDto retentionInfo = calculateRetainedSubscribers(userId, monthOrders, ctx);

        // 仅历史月份统计实际回本天数与进行 D30/D60/D90 ROI 预测，本月进行中不作全月预测
        Integer actualPaybackDays = isPastMonth ? calculateActualPaybackDaysForMonth(userId, yearMonth, monthStats, totalSpend, totalRecharge, ctx) : null;

        BigDecimal predictedDay30Roi = null;
        BigDecimal predictedDay60Roi = null;
        BigDecimal predictedDay90Roi = null;

        if (isPastMonth && totalSpend.compareTo(BigDecimal.ZERO) > 0 && !monthStats.isEmpty()) {
            BigDecimal sumD30Recharge = BigDecimal.ZERO;
            BigDecimal sumD60Recharge = BigDecimal.ZERO;
            BigDecimal sumD90Recharge = BigDecimal.ZERO;

            LocalDate maxToday = LocalDate.now(ZoneId.of("Asia/Shanghai"));

            for (LtvDailyStat s : monthStats) {
                if (s.getSpend() != null && s.getSpend().compareTo(BigDecimal.ZERO) > 0 && s.getLaunchDate() != null) {
                    BigDecimal d30 = s.getPredictedDay30Recharge();
                    BigDecimal d60 = s.getPredictedDay60Recharge();
                    BigDecimal d90 = s.getPredictedDay90Recharge();

                    if (d30 == null || d60 == null || d90 == null) {
                        long daysElapsed = ChronoUnit.DAYS.between(s.getLaunchDate(), maxToday) + 1;
                        PredictionResult pred = ltvPredictService.predictCohort(s, (int) daysElapsed);

                        if (d30 == null) d30 = pred.getPredictedDay30Recharge() != null ? pred.getPredictedDay30Recharge() : (s.getTotalRecharge() != null ? s.getTotalRecharge() : BigDecimal.ZERO);
                        if (d60 == null) d60 = pred.getPredictedDay60Recharge() != null ? pred.getPredictedDay60Recharge() : d30;
                        if (d90 == null) d90 = pred.getPredictedDay90Recharge() != null ? pred.getPredictedDay90Recharge() : d60;
                    }

                    sumD30Recharge = sumD30Recharge.add(d30);
                    sumD60Recharge = sumD60Recharge.add(d60);
                    sumD90Recharge = sumD90Recharge.add(d90);
                }
            }

            predictedDay30Roi = sumD30Recharge.divide(totalSpend, 4, RoundingMode.HALF_UP);
            predictedDay60Roi = sumD60Recharge.divide(totalSpend, 4, RoundingMode.HALF_UP);
            predictedDay90Roi = sumD90Recharge.divide(totalSpend, 4, RoundingMode.HALF_UP);
        }

        SingleMonthSummaryDto dto = new SingleMonthSummaryDto();
        dto.setMonth(monthStr);
        dto.setSpend(totalSpend);
        dto.setRecharge(totalRecharge);
        dto.setRefund(totalRefund);
        dto.setProfit(profit);
        dto.setRoi(roi.setScale(2, RoundingMode.HALF_UP));
        dto.setSubUsers(retentionInfo != null ? retentionInfo.getSubUsers() : 0);
        dto.setRetainedSubUsers(retentionInfo != null ? retentionInfo.getRetainedSubUsers() : 0);
        dto.setRetainedRate(retentionInfo != null ? retentionInfo.getRetainedRate() : "0.00%");
        dto.setActualPaybackDays(actualPaybackDays);
        dto.setPredictedDay30Roi(predictedDay30Roi);
        dto.setPredictedDay60Roi(predictedDay60Roi);
        dto.setPredictedDay90Roi(predictedDay90Roi);
        return dto;
    }

    private Integer calculateActualPaybackDaysForMonth(Long userId, YearMonth yearMonth, List<LtvDailyStat> monthStats, BigDecimal totalSpend, BigDecimal totalRecharge, List<RawOrder> filteredOrders) {
        List<LandingPageConfigItem> userPages = userService.getUserLandingPageConfigs(userId);
        Map<String, String> tzMap = (userPages != null) ? userPages.stream()
                .filter(p -> p.getLandingPageId() != null)
                .collect(Collectors.toMap(p -> p.getLandingPageId().trim(), LandingPageConfigItem::getTimezone, (a, b) -> a)) : Collections.emptyMap();
        return calculateActualPaybackDaysForMonth(userId, yearMonth, monthStats, totalSpend, totalRecharge, filteredOrders, tzMap);
    }

    private Integer calculateActualPaybackDaysForMonth(Long userId, YearMonth yearMonth, List<LtvDailyStat> monthStats, BigDecimal totalSpend, BigDecimal totalRecharge, UserCalculationContext ctx) {
        return calculateActualPaybackDaysForMonth(userId, yearMonth, monthStats, totalSpend, totalRecharge, ctx.userOrders, ctx.tzMap);
    }

    private Integer calculateActualPaybackDaysForMonth(Long userId, YearMonth yearMonth, List<LtvDailyStat> monthStats, BigDecimal totalSpend, BigDecimal totalRecharge, List<RawOrder> filteredOrders, Map<String, String> tzMap) {
        LocalDate todayBj = LocalDate.now(ZoneId.of("Asia/Shanghai"));

        // 1. 严格判断：若总消耗 <= 0 或 截至目前的总充值 < 总消耗 (未回本)，绝对不进行计算与推测，直接返回 null
        if (totalSpend == null || totalSpend.compareTo(BigDecimal.ZERO) <= 0 || totalRecharge == null || totalRecharge.compareTo(totalSpend) < 0) {
            return null;
        }

        // 2. 找出上月首个有消耗的自然日 (firstSpendDate) 作为 Day 1 基准
        List<LtvDailyStat> validSpendStats = monthStats.stream()
                .filter(s -> s.getLaunchDate() != null && s.getSpend() != null && s.getSpend().compareTo(BigDecimal.ZERO) > 0)
                .collect(Collectors.toList());

        if (validSpendStats.isEmpty()) {
            return null;
        }

        LocalDate firstSpendDate = validSpendStats.stream()
                .map(LtvDailyStat::getLaunchDate)
                .min(LocalDate::compareTo)
                .orElse(yearMonth.atDay(1));

        if (firstSpendDate.isAfter(todayBj)) {
            return null;
        }

        // 3. 过滤出上月份 (yearMonth) 注册/投放的用户订单，且支付日期严格截至今天 (绝不上摸未来时间)
        String pCode = (monthStats != null && !monthStats.isEmpty() && monthStats.get(0).getPlatformCode() != null)
                ? monthStats.get(0).getPlatformCode() : "ALL";
        LocalDate platStartDate = getLaunchStartDateForPlatform(pCode);
        List<RawOrder> monthOrders = filteredOrders.stream()
                .filter(o -> {
                    LocalDate regDate = getEffectiveRegisterDate(o, tzMap);
                    LocalDate payDate = getEffectivePayDate(o, tzMap);
                    return regDate != null && !regDate.isBefore(platStartDate) && YearMonth.from(regDate).equals(yearMonth) 
                            && payDate != null && !payDate.isAfter(todayBj);
                })
                .collect(Collectors.toList());

        if (monthOrders.isEmpty()) {
            return null;
        }

        // 4. 按订单生效支付日期 (payDate) 归集每日充值额 (早于首个消耗日的归计到首个消耗日)
        Map<LocalDate, BigDecimal> rechargeByPayDate = new TreeMap<>();
        for (RawOrder o : monthOrders) {
            LocalDate payDate = getEffectivePayDate(o, tzMap);
            if (payDate == null) continue;

            LocalDate effectivePay = payDate.isBefore(firstSpendDate) ? firstSpendDate : payDate;
            BigDecimal price = o.getOrderAmountUsd() != null ? o.getOrderAmountUsd() : BigDecimal.ZERO;
            rechargeByPayDate.put(effectivePay, rechargeByPayDate.getOrDefault(effectivePay, BigDecimal.ZERO).add(price));
        }

        // 5. 截止计算日期严格限制为今天 (todayBj)
        LocalDate maxOrderPayDate = rechargeByPayDate.keySet().stream().max(LocalDate::compareTo).orElse(firstSpendDate);
        LocalDate limitDate = maxOrderPayDate.isAfter(todayBj) ? todayBj : maxOrderPayDate;

        // 6. 从首个消耗日 firstSpendDate 开始按自然日推进累加充值额，N 必须截至今天
        BigDecimal cumulative = BigDecimal.ZERO;
        BigDecimal targetThreshold = totalSpend.subtract(new BigDecimal("0.01")); // 允许1分钱舍入误差
        LocalDate currDate = firstSpendDate;

        while (!currDate.isAfter(limitDate)) {
            BigDecimal dayAmount = rechargeByPayDate.getOrDefault(currDate, BigDecimal.ZERO);
            cumulative = cumulative.add(dayAmount);

            // 当累计充值达到或超越总消耗时，该自然日即为实际回本日期
            if (cumulative.compareTo(targetThreshold) >= 0) {
                // 回本天数 = 回本日期 - 首个消耗日(firstSpendDate) + 1
                long days = ChronoUnit.DAYS.between(firstSpendDate, currDate) + 1;
                return (int) days;
            }
            currDate = currDate.plusDays(1);
        }

        // 7. 若截至今天逐自然日累加后仍未达到总消耗（极特殊数据微差），防护返回 null，决不上摸或假定未来天数
        return null;
    }

    /**
     * 格式化大盘汇总的平台备注前缀：平台名称：xxxx
     */
    private String formatPlatformRemark(String platformCode, String rawRemark) {
        if (rawRemark == null || rawRemark.trim().isEmpty()) {
            return "";
        }
        String pName = PlatformEnum.fromCode(platformCode)
                .map(PlatformEnum::getDisplayName)
                .orElse(platformCode != null && !platformCode.trim().isEmpty() ? platformCode.trim() : "其他");
        String cleanRemark = rawRemark.trim();
        String prefix = pName + "：";
        if (cleanRemark.startsWith(prefix)) {
            return cleanRemark;
        }
        return prefix + cleanRemark;
    }
}
