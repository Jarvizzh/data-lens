package handler

import (
	"strings"
	"time"

	"go_backend/internal/middleware"
	"go_backend/internal/model"
	"go_backend/internal/pkg/locker"
	"go_backend/internal/pkg/response"
	"go_backend/internal/pkg/timeutil"
	"go_backend/internal/repository"
	"go_backend/internal/service"

	"github.com/gin-gonic/gin"
)

type PlatformHandler struct {
	platformRepo *repository.PlatformRepository
	userRepo     *repository.UserRepository
	syncMgr      *service.SyncManager
	locker       *locker.TaskLocker
}

func NewPlatformHandler(
	platformRepo *repository.PlatformRepository,
	userRepo *repository.UserRepository,
	syncMgr *service.SyncManager,
	locker *locker.TaskLocker,
) *PlatformHandler {
	return &PlatformHandler{
		platformRepo: platformRepo,
		userRepo:     userRepo,
		syncMgr:      syncMgr,
		locker:       locker,
	}
}

// HasPlatformAccess 校验用户是否拥有指定平台的访问权限 (与 Java SysUser.hasPlatformAccess 严格对齐)
func HasPlatformAccess(user *model.SysUser, platformCode string) bool {
	if user == nil || user.IsSuperAdmin() {
		return true
	}
	allowed := strings.TrimSpace(user.AllowedPlatforms)
	if allowed == "" {
		allowed = model.PlatformAll
	}
	parts := strings.Split(allowed, ",")
	set := make(map[string]bool)
	for _, p := range parts {
		clean := strings.ToLower(strings.TrimSpace(p))
		if clean != "" {
			set[clean] = true
		}
	}
	if set["all"] {
		return true
	}
	reqCode := strings.ToLower(strings.TrimSpace(platformCode))
	if reqCode == "" || reqCode == "all" {
		return set["all"]
	}
	return set[reqCode]
}

// ListPlatforms 获取系统平台列表 (对齐 Java PlatformController.listPlatforms 格式)
func (h *PlatformHandler) ListPlatforms(c *gin.Context) {
	var user *model.SysUser
	if u := middleware.GetCurrentUser(c); u != nil && h.userRepo != nil {
		user, _ = h.userRepo.FindByID(c.Request.Context(), u.UserID)
	}

	result := make([]model.PlatformItemDto, 0)

	// 1. 若拥有 ALL 权限，首项返回大盘汇总
	if HasPlatformAccess(user, model.PlatformAll) {
		result = append(result, model.PlatformItemDto{
			Code:            model.PlatformAll,
			Name:            "大盘汇总",
			Enabled:         true,
			LaunchStartDate: model.LaunchStartDateRocnovel,
		})
	}

	// 2. 从数据库配置中读取已配置的平台
	addedCodes := make(map[string]bool)
	configs, err := h.platformRepo.FindAll(c.Request.Context())
	if err == nil && len(configs) > 0 {
		for _, cfg := range configs {
			if cfg.Status == 1 && HasPlatformAccess(user, cfg.PlatformCode) {
				startDate := cfg.LaunchStartDate
				if startDate == "" {
					if model.IsFlicknovel(cfg.PlatformCode) {
						startDate = model.LaunchStartDateFlicknovel
					} else {
						startDate = model.LaunchStartDateRocnovel
					}
				} else if len(startDate) >= 10 {
					startDate = startDate[:10]
				}
				result = append(result, model.PlatformItemDto{
					Code:            cfg.PlatformCode,
					Name:            cfg.PlatformName,
					Enabled:         true,
					LaunchStartDate: startDate,
				})
				addedCodes[strings.ToLower(cfg.PlatformCode)] = true
			}
		}
	}

	// 3. 兜底内建平台 (rocnovel, flicknovel) 若未落库时自动补齐
	if !addedCodes[model.PlatformRocnovel] && HasPlatformAccess(user, model.PlatformRocnovel) {
		result = append(result, model.PlatformItemDto{
			Code:            model.PlatformRocnovel,
			Name:            model.PlatformNameRocnovel,
			Enabled:         true,
			LaunchStartDate: model.LaunchStartDateRocnovel,
		})
		addedCodes[model.PlatformRocnovel] = true
	}
	if !addedCodes[model.PlatformFlicknovel] && HasPlatformAccess(user, model.PlatformFlicknovel) {
		result = append(result, model.PlatformItemDto{
			Code:            model.PlatformFlicknovel,
			Name:            model.PlatformNameFlicknovel,
			Enabled:         true,
			LaunchStartDate: model.LaunchStartDateFlicknovel,
		})
		addedCodes[model.PlatformFlicknovel] = true
	}

	response.Success(c, result)
}

func (h *PlatformHandler) TriggerSync(c *gin.Context) {
	unlock, ok := h.locker.GuardGin(c, locker.LockKeySyncOrders, 10*time.Minute, "当前订单同步任务正在执行中，请勿重复操作")
	if !ok {
		return
	}
	defer unlock()

	startTime := c.DefaultQuery("startTime", model.LaunchStartDateRocnovel)
	endTime := c.DefaultQuery("endTime", timeutil.GetTodayCst())

	err := h.syncMgr.SyncOrdersAllPlatforms(c.Request.Context(), startTime, endTime)
	if err != nil {
		response.Error(c, 500, "触发同步失败: "+err.Error())
		return
	}
	response.SuccessWithMsg(c, "同步触发成功", nil)
}
