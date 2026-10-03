package com.ltv.stat.service;

import com.ltv.stat.dto.LandingPageConfigItem;
import com.ltv.stat.dto.TokenInfo;
import com.ltv.stat.dto.VisibleAccountDto;
import com.ltv.stat.entity.FlicknovelPromotion;
import com.ltv.stat.entity.SysUser;
import com.ltv.stat.entity.UserLandingPage;
import com.ltv.stat.entity.UserSubAccount;
import com.ltv.stat.entity.UserViewPermission;
import com.ltv.stat.repository.FlicknovelPromotionRepository;
import com.ltv.stat.repository.RawOrderRepository;
import com.ltv.stat.repository.SubscriptionConfigVersionRepository;
import com.ltv.stat.repository.SysUserRepository;
import com.ltv.stat.repository.UserLandingPageRepository;
import com.ltv.stat.repository.UserSubAccountRepository;
import com.ltv.stat.repository.UserViewPermissionRepository;
import com.ltv.stat.service.flicknovel.FlicknovelApiService;
import org.slf4j.Logger;
import org.slf4j.LoggerFactory;
import org.springframework.beans.factory.annotation.Autowired;
import org.springframework.beans.factory.annotation.Value;
import org.springframework.context.annotation.Lazy;
import org.springframework.stereotype.Service;
import org.springframework.transaction.annotation.Transactional;

import javax.annotation.PostConstruct;
import java.nio.charset.StandardCharsets;
import java.security.MessageDigest;
import java.util.*;
import java.util.stream.Collectors;

@Service
@Transactional
public class UserService {

    private static final Logger log = LoggerFactory.getLogger(UserService.class);

    private final SysUserRepository sysUserRepository;
    private final UserLandingPageRepository userLandingPageRepository;
    private final UserViewPermissionRepository userViewPermissionRepository;
    private final UserSubAccountRepository userSubAccountRepository;
    private final RawOrderRepository rawOrderRepository;
    private final SubscriptionConfigVersionRepository subscriptionConfigVersionRepository;

    @Autowired(required = false)
    private FlicknovelPromotionRepository flicknovelPromotionRepository;

    @Autowired(required = false)
    @Lazy
    private FlicknovelApiService flicknovelApiService;

    @Value("${app.auth.username:superadmin}")
    private String defaultSuperAdminUsername;

    @Value("${app.auth.password:superadmin}")
    private String defaultSuperAdminPassword;

    public UserService(SysUserRepository sysUserRepository,
                       UserLandingPageRepository userLandingPageRepository,
                       UserViewPermissionRepository userViewPermissionRepository,
                       UserSubAccountRepository userSubAccountRepository,
                       RawOrderRepository rawOrderRepository,
                       SubscriptionConfigVersionRepository subscriptionConfigVersionRepository) {
        this.sysUserRepository = sysUserRepository;
        this.userLandingPageRepository = userLandingPageRepository;
        this.userViewPermissionRepository = userViewPermissionRepository;
        this.userSubAccountRepository = userSubAccountRepository;
        this.rawOrderRepository = rawOrderRepository;
        this.subscriptionConfigVersionRepository = subscriptionConfigVersionRepository;
    }

    @PostConstruct
    @Transactional
    public void initDefaultUsers() {
        // 1. 初始化或升级默认超级管理员 superadmin (SUPER_ADMIN)
        Optional<SysUser> superAdminOpt = sysUserRepository.findByUsername(defaultSuperAdminUsername);
        if (!superAdminOpt.isPresent()) {
            SysUser superAdmin = new SysUser();
            superAdmin.setUsername(defaultSuperAdminUsername);
            superAdmin.setPasswordHash(hashPassword(defaultSuperAdminPassword));
            superAdmin.setRole("SUPER_ADMIN");
            superAdmin.setStatus(1);
            sysUserRepository.save(superAdmin);
            log.info("Initialized default super admin user: {}", defaultSuperAdminUsername);
        } else {
            SysUser superAdmin = superAdminOpt.get();
            if (!"SUPER_ADMIN".equalsIgnoreCase(superAdmin.getRole())) {
                superAdmin.setRole("SUPER_ADMIN");
                sysUserRepository.save(superAdmin);
                log.info("Ensured super admin user role: {}", defaultSuperAdminUsername);
            }
        }

        // 2. 确保 admin 账号角色归位为普通管理员 ADMIN
        Optional<SysUser> adminOpt = sysUserRepository.findByUsername("admin");
        if (adminOpt.isPresent()) {
            SysUser admin = adminOpt.get();
            if ("SUPER_ADMIN".equalsIgnoreCase(admin.getRole())) {
                admin.setRole("ADMIN");
                sysUserRepository.save(admin);
                log.info("Reset admin user role back to ADMIN");
            }
        } else {
            SysUser admin = new SysUser();
            admin.setUsername("admin");
            admin.setPasswordHash(hashPassword("admin666"));
            admin.setRole("ADMIN");
            admin.setStatus(1);
            sysUserRepository.save(admin);
            log.info("Initialized default admin user: admin");
        }
    }

    public Optional<SysUser> findByUsername(String username) {
        return sysUserRepository.findByUsername(username);
    }

    public Optional<SysUser> findById(Long id) {
        return sysUserRepository.findById(id);
    }

    public List<SysUser> listAllUsers() {
        return sysUserRepository.findAllByOrderByCreatedAtDesc();
    }

    public boolean validatePassword(SysUser user, String rawPassword) {
        if (user == null || rawPassword == null) return false;
        return user.getPasswordHash().equals(hashPassword(rawPassword));
    }

    @Transactional
    public SysUser createUser(String username, String rawPassword, String role) {
        return createUser(username, rawPassword, role, 0, 0, null, null, 0, 0, 0, 0, 0, 0);
    }

    @Transactional
    public SysUser createUser(
            String username,
            String rawPassword,
            String role,
            Integer isMaster,
            Integer isSettlement,
            List<Long> visibleUserIds,
            List<Long> subUserIds,
            Integer permPredictPayback,
            Integer permRoiPredict,
            Integer permGlobalDistribution,
            Integer permExport,
            Integer permSettlement,
            Integer permVideoGen
    ) {
        return createUser(username, rawPassword, role, isMaster, isSettlement, visibleUserIds, subUserIds,
                permPredictPayback, permRoiPredict, permGlobalDistribution, permExport, permSettlement, permVideoGen, null);
    }

    @Transactional
    public SysUser createUser(
            String username,
            String rawPassword,
            String role,
            Integer isMaster,
            Integer isSettlement,
            List<Long> visibleUserIds,
            List<Long> subUserIds,
            Integer permPredictPayback,
            Integer permRoiPredict,
            Integer permGlobalDistribution,
            Integer permExport,
            Integer permSettlement,
            Integer permVideoGen,
            String allowedPlatforms
    ) {
        if (sysUserRepository.existsByUsername(username)) {
            throw new IllegalArgumentException("用户名已存在: " + username);
        }
        SysUser user = new SysUser();
        user.setUsername(username.trim());
        user.setPasswordHash(hashPassword(rawPassword));
        user.setRole(role != null ? role.toUpperCase() : "USER");
        user.setStatus(1);
        user.setIsMaster(isMaster != null ? isMaster : 0);
        user.setIsSettlement(isSettlement != null ? isSettlement : 0);
        user.setPermPredictPayback(permPredictPayback != null ? permPredictPayback : 0);
        user.setPermRoiPredict(permRoiPredict != null ? permRoiPredict : 0);
        user.setPermGlobalDistribution(permGlobalDistribution != null ? permGlobalDistribution : 0);
        user.setPermExport(permExport != null ? permExport : 0);
        user.setPermSettlement(permSettlement != null ? permSettlement : 0);
        user.setPermVideoGen(permVideoGen != null ? permVideoGen : 0);

        if (user.isSuperAdmin()) {
            user.setAllowedPlatforms("ALL");
        } else if (allowedPlatforms != null && !allowedPlatforms.trim().isEmpty()) {
            user.setAllowedPlatforms(allowedPlatforms.trim());
        } else {
            user.setAllowedPlatforms("ALL");
        }

        SysUser savedUser = sysUserRepository.save(user);

        // 如果配置了只读视图
        if (visibleUserIds != null && !visibleUserIds.isEmpty()) {
            updateUserViewPermissions(savedUser.getId(), visibleUserIds);
        }

        // 如果是主账号并配置了子账号
        if (Integer.valueOf(1).equals(user.getIsMaster()) && subUserIds != null && !subUserIds.isEmpty()) {
            updateMasterSubAccounts(savedUser.getId(), subUserIds);
        }

        return savedUser;
    }

    @Transactional
    public void resetPassword(Long userId, String newRawPassword) {
        SysUser user = sysUserRepository.findById(userId)
                .orElseThrow(() -> new IllegalArgumentException("用户不存在: " + userId));
        user.setPasswordHash(hashPassword(newRawPassword));
        sysUserRepository.save(user);
    }

    @Transactional
    public void updateUserRole(Long userId, String newRole) {
        SysUser user = sysUserRepository.findById(userId)
                .orElseThrow(() -> new IllegalArgumentException("用户不存在: " + userId));
        user.setRole(newRole != null ? newRole.toUpperCase() : "USER");
        sysUserRepository.save(user);
    }

    @Transactional
    public void updateUserStatus(Long userId, Integer status) {
        SysUser user = sysUserRepository.findById(userId)
                .orElseThrow(() -> new IllegalArgumentException("用户不存在: " + userId));
        user.setStatus(status);
        sysUserRepository.save(user);
    }

    @Transactional
    public void deleteUser(Long userId) {
        userLandingPageRepository.deleteByUserId(userId);
        userViewPermissionRepository.deleteByUserId(userId);
        userViewPermissionRepository.deleteByTargetUserId(userId);
        userSubAccountRepository.deleteByMasterUserId(userId);
        userSubAccountRepository.deleteBySubUserId(userId);
        sysUserRepository.deleteById(userId);
    }

    public boolean isMasterAccount(Long userId) {
        if (userId == null) return false;
        SysUser user = sysUserRepository.findById(userId).orElse(null);
        return user != null && user.isMasterAccount();
    }

    public List<Long> getSubUserIdsForMaster(Long masterUserId) {
        if (masterUserId == null) return Collections.emptyList();
        return userSubAccountRepository.findByMasterUserId(masterUserId).stream()
                .map(UserSubAccount::getSubUserId)
                .collect(Collectors.toList());
    }

    public List<Long> getMasterUserIdsForSub(Long subUserId) {
        if (subUserId == null) return Collections.emptyList();
        return userSubAccountRepository.findBySubUserId(subUserId).stream()
                .map(UserSubAccount::getMasterUserId)
                .collect(Collectors.toList());
    }

    @Transactional
    public void updateMasterStatus(Long userId, Integer isMaster) {
        SysUser user = sysUserRepository.findById(userId)
                .orElseThrow(() -> new IllegalArgumentException("用户不存在: " + userId));
        user.setIsMaster(isMaster != null ? isMaster : 0);
        sysUserRepository.save(user);
        if (Integer.valueOf(1).equals(isMaster)) {
            // 提升为主账号时，解除作为其他主账号子账号的关联
            userSubAccountRepository.deleteBySubUserId(userId);
        } else {
            // 取消主账号时，删除关联的所有子账号关系
            userSubAccountRepository.deleteByMasterUserId(userId);
        }
    }

    @Transactional
    public void updateMasterSubAccounts(Long masterUserId, List<Long> subUserIds) {
        SysUser masterUser = sysUserRepository.findById(masterUserId)
                .orElseThrow(() -> new IllegalArgumentException("主账号不存在: " + masterUserId));

        userSubAccountRepository.deleteByMasterUserId(masterUserId);
        userSubAccountRepository.flush();

        if (subUserIds != null && !subUserIds.isEmpty()) {
            List<UserSubAccount> list = new ArrayList<>();
            Set<Long> uniqueSubs = new HashSet<>(subUserIds);
            for (Long subId : uniqueSubs) {
                if (subId != null && !subId.equals(masterUserId)) {
                    SysUser subUser = sysUserRepository.findById(subId).orElse(null);
                    if (subUser != null && !subUser.isMasterAccount()) {
                        list.add(new UserSubAccount(masterUserId, subId));
                    }
                }
            }
            if (!list.isEmpty()) {
                userSubAccountRepository.saveAll(list);
                userSubAccountRepository.flush();
            }
        }
    }

    public List<Long> getUserViewPermissionTargetIds(Long userId) {
        if (userId == null) return Collections.emptyList();
        return userViewPermissionRepository.findByUserId(userId).stream()
                .map(UserViewPermission::getTargetUserId)
                .collect(Collectors.toList());
    }

    @Transactional
    public void updateUserViewPermissions(Long userId, List<Long> targetUserIds) {
        if (userId == null) return;
        if (!sysUserRepository.existsById(userId)) {
            throw new IllegalArgumentException("用户不存在: " + userId);
        }

        userViewPermissionRepository.deleteByUserId(userId);
        userViewPermissionRepository.flush();

        if (targetUserIds != null && !targetUserIds.isEmpty()) {
            List<UserViewPermission> list = new ArrayList<>();
            Set<Long> uniqueTargets = new HashSet<>(targetUserIds);
            for (Long targetId : uniqueTargets) {
                if (targetId != null && !targetId.equals(userId) && sysUserRepository.existsById(targetId)) {
                    UserViewPermission uvp = new UserViewPermission();
                    uvp.setUserId(userId);
                    uvp.setTargetUserId(targetId);
                    list.add(uvp);
                }
            }
            if (!list.isEmpty()) {
                userViewPermissionRepository.saveAll(list);
                userViewPermissionRepository.flush();
            }
        }
    }

    @Transactional
    public void updateUserPermissions(Long userId, Integer permPredictPayback, Integer permRoiPredict, Integer permGlobalDistribution, Integer permExport, Integer permSettlement, Integer permVideoGen) {
        updateUserPermissions(userId, permPredictPayback, permRoiPredict, permGlobalDistribution, permExport, permSettlement, permVideoGen, null);
    }

    @Transactional
    public void updateUserPermissions(Long userId, Integer permPredictPayback, Integer permRoiPredict, Integer permGlobalDistribution, Integer permExport, Integer permSettlement, Integer permVideoGen, String allowedPlatforms) {
        SysUser user = sysUserRepository.findById(userId)
                .orElseThrow(() -> new IllegalArgumentException("用户不存在: " + userId));

        if (permPredictPayback != null) user.setPermPredictPayback(permPredictPayback);
        if (permRoiPredict != null) user.setPermRoiPredict(permRoiPredict);
        if (permGlobalDistribution != null) user.setPermGlobalDistribution(permGlobalDistribution);
        if (permExport != null) user.setPermExport(permExport);
        if (permSettlement != null) user.setPermSettlement(permSettlement);
        if (permVideoGen != null) user.setPermVideoGen(permVideoGen);

        if (user.isSuperAdmin()) {
            user.setAllowedPlatforms("ALL");
        } else if (allowedPlatforms != null && !allowedPlatforms.trim().isEmpty()) {
            user.setAllowedPlatforms(allowedPlatforms.trim());
        }

        sysUserRepository.save(user);
    }

    public boolean hasPermVideoGen(Long userId) {
        if (userId == null) return false;
        SysUser user = sysUserRepository.findById(userId).orElse(null);
        return user != null && user.hasPermVideoGen();
    }

    public boolean hasPermSettlement(Long userId) {
        if (userId == null) return false;
        SysUser user = sysUserRepository.findById(userId).orElse(null);
        return user != null && user.hasPermSettlement();
    }

    public boolean hasPermGlobalDistribution(Long userId) {
        if (userId == null) return false;
        SysUser user = sysUserRepository.findById(userId).orElse(null);
        return user != null && user.hasPermGlobalDistribution();
    }

    public boolean hasPermExport(Long userId) {
        if (userId == null) return false;
        SysUser user = sysUserRepository.findById(userId).orElse(null);
        return user != null && user.hasPermExport();
    }

    public boolean hasPermPredictPayback(Long userId) {
        if (userId == null) return false;
        SysUser user = sysUserRepository.findById(userId).orElse(null);
        return user != null && user.hasPermPredictPayback();
    }

    public boolean hasPermRoiPredict(Long userId) {
        if (userId == null) return false;
        SysUser user = sysUserRepository.findById(userId).orElse(null);
        return user != null && user.hasPermRoiPredict();
    }

    public List<VisibleAccountDto> getVisibleAccountsForUser(Long userId) {
        if (userId == null) return Collections.emptyList();
        SysUser user = sysUserRepository.findById(userId).orElse(null);
        if (user == null) return Collections.emptyList();

        List<SysUser> allUsers = sysUserRepository.findAllByOrderByCreatedAtDesc();

        // 超级管理员：可查看系统中所有活跃用户
        if ("SUPER_ADMIN".equalsIgnoreCase(user.getRole())) {
            List<VisibleAccountDto> list = new ArrayList<>();
            for (SysUser u : allUsers) {
                if (u.getStatus() != null && u.getStatus() == 1) {
                    boolean isSelf = u.getId().equals(userId);
                    int subCount = isMasterAccount(u.getId()) ? getSubUserIdsForMaster(u.getId()).size() : 0;
                    list.add(new VisibleAccountDto(u.getId(), u.getUsername(), u.getRole(), isSelf, u.getIsMaster(), subCount));
                }
            }
            return list;
        }

        // 普通管理员 / 普通用户：包含自身主账户 + 被分配允许查看的目标账户 + 若为主账号则自动包含名下子账号
        List<Long> grantedTargetIds = getUserViewPermissionTargetIds(userId);
        Set<Long> visibleSet = new HashSet<>(grantedTargetIds);
        visibleSet.add(userId);
        if (isMasterAccount(userId)) {
            visibleSet.addAll(getSubUserIdsForMaster(userId));
        }

        List<VisibleAccountDto> result = new ArrayList<>();
        // 首先加入本人账户
        int selfSubCount = isMasterAccount(user.getId()) ? getSubUserIdsForMaster(user.getId()).size() : 0;
        result.add(new VisibleAccountDto(user.getId(), user.getUsername(), user.getRole(), true, user.getIsMaster(), selfSubCount));

        // 其它被授权的账户 (包含权限表授权 + 主子账号归属)
        for (SysUser u : allUsers) {
            if (!u.getId().equals(userId) && visibleSet.contains(u.getId()) && u.getStatus() != null && u.getStatus() == 1) {
                int subCount = isMasterAccount(u.getId()) ? getSubUserIdsForMaster(u.getId()).size() : 0;
                result.add(new VisibleAccountDto(u.getId(), u.getUsername(), u.getRole(), false, u.getIsMaster(), subCount));
            }
        }

        return result;
    }

    @Transactional
    public void updateSettlementStatus(Long userId, Integer isSettlement) {
        SysUser user = sysUserRepository.findById(userId)
                .orElseThrow(() -> new IllegalArgumentException("用户不存在: " + userId));
        user.setIsSettlement(isSettlement != null ? isSettlement : 0);
        sysUserRepository.save(user);
    }

    public List<VisibleAccountDto> getSettlementAccountsForUser(Long userId) {
        if (userId == null) return Collections.emptyList();
        SysUser user = sysUserRepository.findById(userId).orElse(null);
        if (user == null) return Collections.emptyList();

        List<SysUser> allUsers = sysUserRepository.findAllByOrderByCreatedAtDesc();

        // 超级管理员 / 管理员：返回系统中所有被勾选了参与结算属性（isSettlement == 1）的账号，且当前账号置顶
        if (user.isAdmin()) {
            List<VisibleAccountDto> list = new ArrayList<>();
            // 若当前登录账号自身也是结算账号或者管理员/超管，置顶
            if (user.isSettlementAccount() || user.isAdmin()) {
                int selfSubCount = isMasterAccount(user.getId()) ? getSubUserIdsForMaster(user.getId()).size() : 0;
                list.add(new VisibleAccountDto(user.getId(), user.getUsername(), user.getRole(), true, user.getIsMaster(), user.getIsSettlement(), selfSubCount));
            }

            for (SysUser u : allUsers) {
                if (!u.getId().equals(userId) && u.getStatus() != null && u.getStatus() == 1 && u.isSettlementAccount()) {
                    int subCount = isMasterAccount(u.getId()) ? getSubUserIdsForMaster(u.getId()).size() : 0;
                    list.add(new VisibleAccountDto(u.getId(), u.getUsername(), u.getRole(), false, u.getIsMaster(), u.getIsSettlement(), subCount));
                }
            }
            return list;
        }

        // 普通用户：若有结算权限，结算账号列表只能查看自身登录账号，不可查看其他结算账号
        List<VisibleAccountDto> result = new ArrayList<>();
        int selfSubCount = isMasterAccount(user.getId()) ? getSubUserIdsForMaster(user.getId()).size() : 0;
        result.add(new VisibleAccountDto(user.getId(), user.getUsername(), user.getRole(), true, user.getIsMaster(), user.getIsSettlement(), selfSubCount));
        return result;
    }

    public boolean canUserViewTarget(TokenInfo currentUser, Long targetUserId) {
        if (currentUser == null || currentUser.getUserId() == null) return false;
        if (targetUserId == null || targetUserId.equals(currentUser.getUserId())) return true;
        if (currentUser.isSuperAdmin()) return true;

        if (userSubAccountRepository.existsByMasterUserIdAndSubUserId(currentUser.getUserId(), targetUserId)) {
            return true;
        }

        return userViewPermissionRepository.existsByUserIdAndTargetUserId(currentUser.getUserId(), targetUserId);
    }

    public boolean canUserModifyTarget(TokenInfo currentUser, Long targetUserId) {
        if (currentUser == null || currentUser.getUserId() == null) return false;
        if (targetUserId == null || targetUserId.equals(currentUser.getUserId())) return true;
        if (currentUser.isSuperAdmin()) return true;

        if (userSubAccountRepository.existsByMasterUserIdAndSubUserId(currentUser.getUserId(), targetUserId)) {
            return true;
        }

        return false;
    }


    public Set<String> getAdminLandingPageIds(Long excludeUserId) {
        List<SysUser> adminUsers = sysUserRepository.findAll().stream()
                .filter(u -> "ADMIN".equalsIgnoreCase(u.getRole()) || "SUPER_ADMIN".equalsIgnoreCase(u.getRole()))
                .filter(u -> excludeUserId == null || !u.getId().equals(excludeUserId))
                .collect(Collectors.toList());
        Set<String> adminPids = new HashSet<>();
        for (SysUser admin : adminUsers) {
            List<UserLandingPage> pages = userLandingPageRepository.findByUserId(admin.getId());
            for (UserLandingPage page : pages) {
                if (page.getLandingPageId() != null && !page.getLandingPageId().trim().isEmpty()) {
                    adminPids.add(page.getLandingPageId().trim());
                }
            }
        }
        return adminPids;
    }

    public List<String> getUserLandingPageIds(Long userId) {
        return getUserLandingPageIds("ALL", userId);
    }

    public List<String> getUserLandingPageIds(String platformCode, Long userId) {
        if (userId == null) return Collections.emptyList();
        return getUserLandingPageConfigs(platformCode, userId).stream()
                .map(LandingPageConfigItem::getLandingPageId)
                .collect(Collectors.toList());
    }

    public List<LandingPageConfigItem> getUserLandingPageConfigs(Long userId) {
        return getUserLandingPageConfigs("ALL", userId);
    }

    public List<String> getAllPlatformLandingPageIds(String platformCode) {
        String pCode = (platformCode != null && !platformCode.trim().isEmpty()) ? platformCode.trim().toLowerCase() : "rocnovel";
        Set<String> pids = new LinkedHashSet<>();

        // 若为番茄司南，优先从番茄全量推广链接库 (flicknovel_promotion) 读取
        if ("flicknovel".equalsIgnoreCase(pCode) && flicknovelPromotionRepository != null) {
            try {
                if (flicknovelPromotionRepository.count() == 0 && flicknovelApiService != null) {
                    flicknovelApiService.syncPromotionsAndTemplates(true);
                }
                List<String> promoIds = flicknovelPromotionRepository.findAllPromotionIds();
                if (promoIds != null) {
                    for (String promoId : promoIds) {
                        if (promoId != null && !promoId.trim().isEmpty() && !"__EMPTY__".equalsIgnoreCase(promoId.trim())) {
                            pids.add(promoId.trim());
                        }
                    }
                }
            } catch (Exception e) {
                log.error("[UserService] Failed to query flicknovel promotions: {}", e.getMessage());
            }
        }

        List<String> fromOrders = rawOrderRepository.findDistinctLandingPageIdsByPlatformCode(pCode);
        if (fromOrders != null) {
            for (String pid : fromOrders) {
                if (pid != null && !pid.trim().isEmpty() && !"__EMPTY__".equalsIgnoreCase(pid.trim())) {
                    pids.add(pid.trim());
                }
            }
        }
        List<String> fromConfigs = subscriptionConfigVersionRepository.findDistinctLandingPageIdsByPlatformCode(pCode);
        if (fromConfigs != null) {
            for (String pid : fromConfigs) {
                if (pid != null && !pid.trim().isEmpty() && !"__EMPTY__".equalsIgnoreCase(pid.trim())) {
                    pids.add(pid.trim());
                }
            }
        }
        return new ArrayList<>(pids);
    }

    public List<LandingPageConfigItem> getUserLandingPageConfigs(String platformCode, Long userId) {
        if (userId == null) return Collections.emptyList();
        SysUser user = sysUserRepository.findById(userId).orElse(null);
        if (user == null) return Collections.emptyList();

        boolean filterPlatform = (platformCode != null && !platformCode.trim().isEmpty() && !"ALL".equalsIgnoreCase(platformCode.trim()));
        String targetPlatform = filterPlatform ? platformCode.trim().toLowerCase() : null;

        // 若为主账号，自动聚合所有子账号配置的落地页（去重）
        if (user.isMasterAccount()) {
            List<Long> subUserIds = getSubUserIdsForMaster(userId);
            Set<String> uniquePids = new HashSet<>();
            List<LandingPageConfigItem> aggregated = new ArrayList<>();
            for (Long subId : subUserIds) {
                List<LandingPageConfigItem> subConfigs = getUserLandingPageConfigs(platformCode, subId);
                for (LandingPageConfigItem item : subConfigs) {
                    if (item != null && item.getLandingPageId() != null && !item.getLandingPageId().trim().isEmpty() && !"__EMPTY__".equalsIgnoreCase(item.getLandingPageId().trim())) {
                        String pid = item.getLandingPageId().trim();
                        if (!uniquePids.contains(pid)) {
                            uniquePids.add(pid);
                            aggregated.add(item);
                        }
                    }
                }
            }
            return aggregated;
        }

        List<UserLandingPage> list;
        if (filterPlatform) {
            list = userLandingPageRepository.findByPlatformCodeAndUserId(targetPlatform, userId);
        } else {
            list = userLandingPageRepository.findByUserId(userId);
        }

        // 番茄司南 (flicknovel) 初始配置特殊处理：
        // 仅管理员 (ADMIN / SUPER_ADMIN) 初始落地页默认填充系统已知的所有推广ID（时区默认 UTC）；普通用户 (USER) 初始默认为空
        boolean isAdmin = ("ADMIN".equalsIgnoreCase(user.getRole()) || "SUPER_ADMIN".equalsIgnoreCase(user.getRole()));
        if (filterPlatform && "flicknovel".equalsIgnoreCase(targetPlatform) && (list == null || list.isEmpty())) {
            if (isAdmin) {
                List<String> allPids = getAllPlatformLandingPageIds("flicknovel");
                if (!allPids.isEmpty()) {
                    return allPids.stream()
                            .map(pid -> new LandingPageConfigItem("flicknovel", pid, "UTC"))
                            .collect(Collectors.toList());
                }
            } else {
                return Collections.emptyList();
            }
        }

        // 过滤掉用于标记已主动清空的占位记录 __EMPTY__
        list = list.stream()
                .filter(ulp -> ulp.getLandingPageId() != null && !"__EMPTY__".equalsIgnoreCase(ulp.getLandingPageId().trim()))
                .collect(Collectors.toList());

        // 如果是普通用户 (USER)，剔除已被管理员配置的隔离落地页 ID
        if ("USER".equalsIgnoreCase(user.getRole())) {
            Set<String> adminPids = getAdminLandingPageIds(userId);
            list = list.stream()
                    .filter(ulp -> ulp.getLandingPageId() != null && !adminPids.contains(ulp.getLandingPageId().trim()))
                    .collect(Collectors.toList());
        }

        return list.stream()
                .map(ulp -> new LandingPageConfigItem(ulp.getPlatformCode(), ulp.getLandingPageId(), ulp.getTimezone()))
                .collect(Collectors.toList());
    }

    @Transactional
    public void updateUserLandingPageConfigs(String targetPlatform, Long userId, List<LandingPageConfigItem> items) {
        SysUser user = sysUserRepository.findById(userId)
                .orElseThrow(() -> new IllegalArgumentException("用户不存在: " + userId));

        if (user.isMasterAccount()) {
            throw new IllegalArgumentException("主账号为数据汇总账号，落地页由关联子账号自动聚合，不可直接编辑！");
        }

        String pCode = (targetPlatform != null && !targetPlatform.trim().isEmpty()) ? targetPlatform.trim().toLowerCase() : null;
        if (pCode == null && items != null && !items.isEmpty()) {
            for (LandingPageConfigItem item : items) {
                if (item != null && item.getPlatformCode() != null && !item.getPlatformCode().trim().isEmpty()) {
                    pCode = item.getPlatformCode().trim().toLowerCase();
                    break;
                }
            }
        }
        if (pCode == null || pCode.trim().isEmpty()) {
            pCode = "rocnovel";
        }
        pCode = pCode.trim().toLowerCase();

        // 如果是普通用户 (USER)，拦截校验：不允许配置已被管理员 (ADMIN / SUPER_ADMIN) 配置的独占隔离落地页
        if ("USER".equalsIgnoreCase(user.getRole())) {
            Set<String> adminPids = getAdminLandingPageIds(userId);
            if (items != null) {
                for (LandingPageConfigItem item : items) {
                    if (item != null && item.getLandingPageId() != null) {
                        String pid = item.getLandingPageId().trim();
                        if (adminPids.contains(pid)) {
                            throw new IllegalArgumentException("落地页 ID [" + pid + "] 为管理员独占/隔离落地页，普通用户无法配置！");
                        }
                    }
                }
            }
        }

        // 仅删除该平台的旧配置，保证平台间数据隔离
        userLandingPageRepository.deleteByPlatformCodeAndUserId(pCode, userId);
        userLandingPageRepository.flush();

        List<UserLandingPage> list = new ArrayList<>();
        Set<String> seenPlatformPid = new HashSet<>();
        if (items != null) {
            for (LandingPageConfigItem item : items) {
                if (item != null && item.getLandingPageId() != null && !item.getLandingPageId().trim().isEmpty()) {
                    String itemPCode = (item.getPlatformCode() != null && !item.getPlatformCode().trim().isEmpty())
                            ? item.getPlatformCode().trim().toLowerCase() : pCode;
                    String pid = item.getLandingPageId().trim();
                    if ("__EMPTY__".equalsIgnoreCase(pid)) continue;
                    String key = itemPCode + "_" + pid;
                    if (!seenPlatformPid.contains(key)) {
                        seenPlatformPid.add(key);
                        String rawTz = item.getTimezone();
                        boolean isFlicknovel = "flicknovel".equalsIgnoreCase(itemPCode);
                        if (!isFlicknovel && ("all".equalsIgnoreCase(itemPCode) || "all".equalsIgnoreCase(pCode))) {
                            // 若处于 ALL 混合平台上下文，检查该 landingPageId 是否归属于番茄平台
                            isFlicknovel = pid.matches("^[0-9]{15,}$") || pid.startsWith("prmt_");
                        }
                        String defaultTz = isFlicknovel ? "UTC" : "CST";
                        String tz;
                        if (rawTz == null || rawTz.trim().isEmpty()) {
                            tz = defaultTz;
                        } else {
                            String cleanTz = rawTz.trim().toUpperCase();
                            if ("BJ".equals(cleanTz) || "CST".equals(cleanTz)) {
                                tz = "CST";
                            } else if ("UTC".equals(cleanTz)) {
                                tz = "UTC";
                            } else if ("ET".equals(cleanTz)) {
                                tz = "ET";
                            } else {
                                tz = defaultTz;
                            }
                        }
                        UserLandingPage ulp = new UserLandingPage();
                        ulp.setPlatformCode(itemPCode);
                        ulp.setUserId(userId);
                        ulp.setLandingPageId(pid);
                        ulp.setTimezone(tz);
                        list.add(ulp);
                    }
                }
            }
        }

        // 若用户主动清空了该平台的所有落地页，写入一条 __EMPTY__ 占位记录，以标记“已主动清空”，避免被判定为初始未配置而再次自动塞入全量
        if (list.isEmpty()) {
            UserLandingPage emptyMarker = new UserLandingPage();
            emptyMarker.setPlatformCode(pCode);
            emptyMarker.setUserId(userId);
            emptyMarker.setLandingPageId("__EMPTY__");
            emptyMarker.setTimezone("flicknovel".equalsIgnoreCase(pCode) ? "UTC" : "CST");
            list.add(emptyMarker);
        }

        userLandingPageRepository.saveAll(list);
        userLandingPageRepository.flush();
    }

    @Transactional
    public void updateUserLandingPageConfigs(Long userId, List<LandingPageConfigItem> items) {
        String pCode = null;
        if (items != null && !items.isEmpty()) {
            for (LandingPageConfigItem item : items) {
                if (item != null && item.getPlatformCode() != null && !item.getPlatformCode().trim().isEmpty()) {
                    pCode = item.getPlatformCode().trim().toLowerCase();
                    break;
                }
            }
        }
        updateUserLandingPageConfigs(pCode, userId, items);
    }

    @Transactional
    public void updateUserLandingPageIds(String platformCode, Long userId, List<String> pageIds) {
        String pCode = (platformCode != null && !platformCode.trim().isEmpty()) ? platformCode.trim().toLowerCase() : "rocnovel";
        if (pageIds == null || pageIds.isEmpty()) {
            updateUserLandingPageConfigs(pCode, userId, Collections.emptyList());
            return;
        }
        String defTz = "flicknovel".equalsIgnoreCase(pCode) ? "UTC" : "CST";
        List<LandingPageConfigItem> items = pageIds.stream()
                .filter(id -> id != null && !id.trim().isEmpty())
                .map(id -> new LandingPageConfigItem(pCode, id.trim(), defTz))
                .collect(Collectors.toList());
        updateUserLandingPageConfigs(pCode, userId, items);
    }

    @Transactional
    public void updateUserLandingPageIds(Long userId, List<String> pageIds) {
        updateUserLandingPageIds("rocnovel", userId, pageIds);
    }

    /**
     * 定时自动为所有超级管理员与管理员补齐全量番茄司南推广ID
     * 规则：
     * 1. 仅针对 SUPER_ADMIN 和 ADMIN（状态有效，非主账号/聚合账号）；
     * 2. 增量导入当前管理员尚未配置的推广ID，时区默认设为 UTC；
     * 3. 管理员已有配置项的时区保留不变，不覆盖用户的个性化配置；
     * 4. 若之前存在 __EMPTY__ 清空占位符，予以清理。
     *
     * @return 总共为管理员新增导入的落地页配置项数量
     */
    @Transactional
    public int autoImportFlicknovelLandingPagesForAdmins() {
        List<String> allPids = getAllPlatformLandingPageIds("flicknovel");
        if (allPids == null || allPids.isEmpty()) {
            log.info("[UserService] No flicknovel landing pages found in system, skipping auto-import.");
            return 0;
        }

        List<SysUser> admins = sysUserRepository.findAll().stream()
                .filter(u -> u.getStatus() != null && u.getStatus() == 1)
                .filter(u -> !u.isMasterAccount())
                .filter(u -> "SUPER_ADMIN".equalsIgnoreCase(u.getRole()) || "ADMIN".equalsIgnoreCase(u.getRole()))
                .collect(Collectors.toList());

        if (admins.isEmpty()) {
            return 0;
        }

        int totalImported = 0;
        for (SysUser admin : admins) {
            Long userId = admin.getId();
            List<UserLandingPage> existingList = userLandingPageRepository.findByPlatformCodeAndUserId("flicknovel", userId);
            if (existingList == null) {
                existingList = new ArrayList<>();
            }

            // 清理 __EMPTY__ 占位记录
            List<UserLandingPage> emptyMarkers = existingList.stream()
                    .filter(ulp -> ulp.getLandingPageId() != null && "__EMPTY__".equalsIgnoreCase(ulp.getLandingPageId().trim()))
                    .collect(Collectors.toList());
            if (!emptyMarkers.isEmpty()) {
                userLandingPageRepository.deleteAll(emptyMarkers);
                userLandingPageRepository.flush();
            }

            Set<String> existingPids = existingList.stream()
                    .map(UserLandingPage::getLandingPageId)
                    .filter(pid -> pid != null && !pid.trim().isEmpty() && !"__EMPTY__".equalsIgnoreCase(pid.trim()))
                    .map(String::trim)
                    .collect(Collectors.toSet());

            List<UserLandingPage> toAdd = new ArrayList<>();
            for (String pid : allPids) {
                if (pid != null && !pid.trim().isEmpty() && !"__EMPTY__".equalsIgnoreCase(pid.trim())) {
                    String cleanPid = pid.trim();
                    if (!existingPids.contains(cleanPid)) {
                        UserLandingPage ulp = new UserLandingPage();
                        ulp.setPlatformCode("flicknovel");
                        ulp.setUserId(userId);
                        ulp.setLandingPageId(cleanPid);
                        ulp.setTimezone("UTC");
                        toAdd.add(ulp);
                        existingPids.add(cleanPid);
                    }
                }
            }

            if (!toAdd.isEmpty()) {
                userLandingPageRepository.saveAll(toAdd);
                userLandingPageRepository.flush();
                totalImported += toAdd.size();
                log.info("[UserService] Auto-imported {} new flicknovel promotion IDs for admin user: {} (id: {})",
                        toAdd.size(), admin.getUsername(), userId);
            }
        }

        if (totalImported > 0) {
            log.info("[UserService] Completed flicknovel auto-import: added {} total landing page configs across {} admin(s).",
                    totalImported, admins.size());
        }
        return totalImported;
    }

    public static String hashPassword(String rawPassword) {
        try {
            MessageDigest digest = MessageDigest.getInstance("SHA-256");
            byte[] hash = digest.digest(("zw-ltv-salt-" + rawPassword).getBytes(StandardCharsets.UTF_8));
            return Base64.getEncoder().encodeToString(hash);
        } catch (Exception e) {
            return rawPassword;
        }
    }
}
