package com.ltv.stat.service.flicknovel;

import com.fasterxml.jackson.databind.JsonNode;
import com.fasterxml.jackson.databind.ObjectMapper;
import com.ltv.stat.dto.flicknovel.*;
import com.ltv.stat.entity.FlicknovelPromotion;
import com.ltv.stat.entity.FlicknovelRechargeTemplate;
import com.ltv.stat.entity.FlicknovelRelation;
import com.ltv.stat.entity.PlatformConfig;
import com.ltv.stat.entity.RawOrder;
import com.ltv.stat.entity.SubscriptionConfigVersion;
import com.ltv.stat.enums.PlatformEnum;
import com.ltv.stat.repository.FlicknovelPromotionRepository;
import com.ltv.stat.repository.FlicknovelRechargeTemplateRepository;
import com.ltv.stat.repository.FlicknovelRelationRepository;
import com.ltv.stat.repository.RawOrderRepository;
import com.ltv.stat.repository.SubscriptionConfigVersionRepository;
import com.ltv.stat.service.LtvPredictService;
import com.ltv.stat.service.LtvStatService;
import com.ltv.stat.util.TimeUtils;
import org.slf4j.Logger;
import org.slf4j.LoggerFactory;
import org.springframework.beans.factory.annotation.Autowired;
import org.springframework.context.annotation.Lazy;
import org.springframework.stereotype.Service;
import org.springframework.transaction.annotation.Transactional;

import javax.annotation.PostConstruct;
import java.math.BigDecimal;
import java.math.RoundingMode;
import java.time.Instant;
import java.time.LocalDate;
import java.time.LocalDateTime;
import java.util.*;
import java.util.concurrent.ConcurrentHashMap;
import java.util.stream.Collectors;

/**
 * 番茄司南业务服务
 * 封装数据获取、系统级内部同步与工业级订单清洗逻辑
 */
@Service
public class FlicknovelApiService {

    private static final Logger log = LoggerFactory.getLogger(FlicknovelApiService.class);

    private final FlicknovelApiClient apiClient;
    private final RawOrderRepository rawOrderRepository;
    private final SubscriptionConfigVersionRepository subscriptionConfigVersionRepository;
    private final FlicknovelRelationRepository flicknovelRelationRepository;
    private final FlicknovelPromotionRepository flicknovelPromotionRepository;
    private final FlicknovelRechargeTemplateRepository flicknovelRechargeTemplateRepository;
    private final FlicknovelOrderTypeResolver orderTypeResolver;
    private final ObjectMapper objectMapper;

    private static final String NOT_FOUND_TPL = "__NOT_FOUND__";

    /**
     * 高速内存字典: Map<PromotionId, Map<PriceCent, Integer isSubs>> (0=单充, 1=订阅)
     */
    private final Map<String, Map<Integer, Integer>> promotionPriceTypeCache = new ConcurrentHashMap<>();

    /**
     * 充值模板字典: Map<TemplateId, Map<PriceCent, Integer isSubs>>
     */
    private final Map<String, Map<Integer, Integer>> templatePriceTypeCache = new ConcurrentHashMap<>();

    /**
     * 充值模板高级详情字典: Map<TemplateId, TemplatePriceDetail>
     */
    private final Map<String, TemplatePriceDetail> templateDetailCache = new ConcurrentHashMap<>();

    /**
     * 推广链接 -> 模板ID 映射: Map<PromotionId, TemplateId>
     */
    private final Map<String, String> promotionTemplateMap = new ConcurrentHashMap<>();

    private volatile long lastSyncTimeMs = 0L;

    @Autowired(required = false)
    @Lazy
    private LtvPredictService ltvPredictService;

    @Autowired(required = false)
    @Lazy
    private LtvStatService ltvStatService;

    public FlicknovelApiService(FlicknovelApiClient apiClient,
                               RawOrderRepository rawOrderRepository,
                               SubscriptionConfigVersionRepository subscriptionConfigVersionRepository,
                               FlicknovelRelationRepository flicknovelRelationRepository,
                               FlicknovelPromotionRepository flicknovelPromotionRepository,
                               FlicknovelRechargeTemplateRepository flicknovelRechargeTemplateRepository,
                               FlicknovelOrderTypeResolver orderTypeResolver,
                               ObjectMapper objectMapper) {
        this.apiClient = apiClient;
        this.rawOrderRepository = rawOrderRepository;
        this.subscriptionConfigVersionRepository = subscriptionConfigVersionRepository;
        this.flicknovelRelationRepository = flicknovelRelationRepository;
        this.flicknovelPromotionRepository = flicknovelPromotionRepository;
        this.flicknovelRechargeTemplateRepository = flicknovelRechargeTemplateRepository;
        this.orderTypeResolver = orderTypeResolver;
        this.objectMapper = objectMapper;
    }

    /**
     * 系统启动时预热内存字典
     */
    @PostConstruct
    public void initPromotionPriceCache() {
        log.info("[FlicknovelSync] Initializing promotion & template price cache from DB...");
        try {
            loadCacheFromDb();
            if (promotionPriceTypeCache.isEmpty() || templatePriceTypeCache.isEmpty()) {
                log.info("[FlicknovelSync] In-memory cache empty on startup. Triggering initial sync from OpenAPI...");
                syncPromotionsAndTemplates(true);
            } else {
                log.info("[FlicknovelSync] In-memory cache loaded successfully from DB: {} promotions, {} templates",
                        promotionPriceTypeCache.size(), templatePriceTypeCache.size());
            }
        } catch (Exception e) {
            log.error("[FlicknovelSync] Failed to initialize promotion & template price cache: {}", e.getMessage(), e);
        }
    }

    /**
     * 从本地数据库加载数据构建内存字典 (原子重建并清理失效缓存项)
     */
    public synchronized void loadCacheFromDb() {
        Map<String, TemplatePriceDetail> newTemplateDetailCache = new HashMap<>();
        Map<String, Map<Integer, Integer>> newTemplatePriceTypeCache = new HashMap<>();
        Map<String, String> newPromotionTemplateMap = new HashMap<>();
        Map<String, Map<Integer, Integer>> newPromotionPriceTypeCache = new HashMap<>();

        List<FlicknovelRechargeTemplate> templates = flicknovelRechargeTemplateRepository.findAll();
        for (FlicknovelRechargeTemplate tpl : templates) {
            if (tpl.getTemplateId() != null) {
                if (tpl.getRawPayload() != null && !tpl.getRawPayload().trim().isEmpty()) {
                    try {
                        JsonNode tplNode = objectMapper.readTree(tpl.getRawPayload());
                        TemplatePriceDetail detail = parsePriceTypeDetail(tplNode);
                        newTemplateDetailCache.put(tpl.getTemplateId(), detail);
                        newTemplatePriceTypeCache.put(tpl.getTemplateId(), detail.getPriceMap());
                    } catch (Exception e) {
                        log.warn("[FlicknovelSync] Error parsing rawPayload for template {}: {}", tpl.getTemplateId(), e.getMessage());
                    }
                } else if (tpl.getPriceConfigJson() != null && !tpl.getPriceConfigJson().trim().isEmpty()) {
                    try {
                        Map<String, Integer> rawMap = objectMapper.readValue(tpl.getPriceConfigJson(), new com.fasterxml.jackson.core.type.TypeReference<Map<String, Integer>>() {});
                        Map<Integer, Integer> priceMap = new HashMap<>();
                        for (Map.Entry<String, Integer> entry : rawMap.entrySet()) {
                            priceMap.put(Integer.parseInt(entry.getKey()), entry.getValue());
                        }
                        newTemplatePriceTypeCache.put(tpl.getTemplateId(), priceMap);
                        TemplatePriceDetail fallbackDetail = new TemplatePriceDetail(priceMap, Collections.emptySet(), false,
                                priceMap, priceMap, Collections.emptySet(), Collections.emptySet());
                        newTemplateDetailCache.put(tpl.getTemplateId(), fallbackDetail);
                    } catch (Exception ignored) {}
                }
            }
        }

        List<FlicknovelPromotion> promotions = flicknovelPromotionRepository.findAll();
        for (FlicknovelPromotion prmt : promotions) {
            if (prmt.getPromotionId() != null) {
                String tplId = prmt.getRechargeTplId();
                if (tplId != null) {
                    newPromotionTemplateMap.put(prmt.getPromotionId(), tplId);
                    Map<Integer, Integer> priceMap = newTemplatePriceTypeCache.get(tplId);
                    if (priceMap != null) {
                        newPromotionPriceTypeCache.put(prmt.getPromotionId(), priceMap);
                    }
                }
            }
        }

        templateDetailCache.clear();
        templateDetailCache.putAll(newTemplateDetailCache);

        templatePriceTypeCache.clear();
        templatePriceTypeCache.putAll(newTemplatePriceTypeCache);

        promotionTemplateMap.clear();
        promotionTemplateMap.putAll(newPromotionTemplateMap);

        promotionPriceTypeCache.clear();
        promotionPriceTypeCache.putAll(newPromotionPriceTypeCache);
    }

    /**
     * 1. 获取订单列表 (供系统内部拉取使用)
     */
    public FlicknovelOrderResponse getOrderList(FlicknovelOrderQueryRequest request) {
        if (request == null) {
            request = new FlicknovelOrderQueryRequest();
        }
        if (request.getBeginTs() == null || request.getEndTs() == null) {
            long now = Instant.now().getEpochSecond();
            request.setBeginTs(now - 7 * 86400L);
            request.setEndTs(now);
        }
        if (request.getPage() == null || request.getPage() < 1) {
            request.setPage(1L);
        }
        if (request.getPageSize() == null || request.getPageSize() < 100) {
            request.setPageSize(100L);
        }
        return apiClient.getOrderList(request);
    }

    /**
     * 2. 获取推广链接列表 (供系统内部拉取使用)
     */
    public FlicknovelPromotionResponse getPromotionList(FlicknovelPromotionQueryRequest request) {
        if (request == null) {
            request = new FlicknovelPromotionQueryRequest();
        }
        if (request.getEmail() == null || request.getEmail().trim().isEmpty()) {
            request.setEmail(apiClient.getDefaultEmail());
        }
        if (request.getPage() == null || request.getPage() < 1) {
            request.setPage(1L);
        }
        if (request.getPageSize() == null || request.getPageSize() < 10) {
            request.setPageSize(50L);
        }
        return apiClient.getPromotionList(request);
    }

    /**
     * 3. 获取充值模版列表 (供系统内部拉取使用)
     */
    public FlicknovelRechargeTemplateResponse getRechargeTemplateList(FlicknovelRechargeTemplateQueryRequest request) {
        if (request == null) {
            request = new FlicknovelRechargeTemplateQueryRequest();
        }
        if (request.getEmail() == null || request.getEmail().trim().isEmpty()) {
            request.setEmail(apiClient.getDefaultEmail());
        }
        if (request.getPage() == null || request.getPage() < 1) {
            request.setPage(1L);
        }
        if (request.getPageSize() == null || request.getPageSize() < 10) {
            request.setPageSize(50L);
        }
        return apiClient.getRechargeTemplateList(request);
    }

    /**
     * 同步指定日期区间的订单到系统的 raw_order 表中，并执行全流程清洗
     *
     * @param startDate 开始日期 (包含)
     * @param endDate 结束日期 (包含)
     * @param config 平台配置
     * @return 同步保存/更新的订单数量
     */
    public int syncOrders(LocalDate startDate, LocalDate endDate, PlatformConfig config) {
        if (startDate == null) {
            startDate = PlatformEnum.FLICKNOVEL.getLaunchStartDate();
        }
        if (endDate == null) {
            endDate = LocalDate.now(TimeUtils.BEIJING_ZONE);
        }

        log.info("[FlicknovelSync] Starting syncOrders from {} to {}", startDate, endDate);

        // 番茄司南数据默认以 UTC 时区为基准，按 UTC 自然日划分时间窗口
        int totalSavedOrders = 0;
        LocalDate currentStart = startDate;
        while (!currentStart.isAfter(endDate)) {
            LocalDate currentEnd = currentStart.plusDays(25);
            if (currentEnd.isAfter(endDate)) {
                currentEnd = endDate;
            }

            long beginTs = currentStart.atStartOfDay(java.time.ZoneOffset.UTC).toEpochSecond();
            long endTs = currentEnd.plusDays(1).atStartOfDay(java.time.ZoneOffset.UTC).toEpochSecond();

            totalSavedOrders += fetchAndCleanOrderSegment(beginTs, endTs);

            currentStart = currentEnd.plusDays(1);
        }

        log.info("[FlicknovelSync] Finished syncOrders from {} to {}. Total saved/updated: {}",
                startDate, endDate, totalSavedOrders);
        return totalSavedOrders;
    }

    /**
     * 按分段与分页拉取订单并执行批量清洗入库
     */
    private int fetchAndCleanOrderSegment(long beginTs, long endTs) {
        int segmentSavedCount = 0;
        long page = 1L;
        long pageSize = 500L; // [100, 5000]

        // 预拉取本区间内的染色归因记录，构建 relation_id -> relation_begin_time 映射，作为精准注册时间
        Map<String, LocalDateTime> relationTimeMap = fetchRelationBeginTimes(beginTs, endTs);

        while (true) {
            FlicknovelOrderQueryRequest req = new FlicknovelOrderQueryRequest(beginTs, endTs, page, pageSize);
            FlicknovelOrderResponse response;
            try {
                response = apiClient.getOrderList(req);
            } catch (Exception e) {
                log.error("[FlicknovelSync] Error calling getOrderList for segment [{}, {}] page {}: {}",
                        beginTs, endTs, page, e.getMessage());
                break;
            }

            if (response == null || !response.isSuccess() || response.getData() == null) {
                log.warn("[FlicknovelSync] Failed or empty response on page {}: code={}, msg={}",
                        page, response != null ? response.getCode() : null, response != null ? response.getMessage() : null);
                break;
            }

            List<FlicknovelOrderDto> orders = response.getData().getOrders();
            if (orders == null || orders.isEmpty()) {
                break;
            }

            // 执行批量清洗入库
            int cleaned = batchCleanAndSaveOrders(orders, relationTimeMap);
            segmentSavedCount += cleaned;

            if (orders.size() < pageSize) {
                break;
            }

            page++;
        }

        return segmentSavedCount;
    }

    /**
     * 批量清洗并持久化订单集合
     * 核心解决：
     * 1. 识别并对齐用户首充 (renew_type=1) 与老用户复充 (renew_type=2)
     * 2. 锚定用户最初注册/归因时间 (register_time)，保证 LTV 留存与充值准确归集
     * 3. 美元与美分精确换算
     * 4. 双时区 (北京时间 & 美东时间) 标准转换
     */
    public int batchCleanAndSaveOrders(List<FlicknovelOrderDto> orders, Map<String, LocalDateTime> relationTimeMap) {
        if (orders == null || orders.isEmpty()) {
            return 0;
        }

        // 1. 过滤非法订单并提取 memberId
        List<FlicknovelOrderDto> validOrders = new ArrayList<>();
        Set<String> memberIds = new HashSet<>();
        for (FlicknovelOrderDto o : orders) {
            if (o.getOrderId() != null && !o.getOrderId().trim().isEmpty()) {
                validOrders.add(o);
                String mId = extractMemberId(o);
                if (mId != null && !mId.isEmpty()) {
                    memberIds.add(mId);
                }
            }
        }

        if (validOrders.isEmpty()) {
            return 0;
        }

        // 2. 预查库中已有的历史订单，找到各用户的最早支付时间与最早注册时间
        List<RawOrder> existingHistoryOrders = Collections.emptyList();
        if (!memberIds.isEmpty()) {
            existingHistoryOrders = rawOrderRepository.findByPlatformCodeAndMemberIdIn(
                    PlatformEnum.FLICKNOVEL.getCode(), new ArrayList<>(memberIds));
        }

        // 聚合库中已有的用户画像数据
        Map<String, UserProfileSnapshot> userHistoryMap = new HashMap<>();
        for (RawOrder ho : existingHistoryOrders) {
            String mId = ho.getMemberId();
            if (mId == null || mId.trim().isEmpty()) continue;
            userHistoryMap.compute(mId, (k, existing) -> {
                boolean isSubs = ho.getIsSubs() != null && ho.getIsSubs() == 1;
                LocalDateTime subsTime = isSubs ? ho.getPayTimeBj() : null;
                if (existing == null) {
                    return new UserProfileSnapshot(ho.getPayTimeBj(), ho.getRegisterTimeBj(), ho.getLandingPageId(), isSubs, subsTime);
                } else {
                    if (ho.getPayTimeBj() != null && (existing.earliestPayTime == null || ho.getPayTimeBj().isBefore(existing.earliestPayTime))) {
                        existing.earliestPayTime = ho.getPayTimeBj();
                    }
                    if (ho.getRegisterTimeBj() != null && (existing.earliestRegTime == null || ho.getRegisterTimeBj().isBefore(existing.earliestRegTime))) {
                        existing.earliestRegTime = ho.getRegisterTimeBj();
                    }
                    if ((existing.landingPageId == null || existing.landingPageId.isEmpty()) && ho.getLandingPageId() != null) {
                        existing.landingPageId = ho.getLandingPageId();
                    }
                    if (isSubs) {
                        existing.hasSubscribed = true;
                        if (existing.latestSubsPayTime == null || (ho.getPayTimeBj() != null && ho.getPayTimeBj().isAfter(existing.latestSubsPayTime))) {
                            existing.latestSubsPayTime = ho.getPayTimeBj();
                        }
                    }
                    return existing;
                }
            });
        }

        // 3. 将本批次订单按用户聚类，并按支付时间升序排序
        Map<String, List<FlicknovelOrderDto>> ordersByMember = validOrders.stream()
                .collect(Collectors.groupingBy(this::extractMemberId));

        List<RawOrder> toSave = new ArrayList<>();

        for (Map.Entry<String, List<FlicknovelOrderDto>> entry : ordersByMember.entrySet()) {
            String memberId = entry.getKey();
            List<FlicknovelOrderDto> memberOrders = entry.getValue();

            // 按支付时间升序排列
            memberOrders.sort(Comparator.comparingLong(o -> parseEpochSecondSafe(o.getCompletedAt(), o.getCreatedAt())));

            UserProfileSnapshot profile = userHistoryMap.computeIfAbsent(memberId, k -> new UserProfileSnapshot(null, null, null, false, null));

            for (FlicknovelOrderDto dto : memberOrders) {
                RawOrder order = cleanSingleOrder(dto, memberId, profile, relationTimeMap);
                if (order != null) {
                    toSave.add(order);
                }
            }
        }

        if (!toSave.isEmpty()) {
            rawOrderRepository.saveAll(toSave);
            rawOrderRepository.flush();
        }

        return toSave.size();
    }

    /**
     * 单笔订单核心清洗转换与落库
     */
    private RawOrder cleanSingleOrder(FlicknovelOrderDto dto,
                                     String memberId,
                                     UserProfileSnapshot profile,
                                     Map<String, LocalDateTime> relationTimeMap) {
        if (dto == null || dto.getOrderId() == null || dto.getOrderId().trim().isEmpty()) {
            return null;
        }
        String orderId = dto.getOrderId().trim();

        // 1. 落地页 / 渠道清洗
        String landingPageId = dto.getPromotionId();
        if (landingPageId == null || landingPageId.trim().isEmpty() || "null".equalsIgnoreCase(landingPageId.trim())) {
            landingPageId = dto.getPromotionCode();
        }
        if (landingPageId == null || landingPageId.trim().isEmpty() || "null".equalsIgnoreCase(landingPageId.trim())) {
            landingPageId = profile != null ? profile.landingPageId : "";
        }
        landingPageId = landingPageId != null ? landingPageId.trim() : "";

        // 2. 支付时间清洗 (番茄返回 UTC 秒级时间戳，分别精准换算为北京时间、美东时间及原生 UTC 时间)
        long payTs = parseEpochSecondSafe(dto.getCompletedAt(), dto.getCreatedAt());
        Instant payInstant = Instant.ofEpochSecond(payTs);
        LocalDateTime payTimeBj = payInstant.atZone(TimeUtils.BEIJING_ZONE).toLocalDateTime();
        LocalDateTime payTimeEt = payInstant.atZone(TimeUtils.EASTERN_ZONE).toLocalDateTime();
        LocalDate payDateEt = payTimeEt.toLocalDate();
        LocalDateTime payTimeUtc = payInstant.atZone(TimeUtils.UTC_ZONE).toLocalDateTime();
        LocalDate payDateUtc = payTimeUtc.toLocalDate();

        // 3. 首充/续订判断 (renew_type) 与用户注册/归因时间 (register_time) 清洗
        int renewType;
        LocalDateTime regTimeBj;
        LocalDateTime regTimeEt;

        // 染色归因时间判定
        LocalDateTime relationTimeBj = null;
        if (dto.getRelationId() != null) {
            String rId = dto.getRelationId().trim();
            if (relationTimeMap != null && relationTimeMap.containsKey(rId)) {
                relationTimeBj = relationTimeMap.get(rId);
            } else {
                FlicknovelRelation localRel = flicknovelRelationRepository.findByRelationId(rId).orElse(null);
                if (localRel != null && localRel.getRelationBeginTimeBj() != null) {
                    relationTimeBj = localRel.getRelationBeginTimeBj();
                }
            }
        }

        if (profile.earliestPayTime == null) {
            // 该用户在库中尚未有更早订单，本笔订单为首充！
            renewType = 1;
            regTimeBj = relationTimeBj != null ? relationTimeBj : payTimeBj;
            regTimeEt = relationTimeBj != null ? TimeUtils.convertBjToEt(relationTimeBj) : payTimeEt;

            // 更新用户画像快照供该用户后续订单继承
            profile.earliestPayTime = payTimeBj;
            profile.earliestRegTime = regTimeBj;
            if ((profile.landingPageId == null || profile.landingPageId.isEmpty()) && !landingPageId.isEmpty()) {
                profile.landingPageId = landingPageId;
            }
        } else {
            // 库中或本批已有更早订单
            if (payTimeBj.isAfter(profile.earliestPayTime)) {
                // 晚于首单，属于老用户复充！
                renewType = 2;
                regTimeBj = profile.earliestRegTime != null ? profile.earliestRegTime : profile.earliestPayTime;
                regTimeEt = TimeUtils.convertBjToEt(regTimeBj);
            } else {
                // 当前订单时间更早，则当前为首单
                renewType = 1;
                regTimeBj = relationTimeBj != null ? relationTimeBj : payTimeBj;
                regTimeEt = relationTimeBj != null ? TimeUtils.convertBjToEt(relationTimeBj) : payTimeEt;
                profile.earliestPayTime = payTimeBj;
                profile.earliestRegTime = regTimeBj;
            }
        }

        LocalDate regDateEt = regTimeEt != null ? regTimeEt.toLocalDate() : payDateEt;
        LocalDateTime regTimeUtc = TimeUtils.convertBjToUtc(regTimeBj);
        LocalDate regDateUtc = regTimeUtc != null ? regTimeUtc.toLocalDate() : payDateUtc;

        // 4. 金额清洗 (us_price 美元字符串 -> BigDecimal 与 美分整数)
        BigDecimal orderAmountUsd = BigDecimal.ZERO;
        if (dto.getUsPrice() != null && !dto.getUsPrice().trim().isEmpty()) {
            try {
                orderAmountUsd = new BigDecimal(dto.getUsPrice().trim()).setScale(2, RoundingMode.HALF_UP);
            } catch (Exception ignored) {}
        }
        int orderAmountCent = orderAmountUsd.multiply(BigDecimal.valueOf(100)).intValue();

        // 4.1 充值类型清洗识别 (0=单充/代币充值, 1=时长订阅)
        // 依托解耦的 FlicknovelOrderTypeResolver 策略组件进行多级判定与时序消歧
        int isSubs = determineOrderIsSubs(landingPageId, orderAmountCent, renewType, payTimeBj, profile, dto);
        if (isSubs == 1) {
            profile.hasSubscribed = true;
            profile.latestSubsPayTime = payTimeBj;
        }

        // 5. 订单实体装配与幂等落库
        Optional<RawOrder> existingOpt = rawOrderRepository.findByPlatformCodeAndOrderId(PlatformEnum.FLICKNOVEL.getCode(), orderId);
        RawOrder order = existingOpt.orElseGet(RawOrder::new);

        order.setPlatformCode(PlatformEnum.FLICKNOVEL.getCode());
        order.setOrderId(orderId);
        order.setMemberId(memberId);
        order.setLandingPageId(landingPageId);
        order.setRegisterTimeBj(regTimeBj);
        order.setRegisterTimeEt(regTimeEt);
        order.setRegisterDateEt(regDateEt);
        order.setRegisterTimeUtc(regTimeUtc);
        order.setRegisterDateUtc(regDateUtc);
        order.setPayTimeBj(payTimeBj);
        order.setPayTimeEt(payTimeEt);
        order.setPayDateEt(payDateEt);
        order.setPayTimeUtc(payTimeUtc);
        order.setPayDateUtc(payDateUtc);
        order.setOrderAmountCent(orderAmountCent);
        order.setOrderAmountUsd(orderAmountUsd);
        order.setIsSubs(isSubs);
        order.setRenewType(renewType); // 精准首单 1 vs 复充 2
        order.setPayState(1); // 已支付
        order.setRefundStatus(0); // 正常未退款

        try {
            order.setRawPayload(objectMapper.writeValueAsString(dto));
        } catch (Exception ignored) {}

        return order;
    }

    /**
     * 辅助提取 memberId (优先 device_id，若无取 relation_id，兜底 order_id)
     */
    private String extractMemberId(FlicknovelOrderDto dto) {
        if (dto.getDeviceId() != null && !dto.getDeviceId().trim().isEmpty() && !"null".equalsIgnoreCase(dto.getDeviceId().trim())) {
            return dto.getDeviceId().trim();
        }
        if (dto.getRelationId() != null && !dto.getRelationId().trim().isEmpty() && !"null".equalsIgnoreCase(dto.getRelationId().trim())) {
            return dto.getRelationId().trim();
        }
        return dto.getOrderId() != null ? dto.getOrderId().trim() : "UNKNOWN";
    }

    /**
     * 辅助解析秒级时间戳
     */
    private long parseEpochSecondSafe(String primaryTs, String fallbackTs) {
        try {
            if (primaryTs != null && !primaryTs.trim().isEmpty()) {
                return Long.parseLong(primaryTs.trim());
            }
        } catch (Exception ignored) {}
        try {
            if (fallbackTs != null && !fallbackTs.trim().isEmpty()) {
                return Long.parseLong(fallbackTs.trim());
            }
        } catch (Exception ignored) {}
        return Instant.now().getEpochSecond();
    }

    /**
     * 定时/手动同步染色归因记录到 flicknovel_relation 表
     *
     * @param startDate 开始日期 (默认今天 - 2 天)
     * @param endDate 结束日期 (默认今天)
     * @return 同步保存/更新的染色记录数
     */
    public int syncRelations(LocalDate startDate, LocalDate endDate) {
        if (startDate == null) {
            startDate = LocalDate.now(TimeUtils.BEIJING_ZONE).minusDays(2);
        }
        if (endDate == null) {
            endDate = LocalDate.now(TimeUtils.BEIJING_ZONE);
        }
        if (startDate.isAfter(endDate)) {
            LocalDate tmp = startDate;
            startDate = endDate;
            endDate = tmp;
        }

        long beginTs = startDate.atStartOfDay(TimeUtils.BEIJING_ZONE).toEpochSecond();
        long endTs = endDate.plusDays(1).atStartOfDay(TimeUtils.BEIJING_ZONE).toEpochSecond();

        log.info("[FlicknovelRelationSync] Starting syncRelations from {} to {} (ts: [{}, {}))",
                startDate, endDate, beginTs, endTs);

        int totalSaved = 0;
        long page = 1L;
        long pageSize = 1000L;

        while (true) {
            FlicknovelOrderQueryRequest req = new FlicknovelOrderQueryRequest(beginTs, endTs, page, pageSize);
            FlicknovelRelationResponse resp = null;
            try {
                resp = apiClient.getRelationList(req);
            } catch (Exception e) {
                log.error("[FlicknovelRelationSync] Failed to query relations on page {}: {}", page, e.getMessage());
                break;
            }

            if (resp == null || !resp.isSuccess() || resp.getData() == null) {
                break;
            }

            List<FlicknovelRelationDto> relations = resp.getData().getRelations();
            if (relations == null || relations.isEmpty()) {
                break;
            }

            int savedInPage = saveOrUpdateRelations(relations);
            totalSaved += savedInPage;

            if (relations.size() < pageSize) {
                break;
            }
            page++;
        }

        log.info("[FlicknovelRelationSync] Finished syncRelations. Total saved/updated: {}", totalSaved);
        return totalSaved;
    }

    /**
     * 批量持久化保存或更新染色记录到 flicknovel_relation 表
     */
    @Transactional
    public int saveOrUpdateRelations(List<FlicknovelRelationDto> relations) {
        if (relations == null || relations.isEmpty()) {
            return 0;
        }

        List<String> rIds = relations.stream()
                .map(FlicknovelRelationDto::getRelationId)
                .filter(id -> id != null && !id.trim().isEmpty())
                .map(String::trim)
                .collect(Collectors.toList());

        Map<String, FlicknovelRelation> existingMap = new HashMap<>();
        if (!rIds.isEmpty()) {
            List<FlicknovelRelation> existingList = flicknovelRelationRepository.findByRelationIdIn(rIds);
            for (FlicknovelRelation exist : existingList) {
                existingMap.put(exist.getRelationId(), exist);
            }
        }

        List<FlicknovelRelation> toSave = new ArrayList<>();
        for (FlicknovelRelationDto dto : relations) {
            if (dto.getRelationId() == null || dto.getRelationId().trim().isEmpty()) {
                continue;
            }
            String rId = dto.getRelationId().trim();
            FlicknovelRelation entity = existingMap.getOrDefault(rId, new FlicknovelRelation());
            entity.setRelationId(rId);
            entity.setDeviceId(dto.getDeviceId() != null ? dto.getDeviceId().trim() : "");
            entity.setPromotionId(dto.getPromotionId() != null ? dto.getPromotionId().trim() : null);
            entity.setPromotionCode(dto.getPromotionCode() != null ? dto.getPromotionCode().trim() : null);
            entity.setAdId(dto.getAdId() != null ? dto.getAdId().trim() : null);
            entity.setAdsetId(dto.getAdsetId() != null ? dto.getAdsetId().trim() : null);
            entity.setCampaignId(dto.getCampaignId() != null ? dto.getCampaignId().trim() : null);
            entity.setAdAccountId(dto.getAdAccountId() != null ? dto.getAdAccountId().trim() : null);
            entity.setMediaChannel(dto.getMediaChannel() != null ? dto.getMediaChannel().trim() : null);
            entity.setPlatform(dto.getPlatform() != null ? dto.getPlatform().trim() : null);
            entity.setAppId(dto.getAppId() != null ? dto.getAppId().trim() : null);

            if (dto.getRelationBeginTime() != null && !dto.getRelationBeginTime().trim().isEmpty()) {
                try {
                    long sec = Long.parseLong(dto.getRelationBeginTime().trim());
                    entity.setRelationBeginTimestamp(sec);
                    LocalDateTime bjTime = LocalDateTime.ofInstant(Instant.ofEpochSecond(sec), TimeUtils.BEIJING_ZONE);
                    LocalDateTime etTime = LocalDateTime.ofInstant(Instant.ofEpochSecond(sec), TimeUtils.EASTERN_ZONE);
                    entity.setRelationBeginTimeBj(bjTime);
                    entity.setRelationBeginTimeEt(etTime);
                    entity.setRelationBeginDateEt(etTime.toLocalDate());
                } catch (Exception ignored) {}
            }

            try {
                entity.setRawPayload(objectMapper.writeValueAsString(dto));
            } catch (Exception ignored) {}

            toSave.add(entity);
        }

        if (!toSave.isEmpty()) {
            flicknovelRelationRepository.saveAll(toSave);
            flicknovelRelationRepository.flush();
        }
        return toSave.size();
    }

    /**
     * 拉取指定时间区间的染色归因记录，构建 relation_id -> relation_begin_time 字典，并自动落库到 flicknovel_relation 表
     */
    private Map<String, LocalDateTime> fetchRelationBeginTimes(long beginTs, long endTs) {
        Map<String, LocalDateTime> map = new HashMap<>();
        long page = 1L;
        long pageSize = 1000L;
        try {
            while (true) {
                FlicknovelOrderQueryRequest req = new FlicknovelOrderQueryRequest(beginTs, endTs, page, pageSize);
                FlicknovelRelationResponse resp = apiClient.getRelationList(req);
                if (resp == null || !resp.isSuccess() || resp.getData() == null || resp.getData().getRelations() == null) {
                    break;
                }
                List<FlicknovelRelationDto> relations = resp.getData().getRelations();
                if (relations.isEmpty()) {
                    break;
                }

                // 将本批染色记录独立落库
                saveOrUpdateRelations(relations);

                for (FlicknovelRelationDto r : relations) {
                    if (r.getRelationId() != null && r.getRelationBeginTime() != null) {
                        try {
                            long sec = Long.parseLong(r.getRelationBeginTime().trim());
                            LocalDateTime bjTime = LocalDateTime.ofInstant(Instant.ofEpochSecond(sec), TimeUtils.BEIJING_ZONE);
                            map.put(r.getRelationId().trim(), bjTime);
                        } catch (Exception ignored) {}
                    }
                }

                if (relations.size() < pageSize) {
                    break;
                }
                page++;
            }
        } catch (Exception e) {
            log.debug("[FlicknovelSync] Optional relation list fetch skipped or empty: {}", e.getMessage());
        }
        return map;
    }

    /**
     * 充值模板高级详情结构 (包含无歧义价格字典、冲突价格集合、是否存在首购特惠)
     */
    public static class TemplatePriceDetail {
        private final Map<Integer, Integer> priceMap;
        private final Set<Integer> ambiguousPrices;
        private final boolean hasIntroOffer;
        private final Map<Integer, Integer> firstPriceMap;
        private final Map<Integer, Integer> noFirstPriceMap;
        private final Set<Integer> firstAmbiguousPrices;
        private final Set<Integer> noFirstAmbiguousPrices;

        public TemplatePriceDetail(Map<Integer, Integer> priceMap, Set<Integer> ambiguousPrices, boolean hasIntroOffer) {
            this(priceMap, ambiguousPrices, hasIntroOffer, Collections.emptyMap(), Collections.emptyMap(), Collections.emptySet(), Collections.emptySet());
        }

        public TemplatePriceDetail(Map<Integer, Integer> priceMap, Set<Integer> ambiguousPrices, boolean hasIntroOffer,
                                   Map<Integer, Integer> firstPriceMap, Map<Integer, Integer> noFirstPriceMap,
                                   Set<Integer> firstAmbiguousPrices, Set<Integer> noFirstAmbiguousPrices) {
            this.priceMap = priceMap != null ? priceMap : Collections.emptyMap();
            this.ambiguousPrices = ambiguousPrices != null ? ambiguousPrices : Collections.emptySet();
            this.hasIntroOffer = hasIntroOffer;
            this.firstPriceMap = firstPriceMap != null ? firstPriceMap : Collections.emptyMap();
            this.noFirstPriceMap = noFirstPriceMap != null ? noFirstPriceMap : Collections.emptyMap();
            this.firstAmbiguousPrices = firstAmbiguousPrices != null ? firstAmbiguousPrices : Collections.emptySet();
            this.noFirstAmbiguousPrices = noFirstAmbiguousPrices != null ? noFirstAmbiguousPrices : Collections.emptySet();
        }

        public Map<Integer, Integer> getPriceMap() { return priceMap; }
        public Set<Integer> getAmbiguousPrices() { return ambiguousPrices; }
        public boolean isHasIntroOffer() { return hasIntroOffer; }
        public Map<Integer, Integer> getFirstPriceMap() { return firstPriceMap; }
        public Map<Integer, Integer> getNoFirstPriceMap() { return noFirstPriceMap; }
        public Set<Integer> getFirstAmbiguousPrices() { return firstAmbiguousPrices; }
        public Set<Integer> getNoFirstAmbiguousPrices() { return noFirstAmbiguousPrices; }
    }

    /**
     * 用户历史画像快照 (内存轻量级模型)
     */
    private static class UserProfileSnapshot {
        LocalDateTime earliestPayTime;
        LocalDateTime earliestRegTime;
        String landingPageId;
        boolean hasSubscribed;
        LocalDateTime latestSubsPayTime;

        UserProfileSnapshot(LocalDateTime earliestPayTime, LocalDateTime earliestRegTime, String landingPageId,
                            boolean hasSubscribed, LocalDateTime latestSubsPayTime) {
            this.earliestPayTime = earliestPayTime;
            this.earliestRegTime = earliestRegTime;
            this.landingPageId = landingPageId;
            this.hasSubscribed = hasSubscribed;
            this.latestSubsPayTime = latestSubsPayTime;
        }
    }

    /**
     * 同步推广链接配置与商品信息
     */
    @Transactional
    public int syncPromotionsAndConfigs(String email, Long distAppId) {
        log.info("[FlicknovelSync] Starting syncPromotionsAndConfigs for email: {}, distAppId: {}", email, distAppId);
        if (email == null || email.trim().isEmpty()) {
            email = apiClient.getDefaultEmail();
        }

        int savedConfigs = 0;
        long page = 1L;
        long pageSize = 50L;

        while (true) {
            FlicknovelPromotionQueryRequest req = new FlicknovelPromotionQueryRequest(email, page, pageSize);
            if (distAppId != null) {
                req.setDistAppId(Collections.singletonList(distAppId));
            }

            FlicknovelPromotionResponse response;
            try {
                response = apiClient.getPromotionList(req);
            } catch (Exception e) {
                log.error("[FlicknovelSync] Error querying promotion list: {}", e.getMessage());
                break;
            }

            if (response == null || !response.isSuccess() || response.getData() == null) {
                break;
            }

            List<FlicknovelPromotionDto> promotions = response.getData().getPromotions();
            if (promotions == null || promotions.isEmpty()) {
                break;
            }

            for (FlicknovelPromotionDto p : promotions) {
                String landingPageId = p.getPromotionId();
                if (landingPageId == null || landingPageId.trim().isEmpty()) {
                    continue;
                }

                // 写入版本快照 subscription_config_version
                SubscriptionConfigVersion version = new SubscriptionConfigVersion();
                version.setPlatformCode(PlatformEnum.FLICKNOVEL.getCode());
                version.setLandingPageId(landingPageId);
                version.setSubscribeConfigId(p.getRechargeTplId() != null ? p.getRechargeTplId() : "TPL_DEFAULT");
                version.setSubscribeConfigName(p.getRechargeTplName() != null ? p.getRechargeTplName() : p.getPromotionName());
                version.setSaleComboId(p.getDramaId() != null ? p.getDramaId() : "");
                version.setSaleComboName(p.getDramaTitle() != null ? p.getDramaTitle() : "");
                version.setProductId(p.getChapterId() != null ? p.getChapterId() : "PROD_DEFAULT");
                version.setProductName(p.getChapterTitle() != null ? p.getChapterTitle() : "默认内容");
                version.setSubPeriodDays(1);
                version.setFirstPriceCent(0);
                version.setRenewPriceCent(0);
                version.setVersionNum(1);
                version.setEffectiveStartTime(LocalDateTime.now());

                subscriptionConfigVersionRepository.save(version);
                savedConfigs++;
            }

            if (promotions.size() < pageSize) {
                break;
            }
            page++;
        }

        log.info("[FlicknovelSync] Completed syncPromotionsAndConfigs. Saved {} version records.", savedConfigs);
        return savedConfigs;
    }

    /**
     * 充值类型清洗识别 (0=单充/代币充值, 1=时长订阅)
     * 核心逻辑:
     * 1. 优先调用解耦的 FlicknovelOrderTypeResolver (前瞻支持 OpenAPI 透出字段, 零耦合)
     * 2. 无显式字段时，通过模板字典 + 用户时序消歧 (解决单充与订阅同金额如 39.99 冲突)
     */
    public int determineOrderIsSubs(String promotionId, int orderAmountCent, int renewType,
                                    LocalDateTime payTimeBj, UserProfileSnapshot profile, FlicknovelOrderDto dto) {
        FlicknovelOrderTypeResolver.OrderResolveContext ctx = new FlicknovelOrderTypeResolver.OrderResolveContext();
        ctx.setDto(dto);
        ctx.setPromotionId(promotionId);
        ctx.setOrderAmountCent(orderAmountCent);
        ctx.setRenewType(renewType);
        ctx.setPayTimeBj(payTimeBj);
        if (payTimeBj != null) {
            ctx.setPayTimeUtc(TimeUtils.convertBjToUtc(payTimeBj));
        } else if (dto != null) {
            long payTs = parseEpochSecondSafe(dto.getCompletedAt(), dto.getCreatedAt());
            if (payTs > 0) {
                ctx.setPayTimeUtc(Instant.ofEpochSecond(payTs).atZone(TimeUtils.UTC_ZONE).toLocalDateTime());
            }
        }
        ctx.setHasSubscribed(profile != null && profile.hasSubscribed);
        ctx.setLatestSubsPayTime(profile != null ? profile.latestSubsPayTime : null);

        // 填充模板信息与冲突价格信息
        populateTemplateContext(promotionId, ctx);

        return orderTypeResolver.resolve(ctx);
    }

    /**
     * 重载兼容旧接口
     */
    public int determineOrderIsSubs(String promotionId, int orderAmountCent, FlicknovelOrderDto dto) {
        return determineOrderIsSubs(promotionId, orderAmountCent, 1, LocalDateTime.now(), null, dto);
    }

    /**
     * 填充模板上下文（若未命中则自愈拉取 API）
     */
    private void populateTemplateContext(String promotionId, FlicknovelOrderTypeResolver.OrderResolveContext ctx) {
        if (promotionId == null || promotionId.trim().isEmpty()) {
            return;
        }
        String pId = promotionId.trim();
        String tplId = promotionTemplateMap.get(pId);

        if (NOT_FOUND_TPL.equals(tplId)) {
            // 已确认不存在的推广ID（负缓存），直接返回，避免反复击穿
            return;
        }

        if (tplId == null) {
            // 查 DB
            FlicknovelPromotion dbPrmt = flicknovelPromotionRepository.findByPromotionId(pId).orElse(null);
            if (dbPrmt != null && dbPrmt.getRechargeTplId() != null) {
                tplId = dbPrmt.getRechargeTplId();
                promotionTemplateMap.put(pId, tplId);
            } else {
                // DB 也无，触发按需自愈同步
                log.info("[FlicknovelSync] PromotionId {} not found in memory or DB. Triggering on-demand sync...", pId);
                syncPromotionsAndTemplates(false);
                tplId = promotionTemplateMap.get(pId);
                if (tplId == null) {
                    // 同步后依然未找到，记录负缓存，避免后续相同无效订单重复击穿触发全量拉取
                    promotionTemplateMap.put(pId, NOT_FOUND_TPL);
                    return;
                }
            }
        }

        if (tplId != null && !NOT_FOUND_TPL.equals(tplId)) {
            TemplatePriceDetail detail = templateDetailCache.get(tplId);
            if (detail == null) {
                FlicknovelRechargeTemplate dbTpl = flicknovelRechargeTemplateRepository.findByTemplateId(tplId).orElse(null);
                if (dbTpl != null && dbTpl.getRawPayload() != null && !dbTpl.getRawPayload().trim().isEmpty()) {
                    try {
                        JsonNode tplNode = objectMapper.readTree(dbTpl.getRawPayload());
                        detail = parsePriceTypeDetail(tplNode);
                        templateDetailCache.put(tplId, detail);
                        templatePriceTypeCache.put(tplId, detail.getPriceMap());
                    } catch (Exception ignored) {}
                }
            }
            if (detail != null) {
                ctx.setTemplatePriceMap(detail.getPriceMap());
                ctx.setAmbiguousPrices(detail.getAmbiguousPrices());
                ctx.setTemplateHasIntroOffer(detail.isHasIntroOffer());
                ctx.setFirstPriceMap(detail.getFirstPriceMap());
                ctx.setNoFirstPriceMap(detail.getNoFirstPriceMap());
                ctx.setFirstAmbiguousPrices(detail.getFirstAmbiguousPrices());
                ctx.setNoFirstAmbiguousPrices(detail.getNoFirstAmbiguousPrices());
                return;
            }
            Map<Integer, Integer> priceMap = templatePriceTypeCache.get(tplId);
            if (priceMap != null) {
                ctx.setTemplatePriceMap(priceMap);
            }
        }
    }

    /**
     * 预先拉取平台所有推广链接列表以及充值模板列表，入库，且刷新内存字典
     *
     * @param force 是否强制同步（忽略防抖频控）
     */
    public synchronized void syncPromotionsAndTemplates(boolean force) {
        long now = System.currentTimeMillis();
        // 防抖频控：除非强制同步，否则 30 秒内仅允许同步一次
        if (!force && (now - lastSyncTimeMs < 30_000)) {
            log.info("[FlicknovelSync] Skip syncPromotionsAndTemplates due to rate limit (last synced {}ms ago)", now - lastSyncTimeMs);
            return;
        }

        log.info("[FlicknovelSync] Starting syncPromotionsAndTemplates (force={})...", force);
        try {
            // 1. 同步推广链接列表 (/open/promotion/query/v1)，同时收集所有 dist_app_id
            Set<Long> distAppIds = syncPromotions();
            if (apiClient.getDefaultDistAppId() != null) {
                distAppIds.add(apiClient.getDefaultDistAppId());
            }

            // 2. 根据所有出现过的 dist_app_id 分别拉取充值模板列表 (优先 v2，并同步 v1 兼容历史模板)
            for (Long appId : distAppIds) {
                if (appId != null) {
                    syncRechargeTemplatesV2(appId);
                    syncRechargeTemplatesV1(appId);
                }
            }

            // 3. 重新从本地库刷新组装内存缓存字典
            loadCacheFromDb();

            if (ltvPredictService != null) {
                ltvPredictService.clearVersionCache();
            }
            if (ltvStatService != null) {
                ltvStatService.clearResponseCache();
            }

            lastSyncTimeMs = System.currentTimeMillis();
            log.info("[FlicknovelSync] Successfully synced promotions and recharge templates. Cache size: {} promotions, {} templates",
                    promotionPriceTypeCache.size(), templatePriceTypeCache.size());
        } catch (Exception e) {
            log.error("[FlicknovelSync] Error in syncPromotionsAndTemplates: {}", e.getMessage(), e);
        }
    }

    /**
     * 同步短篇充值模板列表 (v2 接口) 并入库
     * 接口: /open/recharge_template/query/v2
     */
    private void syncRechargeTemplatesV2(Long distAppId) {
        String email = apiClient.getDefaultEmail();
        long page = 1L;
        long pageSize = 50L;

        while (true) {
            FlicknovelRechargeTemplateV2QueryRequest req = new FlicknovelRechargeTemplateV2QueryRequest(email, distAppId, page, pageSize);
            JsonNode resp;
            try {
                resp = apiClient.getRechargeTemplateV2List(req);
            } catch (Exception e) {
                log.error("[FlicknovelSync] Error requesting recharge template v2 for appId {} page {}: {}", distAppId, page, e.getMessage());
                break;
            }

            if (resp == null || resp.path("code").asInt(-1) != 0 || !resp.has("data")) {
                log.warn("[FlicknovelSync] Failed or empty response querying templates v2 for appId {} page {}: {}", distAppId, page, resp);
                break;
            }

            JsonNode templatesArr = resp.path("data").path("recharge_templates");
            if (!templatesArr.isArray() || templatesArr.isEmpty()) {
                break;
            }

            for (JsonNode tplNode : templatesArr) {
                String tplId = tplNode.path("recharge_template_id").asText(null);
                if (tplId == null || tplId.trim().isEmpty()) continue;

                String name = tplNode.path("name").asText("");
                TemplatePriceDetail detailObj = parsePriceTypeDetail(tplNode);
                Map<Integer, Integer> priceMap = detailObj.getPriceMap();

                FlicknovelRechargeTemplate entity = flicknovelRechargeTemplateRepository.findByTemplateId(tplId)
                        .orElseGet(FlicknovelRechargeTemplate::new);
                entity.setTemplateId(tplId);
                entity.setName(name);
                entity.setDistAppId(distAppId);
                try {
                    entity.setPriceConfigJson(objectMapper.writeValueAsString(priceMap));
                    entity.setRawPayload(objectMapper.writeValueAsString(tplNode));
                } catch (Exception ignored) {}
                flicknovelRechargeTemplateRepository.save(entity);

                templatePriceTypeCache.put(tplId, priceMap);
                templateDetailCache.put(tplId, detailObj);
            }

            if (templatesArr.size() < pageSize) {
                break;
            }
            page++;
        }
    }

    /**
     * 同步充值模板列表 (v1 接口，兼容未迁移到 v2 的历史模板如 '模版test')
     * 接口: /open/recharge_template/query/v1
     */
    private void syncRechargeTemplatesV1(Long distAppId) {
        String email = apiClient.getDefaultEmail();
        long page = 1L;
        long pageSize = 50L;

        while (true) {
            FlicknovelRechargeTemplateQueryRequest req = new FlicknovelRechargeTemplateQueryRequest(distAppId, email, page, pageSize);
            JsonNode resp;
            try {
                resp = apiClient.getRechargeTemplateV1Json(req);
            } catch (Exception e) {
                log.error("[FlicknovelSync] Error requesting recharge template v1 for appId {} page {}: {}", distAppId, page, e.getMessage());
                break;
            }

            if (resp == null || resp.path("code").asInt(-1) != 0 || !resp.has("data")) {
                break;
            }

            JsonNode templatesArr = resp.path("data").path("recharge_templates");
            if (!templatesArr.isArray() || templatesArr.isEmpty()) {
                break;
            }

            for (JsonNode tplNode : templatesArr) {
                String tplId = tplNode.path("recharge_template_id").asText(null);
                if (tplId == null || tplId.trim().isEmpty()) continue;

                // 若 v2 已包含且不为空，则优先保留 v2，否则以 v1 填充
                String name = tplNode.path("name").asText("");
                TemplatePriceDetail detailObj = parsePriceTypeDetail(tplNode);
                Map<Integer, Integer> priceMap = detailObj.getPriceMap();

                FlicknovelRechargeTemplate entity = flicknovelRechargeTemplateRepository.findByTemplateId(tplId)
                        .orElseGet(FlicknovelRechargeTemplate::new);
                entity.setTemplateId(tplId);
                entity.setName(name);
                entity.setDistAppId(distAppId);
                try {
                    entity.setPriceConfigJson(objectMapper.writeValueAsString(priceMap));
                    entity.setRawPayload(objectMapper.writeValueAsString(tplNode));
                } catch (Exception ignored) {}
                flicknovelRechargeTemplateRepository.save(entity);

                templatePriceTypeCache.putIfAbsent(tplId, priceMap);
                templateDetailCache.putIfAbsent(tplId, detailObj);
            }

            if (templatesArr.size() < pageSize) {
                break;
            }
            page++;
        }
    }

    /**
     * 同步推广链接列表并入库，返回所有涉及的 dist_app_id
     * 接口: /open/promotion/query/v1
     */
    private Set<Long> syncPromotions() {
        Set<Long> distAppIds = new HashSet<>();
        String email = apiClient.getDefaultEmail();
        long page = 1L;
        long pageSize = 50L;

        while (true) {
            FlicknovelPromotionQueryRequest req = new FlicknovelPromotionQueryRequest(email, page, pageSize);
            FlicknovelPromotionResponse resp;
            try {
                resp = apiClient.getPromotionList(req);
            } catch (Exception e) {
                log.error("[FlicknovelSync] Error requesting promotion list page {}: {}", page, e.getMessage());
                break;
            }

            if (resp == null || !resp.isSuccess() || resp.getData() == null) {
                log.warn("[FlicknovelSync] Failed or empty response querying promotions page {}: code={}, msg={}",
                        page, resp != null ? resp.getCode() : null, resp != null ? resp.getMessage() : null);
                break;
            }

            List<FlicknovelPromotionDto> promotions = resp.getData().getPromotions();
            if (promotions == null || promotions.isEmpty()) {
                break;
            }

            for (FlicknovelPromotionDto dto : promotions) {
                String pId = dto.getPromotionId();
                if (pId == null || pId.trim().isEmpty()) continue;

                if (dto.getDistAppId() != null) {
                    distAppIds.add(dto.getDistAppId());
                }

                FlicknovelPromotion entity = flicknovelPromotionRepository.findByPromotionId(pId)
                        .orElseGet(FlicknovelPromotion::new);
                entity.setPromotionId(pId);
                entity.setPromotionName(dto.getPromotionName());
                entity.setRechargeTplId(dto.getRechargeTplId());
                entity.setRechargeTplName(dto.getRechargeTplName());
                entity.setDistAppId(dto.getDistAppId());
                entity.setDramaId(dto.getDramaId());
                entity.setDramaTitle(dto.getDramaTitle());
                entity.setChapterId(dto.getChapterId());
                entity.setChapterTitle(dto.getChapterTitle());
                entity.setMediaChannel(dto.getMediaChannel());
                try {
                    entity.setRawPayload(objectMapper.writeValueAsString(dto));
                } catch (Exception ignored) {}
                flicknovelPromotionRepository.save(entity);

                if (dto.getRechargeTplId() != null) {
                    promotionTemplateMap.put(pId, dto.getRechargeTplId());
                }
            }

            if (promotions.size() < pageSize) {
                break;
            }
            page++;
        }
        return distAppIds;
    }

    /**
     * 解析单个充值模板 v2 报文，生成高级详情结构 (包含无歧义价格字典、首充字典、非首充字典、冲突价格集合、是否存在首购特惠)
     */
    public TemplatePriceDetail parsePriceTypeDetail(JsonNode tplNode) {
        Map<Integer, Integer> allPriceMap = new HashMap<>();
        Set<Integer> allCoinPrices = new HashSet<>();
        Set<Integer> allSubsPrices = new HashSet<>();

        Map<Integer, Integer> firstPriceMap = new HashMap<>();
        Set<Integer> firstCoinPrices = new HashSet<>();
        Set<Integer> firstSubsPrices = new HashSet<>();

        Map<Integer, Integer> noFirstPriceMap = new HashMap<>();
        Set<Integer> noFirstCoinPrices = new HashSet<>();
        Set<Integer> noFirstSubsPrices = new HashSet<>();

        boolean hasIntroOffer = false;

        JsonNode detail = tplNode.path("detail");
        if (!detail.isObject()) {
            return new TemplatePriceDetail(allPriceMap, Collections.emptySet(), false,
                    firstPriceMap, noFirstPriceMap, Collections.emptySet(), Collections.emptySet());
        }

        Iterator<Map.Entry<String, JsonNode>> platformFields = detail.fields();
        while (platformFields.hasNext()) {
            Map.Entry<String, JsonNode> pf = platformFields.next();
            JsonNode platformNode = pf.getValue();
            if (!platformNode.isObject()) continue;

            List<JsonNode> firstProductLists = new ArrayList<>();
            List<JsonNode> noFirstProductLists = new ArrayList<>();

            addArrayIfPresent(firstProductLists, platformNode.path("first_top_products"));
            addArrayIfPresent(firstProductLists, platformNode.path("first_products"));
            addArrayIfPresent(noFirstProductLists, platformNode.path("nofirst_top_products"));
            addArrayIfPresent(noFirstProductLists, platformNode.path("nofirst_products"));

            JsonNode rechargeNode = platformNode.path("recharge");
            if (rechargeNode.isObject()) {
                addArrayIfPresent(firstProductLists, rechargeNode.path("first_products"));
                addArrayIfPresent(noFirstProductLists, rechargeNode.path("nofirst_products"));
            }
            JsonNode subscribeNode = platformNode.path("subscribe");
            if (subscribeNode.isObject()) {
                addArrayIfPresent(firstProductLists, subscribeNode.path("first_products"));
                addArrayIfPresent(noFirstProductLists, subscribeNode.path("nofirst_products"));
            }

            // 1. 处理首充商品池 (first_products)
            for (JsonNode listNode : firstProductLists) {
                for (JsonNode item : listNode) {
                    JsonNode product = item.path("product");
                    int benefitType = product.path("benefit_type").asInt(item.path("benefit_type").asInt(0));
                    int priceCents = product.path("price_cents").asInt(item.path("price_cents").asInt(0));
                    int discountPriceCents = product.path("discount_price_cents").asInt(item.path("discount_price_cents").asInt(0));
                    int customPriceCents = item.path("custom_price_cents").asInt(0);

                    if (benefitType == 2) {
                        // 订阅产品
                        if (discountPriceCents > 0) {
                            firstSubsPrices.add(discountPriceCents);
                            firstPriceMap.put(discountPriceCents, 1);
                            allSubsPrices.add(discountPriceCents);
                            allPriceMap.put(discountPriceCents, 1);
                            if (priceCents > discountPriceCents) {
                                hasIntroOffer = true;
                            }
                        }
                        if (customPriceCents > 0) {
                            firstSubsPrices.add(customPriceCents);
                            firstPriceMap.put(customPriceCents, 1);
                            allSubsPrices.add(customPriceCents);
                            allPriceMap.put(customPriceCents, 1);
                        }
                        if (priceCents > 0) {
                            firstSubsPrices.add(priceCents);
                            if (!firstPriceMap.containsKey(priceCents)) {
                                firstPriceMap.put(priceCents, 1);
                            }
                            allSubsPrices.add(priceCents);
                            if (!allPriceMap.containsKey(priceCents)) {
                                allPriceMap.put(priceCents, 1);
                            }
                        }
                    } else if (benefitType == 1) {
                        // 代币单充
                        if (discountPriceCents > 0) {
                            firstCoinPrices.add(discountPriceCents);
                            if (!firstPriceMap.containsKey(discountPriceCents)) {
                                firstPriceMap.put(discountPriceCents, 0);
                            }
                            allCoinPrices.add(discountPriceCents);
                            if (!allPriceMap.containsKey(discountPriceCents)) {
                                allPriceMap.put(discountPriceCents, 0);
                            }
                        }
                        if (customPriceCents > 0) {
                            firstCoinPrices.add(customPriceCents);
                            if (!firstPriceMap.containsKey(customPriceCents)) {
                                firstPriceMap.put(customPriceCents, 0);
                            }
                            allCoinPrices.add(customPriceCents);
                            if (!allPriceMap.containsKey(customPriceCents)) {
                                allPriceMap.put(customPriceCents, 0);
                            }
                        }
                        if (priceCents > 0) {
                            firstCoinPrices.add(priceCents);
                            if (!firstPriceMap.containsKey(priceCents)) {
                                firstPriceMap.put(priceCents, 0);
                            }
                            allCoinPrices.add(priceCents);
                            if (!allPriceMap.containsKey(priceCents)) {
                                allPriceMap.put(priceCents, 0);
                            }
                        }
                    }
                }
            }

            // 2. 处理非首充商品池 (nofirst_products)
            for (JsonNode listNode : noFirstProductLists) {
                for (JsonNode item : listNode) {
                    JsonNode product = item.path("product");
                    int benefitType = product.path("benefit_type").asInt(item.path("benefit_type").asInt(0));
                    int priceCents = product.path("price_cents").asInt(item.path("price_cents").asInt(0));
                    int discountPriceCents = product.path("discount_price_cents").asInt(item.path("discount_price_cents").asInt(0));
                    int customPriceCents = item.path("custom_price_cents").asInt(0);

                    if (benefitType == 2) {
                        // 订阅产品
                        if (discountPriceCents > 0) {
                            noFirstSubsPrices.add(discountPriceCents);
                            noFirstPriceMap.put(discountPriceCents, 1);
                            allSubsPrices.add(discountPriceCents);
                            allPriceMap.put(discountPriceCents, 1);
                        }
                        if (customPriceCents > 0) {
                            noFirstSubsPrices.add(customPriceCents);
                            noFirstPriceMap.put(customPriceCents, 1);
                            allSubsPrices.add(customPriceCents);
                            allPriceMap.put(customPriceCents, 1);
                        }
                        if (priceCents > 0) {
                            noFirstSubsPrices.add(priceCents);
                            if (!noFirstPriceMap.containsKey(priceCents)) {
                                noFirstPriceMap.put(priceCents, 1);
                            }
                            allSubsPrices.add(priceCents);
                            if (!allPriceMap.containsKey(priceCents)) {
                                allPriceMap.put(priceCents, 1);
                            }
                        }
                    } else if (benefitType == 1) {
                        // 代币单充
                        if (discountPriceCents > 0) {
                            noFirstCoinPrices.add(discountPriceCents);
                            if (!noFirstPriceMap.containsKey(discountPriceCents)) {
                                noFirstPriceMap.put(discountPriceCents, 0);
                            }
                            allCoinPrices.add(discountPriceCents);
                            if (!allPriceMap.containsKey(discountPriceCents)) {
                                allPriceMap.put(discountPriceCents, 0);
                            }
                        }
                        if (customPriceCents > 0) {
                            noFirstCoinPrices.add(customPriceCents);
                            if (!noFirstPriceMap.containsKey(customPriceCents)) {
                                noFirstPriceMap.put(customPriceCents, 0);
                            }
                            allCoinPrices.add(customPriceCents);
                            if (!allPriceMap.containsKey(customPriceCents)) {
                                allPriceMap.put(customPriceCents, 0);
                            }
                        }
                        if (priceCents > 0) {
                            noFirstCoinPrices.add(priceCents);
                            if (!noFirstPriceMap.containsKey(priceCents)) {
                                noFirstPriceMap.put(priceCents, 0);
                            }
                            allCoinPrices.add(priceCents);
                            if (!allPriceMap.containsKey(priceCents)) {
                                allPriceMap.put(priceCents, 0);
                            }
                        }
                    }
                }
            }
        }

        Set<Integer> ambiguousPrices = new HashSet<>(allCoinPrices);
        ambiguousPrices.retainAll(allSubsPrices);

        Set<Integer> firstAmbiguousPrices = new HashSet<>(firstCoinPrices);
        firstAmbiguousPrices.retainAll(firstSubsPrices);

        Set<Integer> noFirstAmbiguousPrices = new HashSet<>(noFirstCoinPrices);
        noFirstAmbiguousPrices.retainAll(noFirstSubsPrices);

        return new TemplatePriceDetail(allPriceMap, ambiguousPrices, hasIntroOffer,
                firstPriceMap, noFirstPriceMap, firstAmbiguousPrices, noFirstAmbiguousPrices);
    }

    /**
     * 解析单个充值模板 v2 报文中的所有产品与价格，生成价格 -> 充值类型 (0=单充, 1=订阅) 映射字典
     */
    public Map<Integer, Integer> parsePriceTypeMap(JsonNode tplNode) {
        return parsePriceTypeDetail(tplNode).getPriceMap();
    }

    private void addArrayIfPresent(List<JsonNode> target, JsonNode node) {
        if (node != null && node.isArray() && !node.isEmpty()) {
            target.add(node);
        }
    }

    private void extractPriceAndSubs(JsonNode item, Map<Integer, Integer> priceMap) {
        JsonNode product = item.path("product");
        int benefitType = product.path("benefit_type").asInt(item.path("benefit_type").asInt(0));
        int isSubs = (benefitType == 2) ? 1 : 0;

        int priceCents = product.path("price_cents").asInt(item.path("price_cents").asInt(0));
        int discountPriceCents = product.path("discount_price_cents").asInt(item.path("discount_price_cents").asInt(0));
        int customPriceCents = item.path("custom_price_cents").asInt(0);

        // 1. 首购优惠价: 对于订阅，首购折后价 (如 1999, 2999, 4999) 具有最高确定性
        if (discountPriceCents > 0) {
            priceMap.put(discountPriceCents, isSubs);
        }
        if (customPriceCents > 0) {
            priceMap.put(customPriceCents, isSubs);
        }
        if (priceCents > 0) {
            // 如果尚不存在，或当前是订阅且已有项不是订阅，优先确认为订阅
            if (!priceMap.containsKey(priceCents) || isSubs == 1) {
                priceMap.put(priceCents, isSubs);
            }
        }
    }

    public Map<String, Map<Integer, Integer>> getPromotionPriceTypeCache() {
        return Collections.unmodifiableMap(promotionPriceTypeCache);
    }

    public Map<String, Map<Integer, Integer>> getTemplatePriceTypeCache() {
        return Collections.unmodifiableMap(templatePriceTypeCache);
    }
}

