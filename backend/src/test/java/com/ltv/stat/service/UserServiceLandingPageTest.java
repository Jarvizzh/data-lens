package com.ltv.stat.service;

import com.ltv.stat.dto.LandingPageConfigItem;
import com.ltv.stat.entity.RawOrder;
import com.ltv.stat.entity.SysUser;
import com.ltv.stat.repository.RawOrderRepository;
import com.ltv.stat.repository.SysUserRepository;
import com.ltv.stat.repository.UserLandingPageRepository;
import org.junit.jupiter.api.AfterEach;
import org.junit.jupiter.api.BeforeEach;
import org.junit.jupiter.api.Test;
import org.springframework.beans.factory.annotation.Autowired;
import org.springframework.boot.test.context.SpringBootTest;
import org.springframework.transaction.annotation.Transactional;

import java.math.BigDecimal;
import java.time.LocalDateTime;
import java.util.Collections;
import java.util.List;

import static org.junit.jupiter.api.Assertions.*;

@SpringBootTest
@Transactional
public class UserServiceLandingPageTest {

    @Autowired
    private UserService userService;

    @Autowired
    private SysUserRepository sysUserRepository;

    @Autowired
    private UserLandingPageRepository userLandingPageRepository;

    @Autowired
    private RawOrderRepository rawOrderRepository;

    private Long testUserId;
    private Long testAdminId;
    private Long testOrderId;

    @BeforeEach
    public void setUp() {
        // 创建一个测试普通用户 (USER)
        SysUser user = new SysUser();
        user.setUsername("test_user_" + System.currentTimeMillis());
        user.setPasswordHash(UserService.hashPassword("123456"));
        user.setRole("USER");
        user.setStatus(1);
        user = sysUserRepository.save(user);
        testUserId = user.getId();

        // 创建一个测试管理员用户 (ADMIN)
        SysUser admin = new SysUser();
        admin.setUsername("test_admin_" + System.currentTimeMillis());
        admin.setPasswordHash(UserService.hashPassword("123456"));
        admin.setRole("ADMIN");
        admin.setStatus(1);
        admin = sysUserRepository.save(admin);
        testAdminId = admin.getId();

        // 插入一条 flicknovel 的测试 RawOrder
        RawOrder order = new RawOrder();
        order.setPlatformCode("flicknovel");
        order.setOrderId("test_order_" + System.currentTimeMillis());
        order.setLandingPageId("TEST_PROMO_999");
        order.setMemberId("mem_999");
        order.setOrderAmountCent(999);
        order.setOrderAmountUsd(new BigDecimal("9.99"));
        order.setRegisterTimeBj(LocalDateTime.now());
        order.setRegisterTimeEt(LocalDateTime.now().minusHours(12));
        order.setRegisterDateEt(java.time.LocalDate.now());
        order.setPayTimeBj(LocalDateTime.now());
        order.setPayTimeEt(LocalDateTime.now().minusHours(12));
        order.setPayDateEt(java.time.LocalDate.now());
        order = rawOrderRepository.save(order);
        testOrderId = order.getId();
    }

    @AfterEach
    public void tearDown() {
        if (testUserId != null) {
            userLandingPageRepository.deleteByPlatformCodeAndUserId("flicknovel", testUserId);
            userLandingPageRepository.deleteByPlatformCodeAndUserId("rocnovel", testUserId);
            sysUserRepository.deleteById(testUserId);
        }
        if (testAdminId != null) {
            userLandingPageRepository.deleteByPlatformCodeAndUserId("flicknovel", testAdminId);
            userLandingPageRepository.deleteByPlatformCodeAndUserId("rocnovel", testAdminId);
            sysUserRepository.deleteById(testAdminId);
        }
        if (testOrderId != null) {
            rawOrderRepository.deleteById(testOrderId);
        }
    }

    @Test
    public void testFlicknovelInitialDefaultAllLandingPages() {
        // 1. 普通用户 (USER) 初始状态：未配置 flicknovel 时，默认为空列表
        List<LandingPageConfigItem> userInitial = userService.getUserLandingPageConfigs("flicknovel", testUserId);
        assertNotNull(userInitial);
        assertTrue(userInitial.isEmpty(), "普通用户初始落地页应默认为空");

        // 2. 管理员 (ADMIN) 初始状态：未配置 flicknovel 时，默认填入所有推广ID，番茄司南默认时区为 UTC
        List<LandingPageConfigItem> adminInitial = userService.getUserLandingPageConfigs("flicknovel", testAdminId);
        assertNotNull(adminInitial);
        assertFalse(adminInitial.isEmpty(), "管理员初始配置应默认返回所有推广ID");
        assertTrue(adminInitial.stream().anyMatch(c -> "TEST_PROMO_999".equals(c.getLandingPageId())), "应包含测试推广ID TEST_PROMO_999");
        assertTrue(adminInitial.stream().allMatch(c -> "UTC".equals(c.getTimezone())), "番茄司南初始配置时区应默认为 UTC");

        // 3. 模拟管理员手动保存（指定 ET 时区与 UTC 时区）
        LandingPageConfigItem retained = new LandingPageConfigItem("flicknovel", "TEST_PROMO_999", "ET");
        userService.updateUserLandingPageConfigs("flicknovel", testAdminId, Collections.singletonList(retained));

        // 验证保存后仅保留选定的项，且更新为用户指定的时区
        List<LandingPageConfigItem> updatedConfigs = userService.getUserLandingPageConfigs("flicknovel", testAdminId);
        assertEquals(1, updatedConfigs.size());
        assertEquals("TEST_PROMO_999", updatedConfigs.get(0).getLandingPageId());
        assertEquals("ET", updatedConfigs.get(0).getTimezone());

        // 4. 验证平台隔离与 CST / 历史 BJ 兼容：给用户配置 rocnovel 落地页（传入 BJ 自动映射为 CST）
        LandingPageConfigItem rocItem = new LandingPageConfigItem("rocnovel", "ROC_PAGE_1", "BJ");
        userService.updateUserLandingPageConfigs("rocnovel", testAdminId, Collections.singletonList(rocItem));

        // 重新更新 flicknovel 为清空（模拟手动全删）
        userService.updateUserLandingPageConfigs("flicknovel", testAdminId, Collections.emptyList());

        // 验证 flicknovel 此时返回空列表，不会再次强制塞满全量
        List<LandingPageConfigItem> flickAfterEmpty = userService.getUserLandingPageConfigs("flicknovel", testAdminId);
        assertTrue(flickAfterEmpty.isEmpty(), "手动清空后不应再触发初始默认全量填充");

        // 验证 rocnovel 保持完好，不受 flicknovel 操作的影响
        List<LandingPageConfigItem> rocConfigs = userService.getUserLandingPageConfigs("rocnovel", testAdminId);
        assertEquals(1, rocConfigs.size());
        assertEquals("ROC_PAGE_1", rocConfigs.get(0).getLandingPageId());
        assertEquals("CST", rocConfigs.get(0).getTimezone(), "传入 BJ 应平滑升级为 CST");
    }

    @Test
    public void testAutoImportFlicknovelLandingPagesForAdmins() {
        // 1. 管理员初始配置仅保留 TEST_PROMO_999，并自定义时区为 ET
        LandingPageConfigItem existingItem = new LandingPageConfigItem("flicknovel", "TEST_PROMO_999", "ET");
        userService.updateUserLandingPageConfigs("flicknovel", testAdminId, Collections.singletonList(existingItem));

        // 2. 模拟系统产生新的番茄司南推广ID（插入一条带有 NEW_PROMO_888 的订单）
        RawOrder newOrder = new RawOrder();
        newOrder.setPlatformCode("flicknovel");
        newOrder.setOrderId("test_order_new_" + System.currentTimeMillis());
        newOrder.setLandingPageId("NEW_PROMO_888");
        newOrder.setMemberId("mem_888");
        newOrder.setOrderAmountCent(888);
        newOrder.setOrderAmountUsd(new BigDecimal("8.88"));
        newOrder.setRegisterTimeBj(LocalDateTime.now());
        newOrder.setRegisterTimeEt(LocalDateTime.now().minusHours(12));
        newOrder.setRegisterDateEt(java.time.LocalDate.now());
        newOrder.setPayTimeBj(LocalDateTime.now());
        newOrder.setPayTimeEt(LocalDateTime.now().minusHours(12));
        newOrder.setPayDateEt(java.time.LocalDate.now());
        newOrder = rawOrderRepository.save(newOrder);

        try {
            // 3. 执行定时自动导入方法
            int importedCount = userService.autoImportFlicknovelLandingPagesForAdmins();
            assertTrue(importedCount >= 1, "应至少为管理员成功导入 1 个新推广ID");

            // 4. 验证管理员配置：应包含原有的 TEST_PROMO_999（保留 ET 时区）和新追加的 NEW_PROMO_888（默认 UTC 时区）
            List<LandingPageConfigItem> adminConfigs = userService.getUserLandingPageConfigs("flicknovel", testAdminId);
            assertNotNull(adminConfigs);

            LandingPageConfigItem promo999 = adminConfigs.stream()
                    .filter(c -> "TEST_PROMO_999".equals(c.getLandingPageId()))
                    .findFirst().orElse(null);
            assertNotNull(promo999, "应保留管理员原有配置 TEST_PROMO_999");
            assertEquals("ET", promo999.getTimezone(), "原有自定义时区 ET 不应被覆盖");

            LandingPageConfigItem promo888 = adminConfigs.stream()
                    .filter(c -> "NEW_PROMO_888".equals(c.getLandingPageId()))
                    .findFirst().orElse(null);
            assertNotNull(promo888, "应自动补齐新增的推广ID NEW_PROMO_888");
            assertEquals("UTC", promo888.getTimezone(), "自动补齐的番茄司南推广ID时区应默认为 UTC");

            // 5. 验证普通用户 (USER)：绝不会被自动导入
            List<LandingPageConfigItem> userConfigs = userService.getUserLandingPageConfigs("flicknovel", testUserId);
            assertTrue(userConfigs.isEmpty(), "普通用户绝不应自动导入全量推广ID");

            // 6. 验证幂等性：再次执行导入，由于已全部拥有，导入计数应为 0
            int secondRunImported = userService.autoImportFlicknovelLandingPagesForAdmins();
            assertEquals(0, secondRunImported, "已全部同步时再次执行导入数应为 0");
        } finally {
            rawOrderRepository.deleteById(newOrder.getId());
        }
    }
}
