package com.ltv.stat.service;

import com.ltv.stat.dto.PredictionResult;
import com.ltv.stat.entity.LtvDailyStat;
import com.ltv.stat.entity.LtvPredictBenchmark;
import com.ltv.stat.entity.SubscriptionConfigVersion;
import com.ltv.stat.repository.SubscriptionConfigVersionRepository;
import com.ltv.stat.service.engine.*;
import com.ltv.stat.util.CohortStatHelper;
import org.slf4j.Logger;
import org.slf4j.LoggerFactory;
import org.springframework.beans.factory.annotation.Autowired;
import org.springframework.stereotype.Service;

import java.math.BigDecimal;
import java.math.RoundingMode;
import java.time.LocalDate;
import java.time.LocalDateTime;
import java.time.ZoneId;
import java.time.temporal.ChronoUnit;
import java.util.*;
import java.util.concurrent.ConcurrentHashMap;
import java.util.stream.Collectors;

@Service
public class LtvPredictService {

    private static final Logger log = LoggerFactory.getLogger(LtvPredictService.class);

    private final LtvBenchmarkService benchmarkService;
    private final SubscriptionConfigVersionRepository versionRepository;
    private final LtvPredictFacade ltvPredictFacade;
    private final CohortCurveExtrapolator extrapolator;
    private static final int MAX_VERSION_CACHE_SIZE = 500;
    private final Map<String, List<SubscriptionConfigVersion>> matchingVersionCache = Collections.synchronizedMap(
            new LinkedHashMap<String, List<SubscriptionConfigVersion>>(128, 0.75f, true) {
                @Override
                protected boolean removeEldestEntry(Map.Entry<String, List<SubscriptionConfigVersion>> eldest) {
                    return size() > MAX_VERSION_CACHE_SIZE;
                }
            }
    );

    @Autowired
    public LtvPredictService(LtvBenchmarkService benchmarkService,
                             @Autowired(required = false) SubscriptionConfigVersionRepository versionRepository,
                             LtvPredictFacade ltvPredictFacade,
                             CohortCurveExtrapolator extrapolator) {
        this.benchmarkService = benchmarkService;
        this.versionRepository = versionRepository;
        this.ltvPredictFacade = ltvPredictFacade != null ? ltvPredictFacade : new LtvPredictFacade(new PaybackPredictEngine(), new RoiPredictEngine());
        this.extrapolator = extrapolator != null ? extrapolator : new CohortCurveExtrapolator();
    }

    public void clearVersionCache() {
        matchingVersionCache.clear();
    }

    public LtvPredictService(LtvBenchmarkService benchmarkService) {
        this(benchmarkService, null, new LtvPredictFacade(new PaybackPredictEngine(), new RoiPredictEngine()), new CohortCurveExtrapolator());
    }

    /**
     * 单个 Cohort 回本与 ROI 预测入口 (委托给 LtvPredictFacade 引擎组装)
     */
    public PredictionResult predictCohort(LtvDailyStat stat, int daysElapsed) {
        if (stat == null || stat.getSpend() == null || stat.getSpend().compareTo(BigDecimal.ZERO) <= 0) {
            return new PredictionResult(null);
        }

        double[] cumRecharge = predictCohortDailyRechargeCurve(stat, daysElapsed);
        return ltvPredictFacade.assembleCohortPrediction(stat, daysElapsed, cumRecharge);
    }

    /**
     * 自下而上（Bottom-Up）独立 Cohort 预测叠加方案 (委托给 LtvPredictFacade 引擎组装)
     */
    public PredictionResult predictOverallCohort(List<LtvDailyStat> statList) {
        if (statList == null || statList.isEmpty()) {
            return new PredictionResult(null);
        }

        LocalDate today = LocalDate.now(ZoneId.of("Asia/Shanghai"));

        List<LtvDailyStat> validStats = statList.stream()
                .filter(s -> s != null && s.getSpend() != null && s.getSpend().compareTo(BigDecimal.ZERO) > 0)
                .collect(Collectors.toList());

        if (validStats.isEmpty()) {
            return new PredictionResult(null);
        }

        BigDecimal totalSpendAll = validStats.stream().map(LtvDailyStat::getSpend).reduce(BigDecimal.ZERO, BigDecimal::add);
        BigDecimal totalRechargeAll = validStats.stream().map(LtvDailyStat::getTotalRecharge).filter(Objects::nonNull).reduce(BigDecimal.ZERO, BigDecimal::add);
        if (totalSpendAll.compareTo(BigDecimal.ZERO) <= 0) {
            return new PredictionResult(null);
        }

        LocalDate minLaunchDate = validStats.stream()
                .map(LtvDailyStat::getLaunchDate)
                .filter(Objects::nonNull)
                .min(LocalDate::compareTo)
                .orElse(today);

        Map<LtvDailyStat, double[]> cohortCurves = new HashMap<>();
        for (LtvDailyStat s : validStats) {
            LocalDate lDate = s.getLaunchDate() != null ? s.getLaunchDate() : minLaunchDate;
            int age = (int) ChronoUnit.DAYS.between(lDate, today) + 1;
            age = Math.max(1, Math.min(age, 60));

            double[] curve = predictCohortDailyRechargeCurve(s, age);
            cohortCurves.put(s, curve);
        }

        return ltvPredictFacade.assembleOverallPrediction(totalSpendAll, totalRechargeAll, validStats, cohortCurves, minLaunchDate, today);
    }

    /**
     * 生成单个 Cohort 从第 1 天到第 365 天的累计预测充值曲线 double[367]
     */
    public double[] predictCohortDailyRechargeCurve(LtvDailyStat stat, int daysElapsed) {
        double[] cumRecharge = new double[367];
        if (stat == null || stat.getSpend() == null || stat.getSpend().compareTo(BigDecimal.ZERO) <= 0) {
            return cumRecharge;
        }
        int maxDays = Math.min(daysElapsed, 60);
        BigDecimal spend = stat.getSpend();

        // 填充 1 到 maxDays 的真实历史充值
        for (int d = 1; d <= maxDays; d++) {
            BigDecimal r = getRechargeForDay(stat, d);
            if (r != null) {
                cumRecharge[d] = r.doubleValue();
            } else {
                cumRecharge[d] = cumRecharge[d - 1];
            }
        }

        BigDecimal actualRecharge = BigDecimal.valueOf(cumRecharge[maxDays]);
        Integer subPeriodDays = stat.getSubPeriodDays() != null ? stat.getSubPeriodDays() : 1;
        int subUserCount = stat.getSubUserCount() != null ? stat.getSubUserCount() : 0;

        if (subUserCount <= 0 && actualRecharge.compareTo(BigDecimal.ZERO) <= 0) {
            for (int d = maxDays + 1; d <= 365; d++) cumRecharge[d] = cumRecharge[maxDays];
            return cumRecharge;
        }

        int effectiveSubUserCount = Math.max(1, subUserCount);
        if (isSubscriptionStagnant(stat, maxDays)) {
            for (int d = maxDays + 1; d <= 365; d++) cumRecharge[d] = cumRecharge[maxDays];
            return cumRecharge;
        }
        LocalDateTime launchTime = stat.getLaunchDate() != null ? stat.getLaunchDate().atStartOfDay() : LocalDateTime.now(ZoneId.of("Asia/Shanghai"));
        String dimType = "ALL";
        String dimValue = "DEFAULT";
        if (stat.getUserId() != null) {
            dimType = "USER";
            dimValue = String.valueOf(stat.getUserId());
        }

        Map<Integer, Integer> periodDistMap = parsePeriodDistribution(stat.getSubPeriodDistribution(), subPeriodDays, effectiveSubUserCount);
        List<PeriodContext> contexts = new ArrayList<>();
        for (Map.Entry<Integer, Integer> entry : periodDistMap.entrySet()) {
            int period = entry.getKey();
            int count = entry.getValue();

            PeriodContext ctx = new PeriodContext();
            ctx.periodDays = period;
            ctx.userCount = count;

            if (versionRepository != null) {
                String vKey = period + ":" + (launchTime != null ? launchTime.toLocalDate().toString() : "now");
                List<SubscriptionConfigVersion> matchedVersions = matchingVersionCache.computeIfAbsent(vKey,
                        k -> versionRepository.findMatchingPeriodVersions(period, launchTime));
                if (!matchedVersions.isEmpty()) {
                    SubscriptionConfigVersion ver = matchedVersions.get(0);
                    if (ver.getRenewPriceCent() != null && ver.getRenewPriceCent() > 0) {
                        ctx.configRenewUsd = ver.getRenewPriceCent() / 100.0;
                    }
                    if (ver.getFirstPriceCent() != null && ver.getFirstPriceCent() > 0) {
                        ctx.configFirstUsd = ver.getFirstPriceCent() / 100.0;
                    }
                }
            }

            List<LtvPredictBenchmark> benchmarkList = benchmarkService.getBenchmarkCurve(stat.getPlatformCode(), dimType, dimValue, period);

            if (benchmarkList != null && !benchmarkList.isEmpty()) {
                for (LtvPredictBenchmark b : benchmarkList) {
                    int idx = b.getDayIndex();
                    if (idx >= 1 && idx <= 365) {
                        ctx.baseRet[idx] = b.getBaseRetentionRate() != null ? b.getBaseRetentionRate().doubleValue() : 0.0;
                        ctx.baseArpu[idx] = b.getBaseArpu() != null ? b.getBaseArpu().doubleValue() : 0.0;
                    }
                }
            } else {
                // 标准合成基准线兜底 (Synthetic Standard Benchmark Fallback)
                double defaultUnitPrice = (period == 1) ? PredictAlgorithmConstants.DEFAULT_DAILY_SUB_PRICE : PredictAlgorithmConstants.DEFAULT_WEEKLY_SUB_PRICE;
                double unitPrice = ctx.configRenewUsd != null ? ctx.configRenewUsd : (ctx.configFirstUsd != null ? ctx.configFirstUsd : defaultUnitPrice);
                for (int d = 1; d <= 90; d++) {
                    if (period > 1) {
                        if ((d - 1) % period == 0) {
                            int cycleIndex = (d - 1) / period + 1;
                            ctx.baseRet[d] = Math.pow(0.55, cycleIndex - 1);
                        } else {
                            ctx.baseRet[d] = 0.0;
                        }
                    } else {
                        ctx.baseRet[d] = 1.0 / Math.pow(d, 0.75);
                    }
                    ctx.baseArpu[d] = unitPrice;
                }
            }

            int anchorDay = 90;
            if (period > 1) {
                while (anchorDay >= 1 && (anchorDay - 1) % period != 0) {
                    anchorDay--;
                }
            }
            if (anchorDay < 1) anchorDay = 1;

            double lastRet = ctx.baseRet[anchorDay];
            double lastArpu = ctx.baseArpu[anchorDay];

            for (int d = 91; d <= 365; d++) {
                if (ctx.baseRet[d] == 0.0) {
                    if (period > 1 && (d - 1) % period != 0) {
                        ctx.baseRet[d] = 0.0;
                    } else {
                        ctx.baseRet[d] = lastRet * Math.pow((double) anchorDay / d, 1.2);
                    }
                    ctx.baseArpu[d] = lastArpu;
                }
            }
            contexts.add(ctx);
        }

        boolean hasValidBenchmark = false;
        for (PeriodContext ctx : contexts) {
            for (int d = 1; d <= 365; d++) {
                if (ctx.baseRet[d] > 0) {
                    hasValidBenchmark = true;
                    break;
                }
            }
            if (hasValidBenchmark) break;
        }

        if (!hasValidBenchmark) {
            for (int d = maxDays + 1; d <= 365; d++) {
                cumRecharge[d] = actualRecharge.doubleValue();
            }
            return cumRecharge;
        }

        double actualRoi = actualRecharge.divide(spend, 4, RoundingMode.HALF_UP).doubleValue();

        // P2 模块 3：小样本强活跃大户特征识别与充值动量下界
        double continuityRatio = CohortStatHelper.getRechargeContinuityRatio(stat, maxDays, 7);
        boolean isSmallCohortActive = (effectiveSubUserCount <= PredictAlgorithmConstants.SMALL_COHORT_MAX_USERS
                && continuityRatio >= PredictAlgorithmConstants.SMALL_COHORT_CONTINUITY_THRESHOLD
                && actualRoi >= PredictAlgorithmConstants.SMALL_COHORT_MIN_ROI && maxDays >= 10);

        double recentDailyVelocity = 0.0;
        if (isSmallCohortActive && maxDays >= 7) {
            BigDecimal rNow = getRechargeForDay(stat, maxDays);
            BigDecimal r7Ago = getRechargeForDay(stat, maxDays - 7);
            if (rNow != null && r7Ago != null) {
                recentDailyVelocity = Math.max(0.0, rNow.subtract(r7Ago).doubleValue() / 7.0);
            }
        }

        // P2 模块 1：Cohort 自身单客充值力 (Realized ARPU) 贝叶斯动态萃取
        double timeWeight = 0.0;
        if (maxDays > 7 && maxDays <= 14) {
            timeWeight = 0.10 + 0.40 * ((double) (maxDays - 7) / 7.0);
        } else if (maxDays > 14) {
            timeWeight = 0.50 + 0.35 * ((double) Math.min(46, maxDays - 14) / 46.0);
        }
        double userWeight = (double) effectiveSubUserCount / (effectiveSubUserCount + PredictAlgorithmConstants.ARPU_SHRINKAGE_K_USER);
        double arpuWeight = timeWeight * userWeight;

        for (PeriodContext ctx : contexts) {
            double retSum = 0.0;
            for (int d = 1; d <= maxDays; d++) {
                retSum += ctx.baseRet[d];
            }
            double defaultFallback = (ctx.periodDays == 1) ? PredictAlgorithmConstants.DEFAULT_DAILY_SUB_PRICE : PredictAlgorithmConstants.DEFAULT_WEEKLY_SUB_PRICE;
            double baseArpuAnchor = (ctx.configRenewUsd != null) ? ctx.configRenewUsd
                    : ((ctx.baseArpu != null && ctx.baseArpu[maxDays] > 0) ? ctx.baseArpu[maxDays] : defaultFallback);
            if (retSum > 0.01 && effectiveSubUserCount > 0 && arpuWeight > 0.001) {
                double realizedArpu = actualRecharge.doubleValue() / (effectiveSubUserCount * retSum);
                double boundedRealizedArpu = Math.max(0.5 * baseArpuAnchor, Math.min(8.0 * baseArpuAnchor, realizedArpu));
                ctx.effectiveArpu = arpuWeight * boundedRealizedArpu + (1.0 - arpuWeight) * baseArpuAnchor;
            } else {
                ctx.effectiveArpu = baseArpuAnchor;
            }
        }

        double baseRoiSum = 0;
        for (int d = 1; d <= maxDays; d++) {
            for (PeriodContext ctx : contexts) {
                if (ctx.periodDays == 1 || (d - 1) % ctx.periodDays == 0) {
                    double unitPrice = ctx.baseArpu[d];
                    if (d == 1 && ctx.configFirstUsd != null) {
                        unitPrice = ctx.configFirstUsd;
                    } else if (d > 1 && ctx.configRenewUsd != null) {
                        unitPrice = ctx.configRenewUsd;
                    }
                    baseRoiSum += (ctx.baseRet[d] * unitPrice * ctx.userCount) / spend.doubleValue();
                }
            }
        }
        baseRoiSum = Math.max(0.0001, baseRoiSum);
        double scaleFactor = computeOptimalScaleFactor(actualRoi, baseRoiSum, maxDays, spend, isSmallCohortActive);

        // 轨道 A：留存衰减基准外推曲线推导
        double scaleDecayExp = isSmallCohortActive ? PredictAlgorithmConstants.SMALL_COHORT_SCALE_DECAY_EXPONENT : PredictAlgorithmConstants.SCALE_DECAY_EXPONENT;
        double currentCumRecharge = actualRecharge.doubleValue();
        for (int t = maxDays + 1; t <= 365; t++) {
            // 1. 均值回归机制：放缩因子向 1.0 平滑回归
            double scaleDecay = Math.pow((double) maxDays / t, scaleDecayExp);
            double effectiveScaleFactor = 1.0 + (scaleFactor - 1.0) * scaleDecay;

            // 2. 周期续订自然衰减校准 (使用常量 CYCLE_DECAY_EXPONENT = 0.06)
            double cycleDecay = (t > 7) ? Math.pow(7.0 / t, PredictAlgorithmConstants.CYCLE_DECAY_EXPONENT) : 1.0;

            double dailyDeltaIncome = 0;
            for (PeriodContext ctx : contexts) {
                if (ctx.periodDays == 1 || (t - 1) % ctx.periodDays == 0) {
                    double predRet = ctx.baseRet[t] * effectiveScaleFactor * cycleDecay;
                    double defaultArpu = (ctx.configRenewUsd != null) ? ctx.configRenewUsd : ctx.baseArpu[t];
                    double predArpu = (arpuWeight > 0.001 && ctx.effectiveArpu > 0) ? ctx.effectiveArpu : defaultArpu;
                    dailyDeltaIncome += predRet * predArpu * ctx.userCount;
                }
            }
            if (isSmallCohortActive && recentDailyVelocity > 0.01) {
                double velDecay = Math.pow((double) maxDays / t, PredictAlgorithmConstants.SMALL_COHORT_SCALE_DECAY_EXPONENT);
                dailyDeltaIncome = Math.max(dailyDeltaIncome, recentDailyVelocity * velDecay);
            }
            currentCumRecharge += dailyDeltaIncome;
            cumRecharge[t] = currentCumRecharge;
        }

        // P2 模块 2：成熟期 (D14+) 双轨 OLS 动量动态系综融合 (Ensemble)
        if (maxDays >= PredictAlgorithmConstants.OLS_ENSEMBLE_MIN_DAYS) {
            CohortCurveExtrapolator.OlsFitResult olsFit = CohortCurveExtrapolator.computeOlsFit(stat, maxDays);
            if (olsFit.valid) {
                double lambda = CohortCurveExtrapolator.computeOlsEnsembleWeight(olsFit.r2, maxDays);
                if (lambda > 0.001) {
                    double spendVal = spend.doubleValue();
                    double baseRechargeAnchor = actualRecharge.doubleValue();
                    for (int t = maxDays + 1; t <= 365; t++) {
                        double olsRoi = olsFit.a * Math.log(t) + olsFit.b;
                        double olsRecharge = spendVal * olsRoi;
                        double effectiveOlsRecharge = Math.max(baseRechargeAnchor, olsRecharge);
                        cumRecharge[t] = (1.0 - lambda) * cumRecharge[t] + lambda * effectiveOlsRecharge;
                    }
                }
            }
        }

        return cumRecharge;
    }

    /**
     * 委托给 CohortCurveExtrapolator 算法算子
     */
    public PredictionResult predictCohortOlsFallback(LtvDailyStat stat, int daysElapsed) {
        return extrapolator.predictCohortOlsFallback(stat, daysElapsed);
    }

    public static BigDecimal getRechargeForDay(LtvDailyStat stat, int day) {
        return CohortStatHelper.getRechargeForDay(stat, day);
    }

    public static BigDecimal getRoiForDay(LtvDailyStat stat, int day) {
        return CohortStatHelper.getRoiForDay(stat, day);
    }

    public static double computeOptimalScaleFactor(double actualRoi, double baseRoiSum, int maxDays) {
        return CohortCurveExtrapolator.computeOptimalScaleFactor(actualRoi, baseRoiSum, maxDays, null);
    }

    public static double computeOptimalScaleFactor(double actualRoi, double baseRoiSum, int maxDays, BigDecimal spend) {
        return CohortCurveExtrapolator.computeOptimalScaleFactor(actualRoi, baseRoiSum, maxDays, spend);
    }

    public static double computeOptimalScaleFactor(double actualRoi, double baseRoiSum, int maxDays, BigDecimal spend, boolean isSmallCohortActive) {
        return CohortCurveExtrapolator.computeOptimalScaleFactor(actualRoi, baseRoiSum, maxDays, spend, isSmallCohortActive);
    }

    public static int getMaxPeriodInDistribution(LtvDailyStat stat) {
        return CohortStatHelper.getMaxPeriodInDistribution(stat);
    }

    public static boolean isSubscriptionStagnant(LtvDailyStat stat, int maxDays) {
        return CohortStatHelper.isSubscriptionStagnant(stat, maxDays);
    }

    /**
     * 解析 Cohort 订阅分布 JSON 字符串，例如 "{"1":3,"7":12}" -> Map<Period, Count>
     */
    private Map<Integer, Integer> parsePeriodDistribution(String json, int fallbackPeriod, int fallbackCount) {
        Map<Integer, Integer> map = new LinkedHashMap<>();
        if (json != null && !json.trim().isEmpty()) {
            try {
                String clean = json.trim().replaceAll("[{}\"]", "");
                if (!clean.isEmpty()) {
                    String[] pairs = clean.split(",");
                    for (String p : pairs) {
                        String[] kv = p.split(":");
                        if (kv.length == 2) {
                            int period = Integer.parseInt(kv[0].trim());
                            int count = Integer.parseInt(kv[1].trim());
                            if (count > 0) {
                                map.put(Math.max(1, period), count);
                            }
                        }
                    }
                }
            } catch (Exception ignored) {}
        }
        if (map.isEmpty()) {
            map.put(Math.max(1, fallbackPeriod), Math.max(1, fallbackCount));
        } else {
            int sumCount = map.values().stream().mapToInt(Integer::intValue).sum();
            if (sumCount < fallbackCount) {
                int diff = fallbackCount - sumCount;
                map.merge(Math.max(1, fallbackPeriod), diff, Integer::sum);
            }
        }
        return map;
    }
}
