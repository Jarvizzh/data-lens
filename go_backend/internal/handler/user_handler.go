package handler

import (
	"net/http"
	"strconv"
	"strings"

	"go_backend/internal/middleware"
	"go_backend/internal/pkg/response"
	"go_backend/internal/service"
	"go_backend/internal/service/dto"

	"github.com/gin-gonic/gin"
)

type UserHandler struct {
	userSvc *service.UserService
	permSvc *service.UserPermissionService
	ltvSvc  *service.LtvService
}

func NewUserHandler(
	userSvc *service.UserService,
	permSvc *service.UserPermissionService,
	ltvSvc *service.LtvService,
) *UserHandler {
	return &UserHandler{
		userSvc: userSvc,
		permSvc: permSvc,
		ltvSvc:  ltvSvc,
	}
}

// GetVisibleAccounts 返回当前用户可见的账户切换列表 (/api/user/visible-accounts)
func (h *UserHandler) GetVisibleAccounts(c *gin.Context) {
	u := middleware.GetCurrentUser(c)
	if u == nil {
		response.Unauthorized(c, "未登录")
		return
	}

	accounts, err := h.permSvc.GetVisibleAccountsForUser(c.Request.Context(), u.UserID)
	if err != nil {
		response.Error(c, 500, "获取账户列表失败: "+err.Error())
		return
	}

	response.Success(c, accounts)
}

// GetLandingPages 获取用户的落地页配置列表 (/api/user/landing-pages)
func (h *UserHandler) GetLandingPages(c *gin.Context) {
	u := middleware.GetCurrentUser(c)
	if u == nil {
		response.Unauthorized(c, "未登录")
		return
	}

	platformCode := c.DefaultQuery("platformCode", "rocnovel")
	targetUID := u.UserID
	if uidStr := c.Query("targetUserId"); uidStr != "" {
		if uid, err := strconv.ParseInt(uidStr, 10, 64); err == nil && uid > 0 {
			targetUID = uid
		}
	}

	if !h.permSvc.CanUserViewTarget(c.Request.Context(), u.UserID, u.Role, targetUID) {
		response.Error(c, 403, "无权访问该账户的视图")
		return
	}

	items, ids, err := h.userSvc.GetLandingPageConfigs(c.Request.Context(), platformCode, targetUID)
	if err != nil {
		response.Error(c, 500, "查询落地页失败: "+err.Error())
		return
	}

	c.JSON(http.StatusOK, dto.UserLandingPageConfigResponseDto{
		Code:           0,
		Data:           items,
		LandingPageIDs: ids,
	})
}

type UserLandingPageUpdateRequest struct {
	PlatformCode   string                      `json:"platformCode"`
	TargetUserID   *int64                      `json:"targetUserId"`
	LandingPages   []dto.LandingPageConfigItem `json:"landingPages"`
	LandingPageIDs []string                    `json:"landingPageIds"`
}

// UpdateLandingPages 更新用户的落地页配置 (/api/user/landing-pages)
func (h *UserHandler) UpdateLandingPages(c *gin.Context) {
	u := middleware.GetCurrentUser(c)
	if u == nil {
		response.Unauthorized(c, "未登录")
		return
	}

	var req UserLandingPageUpdateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, 400, "参数错误")
		return
	}

	targetUID := u.UserID
	if req.TargetUserID != nil && *req.TargetUserID > 0 {
		targetUID = *req.TargetUserID
	}

	if !h.permSvc.CanUserModifyTarget(c.Request.Context(), u.UserID, u.Role, targetUID) {
		response.Error(c, 403, "该账户视图为只读模式，无法修改落地页配置")
		return
	}

	platformCode := req.PlatformCode
	if platformCode == "" {
		platformCode = c.DefaultQuery("platformCode", "rocnovel")
	}

	items := req.LandingPages
	if len(items) == 0 && len(req.LandingPageIDs) > 0 {
		for _, id := range req.LandingPageIDs {
			items = append(items, dto.LandingPageConfigItem{
				PlatformCode:  platformCode,
				LandingPageID: id,
				Timezone:      "CST",
			})
		}
	}

	if err := h.userSvc.UpdateLandingPageConfigs(c.Request.Context(), platformCode, targetUID, items); err != nil {
		response.Error(c, 500, "保存失败: "+err.Error())
		return
	}

	if h.ltvSvc != nil {
		_ = h.ltvSvc.CalculateLtvStatsForUserDirect(c.Request.Context(), platformCode, targetUID)
		_ = h.ltvSvc.CalculateLtvStatsForUserDirect(c.Request.Context(), "ALL", targetUID)
	}

	response.SuccessWithMsg(c, "落地页配置已更新，并完成个人报表秒级重算！", nil)
}

// GetAllPlatformLandingPages 获取平台全量落地页 (/api/user/all-landing-pages)
func (h *UserHandler) GetAllPlatformLandingPages(c *gin.Context) {
	u := middleware.GetCurrentUser(c)
	if u == nil {
		response.Unauthorized(c, "未登录")
		return
	}

	if !strings.EqualFold(u.Role, "ADMIN") && !strings.EqualFold(u.Role, "SUPER_ADMIN") {
		response.Error(c, 403, "普通用户无权载入全量推广ID")
		return
	}

	platformCode := c.DefaultQuery("platformCode", "rocnovel")
	allPids, err := h.userSvc.GetAllPlatformLandingPageIds(c.Request.Context(), platformCode)
	if err != nil {
		response.Error(c, 500, "查询失败: "+err.Error())
		return
	}

	response.Success(c, allPids)
}
