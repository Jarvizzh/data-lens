package com.ltv.stat.service.flicknovel;

import com.fasterxml.jackson.databind.ObjectMapper;
import com.ltv.stat.dto.flicknovel.*;
import org.junit.jupiter.api.Test;
import org.springframework.web.client.RestTemplate;

import java.time.Instant;

public class FlicknovelIntegrationTest {

    @Test
    public void testLiveApiCall() {
        RestTemplate restTemplate = new RestTemplate();
        ObjectMapper objectMapper = new ObjectMapper();

        // 直接构造 client，不需要启动整套 Spring Context
        FlicknovelApiClient client = new FlicknovelApiClient(restTemplate, objectMapper, null);

        System.out.println("=== Testing Flicknovel OpenAPI Live Connection ===");

        // 1. 测试获取订单列表接口
        long now = Instant.now().getEpochSecond();
        long beginTs = now - 7 * 86400; // 最近7天
        FlicknovelOrderQueryRequest orderReq = new FlicknovelOrderQueryRequest(beginTs, now, 1L, 100L);
        try {
            FlicknovelOrderResponse orderResp = client.getOrderList(orderReq);
            System.out.println("[Order API Response] code=" + (orderResp != null ? orderResp.getCode() : null)
                    + ", message=" + (orderResp != null ? orderResp.getMessage() : null)
                    + ", data.orders.size=" + (orderResp != null && orderResp.getData() != null && orderResp.getData().getOrders() != null ? orderResp.getData().getOrders().size() : 0));
        } catch (Exception e) {
            System.err.println("[Order API Error]: ");
            e.printStackTrace();
        }

        // 2. 测试获取推广链列表接口 (用测试邮箱或空邮箱)
        FlicknovelPromotionQueryRequest prmtReq = new FlicknovelPromotionQueryRequest("test@sinan-partner.com", 1L, 10L);
        try {
            FlicknovelPromotionResponse prmtResp = client.getPromotionList(prmtReq);
            System.out.println("[Promotion API Response] code=" + (prmtResp != null ? prmtResp.getCode() : null)
                    + ", message=" + (prmtResp != null ? prmtResp.getMessage() : null)
                    + ", data.promotions.size=" + (prmtResp != null && prmtResp.getData() != null && prmtResp.getData().getPromotions() != null ? prmtResp.getData().getPromotions().size() : 0));
        } catch (Exception e) {
            System.err.println("[Promotion API Error]: ");
            e.printStackTrace();
        }

        // 3. 测试获取充值模板接口
        FlicknovelRechargeTemplateQueryRequest rchgReq = new FlicknovelRechargeTemplateQueryRequest(984582L, "test@sinan-partner.com", 1L, 10L);
        try {
            FlicknovelRechargeTemplateResponse rchgResp = client.getRechargeTemplateList(rchgReq);
            System.out.println("[Recharge Template Response] code=" + (rchgResp != null ? rchgResp.getCode() : null)
                    + ", message=" + (rchgResp != null ? rchgResp.getMessage() : null)
                    + ", data.templates.size=" + (rchgResp != null && rchgResp.getData() != null && rchgResp.getData().getRechargeTemplates() != null ? rchgResp.getData().getRechargeTemplates().size() : 0));
        } catch (Exception e) {
            System.err.println("[Recharge Template Error]: ");
            e.printStackTrace();
        }
    }

    @Test
    public void testFetchAllOrders() {
        RestTemplate restTemplate = new RestTemplate();
        ObjectMapper objectMapper = new ObjectMapper();
        FlicknovelApiClient client = new FlicknovelApiClient(restTemplate, objectMapper, null);

        System.out.println("=================================================================");
        System.out.println("=== Starting Live Pull of ALL Orders from 番茄司南 (FlickNovel) ===");
        System.out.println("=================================================================");

        java.time.LocalDate startDate = java.time.LocalDate.of(2024, 1, 1);
        java.time.LocalDate endDate = java.time.LocalDate.now(java.time.ZoneOffset.UTC).plusDays(1);

        java.util.Map<String, FlicknovelOrderDto> allOrdersMap = new java.util.LinkedHashMap<>();
        int chunkCount = 0;
        int nonZeroChunks = 0;

        java.time.LocalDate currentStart = startDate;
        while (!currentStart.isAfter(endDate)) {
            java.time.LocalDate currentEnd = currentStart.plusDays(25);
            if (currentEnd.isAfter(endDate)) {
                currentEnd = endDate;
            }

            long beginTs = currentStart.atStartOfDay(java.time.ZoneOffset.UTC).toEpochSecond();
            long endTs = currentEnd.plusDays(1).atStartOfDay(java.time.ZoneOffset.UTC).toEpochSecond();
            chunkCount++;

            long page = 1L;
            long pageSize = 5000L;

            while (true) {
                FlicknovelOrderQueryRequest req = new FlicknovelOrderQueryRequest(beginTs, endTs, page, pageSize);
                try {
                    FlicknovelOrderResponse resp = client.getOrderList(req);
                    if (resp != null && resp.isSuccess() && resp.getData() != null && resp.getData().getOrders() != null) {
                        java.util.List<FlicknovelOrderDto> orders = resp.getData().getOrders();
                        if (!orders.isEmpty()) {
                            nonZeroChunks++;
                            for (FlicknovelOrderDto o : orders) {
                                if (o.getOrderId() != null && !o.getOrderId().trim().isEmpty()) {
                                    allOrdersMap.put(o.getOrderId().trim(), o);
                                }
                            }
                            System.out.printf("[Chunk %d] %s ~ %s (page %d): fetched %d orders (total unique so far: %d)%n",
                                    chunkCount, currentStart, currentEnd, page, orders.size(), allOrdersMap.size());
                        }
                        if (orders.size() < pageSize) {
                            break;
                        }
                        page++;
                    } else {
                        System.out.printf("[Chunk %d] %s ~ %s: API returned code=%s, msg=%s%n",
                                chunkCount, currentStart, currentEnd,
                                resp != null ? resp.getCode() : "null",
                                resp != null ? resp.getMessage() : "null");
                        break;
                    }
                } catch (Exception e) {
                    System.err.printf("[Chunk %d] %s ~ %s: Exception %s%n", chunkCount, currentStart, currentEnd, e.getMessage());
                    break;
                }
            }

            currentStart = currentEnd.plusDays(1);
        }

        System.out.println("\n=================================================================");
        System.out.println("=== 番茄司南 (FlickNovel) 全量订单统计汇总 ===");
        System.out.println("=================================================================");
        System.out.printf("扫描时间跨度: %s ~ %s (共 %d 个 25 天分段查询)%n", startDate, endDate, chunkCount);
        System.out.printf("含有订单的分段数: %d%n", nonZeroChunks);
        System.out.printf("全量订单总数 (去重后): %d 单%n", allOrdersMap.size());

        if (allOrdersMap.isEmpty()) {
            System.out.println("未查询到任何订单记录！");
            return;
        }

        java.math.BigDecimal totalUsd = java.math.BigDecimal.ZERO;
        java.util.Map<String, Integer> dateCountMap = new java.util.TreeMap<>();
        java.util.Map<String, java.math.BigDecimal> dateAmountMap = new java.util.TreeMap<>();
        java.util.Map<String, Integer> appCountMap = new java.util.HashMap<>();
        java.util.Map<String, Integer> channelCountMap = new java.util.HashMap<>();
        java.util.Map<String, Integer> benefitTypeMap = new java.util.HashMap<>();
        java.util.Map<String, Integer> rechargeTypeMap = new java.util.HashMap<>();

        long minTime = Long.MAX_VALUE;
        long maxTime = Long.MIN_VALUE;
        FlicknovelOrderDto earliestOrder = null;
        FlicknovelOrderDto latestOrder = null;

        for (FlicknovelOrderDto o : allOrdersMap.values()) {
            // 金额
            if (o.getUsPrice() != null) {
                try {
                    java.math.BigDecimal price = new java.math.BigDecimal(o.getUsPrice().trim());
                    totalUsd = totalUsd.add(price);
                } catch (Exception ignored) {}
            }

            // 时间戳解析
            long timeTs = 0L;
            if (o.getCompletedAt() != null) {
                try { timeTs = Long.parseLong(o.getCompletedAt().trim()); } catch (Exception ignored) {}
            }
            if (timeTs == 0L && o.getCreatedAt() != null) {
                try { timeTs = Long.parseLong(o.getCreatedAt().trim()); } catch (Exception ignored) {}
            }

            if (timeTs > 0) {
                if (timeTs < minTime) {
                    minTime = timeTs;
                    earliestOrder = o;
                }
                if (timeTs > maxTime) {
                    maxTime = timeTs;
                    latestOrder = o;
                }

                java.time.LocalDate d = java.time.Instant.ofEpochSecond(timeTs)
                        .atZone(java.time.ZoneId.of("Asia/Shanghai")).toLocalDate();
                String dStr = d.toString();
                dateCountMap.put(dStr, dateCountMap.getOrDefault(dStr, 0) + 1);

                if (o.getUsPrice() != null) {
                    try {
                        java.math.BigDecimal price = new java.math.BigDecimal(o.getUsPrice().trim());
                        dateAmountMap.put(dStr, dateAmountMap.getOrDefault(dStr, java.math.BigDecimal.ZERO).add(price));
                    } catch (Exception ignored) {}
                }
            }

            // 维度分布
            String app = o.getAppName() != null ? o.getAppName() : "UNKNOWN";
            appCountMap.put(app, appCountMap.getOrDefault(app, 0) + 1);

            String channel = o.getMediaChannel() != null ? o.getMediaChannel() : "UNKNOWN";
            channelCountMap.put(channel, channelCountMap.getOrDefault(channel, 0) + 1);

            String bType = o.getBenefitType() != null ? String.valueOf(o.getBenefitType()) : "null";
            benefitTypeMap.put(bType, benefitTypeMap.getOrDefault(bType, 0) + 1);

            String rType = o.getRechargeType() != null ? String.valueOf(o.getRechargeType()) : "null";
            rechargeTypeMap.put(rType, rechargeTypeMap.getOrDefault(rType, 0) + 1);
        }

        System.out.printf("订单总金额 (USD): $%.2f%n", totalUsd.doubleValue());
        if (earliestOrder != null) {
            System.out.printf("最早订单时间 (北京时间): %s (订单号: %s, 金额: $%s)%n",
                    java.time.Instant.ofEpochSecond(minTime).atZone(java.time.ZoneId.of("Asia/Shanghai")),
                    earliestOrder.getOrderId(), earliestOrder.getUsPrice());
        }
        if (latestOrder != null) {
            System.out.printf("最新订单时间 (北京时间): %s (订单号: %s, 金额: $%s)%n",
                    java.time.Instant.ofEpochSecond(maxTime).atZone(java.time.ZoneId.of("Asia/Shanghai")),
                    latestOrder.getOrderId(), latestOrder.getUsPrice());
        }

        System.out.println("\n--- 按应用分布 (App) ---");
        appCountMap.forEach((k, v) -> System.out.printf("  %s: %d 单%n", k, v));

        System.out.println("\n--- 按媒体渠道分布 (Media Channel) ---");
        channelCountMap.forEach((k, v) -> System.out.printf("  %s: %d 单%n", k, v));

        System.out.println("\n--- 按商品类型分布 (benefit_type: 1=代币充值, 2=时长订阅) ---");
        benefitTypeMap.forEach((k, v) -> System.out.printf("  benefit_type=%s: %d 单%n", k, v));

        System.out.println("\n--- 按充值类型分布 (recharge_type) ---");
        rechargeTypeMap.forEach((k, v) -> System.out.printf("  recharge_type=%s: %d 单%n", k, v));

        System.out.println("\n--- 按日期分布 (北京时间) ---");
        for (String d : dateCountMap.keySet()) {
            System.out.printf("  %s: %4d 单,  金额: $%8.2f%n", d, dateCountMap.get(d), dateAmountMap.getOrDefault(d, java.math.BigDecimal.ZERO).doubleValue());
        }

        // 另外统计 UTC 日期分布
        java.util.Map<String, Integer> utcDateCountMap = new java.util.TreeMap<>();
        java.util.Map<String, java.math.BigDecimal> utcDateAmountMap = new java.util.TreeMap<>();
        for (FlicknovelOrderDto o : allOrdersMap.values()) {
            long timeTs = 0L;
            if (o.getCompletedAt() != null) {
                try { timeTs = Long.parseLong(o.getCompletedAt().trim()); } catch (Exception ignored) {}
            }
            if (timeTs == 0L && o.getCreatedAt() != null) {
                try { timeTs = Long.parseLong(o.getCreatedAt().trim()); } catch (Exception ignored) {}
            }
            if (timeTs > 0) {
                java.time.LocalDate dUtc = java.time.Instant.ofEpochSecond(timeTs)
                        .atZone(java.time.ZoneOffset.UTC).toLocalDate();
                String dStr = dUtc.toString();
                utcDateCountMap.put(dStr, utcDateCountMap.getOrDefault(dStr, 0) + 1);
                if (o.getUsPrice() != null) {
                    try {
                        java.math.BigDecimal price = new java.math.BigDecimal(o.getUsPrice().trim());
                        utcDateAmountMap.put(dStr, utcDateAmountMap.getOrDefault(dStr, java.math.BigDecimal.ZERO).add(price));
                    } catch (Exception ignored) {}
                }
            }
        }
        System.out.println("\n--- 按日期分布 (UTC 时间) ---");
        for (String d : utcDateCountMap.keySet()) {
            System.out.printf("  %s: %4d 单,  金额: $%8.2f%n", d, utcDateCountMap.get(d), utcDateAmountMap.getOrDefault(d, java.math.BigDecimal.ZERO).doubleValue());
        }

        System.out.println("\n--- 检查最早那笔订单详情 (minTime) ---");
        if (earliestOrder != null) {
            System.out.printf("  orderId: %s%n  usPrice: %s%n  completedAt: %s%n  createdAt: %s%n  BJ Time: %s%n  UTC Time: %s%n  relationId: %s%n  promotionId: %s%n  mediaChannel: %s%n",
                    earliestOrder.getOrderId(), earliestOrder.getUsPrice(), earliestOrder.getCompletedAt(), earliestOrder.getCreatedAt(),
                    java.time.Instant.ofEpochSecond(minTime).atZone(java.time.ZoneId.of("Asia/Shanghai")),
                    java.time.Instant.ofEpochSecond(minTime).atZone(java.time.ZoneOffset.UTC),
                    earliestOrder.getRelationId(), earliestOrder.getPromotionId(), earliestOrder.getMediaChannel());
        }

        System.out.println("\n--- 抽样订单明细 (前 5 单) ---");
        int sampleIdx = 0;
        for (FlicknovelOrderDto o : allOrdersMap.values()) {
            if (sampleIdx++ >= 5) break;
            System.out.printf("  [%d] orderId=%s, deviceId=%s, usPrice=$%s, benefitType=%s, rechargeType=%s, completedAt=%s, adAccountId=%s, promotionId=%s%n",
                    sampleIdx, o.getOrderId(), o.getDeviceId(), o.getUsPrice(), o.getBenefitType(), o.getRechargeType(), o.getCompletedAt(), o.getAdAccountId(), o.getPromotionId());
        }
        System.out.println("=================================================================");
    }

    @Test
    public void testSubscriptionUsersOn925And926() {
        RestTemplate restTemplate = new RestTemplate();
        ObjectMapper objectMapper = new ObjectMapper();
        FlicknovelApiClient client = new FlicknovelApiClient(restTemplate, objectMapper, null);
        FlicknovelOrderTypeResolver resolver = new FlicknovelOrderTypeResolver();
        FlicknovelApiService apiService = new FlicknovelApiService(client, null, null, null, null, null, resolver, objectMapper);

        System.out.println("=================================================================");
        System.out.println("=== 正在拉取番茄司南 推广链接与充值模板以构建规则字典 ===");
        System.out.println("=================================================================");

        // 1. 获取推广链接
        java.util.Map<String, String> promotionToTemplateMap = new java.util.HashMap<>();
        try {
            FlicknovelPromotionResponse prmtResp = client.getPromotionList(new FlicknovelPromotionQueryRequest("charles_z0@163.com", 1L, 100L));
            if (prmtResp != null && prmtResp.getData() != null && prmtResp.getData().getPromotions() != null) {
                for (FlicknovelPromotionDto p : prmtResp.getData().getPromotions()) {
                    if (p.getPromotionId() != null && p.getRechargeTplId() != null) {
                        promotionToTemplateMap.put(p.getPromotionId().trim(), p.getRechargeTplId().trim());
                    }
                }
            }
        } catch (Exception e) {
            System.err.println("拉取推广链接失败: " + e.getMessage());
        }
        System.out.println("已获取推广链接数: " + promotionToTemplateMap.size());

        // 2. 获取充值模板
        java.util.Map<String, FlicknovelApiService.TemplatePriceDetail> templateDetailMap = new java.util.HashMap<>();
        try {
            com.fasterxml.jackson.databind.JsonNode v2Json = client.getRechargeTemplateV2List(new FlicknovelRechargeTemplateV2QueryRequest("charles_z0@163.com", 2000019L, 1L, 100L));
            if (v2Json != null && v2Json.has("data") && v2Json.path("data").has("recharge_templates")) {
                for (com.fasterxml.jackson.databind.JsonNode tNode : v2Json.path("data").path("recharge_templates")) {
                    String tplId = tNode.path("recharge_template_id").asText(null);
                    if (tplId != null) {
                        FlicknovelApiService.TemplatePriceDetail detail = apiService.parsePriceTypeDetail(tNode);
                        templateDetailMap.put(tplId.trim(), detail);
                        System.out.printf("  模板 [%s] %s: 价格字典=%s, 首购优惠=%s, 冲突价=%s%n",
                                tplId, tNode.path("name").asText(), detail.getPriceMap(), detail.isHasIntroOffer(), detail.getAmbiguousPrices());
                    }
                }
            }
        } catch (Exception e) {
            System.err.println("拉取充值模板失败: " + e.getMessage());
        }

        // 3. 拉取最近订单 (2026-09-16 到 2026-09-27)
        long beginTs = java.time.LocalDate.of(2026, 9, 16).atStartOfDay(java.time.ZoneOffset.UTC).toEpochSecond();
        long endTs = java.time.LocalDate.of(2026, 9, 27).atStartOfDay(java.time.ZoneOffset.UTC).toEpochSecond();

        java.util.Map<String, FlicknovelOrderDto> allOrdersMap = new java.util.LinkedHashMap<>();
        long page = 1L;
        while (true) {
            FlicknovelOrderQueryRequest req = new FlicknovelOrderQueryRequest(beginTs, endTs, page, 5000L);
            FlicknovelOrderResponse resp = client.getOrderList(req);
            if (resp != null && resp.isSuccess() && resp.getData() != null && resp.getData().getOrders() != null) {
                for (FlicknovelOrderDto o : resp.getData().getOrders()) {
                    if (o.getOrderId() != null) allOrdersMap.put(o.getOrderId().trim(), o);
                }
                if (resp.getData().getOrders().size() < 5000) break;
                page++;
            } else {
                break;
            }
        }
        System.out.println("\n已获取订单总数: " + allOrdersMap.size());

        // 4. 按用户提取 memberId，并按时间升序排序清洗
        java.util.Map<String, java.util.List<FlicknovelOrderDto>> userOrdersMap = new java.util.HashMap<>();
        for (FlicknovelOrderDto o : allOrdersMap.values()) {
            String mId = o.getDeviceId() != null && !o.getDeviceId().trim().isEmpty() ? o.getDeviceId().trim()
                    : (o.getRelationId() != null && !o.getRelationId().trim().isEmpty() ? o.getRelationId().trim() : o.getOrderId().trim());
            userOrdersMap.computeIfAbsent(mId, k -> new java.util.ArrayList<>()).add(o);
        }

        // 5. 逐用户、逐订单判定 isSubs
        class ResolvedOrder {
            FlicknovelOrderDto dto;
            String memberId;
            int orderAmountCent;
            int renewType;
            java.time.LocalDateTime payTimeBj;
            java.time.LocalDateTime payTimeUtc;
            int isSubs;
            String reason;
        }

        java.util.List<ResolvedOrder> allResolved = new java.util.ArrayList<>();

        for (java.util.Map.Entry<String, java.util.List<FlicknovelOrderDto>> entry : userOrdersMap.entrySet()) {
            String mId = entry.getKey();
            java.util.List<FlicknovelOrderDto> list = entry.getValue();
            list.sort(java.util.Comparator.comparingLong(o -> {
                long t = 0L;
                if (o.getCompletedAt() != null) try { t = Long.parseLong(o.getCompletedAt().trim()); } catch (Exception ignored) {}
                if (t == 0L && o.getCreatedAt() != null) try { t = Long.parseLong(o.getCreatedAt().trim()); } catch (Exception ignored) {}
                return t;
            }));

            boolean userHasSubscribed = false;
            java.time.LocalDateTime latestSubsTime = null;
            java.time.LocalDateTime earliestPayTime = null;

            for (FlicknovelOrderDto dto : list) {
                long timeTs = 0L;
                if (dto.getCompletedAt() != null) try { timeTs = Long.parseLong(dto.getCompletedAt().trim()); } catch (Exception ignored) {}
                if (timeTs == 0L && dto.getCreatedAt() != null) try { timeTs = Long.parseLong(dto.getCreatedAt().trim()); } catch (Exception ignored) {}

                java.time.LocalDateTime payBj = java.time.Instant.ofEpochSecond(timeTs).atZone(java.time.ZoneId.of("Asia/Shanghai")).toLocalDateTime();
                java.time.LocalDateTime payUtc = java.time.Instant.ofEpochSecond(timeTs).atZone(java.time.ZoneOffset.UTC).toLocalDateTime();

                int renewType;
                if (earliestPayTime == null) {
                    renewType = 1;
                    earliestPayTime = payBj;
                } else {
                    renewType = payBj.isAfter(earliestPayTime) ? 2 : 1;
                }

                int amountCent = 0;
                if (dto.getUsPrice() != null) {
                    try {
                        amountCent = new java.math.BigDecimal(dto.getUsPrice().trim()).multiply(java.math.BigDecimal.valueOf(100)).intValue();
                    } catch (Exception ignored) {}
                }

                String promotionId = dto.getPromotionId() != null ? dto.getPromotionId().trim() : "";
                String tplId = promotionToTemplateMap.get(promotionId);
                FlicknovelApiService.TemplatePriceDetail detail = tplId != null ? templateDetailMap.get(tplId) : null;

                FlicknovelOrderTypeResolver.OrderResolveContext ctx = new FlicknovelOrderTypeResolver.OrderResolveContext();
                ctx.setDto(dto);
                ctx.setPromotionId(promotionId);
                ctx.setOrderAmountCent(amountCent);
                ctx.setRenewType(renewType);
                ctx.setPayTimeBj(payBj);
                ctx.setHasSubscribed(userHasSubscribed);
                ctx.setLatestSubsPayTime(latestSubsTime);

                if (detail != null) {
                    ctx.setTemplatePriceMap(detail.getPriceMap());
                    ctx.setAmbiguousPrices(detail.getAmbiguousPrices());
                    ctx.setTemplateHasIntroOffer(detail.isHasIntroOffer());
                    ctx.setFirstPriceMap(detail.getFirstPriceMap());
                    ctx.setNoFirstPriceMap(detail.getNoFirstPriceMap());
                    ctx.setFirstAmbiguousPrices(detail.getFirstAmbiguousPrices());
                    ctx.setNoFirstAmbiguousPrices(detail.getNoFirstAmbiguousPrices());
                }

                int isSubs = resolver.resolve(ctx);
                if (isSubs == 1) {
                    userHasSubscribed = true;
                    latestSubsTime = payBj;
                }

                ResolvedOrder ro = new ResolvedOrder();
                ro.dto = dto;
                ro.memberId = mId;
                ro.orderAmountCent = amountCent;
                ro.renewType = renewType;
                ro.payTimeBj = payBj;
                ro.payTimeUtc = payUtc;
                ro.isSubs = isSubs;
                ro.reason = String.format("tpl=%s, amt=%d, renew=%d, intro=%s",
                        tplId, amountCent, renewType, detail != null ? detail.isHasIntroOffer() : "none");
                allResolved.add(ro);
            }
        }

        // 6. 预处理各用户的首单日期 (UTC & BJ)
        java.util.Map<String, java.time.LocalDate> userCohortBjMap = new java.util.HashMap<>();
        java.util.Map<String, java.time.LocalDate> userCohortUtcMap = new java.util.HashMap<>();
        java.util.Set<String> allSubUsersTotal = new java.util.HashSet<>();

        for (ResolvedOrder ro : allResolved) {
            userCohortBjMap.putIfAbsent(ro.memberId, ro.payTimeBj.toLocalDate());
            userCohortUtcMap.putIfAbsent(ro.memberId, ro.payTimeUtc.toLocalDate());
            if (ro.isSubs == 1) {
                allSubUsersTotal.add(ro.memberId);
            }
        }

        // 7. 统计 9.25 和 9.26 的订阅与充值用户数 (北京时间)
        System.out.println("\n=================================================================");
        System.out.println("=== 9.25 和 9.26 充值与订阅用户详细统计 (按北京时间支付日期) ===");
        System.out.println("=================================================================");
        for (String targetDateStr : new String[]{"2026-09-25", "2026-09-26"}) {
            java.time.LocalDate targetDate = java.time.LocalDate.parse(targetDateStr);
            java.util.List<ResolvedOrder> dayOrders = allResolved.stream()
                    .filter(ro -> ro.payTimeBj.toLocalDate().equals(targetDate))
                    .collect(java.util.stream.Collectors.toList());

            java.util.List<ResolvedOrder> subOrders = dayOrders.stream()
                    .filter(ro -> ro.isSubs == 1)
                    .collect(java.util.stream.Collectors.toList());

            java.util.Set<String> allUsers = dayOrders.stream().map(ro -> ro.memberId).collect(java.util.stream.Collectors.toSet());
            java.util.Set<String> subUsers = subOrders.stream().map(ro -> ro.memberId).collect(java.util.stream.Collectors.toSet());

            long firstRechargeUserCount = allUsers.stream()
                    .filter(u -> targetDate.equals(userCohortBjMap.get(u)))
                    .count();
            long repeatRechargeUserCount = allUsers.size() - firstRechargeUserCount;

            java.math.BigDecimal totalAmt = dayOrders.stream()
                    .map(ro -> ro.dto.getUsPrice() != null ? new java.math.BigDecimal(ro.dto.getUsPrice().trim()) : java.math.BigDecimal.ZERO)
                    .reduce(java.math.BigDecimal.ZERO, java.math.BigDecimal::add);

            System.out.printf("[%s (北京时间)]%n", targetDateStr);
            System.out.printf("  总订单数: %d 单,  总充值金额: $%.2f%n", dayOrders.size(), totalAmt.doubleValue());
            System.out.printf("  【充值总人数 (去重)】: %d 人%n", allUsers.size());
            System.out.printf("    - 当日首充新客: %d 人%n", firstRechargeUserCount);
            System.out.printf("    - 历史老客复充: %d 人%n", repeatRechargeUserCount);
            System.out.printf("  【订阅总人数 (去重)】: %d 人 (占比: %.1f%%)%n", subUsers.size(), (subUsers.size() * 100.0 / allUsers.size()));
            System.out.printf("  【纯代币单充人数】: %d 人%n%n", (allUsers.size() - subUsers.size()));
        }

        System.out.println("=================================================================");
        System.out.println("=== 9.25 和 9.26 充值与订阅用户详细统计 (按 UTC 时间支付日期) ===");
        System.out.println("=================================================================");
        for (String targetDateStr : new String[]{"2026-09-25", "2026-09-26"}) {
            java.time.LocalDate targetDate = java.time.LocalDate.parse(targetDateStr);
            java.util.List<ResolvedOrder> dayOrders = allResolved.stream()
                    .filter(ro -> ro.payTimeUtc.toLocalDate().equals(targetDate))
                    .collect(java.util.stream.Collectors.toList());

            java.util.List<ResolvedOrder> subOrders = dayOrders.stream()
                    .filter(ro -> ro.isSubs == 1)
                    .collect(java.util.stream.Collectors.toList());

            java.util.Set<String> allUsers = dayOrders.stream().map(ro -> ro.memberId).collect(java.util.stream.Collectors.toSet());
            java.util.Set<String> subUsers = subOrders.stream().map(ro -> ro.memberId).collect(java.util.stream.Collectors.toSet());

            long firstRechargeUserCount = allUsers.stream()
                    .filter(u -> targetDate.equals(userCohortUtcMap.get(u)))
                    .count();
            long repeatRechargeUserCount = allUsers.size() - firstRechargeUserCount;

            java.math.BigDecimal totalAmt = dayOrders.stream()
                    .map(ro -> ro.dto.getUsPrice() != null ? new java.math.BigDecimal(ro.dto.getUsPrice().trim()) : java.math.BigDecimal.ZERO)
                    .reduce(java.math.BigDecimal.ZERO, java.math.BigDecimal::add);

            System.out.printf("[%s (UTC 时间)]%n", targetDateStr);
            System.out.printf("  总订单数: %d 单,  总充值金额: $%.2f%n", dayOrders.size(), totalAmt.doubleValue());
            System.out.printf("  【充值总人数 (去重)】: %d 人%n", allUsers.size());
            System.out.printf("    - 当日首充新客: %d 人%n", firstRechargeUserCount);
            System.out.printf("    - 历史老客复充: %d 人%n", repeatRechargeUserCount);
            System.out.printf("  【订阅总人数 (去重)】: %d 人 (占比: %.1f%%)%n", subUsers.size(), (subUsers.size() * 100.0 / allUsers.size()));
            System.out.printf("  【纯代币单充人数】: %d 人%n%n", (allUsers.size() - subUsers.size()));
        }

        // 8. 统计 Cohort 注册日期为 9.25 和 9.26 的订阅用户数 (LTV 报表维度)
        System.out.println("\n=================================================================");
        System.out.println("=== 9.25 和 9.26 Cohort 维度订阅用户数 (按首充注册日期) ===");
        System.out.println("=================================================================");

        for (String targetDateStr : new String[]{"2026-09-25", "2026-09-26"}) {
            java.time.LocalDate targetDate = java.time.LocalDate.parse(targetDateStr);

            long subCountBj = allSubUsersTotal.stream()
                    .filter(u -> targetDate.equals(userCohortBjMap.get(u)))
                    .count();
            long totalCohortUsersBj = userCohortBjMap.values().stream().filter(targetDate::equals).count();

            long subCountUtc = allSubUsersTotal.stream()
                    .filter(u -> targetDate.equals(userCohortUtcMap.get(u)))
                    .count();
            long totalCohortUsersUtc = userCohortUtcMap.values().stream().filter(targetDate::equals).count();

            System.out.printf("[%s Cohort 维度]%n", targetDateStr);
            System.out.printf("  北京时间 Cohort: 新增用户数=%d, 其中订阅用户数=%d%n", totalCohortUsersBj, subCountBj);
            System.out.printf("  UTC 时间 Cohort: 新增用户数=%d, 其中订阅用户数=%d%n", totalCohortUsersUtc, subCountUtc);
        }
        System.out.println("=================================================================");

        // 9. 专项提取 9.25 (UTC) 新增首充用户中，未订阅的 6 个用户的全部订单明细
        System.out.println("\n=================================================================");
        System.out.println("=== 9.25 (UTC) 新增首充用户中【6 个非订阅用户】订单明细 ===");
        System.out.println("=================================================================");
        java.time.LocalDate utc925 = java.time.LocalDate.of(2026, 9, 25);
        java.util.Set<String> nonSubUsers925 = userCohortUtcMap.entrySet().stream()
                .filter(e -> utc925.equals(e.getValue()))
                .map(java.util.Map.Entry::getKey)
                .filter(u -> !allSubUsersTotal.contains(u))
                .collect(java.util.stream.Collectors.toSet());

        System.out.printf("9.25 (UTC) 新增未订阅用户数: %d 人%n%n", nonSubUsers925.size());
        int userIdx = 0;
        for (String uId : nonSubUsers925) {
            userIdx++;
            java.util.List<ResolvedOrder> uOrders = allResolved.stream()
                    .filter(ro -> uId.equals(ro.memberId))
                    .collect(java.util.stream.Collectors.toList());

            System.out.printf("【第 %d 位未订阅用户】 memberId/deviceId: %s%n", userIdx, uId);
            System.out.printf("  该用户在系统中的历史全部订单数: %d 单%n", uOrders.size());
            for (ResolvedOrder ro : uOrders) {
                String promotionId = ro.dto.getPromotionId() != null ? ro.dto.getPromotionId().trim() : "";
                String tplId = promotionToTemplateMap.get(promotionId);
                FlicknovelApiService.TemplatePriceDetail detail = tplId != null ? templateDetailMap.get(tplId) : null;

                System.out.printf("  -> 订单号: %s%n", ro.dto.getOrderId());
                System.out.printf("     金额: $%s (%d 美分)%n", ro.dto.getUsPrice(), ro.orderAmountCent);
                System.out.printf("     支付时间 (UTC): %s%n", ro.payTimeUtc);
                System.out.printf("     支付时间 (北京时间): %s%n", ro.payTimeBj);
                System.out.printf("     推广链接 ID: %s%n", promotionId);
                System.out.printf("     关联充值模板 ID: %s%n", tplId);
                if (detail != null) {
                    System.out.printf("     模板价格字典: %s%n", detail.getPriceMap());
                    System.out.printf("     模板首充字典 (firstPriceMap): %s%n", detail.getFirstPriceMap());
                    System.out.printf("     模板非首充字典 (noFirstPriceMap): %s%n", detail.getNoFirstPriceMap());
                    System.out.printf("     模板冲突价集合: %s, 是否含首购优惠: %s%n", detail.getAmbiguousPrices(), detail.isHasIntroOffer());
                } else {
                    System.out.println("     （未获取到该推广链接的模板详情）");
                }
                System.out.printf("     OpenAPI 返回的 benefit_type: %s, product_id: %s%n", ro.dto.getBenefitType(), ro.dto.getProductId());
                System.out.printf("     系统判定结果: isSubs = %d (理由: %s)%n%n", ro.isSubs, ro.reason);
            }
        }
        System.out.println("=================================================================");
    }
}
