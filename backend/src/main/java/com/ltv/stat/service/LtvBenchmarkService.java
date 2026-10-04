package com.ltv.stat.service;

import com.ltv.stat.dto.LandingPageConfigItem;
import com.ltv.stat.entity.LtvPredictBenchmark;
import com.ltv.stat.entity.RawOrder;
import com.ltv.stat.entity.SysUser;
import com.ltv.stat.entity.UserSubscriptionPeriod;
import com.ltv.stat.repository.LtvPredictBenchmarkRepository;
import com.ltv.stat.repository.RawOrderRepository;
import com.ltv.stat.repository.UserSubscriptionPeriodRepository;
import com.ltv.stat.service.engine.PredictAlgorithmConstants;
import org.slf4j.Logger;
import org.slf4j.LoggerFactory;
import org.springframework.beans.factory.annotation.Autowired;
import org.springframework.scheduling.annotation.Scheduled;
import org.springframework.stereotype.Service;
import org.springframework.transaction.PlatformTransactionManager;
import org.springframework.transaction.annotation.Transactional;
import org.springframework.transaction.support.TransactionTemplate;

import java.math.BigDecimal;
import java.math.RoundingMode;
import java.time.LocalDate;
import java.time.temporal.ChronoUnit;
import java.util.*;
import java.util.concurrent.ConcurrentHashMap;
import java.util.stream.Collectors;

@Service
public class LtvBenchmarkService {

    private static final Logger log = LoggerFactory.getLogger(LtvBenchmarkService.class);

    private final RawOrderRepository rawOrderRepository;
    private final LtvPredictBenchmarkRepository benchmarkRepository;
    private final UserSubscriptionPeriodRepository userSubscriptionPeriodRepository;
    private final UserService userService;
    private final Map<String, List<LtvPredictBenchmark>> benchmarkCurveCache = new ConcurrentHashMap<>();
    private final TransactionTemplate transactionTemplate;

    @Autowired
    public LtvBenchmarkService(RawOrderRepository rawOrderRepository,
                               LtvPredictBenchmarkRepository benchmarkRepository,
                               UserSubscriptionPeriodRepository userSubscriptionPeriodRepository,
                               @Autowired(required = false) UserService userService,
                               @Autowired(required = false) PlatformTransactionManager transactionManager) {
        this.rawOrderRepository = rawOrderRepository;
        this.benchmarkRepository = benchmarkRepository;
        this.userSubscriptionPeriodRepository = userSubscriptionPeriodRepository;
        this.userService = userService;
        this.transactionTemplate = transactionManager != null ? new TransactionTemplate(transactionManager) : null;
    }

    public LtvBenchmarkService(RawOrderRepository rawOrderRepository,
                               LtvPredictBenchmarkRepository benchmarkRepository,
                               UserSubscriptionPeriodRepository userSubscriptionPeriodRepository) {
        this(rawOrderRepository, benchmarkRepository, userSubscriptionPeriodRepository, null, null);
    }

    public void clearBenchmarkCache() {
        benchmarkCurveCache.clear();
    }

    /**
     * 按特定系统用户配置的落地页 ID 重新计算并保存该用户的专属基准衰减曲线 (USER 维度)
     */
    @Transactional
    public void recalculateBenchmarksForUser(Long userId) {
        if (userId == null) userId = 1L;
        if (userService == null) return;

        List<String> userPIds = userService.getUserLandingPageIds(userId);
        if (userPIds == null || userPIds.isEmpty()) {
            return;
        }

        Set<String> pidSet = userPIds.stream().filter(p -> p != null && !p.trim().isEmpty()).map(String::trim).collect(Collectors.toSet());
        if (pidSet.isEmpty()) return;

        LocalDate cutoffDate = LocalDate.now().minusDays(65);
        List<RawOrder> userOrders = rawOrderRepository.findByLandingPageIdInAndRegisterDateEtGreaterThanEqual(new ArrayList<>(pidSet), cutoffDate);

        if (userOrders.isEmpty()) {
            return;
        }

        Set<String> memberIds = userOrders.stream()
                .map(RawOrder::getMemberId)
                .filter(id -> id != null && !id.trim().isEmpty())
                .collect(Collectors.toSet());

        Map<String, Integer> userPeriodMap = memberIds.isEmpty() ? Collections.emptyMap() :
                userSubscriptionPeriodRepository.findByMemberIdIn(memberIds).stream()
                        .filter(p -> p.getMemberId() != null && p.getSubPeriodDays() != null)
                        .collect(Collectors.toMap(p -> p.getMemberId().trim(),
                                UserSubscriptionPeriod::getSubPeriodDays, (a, b) -> a));

        Map<Integer, List<RawOrder>> ordersByPeriod = userOrders.stream()
                .collect(Collectors.groupingBy(o -> userPeriodMap.getOrDefault(o.getMemberId(), 1)));

        String dimType = "USER";
        String dimValue = String.valueOf(userId);

        Map<String, String> tzMap = null;
        if (userService != null) {
            List<LandingPageConfigItem> userPages = userService.getUserLandingPageConfigs(userId);
            if (userPages != null) {
                tzMap = userPages.stream()
                        .filter(p -> p.getLandingPageId() != null)
                        .collect(Collectors.toMap(p -> p.getLandingPageId().trim(), LandingPageConfigItem::getTimezone, (a, b) -> a));
            }
        }

        for (Map.Entry<Integer, List<RawOrder>> entry : ordersByPeriod.entrySet()) {
            Integer subPeriod = entry.getKey();
            List<RawOrder> periodOrders = entry.getValue();
            calculateBenchmarkForGroup(dimType, dimValue, subPeriod, periodOrders, tzMap);
        }
        clearBenchmarkCache();
    }

    /**
     * 定时任务：每日夜间（或手动触发）更新 LTV 预测基准数据表
     * 仅按近65天成熟期订单及关联用户过滤，避免全量历史订单进内存，各分组独立事务
     */
    @Scheduled(cron = "0 0 3 * * ?")
    public void recalculateAllBenchmarks() {
        log.info("Starting LTV prediction benchmark calculation (recent cohorts)...");
        LocalDate cutoffDate = LocalDate.now().minusDays(65);
        List<RawOrder> recentOrders = rawOrderRepository.findByRegisterDateEtGreaterThanEqual(cutoffDate);
        if (recentOrders.isEmpty()) {
            log.info("No recent raw orders found, populating seed default benchmarks...");
            populateSeedBenchmarks("ALL", "DEFAULT", 1);
            populateSeedBenchmarks("ALL", "DEFAULT", 7);
            return;
        }

        // 仅查询最近活跃用户群体的订阅周期映射 (member_id -> sub_period_days)，避免全量表装载
        Set<String> memberIds = recentOrders.stream()
                .map(RawOrder::getMemberId)
                .filter(id -> id != null && !id.trim().isEmpty())
                .collect(Collectors.toSet());

        Map<String, Integer> userPeriodMap = memberIds.isEmpty() ? Collections.emptyMap() :
                userSubscriptionPeriodRepository.findByMemberIdIn(memberIds).stream()
                        .filter(p -> p.getMemberId() != null && p.getSubPeriodDays() != null)
                        .collect(Collectors.toMap(UserSubscriptionPeriod::getMemberId,
                                UserSubscriptionPeriod::getSubPeriodDays, (a, b) -> a));

        // 按 memberId 对应的 subPeriodDays 分组处理 (1: 日订, 7: 周订 等)
        Map<Integer, List<RawOrder>> ordersByPeriod = recentOrders.stream()
                .collect(Collectors.groupingBy(o -> userPeriodMap.getOrDefault(o.getMemberId(), 1)));

        for (Map.Entry<Integer, List<RawOrder>> entry : ordersByPeriod.entrySet()) {
            Integer subPeriod = entry.getKey();
            List<RawOrder> periodOrders = entry.getValue();
            calculateBenchmarkForGroup("ALL", "DEFAULT", subPeriod, periodOrders, null);
        }

        // 如果没有周订订单，为 weekly (sub_period=7) 生成基于日订衍生的基准
        if (!ordersByPeriod.containsKey(7)) {
            populateSeedBenchmarks("ALL", "DEFAULT", 7);
        }

        // 遍历每个活跃系统用户，生成专属的 USER 维度基准曲线 (已在 recalculateBenchmarksForUser 内部做独立小事务)
        if (userService != null) {
            List<SysUser> users = userService.listAllUsers();
            for (SysUser user : users) {
                recalculateBenchmarksForUser(user.getId());
            }
        }

        log.info("LTV prediction benchmark calculation finished.");
        clearBenchmarkCache();
    }

    @Transactional
    public List<LtvPredictBenchmark> getBenchmarkCurve(String platformCode, String dimensionType, String dimensionValue, Integer subPeriodDays) {
        String pCode = (platformCode != null && !platformCode.trim().isEmpty() && !"ALL".equalsIgnoreCase(platformCode.trim()))
                ? platformCode.trim().toLowerCase() : "all";
        int period = subPeriodDays != null ? subPeriodDays : 1;
        String cacheKey = pCode + ":" + (dimensionType != null ? dimensionType : "") + ":" + (dimensionValue != null ? dimensionValue : "") + ":" + period;

        List<LtvPredictBenchmark> cached = benchmarkCurveCache.get(cacheKey);
        if (cached != null) {
            return cached;
        }

        List<LtvPredictBenchmark> list = Collections.emptyList();

        // Level 1: (PLATFORM_USER, platformCode + ":" + userId) if platformCode is specified and dimType is USER
        if (!"all".equalsIgnoreCase(pCode) && "USER".equalsIgnoreCase(dimensionType)) {
            list = benchmarkRepository.findByDimensionTypeAndDimensionValueAndSubPeriodDaysOrderByDayIndexAsc(
                    "PLATFORM_USER", pCode + ":" + dimensionValue, period);
        }

        // Level 2: (PLATFORM, platformCode) if platformCode is specified
        if (list.isEmpty() && !"all".equalsIgnoreCase(pCode)) {
            list = benchmarkRepository.findByDimensionTypeAndDimensionValueAndSubPeriodDaysOrderByDayIndexAsc(
                    "PLATFORM", pCode, period);
        }

        // Level 3: (USER, userId)
        if (list.isEmpty() && "USER".equalsIgnoreCase(dimensionType)) {
            list = benchmarkRepository.findByDimensionTypeAndDimensionValueAndSubPeriodDaysOrderByDayIndexAsc(
                    "USER", dimensionValue, period);
        }

        // Level 4: (ALL, DEFAULT)
        if (list.isEmpty()) {
            list = benchmarkRepository.findByDimensionTypeAndDimensionValueAndSubPeriodDaysOrderByDayIndexAsc(
                    "ALL", "DEFAULT", period);
        }

        // Fallback: Populate seed benchmarks if still empty
        if (list.isEmpty()) {
            populateSeedBenchmarks("ALL", "DEFAULT", period);
            list = benchmarkRepository.findByDimensionTypeAndDimensionValueAndSubPeriodDaysOrderByDayIndexAsc(
                    "ALL", "DEFAULT", period);
        }

        if (list != null && !list.isEmpty()) {
            benchmarkCurveCache.put(cacheKey, list);
        }
        return list;
    }

    @Transactional
    public List<LtvPredictBenchmark> getBenchmarkCurve(String dimensionType, String dimensionValue, Integer subPeriodDays) {
        return getBenchmarkCurve(null, dimensionType, dimensionValue, subPeriodDays);
    }

    @Transactional
    public void calculateBenchmarkForGroup(String dimensionType, String dimensionValue, Integer subPeriodDays, List<RawOrder> orders) {
        calculateBenchmarkForGroup(dimensionType, dimensionValue, subPeriodDays, orders, null);
    }

    @Transactional
    public void calculateBenchmarkForGroup(String dimensionType, String dimensionValue, Integer subPeriodDays, List<RawOrder> orders, Map<String, String> tzMap) {
        if (orders == null || orders.isEmpty()) {
            populateSeedBenchmarks(dimensionType, dimensionValue, subPeriodDays);
            return;
        }

        // 找出所有注册日期 Cohort
        Map<LocalDate, List<RawOrder>> cohortMap = orders.stream()
                .collect(Collectors.groupingBy(o -> LtvStatService.getEffectiveRegisterDate(o, tzMap)));

        LocalDate minRegisterDate = cohortMap.keySet().stream().filter(Objects::nonNull).min(LocalDate::compareTo).orElse(LocalDate.now());
        LocalDate maxRegisterDate = cohortMap.keySet().stream().filter(Objects::nonNull).max(LocalDate::compareTo).orElse(LocalDate.now());

        long totalDaysAvailable = ChronoUnit.DAYS.between(minRegisterDate, LocalDate.now()) + 1;
        int maxMatureDay = (int) Math.min(90, Math.max(7, totalDaysAvailable - 2));

        // 筛选过去 60 天内注册、且注册天数满足要求的成熟 Cohort
        Map<LocalDate, List<RawOrder>> matureCohorts = cohortMap.entrySet().stream()
                .filter(e -> {
                    if (e.getKey() == null) return false;
                    long age = ChronoUnit.DAYS.between(e.getKey(), LocalDate.now());
                    return age <= 60 && age >= Math.min(30, maxMatureDay);
                })
                .collect(Collectors.toMap(Map.Entry::getKey, Map.Entry::getValue));

        if (matureCohorts.isEmpty()) {
            matureCohorts = cohortMap; // 降级：使用所有可用 Cohort
        }

        int cohortCount = matureCohorts.size();
        int totalInitialSubs = 0;
        Map<Integer, Integer> totalActiveMembersCount = new HashMap<>();
        Map<Integer, BigDecimal> totalRechargeMap = new HashMap<>();

        for (Map.Entry<LocalDate, List<RawOrder>> entry : matureCohorts.entrySet()) {
            LocalDate regDate = entry.getKey();
            List<RawOrder> cohortOrders = entry.getValue();

            // 统计 Cohort 初始订阅人数 N1 (renewType=1 首次订阅)
            Set<String> initialSubMembers = cohortOrders.stream()
                    .filter(o -> o.getIsSubs() != null && o.getIsSubs() == 1 && (o.getRenewType() == null || o.getRenewType() == 1))
                    .map(RawOrder::getMemberId)
                    .collect(Collectors.toSet());
            totalInitialSubs += initialSubMembers.size();

            // 按 Day 统计划扣人数与金额
            Map<Integer, Set<String>> dayActiveMembers = new HashMap<>();
            for (RawOrder order : cohortOrders) {
                LocalDate payDate = LtvStatService.getEffectivePayDate(order, tzMap);
                if (payDate == null || order.getPayState() == null || order.getPayState() != 1) continue;
                int dayIndex = (int) ChronoUnit.DAYS.between(regDate, payDate) + 1;
                if (dayIndex >= 1 && dayIndex <= maxMatureDay) {
                    dayActiveMembers.computeIfAbsent(dayIndex, k -> new HashSet<>()).add(order.getMemberId());
                    totalRechargeMap.merge(dayIndex, order.getOrderAmountUsd(), BigDecimal::add);
                }
            }

            for (Map.Entry<Integer, Set<String>> dayEntry : dayActiveMembers.entrySet()) {
                totalActiveMembersCount.merge(dayEntry.getKey(), dayEntry.getValue().size(), Integer::sum);
            }
        }

        int poolInitialSubs = Math.max(1, totalInitialSubs);

        List<LtvPredictBenchmark> benchmarksToSave = new ArrayList<>();

        // 构造 Day 1 ~ maxMatureDay 的加权池平均基准
        double[] baseRet = new double[91];
        double[] baseArpu = new double[91];

        for (int d = 1; d <= maxMatureDay; d++) {
            int activeCount = totalActiveMembersCount.getOrDefault(d, 0);
            double avgRet = (double) activeCount / poolInitialSubs;
            BigDecimal recharge = totalRechargeMap.getOrDefault(d, BigDecimal.ZERO);
            double defaultPrice = (subPeriodDays == 1) ? PredictAlgorithmConstants.DEFAULT_DAILY_SUB_PRICE : PredictAlgorithmConstants.DEFAULT_WEEKLY_SUB_PRICE;
            double avgArpu = activeCount > 0 ? recharge.doubleValue() / activeCount : (d == 1 ? defaultPrice : 0.0);

            baseRet[d] = avgRet;
            baseArpu[d] = avgArpu;

            LtvPredictBenchmark bench = new LtvPredictBenchmark();
            bench.setDimensionType(dimensionType);
            bench.setDimensionValue(dimensionValue);
            bench.setSubPeriodDays(subPeriodDays);
            bench.setDayIndex(d);
            bench.setBaseRetentionRate(BigDecimal.valueOf(avgRet).setScale(6, RoundingMode.HALF_UP));
            bench.setBaseArpu(BigDecimal.valueOf(avgArpu).setScale(2, RoundingMode.HALF_UP));
            bench.setSampleCohortCount(cohortCount);
            bench.setIsExtrapolated(0);
            benchmarksToSave.add(bench);
        }

        // 如果 maxMatureDay < 90，使用移位幂律进行尾部外推延伸 (Power Law Extension)
        if (maxMatureDay < 90) {
            extrapolatePowerLawTail(dimensionType, dimensionValue, subPeriodDays, maxMatureDay, 90, baseRet, baseArpu, cohortCount, benchmarksToSave);
        }

        if (transactionTemplate != null) {
            transactionTemplate.execute(status -> {
                benchmarkRepository.deleteByDimensionTypeAndDimensionValueAndSubPeriodDays(dimensionType, dimensionValue, subPeriodDays);
                benchmarkRepository.flush();
                benchmarkRepository.saveAll(benchmarksToSave);
                benchmarkRepository.flush();
                return null;
            });
        } else {
            benchmarkRepository.deleteByDimensionTypeAndDimensionValueAndSubPeriodDays(dimensionType, dimensionValue, subPeriodDays);
            benchmarkRepository.flush();
            benchmarkRepository.saveAll(benchmarksToSave);
            benchmarkRepository.flush();
        }
        clearBenchmarkCache();
    }

    private void extrapolatePowerLawTail(String dimensionType, String dimensionValue, Integer subPeriodDays,
                                         int matureDays, int targetDays,
                                         double[] baseRet, double[] baseArpu,
                                         int cohortCount, List<LtvPredictBenchmark> outList) {
        int period = subPeriodDays != null ? subPeriodDays : 1;

        // 寻找 matureDays 以内最近的一个划扣日 (Renewal Day)
        int lastRenewalDay = matureDays;
        if (period > 1) {
            while (lastRenewalDay >= 1 && (lastRenewalDay - 1) % period != 0) {
                lastRenewalDay--;
            }
        }
        if (lastRenewalDay < 1) lastRenewalDay = 1;

        int firstRenewalDay = Math.max(1, lastRenewalDay / 2);
        if (period > 1) {
            while (firstRenewalDay >= 1 && (firstRenewalDay - 1) % period != 0) {
                firstRenewalDay--;
            }
            if (firstRenewalDay < 1) firstRenewalDay = 1;
        }

        // 使用近两期的划扣日数据估计衰减指数 gamma
        double retStart = baseRet[firstRenewalDay];
        double retEnd = baseRet[lastRenewalDay];
        double gamma = 1.2;
        if (retStart > 0 && retEnd > 0 && retStart > retEnd && lastRenewalDay > firstRenewalDay) {
            double ratio = retEnd / retStart;
            double tRatio = (double) lastRenewalDay / firstRenewalDay;
            gamma = Math.max(0.5, Math.min(2.5, -Math.log(ratio) / Math.log(tRatio)));
        }

        double lastArpu = baseArpu[lastRenewalDay];
        double anchorRet = baseRet[lastRenewalDay] > 0 ? baseRet[lastRenewalDay] : (period > 1 ? 0.5 : 1.0 / Math.pow(lastRenewalDay, 0.75));

        for (int d = matureDays + 1; d <= targetDays; d++) {
            double extrapolatedRet = 0.0;
            if (period == 1 || (d - 1) % period == 0) {
                extrapolatedRet = anchorRet * Math.pow((double) lastRenewalDay / d, gamma);
            }

            LtvPredictBenchmark bench = new LtvPredictBenchmark();
            bench.setDimensionType(dimensionType);
            bench.setDimensionValue(dimensionValue);
            bench.setSubPeriodDays(period);
            bench.setDayIndex(d);
            bench.setBaseRetentionRate(BigDecimal.valueOf(extrapolatedRet).setScale(6, RoundingMode.HALF_UP));
            bench.setBaseArpu(BigDecimal.valueOf(lastArpu).setScale(2, RoundingMode.HALF_UP));
            bench.setSampleCohortCount(cohortCount);
            bench.setIsExtrapolated(1);
            outList.add(bench);
        }
    }

    @Transactional
    public void populateSeedBenchmarks(String dimensionType, String dimensionValue, Integer subPeriodDays) {
        List<LtvPredictBenchmark> list = new ArrayList<>();

        int period = subPeriodDays != null ? subPeriodDays : 1;
        for (int d = 1; d <= 90; d++) {
            double ret;
            if (period > 1) {
                // 周期订：只在划扣日计算衰减
                if ((d - 1) % period == 0) {
                    int cycleIndex = (d - 1) / period + 1;
                    ret = Math.pow(0.55, cycleIndex - 1);
                } else {
                    ret = 0.0;
                }
            } else {
                // 日订
                ret = 1.0 / Math.pow(d, 0.75);
            }

            LtvPredictBenchmark bench = new LtvPredictBenchmark();
            bench.setDimensionType(dimensionType);
            bench.setDimensionValue(dimensionValue);
            bench.setSubPeriodDays(period);
            bench.setDayIndex(d);
            bench.setBaseRetentionRate(BigDecimal.valueOf(ret).setScale(6, RoundingMode.HALF_UP));
            bench.setBaseArpu(BigDecimal.ZERO);
            bench.setSampleCohortCount(10);
            list.add(bench);
        }

        if (transactionTemplate != null) {
            transactionTemplate.execute(status -> {
                benchmarkRepository.deleteByDimensionTypeAndDimensionValueAndSubPeriodDays(dimensionType, dimensionValue, subPeriodDays);
                benchmarkRepository.flush();
                benchmarkRepository.saveAll(list);
                benchmarkRepository.flush();
                return null;
            });
        } else {
            benchmarkRepository.deleteByDimensionTypeAndDimensionValueAndSubPeriodDays(dimensionType, dimensionValue, subPeriodDays);
            benchmarkRepository.flush();
            benchmarkRepository.saveAll(list);
            benchmarkRepository.flush();
        }
        clearBenchmarkCache();
    }
}
