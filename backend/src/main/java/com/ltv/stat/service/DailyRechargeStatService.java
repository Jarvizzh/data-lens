package com.ltv.stat.service;

import com.ltv.stat.dto.DailyDistributionSummaryDto;
import com.ltv.stat.entity.DailyRechargeDistribution;
import com.ltv.stat.entity.RawOrder;
import com.ltv.stat.entity.SysUser;
import com.ltv.stat.repository.DailyRechargeDistributionRepository;
import com.ltv.stat.repository.RawOrderRepository;
import org.slf4j.Logger;
import org.slf4j.LoggerFactory;
import org.springframework.beans.factory.annotation.Autowired;
import org.springframework.context.annotation.Lazy;
import org.springframework.stereotype.Service;
import org.springframework.transaction.PlatformTransactionManager;
import org.springframework.transaction.annotation.Transactional;
import org.springframework.transaction.support.TransactionTemplate;

import java.math.BigDecimal;
import java.math.RoundingMode;
import java.time.LocalDate;
import java.time.YearMonth;
import java.time.ZoneId;
import java.time.format.DateTimeFormatter;
import java.util.*;
import java.util.stream.Collectors;

import com.ltv.stat.util.TimeUtils;

import static com.ltv.stat.service.LtvStatService.START_DATE;
import static com.ltv.stat.service.LtvStatService.getLaunchStartDateForPlatform;

/**
 * 每日充值统计与平台汇总服务类 (遵循单一职责原则 SRP)
 * 专职负责按“实际支付日期 (Pay Date)”维度的每日充值分布、新老客充值分布及全盘平台汇总计算。
 */
@Service
public class DailyRechargeStatService {

    private static final Logger log = LoggerFactory.getLogger(DailyRechargeStatService.class);

    private final RawOrderRepository rawOrderRepository;
    private final DailyRechargeDistributionRepository dailyRechargeDistributionRepository;
    private final UserService userService;
    private final LtvStatService ltvStatService;
    private final AsyncRecalculateService asyncRecalculateService;
    private final TransactionTemplate transactionTemplate;

    public DailyRechargeStatService(RawOrderRepository rawOrderRepository,
                                    DailyRechargeDistributionRepository dailyRechargeDistributionRepository,
                                    UserService userService,
                                    LtvStatService ltvStatService,
                                    @Lazy AsyncRecalculateService asyncRecalculateService,
                                    @Autowired(required = false) PlatformTransactionManager transactionManager) {
        this.rawOrderRepository = rawOrderRepository;
        this.dailyRechargeDistributionRepository = dailyRechargeDistributionRepository;
        this.userService = userService;
        this.ltvStatService = ltvStatService;
        this.asyncRecalculateService = asyncRecalculateService;
        this.transactionTemplate = transactionManager != null ? new TransactionTemplate(transactionManager) : null;
    }

    private LocalDate getOrderPayDateForPlatform(String platformCode, RawOrder o) {
        if (o == null) return null;
        if ("flicknovel".equalsIgnoreCase(platformCode) || "flicknovel".equalsIgnoreCase(o.getPlatformCode())) {
            return LtvStatService.getUtcPayDate(o);
        }
        return LtvStatService.getBjPayDate(o);
    }

    private LocalDate getOrderRegisterDateForPlatform(String platformCode, RawOrder o) {
        if (o == null) return null;
        if ("flicknovel".equalsIgnoreCase(platformCode) || "flicknovel".equalsIgnoreCase(o.getPlatformCode())) {
            return LtvStatService.getUtcRegisterDate(o);
        }
        return LtvStatService.getBjRegisterDate(o);
    }

    private LocalDate getTodayForPlatform(String platformCode) {
        if ("flicknovel".equalsIgnoreCase(platformCode)) {
            return TimeUtils.getTodayUtc();
        }
        return TimeUtils.getTodayCst();
    }

    @Transactional
    public List<DailyRechargeDistribution> getDailyDistributionStats(String platformCode, Long userId) {
        if (userId == null) userId = 1L;
        String pCode = (platformCode != null && !platformCode.trim().isEmpty() && !"ALL".equalsIgnoreCase(platformCode.trim()))
                ? platformCode.trim().toLowerCase() : "ALL";
        LocalDate platformStartDate = getLaunchStartDateForPlatform(pCode);
        List<DailyRechargeDistribution> list = dailyRechargeDistributionRepository.findByPlatformCodeAndUserIdAndDateGreaterThanEqualOrderByDateDesc(pCode, userId, platformStartDate);
        boolean allZeros = !list.isEmpty() && list.stream().allMatch(s ->
                (s.getTotalRecharge() == null || s.getTotalRecharge().compareTo(BigDecimal.ZERO) == 0)
        );
        LocalDate today = getTodayForPlatform(pCode);
        boolean isMissingToday = !list.isEmpty() && list.get(0).getDate().isBefore(today);
        boolean isMissingStartDate = !list.isEmpty() && list.get(list.size() - 1).getDate().isAfter(platformStartDate);

        if (list.isEmpty() || allZeros || isMissingToday || isMissingStartDate) {
            calculateDailyDistributionStatsForUser(pCode, userId);
            list = dailyRechargeDistributionRepository.findByPlatformCodeAndUserIdAndDateGreaterThanEqualOrderByDateDesc(pCode, userId, platformStartDate);
        }
        return list;
    }

    @Transactional
    public List<DailyRechargeDistribution> getDailyDistributionStats(Long userId) {
        return getDailyDistributionStats("ALL", userId);
    }

    @Transactional
    public List<DailyRechargeDistribution> getDailyDistributionStats() {
        return getDailyDistributionStats("ALL", 1L);
    }

    public List<DailyRechargeDistribution> getGlobalDailyDistributionStats(String platformCode) {
        String pCode = (platformCode != null && !platformCode.trim().isEmpty() && !"ALL".equalsIgnoreCase(platformCode.trim()))
                ? platformCode.trim().toLowerCase() : "ALL";
        LocalDate today = getTodayForPlatform(pCode);
        LocalDate platformStartDate = getLaunchStartDateForPlatform(pCode);
        List<RawOrder> allOrders = "ALL".equals(pCode)
                ? rawOrderRepository.findAll()
                : rawOrderRepository.findByPlatformCode(pCode);

        Map<LocalDate, List<RawOrder>> ordersByPayDate = allOrders.stream()
                .filter(o -> {
                    LocalDate payDate = getOrderPayDateForPlatform(pCode, o);
                    return payDate != null && !payDate.isBefore(platformStartDate);
                })
                .collect(Collectors.groupingBy(o -> getOrderPayDateForPlatform(pCode, o)));

        List<DailyRechargeDistribution> statList = new ArrayList<>();
        LocalDate currDate = platformStartDate;

        while (!currDate.isAfter(today)) {
            List<RawOrder> dayOrders = ordersByPayDate.getOrDefault(currDate, Collections.emptyList());
            DailyRechargeDistribution stat = calculateSingleDayDistribution(pCode, 0L, currDate, dayOrders);
            statList.add(stat);
            currDate = currDate.plusDays(1);
        }

        statList.sort(Comparator.comparing(DailyRechargeDistribution::getDate).reversed());
        return statList;
    }

    public List<DailyRechargeDistribution> getGlobalDailyDistributionStats() {
        return getGlobalDailyDistributionStats("ALL");
    }

    public DailyDistributionSummaryDto getGlobalDailyDistributionSummary(String platformCode) {
        String pCode = (platformCode != null && !platformCode.trim().isEmpty() && !"ALL".equalsIgnoreCase(platformCode.trim()))
                ? platformCode.trim().toLowerCase() : "ALL";
        LocalDate platformStartDate = getLaunchStartDateForPlatform(pCode);
        List<RawOrder> orders = ("ALL".equals(pCode) ? rawOrderRepository.findAll() : rawOrderRepository.findByPlatformCode(pCode)).stream()
                .filter(o -> {
                    LocalDate payDate = getOrderPayDateForPlatform(pCode, o);
                    return payDate != null && !payDate.isBefore(platformStartDate);
                })
                .collect(Collectors.toList());
        return calculateDistributionSummaryFromOrders(pCode, orders);
    }

    public DailyDistributionSummaryDto getGlobalDailyDistributionSummary() {
        return getGlobalDailyDistributionSummary("ALL");
    }

    public DailyDistributionSummaryDto getDailyDistributionSummary(String platformCode, Long userId) {
        if (userId == null) userId = 1L;
        String pCode = (platformCode != null && !platformCode.trim().isEmpty() && !"ALL".equalsIgnoreCase(platformCode.trim()))
                ? platformCode.trim().toLowerCase() : "ALL";
        LocalDate platformStartDate = getLaunchStartDateForPlatform(pCode);
        List<RawOrder> orders = ltvStatService.getOrdersFilteredForUser(pCode, userId).stream()
                .filter(o -> {
                    LocalDate payDate = getOrderPayDateForPlatform(pCode, o);
                    return payDate != null && !payDate.isBefore(platformStartDate);
                })
                .collect(Collectors.toList());
        return calculateDistributionSummaryFromOrders(pCode, orders);
    }

    public DailyDistributionSummaryDto getDailyDistributionSummary(Long userId) {
        return getDailyDistributionSummary("ALL", userId);
    }

    public DailyDistributionSummaryDto getDailyDistributionSummary() {
        return getDailyDistributionSummary("ALL", 1L);
    }

    public DailyDistributionSummaryDto calculateDistributionSummaryFromOrders(List<RawOrder> orders) {
        return calculateDistributionSummaryFromOrders("ALL", orders);
    }

    public DailyDistributionSummaryDto calculateDistributionSummaryFromOrders(String platformCode, List<RawOrder> orders) {
        BigDecimal totalRecharge = orders.stream()
                .map(RawOrder::getOrderAmountUsd)
                .reduce(BigDecimal.ZERO, BigDecimal::add);

        List<RawOrder> newOrders = orders.stream().filter(o -> {
            if (o.getRenewType() != null) {
                return o.getRenewType() == 1;
            }
            LocalDate regDate = getOrderRegisterDateForPlatform(platformCode, o);
            LocalDate payDate = getOrderPayDateForPlatform(platformCode, o);
            return regDate != null && regDate.equals(payDate);
        }).collect(Collectors.toList());

        List<RawOrder> oldOrders = orders.stream().filter(o -> {
            if (o.getRenewType() != null) {
                return o.getRenewType() == 2;
            }
            LocalDate regDate = getOrderRegisterDateForPlatform(platformCode, o);
            LocalDate payDate = getOrderPayDateForPlatform(platformCode, o);
            return regDate != null && regDate.isBefore(payDate);
        }).collect(Collectors.toList());

        BigDecimal newRecharge = newOrders.stream().map(RawOrder::getOrderAmountUsd).reduce(BigDecimal.ZERO, BigDecimal::add);
        BigDecimal oldRecharge = oldOrders.stream().map(RawOrder::getOrderAmountUsd).reduce(BigDecimal.ZERO, BigDecimal::add);

        Set<String> totalPaidUsersSet = orders.stream().map(RawOrder::getMemberId).collect(Collectors.toSet());
        Set<String> newPaidUsersSet = newOrders.stream().map(RawOrder::getMemberId).collect(Collectors.toSet());
        Set<String> oldPaidUsersSet = oldOrders.stream().map(RawOrder::getMemberId).collect(Collectors.toSet());

        int totalPaidUsers = totalPaidUsersSet.size();
        int newPaidUsers = newPaidUsersSet.size();
        int oldPaidUsers = oldPaidUsersSet.size();

        BigDecimal newArpu = newPaidUsers > 0 ? newRecharge.divide(BigDecimal.valueOf(newPaidUsers), 2, RoundingMode.HALF_UP) : BigDecimal.ZERO;
        BigDecimal oldArpu = oldPaidUsers > 0 ? oldRecharge.divide(BigDecimal.valueOf(oldPaidUsers), 2, RoundingMode.HALF_UP) : BigDecimal.ZERO;

        BigDecimal newRechargeRatio = totalRecharge.compareTo(BigDecimal.ZERO) > 0 ? newRecharge.divide(totalRecharge, 4, RoundingMode.HALF_UP) : BigDecimal.ZERO;
        BigDecimal oldRechargeRatio = totalRecharge.compareTo(BigDecimal.ZERO) > 0 ? oldRecharge.divide(totalRecharge, 4, RoundingMode.HALF_UP) : BigDecimal.ZERO;

        Map<String, Long> userOrderCounts = orders.stream()
                .collect(Collectors.groupingBy(RawOrder::getMemberId, Collectors.counting()));

        long repeatPaidUsers = userOrderCounts.values().stream().filter(count -> count >= 2).count();
        BigDecimal repeatRate = totalPaidUsers > 0 ? BigDecimal.valueOf(repeatPaidUsers).divide(BigDecimal.valueOf(totalPaidUsers), 4, RoundingMode.HALF_UP) : BigDecimal.ZERO;

        LocalDate today = getTodayForPlatform(platformCode);
        DateTimeFormatter monthFormatter = DateTimeFormatter.ofPattern("yyyy-MM");
        String thisMonthStr = today.format(monthFormatter);
        String lastMonthStr = today.minusMonths(1).format(monthFormatter);

        BigDecimal thisMonthRecharge = orders.stream()
                .filter(o -> {
                    LocalDate payDate = getOrderPayDateForPlatform(platformCode, o);
                    return payDate != null && YearMonth.from(payDate).toString().equals(thisMonthStr);
                })
                .map(RawOrder::getOrderAmountUsd)
                .reduce(BigDecimal.ZERO, BigDecimal::add);

        BigDecimal thisMonthRefund = orders.stream()
                .filter(o -> {
                    LocalDate payDate = getOrderPayDateForPlatform(platformCode, o);
                    return payDate != null && YearMonth.from(payDate).toString().equals(thisMonthStr)
                            && o.getRefundStatus() != null && o.getRefundStatus() == 2;
                })
                .map(o -> o.getOrderAmountUsd() != null ? o.getOrderAmountUsd() : BigDecimal.ZERO)
                .reduce(BigDecimal.ZERO, BigDecimal::add);

        BigDecimal lastMonthRecharge = orders.stream()
                .filter(o -> {
                    LocalDate payDate = getOrderPayDateForPlatform(platformCode, o);
                    return payDate != null && YearMonth.from(payDate).toString().equals(lastMonthStr);
                })
                .map(RawOrder::getOrderAmountUsd)
                .reduce(BigDecimal.ZERO, BigDecimal::add);

        BigDecimal lastMonthRefund = orders.stream()
                .filter(o -> {
                    LocalDate payDate = getOrderPayDateForPlatform(platformCode, o);
                    return payDate != null && YearMonth.from(payDate).toString().equals(lastMonthStr)
                            && o.getRefundStatus() != null && o.getRefundStatus() == 2;
                })
                .map(o -> o.getOrderAmountUsd() != null ? o.getOrderAmountUsd() : BigDecimal.ZERO)
                .reduce(BigDecimal.ZERO, BigDecimal::add);

        DailyDistributionSummaryDto summary = new DailyDistributionSummaryDto();
        summary.setTotalRecharge(totalRecharge);
        summary.setNewRecharge(newRecharge);
        summary.setOldRecharge(oldRecharge);
        summary.setThisMonthRecharge(thisMonthRecharge);
        summary.setThisMonthRefund(thisMonthRefund);
        summary.setLastMonthRecharge(lastMonthRecharge);
        summary.setLastMonthRefund(lastMonthRefund);
        summary.setThisMonthStr(thisMonthStr);
        summary.setLastMonthStr(lastMonthStr);
        summary.setNewRechargeRatio(newRechargeRatio);
        summary.setOldRechargeRatio(oldRechargeRatio);
        summary.setTotalPaidUsers(totalPaidUsers);
        summary.setNewPaidUsers(newPaidUsers);
        summary.setOldPaidUsers(oldPaidUsers);
        summary.setNewArpu(newArpu);
        summary.setOldArpu(oldArpu);
        summary.setRepeatPaidUsers((int) repeatPaidUsers);
        summary.setRepeatRate(repeatRate);

        return summary;
    }

    /**
     * 计算并持久化指定用户和平台的每日充值分布表 (仅针对指定用户本身，不触发父级主账号)
     */
    @Transactional
    public void calculateDailyDistributionStatsForUserDirect(String platformCode, Long targetUserId) {
        final Long userId = targetUserId != null ? targetUserId : 1L;
        String pCode = (platformCode != null && !platformCode.trim().isEmpty() && !"ALL".equalsIgnoreCase(platformCode.trim()))
                ? platformCode.trim().toLowerCase() : "ALL";
        LocalDate today = getTodayForPlatform(pCode);
        LocalDate platformStartDate = getLaunchStartDateForPlatform(pCode);

        List<RawOrder> orders = ltvStatService.getOrdersFilteredForUser(pCode, userId);

        Map<LocalDate, List<RawOrder>> ordersByPayDate = orders.stream()
                .filter(o -> {
                    LocalDate payDate = getOrderPayDateForPlatform(pCode, o);
                    return payDate != null && !payDate.isBefore(platformStartDate);
                })
                .collect(Collectors.groupingBy(o -> getOrderPayDateForPlatform(pCode, o)));

        List<DailyRechargeDistribution> statList = new ArrayList<>();
        LocalDate currDate = platformStartDate;

        while (!currDate.isAfter(today)) {
            List<RawOrder> dayOrders = ordersByPayDate.getOrDefault(currDate, Collections.emptyList());
            DailyRechargeDistribution stat = calculateSingleDayDistribution(pCode, userId, currDate, dayOrders);
            statList.add(stat);
            currDate = currDate.plusDays(1);
        }

        if (transactionTemplate != null) {
            transactionTemplate.execute(status -> {
                dailyRechargeDistributionRepository.deleteByPlatformCodeAndUserId(pCode, userId);
                dailyRechargeDistributionRepository.flush();
                dailyRechargeDistributionRepository.saveAll(statList);
                dailyRechargeDistributionRepository.flush();
                return null;
            });
        } else {
            dailyRechargeDistributionRepository.deleteByPlatformCodeAndUserId(pCode, userId);
            dailyRechargeDistributionRepository.flush();
            dailyRechargeDistributionRepository.saveAll(statList);
            dailyRechargeDistributionRepository.flush();
        }
    }

    @Transactional
    public void calculateDailyDistributionStatsForUserDirect(Long userId) {
        calculateDailyDistributionStatsForUserDirect("ALL", userId);
    }

    /**
     * 计算并持久化指定用户和平台的每日充值分布表，并异步触发关联主账号数据重算
     */
    @Transactional
    public void calculateDailyDistributionStatsForUser(String platformCode, Long userId) {
        calculateDailyDistributionStatsForUserDirect(platformCode, userId);

        // 异步触发所属主账号的每日充值分布重算 (后台线程执行，不阻塞前端)
        if (asyncRecalculateService != null) {
            asyncRecalculateService.asyncRecalculateMastersForSubUser(userId);
        }
    }

    @Transactional
    public void calculateDailyDistributionStatsForUser(Long userId) {
        calculateDailyDistributionStatsForUser("ALL", userId);
    }

    public void calculateAllDailyDistributionStats() {
        List<SysUser> users = userService.listAllUsers();
        String[] platforms = new String[]{"ALL", "rocnovel", "flicknovel"};
        if (users.isEmpty()) {
            for (String p : platforms) {
                calculateDailyDistributionStatsForUserDirect(p, 1L);
            }
        } else {
            for (SysUser user : users) {
                for (String p : platforms) {
                    calculateDailyDistributionStatsForUserDirect(p, user.getId());
                }
            }
        }
        log.info("Daily recharge distribution calculation completed for all active users across platforms.");
    }

    private DailyRechargeDistribution calculateSingleDayDistribution(String platformCode, Long userId, LocalDate payDate, List<RawOrder> dayOrders) {
        DailyRechargeDistribution stat = new DailyRechargeDistribution();
        stat.setPlatformCode(platformCode != null ? platformCode : "ALL");
        stat.setUserId(userId);
        stat.setDate(payDate);

        if (dayOrders.isEmpty()) {
            stat.setTotalRecharge(BigDecimal.ZERO);
            stat.setSingleRecharge(BigDecimal.ZERO);
            stat.setSubsRecharge(BigDecimal.ZERO);
            stat.setTotalPaidUsers(0);
            stat.setSinglePaidUsers(0);
            stat.setSubsPaidUsers(0);

            stat.setNewRecharge(BigDecimal.ZERO);
            stat.setNewArpu(BigDecimal.ZERO);
            stat.setNewPaidUsers(0);
            stat.setNewSinglePaidUsers(0);
            stat.setNewSubsPaidUsers(0);

            stat.setOldRecharge(BigDecimal.ZERO);
            stat.setOldArpu(BigDecimal.ZERO);
            stat.setOldPaidUsers(0);
            stat.setOldSinglePaidUsers(0);
            stat.setOldSubsPaidUsers(0);

            stat.setRepeatPaidUsers(0);
            stat.setRepeatRate(BigDecimal.ZERO);
            return stat;
        }

        BigDecimal totalRecharge = dayOrders.stream().map(o -> o.getOrderAmountUsd() != null ? o.getOrderAmountUsd() : BigDecimal.ZERO).reduce(BigDecimal.ZERO, BigDecimal::add);
        BigDecimal singleRecharge = dayOrders.stream().filter(o -> o.getIsSubs() == null || o.getIsSubs() == 0).map(o -> o.getOrderAmountUsd() != null ? o.getOrderAmountUsd() : BigDecimal.ZERO).reduce(BigDecimal.ZERO, BigDecimal::add);
        BigDecimal subsRecharge = dayOrders.stream().filter(o -> o.getIsSubs() != null && o.getIsSubs() == 1).map(o -> o.getOrderAmountUsd() != null ? o.getOrderAmountUsd() : BigDecimal.ZERO).reduce(BigDecimal.ZERO, BigDecimal::add);

        Set<String> totalPaidUserSet = dayOrders.stream().map(RawOrder::getMemberId).filter(id -> id != null && !id.trim().isEmpty()).collect(Collectors.toSet());
        Set<String> singlePaidUserSet = dayOrders.stream().filter(o -> o.getIsSubs() == null || o.getIsSubs() == 0).map(RawOrder::getMemberId).filter(id -> id != null && !id.trim().isEmpty()).collect(Collectors.toSet());
        Set<String> subsPaidUserSet = dayOrders.stream().filter(o -> o.getIsSubs() != null && o.getIsSubs() == 1).map(RawOrder::getMemberId).filter(id -> id != null && !id.trim().isEmpty()).collect(Collectors.toSet());

        List<RawOrder> newOrders = dayOrders.stream().filter(o -> {
            if (o.getRenewType() != null) {
                return o.getRenewType() == 1;
            }
            LocalDate regDate = getOrderRegisterDateForPlatform(platformCode, o);
            return regDate != null && regDate.equals(payDate);
        }).collect(Collectors.toList());

        List<RawOrder> oldOrders = dayOrders.stream().filter(o -> {
            if (o.getRenewType() != null) {
                return o.getRenewType() == 2;
            }
            LocalDate regDate = getOrderRegisterDateForPlatform(platformCode, o);
            return regDate != null && regDate.isBefore(payDate);
        }).collect(Collectors.toList());

        BigDecimal newRecharge = newOrders.stream().map(RawOrder::getOrderAmountUsd).reduce(BigDecimal.ZERO, BigDecimal::add);
        Set<String> newPaidUserSet = newOrders.stream().map(RawOrder::getMemberId).collect(Collectors.toSet());
        Set<String> newSinglePaidUserSet = newOrders.stream().filter(o -> o.getIsSubs() == null || o.getIsSubs() == 0).map(RawOrder::getMemberId).collect(Collectors.toSet());
        Set<String> newSubsPaidUserSet = newOrders.stream().filter(o -> o.getIsSubs() != null && o.getIsSubs() == 1).map(RawOrder::getMemberId).collect(Collectors.toSet());

        int newPaidCount = newPaidUserSet.size();
        BigDecimal newArpu = newPaidCount > 0 ? newRecharge.divide(BigDecimal.valueOf(newPaidCount), 2, RoundingMode.HALF_UP) : BigDecimal.ZERO;

        BigDecimal oldRecharge = oldOrders.stream().map(RawOrder::getOrderAmountUsd).reduce(BigDecimal.ZERO, BigDecimal::add);
        Set<String> oldPaidUserSet = oldOrders.stream().map(RawOrder::getMemberId).collect(Collectors.toSet());
        Set<String> oldSinglePaidUserSet = oldOrders.stream().filter(o -> o.getIsSubs() == null || o.getIsSubs() == 0).map(RawOrder::getMemberId).collect(Collectors.toSet());
        Set<String> oldSubsPaidUserSet = oldOrders.stream().filter(o -> o.getIsSubs() != null && o.getIsSubs() == 1).map(RawOrder::getMemberId).collect(Collectors.toSet());

        int oldPaidCount = oldPaidUserSet.size();
        BigDecimal oldArpu = oldPaidCount > 0 ? oldRecharge.divide(BigDecimal.valueOf(oldPaidCount), 2, RoundingMode.HALF_UP) : BigDecimal.ZERO;

        BigDecimal newRechargeRatio = totalRecharge.compareTo(BigDecimal.ZERO) > 0 ? newRecharge.divide(totalRecharge, 4, RoundingMode.HALF_UP) : BigDecimal.ZERO;
        BigDecimal oldRechargeRatio = totalRecharge.compareTo(BigDecimal.ZERO) > 0 ? oldRecharge.divide(totalRecharge, 4, RoundingMode.HALF_UP) : BigDecimal.ZERO;

        Map<String, Long> dayUserOrderCounts = dayOrders.stream().collect(Collectors.groupingBy(RawOrder::getMemberId, Collectors.counting()));
        long dayRepeatPaidUsers = dayUserOrderCounts.values().stream().filter(c -> c >= 2).count();
        int totalPaidCount = totalPaidUserSet.size();
        BigDecimal repeatRate = totalPaidCount > 0 ? BigDecimal.valueOf(dayRepeatPaidUsers).divide(BigDecimal.valueOf(totalPaidCount), 4, RoundingMode.HALF_UP) : BigDecimal.ZERO;

        stat.setTotalRecharge(totalRecharge);
        stat.setSingleRecharge(singleRecharge);
        stat.setSubsRecharge(subsRecharge);
        stat.setTotalPaidUsers(totalPaidCount);
        stat.setSinglePaidUsers(singlePaidUserSet.size());
        stat.setSubsPaidUsers(subsPaidUserSet.size());

        stat.setNewRecharge(newRecharge);
        stat.setNewRechargeRatio(newRechargeRatio);
        stat.setNewArpu(newArpu);
        stat.setNewPaidUsers(newPaidCount);
        stat.setNewSinglePaidUsers(newSinglePaidUserSet.size());
        stat.setNewSubsPaidUsers(newSubsPaidUserSet.size());

        stat.setOldRecharge(oldRecharge);
        stat.setOldRechargeRatio(oldRechargeRatio);
        stat.setOldArpu(oldArpu);
        stat.setOldPaidUsers(oldPaidCount);
        stat.setOldSinglePaidUsers(oldSinglePaidUserSet.size());
        stat.setOldSubsPaidUsers(oldSubsPaidUserSet.size());

        stat.setRepeatPaidUsers((int) dayRepeatPaidUsers);
        stat.setRepeatRate(repeatRate);

        return stat;
    }
}
