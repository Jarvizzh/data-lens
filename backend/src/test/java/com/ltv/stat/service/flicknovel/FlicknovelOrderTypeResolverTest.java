package com.ltv.stat.service.flicknovel;

import com.ltv.stat.dto.flicknovel.FlicknovelOrderDto;
import org.junit.jupiter.api.BeforeEach;
import org.junit.jupiter.api.Test;

import java.time.LocalDateTime;
import java.util.*;

import static org.junit.jupiter.api.Assertions.*;

public class FlicknovelOrderTypeResolverTest {

    private FlicknovelOrderTypeResolver resolver;

    @BeforeEach
    public void setUp() {
        resolver = new FlicknovelOrderTypeResolver();
    }

    /**
     * 测试第一优先级: 零耦合 OpenAPI 显式字段透出
     * 当番茄 OpenAPI 未来返回 benefit_type 或 product_id 时，直接命中生效
     */
    @Test
    public void testPriority1_ExplicitOpenApiFields() {
        FlicknovelOrderTypeResolver.OrderResolveContext ctx = new FlicknovelOrderTypeResolver.OrderResolveContext();
        FlicknovelOrderDto dto = new FlicknovelOrderDto();
        ctx.setDto(dto);
        ctx.setOrderAmountCent(3999);

        // 1. benefit_type 显式指定
        dto.setBenefitType(2); // 订阅
        assertEquals(1, resolver.resolve(ctx), "benefit_type=2 必须直接判定为订阅 (1)");

        dto.setBenefitType(1); // 代币单充
        assertEquals(0, resolver.resolve(ctx), "benefit_type=1 必须直接判定为代币单充 (0)");

        // 2. product_id 显式指定
        dto.setBenefitType(null);
        dto.setProductId("novel.h5.sub.week.4000");
        assertEquals(1, resolver.resolve(ctx), "productId 包含 sub 必须直接判定为订阅 (1)");

        dto.setProductId("novel.h5.coins.4000");
        assertEquals(0, resolver.resolve(ctx), "productId 包含 coins 必须直接判定为代币单充 (0)");
    }

    /**
     * 测试第二优先级: 模板无歧义金额直接映射
     */
    @Test
    public void testPriority2_UnambiguousPrices() {
        FlicknovelOrderTypeResolver.OrderResolveContext ctx = new FlicknovelOrderTypeResolver.OrderResolveContext();
        ctx.setDto(new FlicknovelOrderDto());

        Map<Integer, Integer> priceMap = new HashMap<>();
        priceMap.put(1199, 0); // 代币单充
        priceMap.put(19899, 1); // 年卡订阅
        ctx.setTemplatePriceMap(priceMap);
        ctx.setAmbiguousPrices(Collections.singleton(3999)); // 3999 为冲突金额

        // 1199 -> 0
        ctx.setOrderAmountCent(1199);
        assertEquals(0, resolver.resolve(ctx), "1199 无歧义直接映射为单充 (0)");

        // 19899 -> 1
        ctx.setOrderAmountCent(19899);
        assertEquals(1, resolver.resolve(ctx), "19899 无歧义直接映射为订阅 (1)");
    }

    /**
     * 测试第三优先级: 冲突金额时序消歧 (以 39.99 为例)
     */
    @Test
    public void testPriority3_AmbiguousPrice_TemporalResolution() {
        FlicknovelOrderTypeResolver.OrderResolveContext ctx = new FlicknovelOrderTypeResolver.OrderResolveContext();
        ctx.setDto(new FlicknovelOrderDto());
        ctx.setOrderAmountCent(3999);
        ctx.setAmbiguousPrices(Collections.singleton(3999)); // 3999 属于冲突金额
        ctx.setTemplateHasIntroOffer(true); // 模板有首购优惠 (如 19.99)

        // 场景 A: 首单充值 39.99
        // 模板有 $19.99 首购特惠，用户支付了 $39.99 原价，说明没选订阅优惠，而是主动充值了代币
        ctx.setRenewType(1); // 首充
        ctx.setHasSubscribed(false);
        int resultFirstPay = resolver.resolve(ctx);
        assertEquals(0, resultFirstPay, "首单支付冲突原价 3999 且存在首充优惠时，必须判定为单充代币 (0)");

        // 场景 B: 老用户复充 39.99，且该用户在 7 天前曾订阅过
        LocalDateTime now = LocalDateTime.of(2026, 9, 18, 10, 0, 0);
        LocalDateTime sevenDaysAgo = now.minusDays(7); // 7天前购买首周

        ctx.setRenewType(2); // 复充
        ctx.setPayTimeBj(now);
        ctx.setHasSubscribed(true);
        ctx.setLatestSubsPayTime(sevenDaysAgo);
        int resultRenewal = resolver.resolve(ctx);
        assertEquals(1, resultRenewal, "距离上次订阅 7 天的复充 3999，必须判定为周订自动续费 (1)");

        // 场景 C: 老用户复充 39.99，但用户此前从无订阅记录
        ctx.setRenewType(2);
        ctx.setHasSubscribed(false);
        ctx.setLatestSubsPayTime(null);
        int resultRepeatCoins = resolver.resolve(ctx);
        assertEquals(0, resultRepeatCoins, "无订阅历史的复充 3999，必须判定为代币单充 (0)");
    }

    /**
     * 测试第二优先级: 首充专属字典与非首充专属字典独立路由
     * 针对同一金额在首充池和非首充池代表不同业务类型（如 29.99 在首充池为订阅原价，在非首充池为代币）
     */
    @Test
    public void testPriority2_DualPoolIndependentRouting() {
        FlicknovelOrderTypeResolver.OrderResolveContext ctx = new FlicknovelOrderTypeResolver.OrderResolveContext();
        ctx.setDto(new FlicknovelOrderDto());

        // 配置首充池: 2999 -> 订阅 (1)
        Map<Integer, Integer> firstMap = new HashMap<>();
        firstMap.put(999, 0); // 首充代币特惠
        firstMap.put(2999, 1); // 首充订阅
        ctx.setFirstPriceMap(firstMap);

        // 配置非首充池: 2999 -> 代币 (0), 3999 -> 订阅 (1)
        Map<Integer, Integer> noFirstMap = new HashMap<>();
        noFirstMap.put(2999, 0); // 复充代币
        noFirstMap.put(3999, 1); // 复充订阅
        ctx.setNoFirstPriceMap(noFirstMap);

        // 全局合并池 (兜底)
        Map<Integer, Integer> allMap = new HashMap<>();
        allMap.put(999, 0);
        allMap.put(3999, 1);
        ctx.setTemplatePriceMap(allMap);

        // 1. 首充订单金额 2999，必须命中 firstPriceMap -> 订阅 (1)
        ctx.setOrderAmountCent(2999);
        ctx.setRenewType(1);
        assertEquals(1, resolver.resolve(ctx), "首充订单 2999 必须路由至首充池判定为订阅 (1)");

        // 2. 复充订单金额 2999，必须命中 noFirstPriceMap -> 代币 (0)
        ctx.setRenewType(2);
        assertEquals(0, resolver.resolve(ctx), "复充订单 2999 必须路由至非首充池判定为代币 (0)");

        // 3. 首充池独有商品 999
        ctx.setOrderAmountCent(999);
        ctx.setRenewType(1);
        assertEquals(0, resolver.resolve(ctx), "首充 999 命中首充池代币 (0)");

        // 4. 复充订单金额 999 (非首充池未配置，优雅降级至全局合并池)
        ctx.setRenewType(2);
        assertEquals(0, resolver.resolve(ctx), "非首充池未配置时，降级全局池命中 999 -> 代币 (0)");

        // 5. 边缘情况: renewType 未定义 (0 或 -1)，直接走全局合并池，不武断假定为非首充
        ctx.setRenewType(0);
        ctx.setOrderAmountCent(3999);
        assertEquals(1, resolver.resolve(ctx), "renewType 未知时直接走全局池命中 3999 -> 订阅 (1)");

        // 6. 降级消歧场景: 模板无首购优惠 (hasIntroOffer=false)，首充订单遇到冲突金额，但非首充池中明确配置了代币 (0)
        ctx.setRenewType(1);
        ctx.setOrderAmountCent(2999);
        ctx.setFirstPriceMap(Collections.emptyMap()); // 模拟首充池缺失 2999
        ctx.setAmbiguousPrices(Collections.singleton(2999));
        ctx.setTemplateHasIntroOffer(false);
        assertEquals(0, resolver.resolve(ctx), "无首购优惠但非首充池明确为代币时，首单冲突金额应准确判为代币 (0)");
    }

    /**
     * 测试第四优先级: 从 2026-09-20 (UTC) 开始，模板价格字典找不到档位时，兜底策略为订阅 (1)；
     * 2026-09-20 (UTC) 之前未命中则兜底为单充 (0)。
     */
    @Test
    public void testPriority4_FallbackSubscriptionFromSept20() {
        FlicknovelOrderTypeResolver.OrderResolveContext ctx = new FlicknovelOrderTypeResolver.OrderResolveContext();
        ctx.setDto(new FlicknovelOrderDto());
        ctx.setTemplatePriceMap(Collections.emptyMap()); // 模板字典完全未配置该档位
        ctx.setOrderAmountCent(888); // 888 美分档位完全不存在

        // 场景 A: 2026-09-19 (UTC)，即 9.20 之前，未找到档位兜底为代币单充 (0)
        ctx.setPayTimeUtc(java.time.LocalDateTime.of(2026, 9, 19, 23, 59, 59));
        assertEquals(0, resolver.resolve(ctx), "9.20 UTC 之前未命中模板字典应兜底为代币单充 (0)");

        // 场景 B: 2026-09-20 (UTC)，刚好在 9.20 当天，未找到档位应兜底为订阅 (1)
        ctx.setPayTimeUtc(java.time.LocalDateTime.of(2026, 9, 20, 0, 0, 1));
        assertEquals(1, resolver.resolve(ctx), "9.20 UTC 当天未命中模板字典应兜底为订阅 (1)");

        // 场景 C: 2026-09-25 (UTC)，9.20 之后，未找到档位应兜底为订阅 (1)
        ctx.setPayTimeUtc(java.time.LocalDateTime.of(2026, 9, 25, 12, 0, 0));
        assertEquals(1, resolver.resolve(ctx), "9.25 UTC 未命中模板字典应兜底为订阅 (1)");

        // 场景 D: 仅提供北京时间 (例如 2026-09-20 07:00:00 BJ 对应 2026-09-19 23:00:00 UTC，仍属 9.19 UTC)
        ctx.setPayTimeUtc(null);
        ctx.setPayTimeBj(java.time.LocalDateTime.of(2026, 9, 20, 7, 0, 0));
        assertEquals(0, resolver.resolve(ctx), "北京时间 9.20 07:00 (UTC 9.19 23:00) 仍应兜底为单充 (0)");

        // 场景 E: 北京时间 2026-09-20 09:00:00 BJ 对应 2026-09-20 01:00:00 UTC
        ctx.setPayTimeUtc(null);
        ctx.setPayTimeBj(java.time.LocalDateTime.of(2026, 9, 20, 9, 0, 0));
        assertEquals(1, resolver.resolve(ctx), "北京时间 9.20 09:00 (UTC 9.20 01:00) 应兜底为订阅 (1)");
    }
}
