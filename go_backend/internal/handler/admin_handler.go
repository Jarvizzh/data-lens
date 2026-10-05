package handler

import (
	"strconv"
	"strings"

	"go_backend/internal/middleware"
	"go_backend/internal/pkg/response"
	"go_backend/internal/service"
	"go_backend/internal/service/dto"

	"github.com/gin-gonic/gin"
)

type AdminHandler struct {
	userSvc *service.UserService
	permSvc *service.UserPermissionService
	ltvSvc  *service.LtvService
}

func NewAdminHandler(
	userSvc *service.UserService,
	permSvc *service.UserPermissionService,
	ltvSvc *service.LtvService,
) *AdminHandler {
	return &AdminHandler{
		userSvc: userSvc,
		permSvc: permSvc,
		ltvSvc:  ltvSvc,
	}
}

func (h *AdminHandler) checkSuperAdmin(c *gin.Context) bool {
	u := middleware.GetCurrentUser(c)
	if u == nil || !strings.EqualFold(u.Role, "SUPER_ADMIN") {
		response.Error(c, 403, "无权访问，仅超级管理员可管理用户")
		return false
	}
	return true
}

func parseID(c *gin.Context) int64 {
	id, _ := strconv.ParseInt(c.Param("id"), 10, 64)
	return id
}

type UserInfoDto struct {
	ID                     int64    `json:"id"`
	Username               string   `json:"username"`
	Role                   string   `json:"role"`
	Status                 int      `json:"status"`
	CreatedAt              string   `json:"createdAt"`
	LandingPageIDs         []string `json:"landingPageIds"`
	LandingPageCount       int      `json:"landingPageCount"`
	VisibleUserIDs         []int64  `json:"visibleUserIds"`
	IsMaster               int      `json:"isMaster"`
	IsSettlement           int      `json:"isSettlement"`
	SubUserIDs             []int64  `json:"subUserIds"`
	PermPredictPayback     int      `json:"permPredictPayback"`
	PermRoiPredict         int      `json:"permRoiPredict"`
	PermGlobalDistribution int      `json:"permGlobalDistribution"`
	PermExport             int      `json:"permExport"`
	PermSettlement         int      `json:"permSettlement"`
	PermVideoGen           int      `json:"permVideoGen"`
	AllowedPlatforms       string   `json:"allowedPlatforms"`
}

// ListUsers 获取所有用户列表 (/api/admin/users)
func (h *AdminHandler) ListUsers(c *gin.Context) {
	if !h.checkSuperAdmin(c) {
		return
	}

	users, err := h.userSvc.ListAllUsers(c.Request.Context())
	if err != nil {
		response.Error(c, 500, "查询用户列表失败: "+err.Error())
		return
	}

	result := make([]UserInfoDto, 0, len(users))
	for _, u := range users {
		_, pids, _ := h.userSvc.GetLandingPageConfigs(c.Request.Context(), "ALL", u.ID)
		targetIDs, _ := h.permSvc.GetUserViewPermissionTargetIDs(c.Request.Context(), u.ID)
		subIDs, _ := h.permSvc.GetSubUserIDsForMaster(c.Request.Context(), u.ID)

		platforms := u.AllowedPlatforms
		if strings.EqualFold(u.Role, "SUPER_ADMIN") {
			platforms = "ALL"
		}

		result = append(result, UserInfoDto{
			ID:                     u.ID,
			Username:               u.Username,
			Role:                   u.Role,
			Status:                 u.Status,
			CreatedAt:              u.CreatedAt.Format("2006-01-02 15:04:05"),
			LandingPageIDs:         pids,
			LandingPageCount:       len(pids),
			VisibleUserIDs:         targetIDs,
			IsMaster:               u.IsMaster,
			IsSettlement:           u.IsSettlement,
			SubUserIDs:             subIDs,
			PermPredictPayback:     u.PermPredictPayback,
			PermRoiPredict:         u.PermRoiPredict,
			PermGlobalDistribution: u.PermGlobalDistribution,
			PermExport:             u.PermExport,
			PermSettlement:         u.PermSettlement,
			PermVideoGen:           u.PermVideoGen,
			AllowedPlatforms:       platforms,
		})
	}

	response.Success(c, result)
}

// CreateUser 创建用户 (/api/admin/users)
func (h *AdminHandler) CreateUser(c *gin.Context) {
	if !h.checkSuperAdmin(c) {
		return
	}

	var req struct {
		Username               string  `json:"username"`
		Password               string  `json:"password"`
		Role                   string  `json:"role"`
		IsMaster               int     `json:"isMaster"`
		IsSettlement           int     `json:"isSettlement"`
		VisibleUserIDs         []int64 `json:"visibleUserIds"`
		SubUserIDs             []int64 `json:"subUserIds"`
		PermPredictPayback     int     `json:"permPredictPayback"`
		PermRoiPredict         int     `json:"permRoiPredict"`
		PermGlobalDistribution int     `json:"permGlobalDistribution"`
		PermExport             int     `json:"permExport"`
		PermSettlement         int     `json:"permSettlement"`
		PermVideoGen           int     `json:"permVideoGen"`
		AllowedPlatforms       string  `json:"allowedPlatforms"`
	}

	if err := c.ShouldBindJSON(&req); err != nil || req.Username == "" || req.Password == "" {
		response.Error(c, 400, "用户名和密码不能为空")
		return
	}

	user, err := h.userSvc.CreateUser(c.Request.Context(), service.CreateUserParam{
		Username:               req.Username,
		RawPassword:            req.Password,
		Role:                   req.Role,
		IsMaster:               req.IsMaster,
		IsSettlement:           req.IsSettlement,
		PermPredictPayback:     req.PermPredictPayback,
		PermRoiPredict:         req.PermRoiPredict,
		PermGlobalDistribution: req.PermGlobalDistribution,
		PermExport:             req.PermExport,
		PermSettlement:         req.PermSettlement,
		PermVideoGen:           req.PermVideoGen,
		AllowedPlatforms:       req.AllowedPlatforms,
	})
	if err != nil {
		response.Error(c, 400, err.Error())
		return
	}

	if len(req.VisibleUserIDs) > 0 {
		_ = h.permSvc.UpdateUserViewPermissions(c.Request.Context(), user.ID, req.VisibleUserIDs)
	}
	if req.IsMaster == 1 && len(req.SubUserIDs) > 0 {
		_ = h.permSvc.UpdateMasterSubAccounts(c.Request.Context(), user.ID, req.SubUserIDs)
	}

	response.SuccessWithMsg(c, "创建成功", nil)
}

// UpdateMasterStatus 更新主账号状态 (/api/admin/users/:id/master-status)
func (h *AdminHandler) UpdateMasterStatus(c *gin.Context) {
	if !h.checkSuperAdmin(c) {
		return
	}
	id := parseID(c)
	var body struct {
		IsMaster interface{} `json:"isMaster"`
	}
	_ = c.ShouldBindJSON(&body)
	isMaster := 0
	if b, ok := body.IsMaster.(bool); ok && b {
		isMaster = 1
	} else if f, ok := body.IsMaster.(float64); ok && f > 0 {
		isMaster = 1
	}

	_ = h.permSvc.UpdateMasterStatus(c.Request.Context(), id, isMaster)
	response.SuccessWithMsg(c, "账号类型更新成功！", nil)
}

// UpdateSettlementStatus 更新结算账号属性 (/api/admin/users/:id/settlement-status)
func (h *AdminHandler) UpdateSettlementStatus(c *gin.Context) {
	if !h.checkSuperAdmin(c) {
		return
	}
	id := parseID(c)
	var body struct {
		IsSettlement interface{} `json:"isSettlement"`
	}
	_ = c.ShouldBindJSON(&body)
	isSettlement := 0
	if b, ok := body.IsSettlement.(bool); ok && b {
		isSettlement = 1
	} else if f, ok := body.IsSettlement.(float64); ok && f > 0 {
		isSettlement = 1
	}

	_ = h.permSvc.UpdateSettlementStatus(c.Request.Context(), id, isSettlement)
	response.SuccessWithMsg(c, "结算账号属性更新成功！", nil)
}

// SubAccounts
func (h *AdminHandler) GetSubAccounts(c *gin.Context) {
	if !h.checkSuperAdmin(c) {
		return
	}
	id := parseID(c)
	subIDs, _ := h.permSvc.GetSubUserIDsForMaster(c.Request.Context(), id)
	response.Success(c, subIDs)
}

func (h *AdminHandler) UpdateSubAccounts(c *gin.Context) {
	if !h.checkSuperAdmin(c) {
		return
	}
	id := parseID(c)
	var body struct {
		SubUserIDs []int64 `json:"subUserIds"`
	}
	_ = c.ShouldBindJSON(&body)
	_ = h.permSvc.UpdateMasterSubAccounts(c.Request.Context(), id, body.SubUserIDs)
	response.SuccessWithMsg(c, "子账号关联分配成功！", nil)
}

// ViewPermissions
func (h *AdminHandler) GetViewPermissions(c *gin.Context) {
	if !h.checkSuperAdmin(c) {
		return
	}
	id := parseID(c)
	targetIDs, _ := h.permSvc.GetUserViewPermissionTargetIDs(c.Request.Context(), id)
	response.Success(c, targetIDs)
}

func (h *AdminHandler) UpdateViewPermissions(c *gin.Context) {
	if !h.checkSuperAdmin(c) {
		return
	}
	id := parseID(c)
	var body struct {
		TargetUserIDs []int64 `json:"targetUserIds"`
	}
	_ = c.ShouldBindJSON(&body)
	_ = h.permSvc.UpdateUserViewPermissions(c.Request.Context(), id, body.TargetUserIDs)
	response.SuccessWithMsg(c, "只读视图权限分配保存成功！", nil)
}

// Permissions
func (h *AdminHandler) UpdatePermissions(c *gin.Context) {
	if !h.checkSuperAdmin(c) {
		return
	}
	id := parseID(c)
	var body struct {
		PermPredictPayback     int    `json:"permPredictPayback"`
		PermRoiPredict         int    `json:"permRoiPredict"`
		PermGlobalDistribution int    `json:"permGlobalDistribution"`
		PermExport             int    `json:"permExport"`
		PermSettlement         int    `json:"permSettlement"`
		PermVideoGen           int    `json:"permVideoGen"`
		AllowedPlatforms       string `json:"allowedPlatforms"`
	}
	_ = c.ShouldBindJSON(&body)
	_ = h.permSvc.UpdateUserPermissions(c.Request.Context(), id, service.UserPermissionsParam{
		PermPredictPayback:     body.PermPredictPayback,
		PermRoiPredict:         body.PermRoiPredict,
		PermGlobalDistribution: body.PermGlobalDistribution,
		PermExport:             body.PermExport,
		PermSettlement:         body.PermSettlement,
		PermVideoGen:           body.PermVideoGen,
		AllowedPlatforms:       body.AllowedPlatforms,
	})
	response.SuccessWithMsg(c, "权限分配保存成功！", nil)
}

// ResetPassword
func (h *AdminHandler) ResetPassword(c *gin.Context) {
	if !h.checkSuperAdmin(c) {
		return
	}
	id := parseID(c)
	var body struct {
		NewPassword string `json:"newPassword"`
	}
	_ = c.ShouldBindJSON(&body)
	if body.NewPassword == "" {
		response.Error(c, 400, "新密码不能为空")
		return
	}
	_ = h.userSvc.ResetPassword(c.Request.Context(), id, body.NewPassword)
	response.SuccessWithMsg(c, "密码重置成功", nil)
}

// UpdateRole
func (h *AdminHandler) UpdateRole(c *gin.Context) {
	if !h.checkSuperAdmin(c) {
		return
	}
	id := parseID(c)
	var body struct {
		Role string `json:"role"`
	}
	_ = c.ShouldBindJSON(&body)
	_ = h.userSvc.UpdateUserRole(c.Request.Context(), id, body.Role)
	response.SuccessWithMsg(c, "角色更新成功", nil)
}

// UpdateLandingPages
func (h *AdminHandler) UpdateLandingPages(c *gin.Context) {
	if !h.checkSuperAdmin(c) {
		return
	}
	id := parseID(c)
	var body struct {
		PlatformCode   string                      `json:"platformCode"`
		LandingPages   []dto.LandingPageConfigItem `json:"landingPages"`
		LandingPageIDs []string                    `json:"landingPageIds"`
	}
	_ = c.ShouldBindJSON(&body)

	pCode := body.PlatformCode
	if pCode == "" {
		pCode = "rocnovel"
	}
	items := body.LandingPages
	if len(items) == 0 && len(body.LandingPageIDs) > 0 {
		for _, lpid := range body.LandingPageIDs {
			items = append(items, dto.LandingPageConfigItem{
				PlatformCode:  pCode,
				LandingPageID: lpid,
				Timezone:      "CST",
			})
		}
	}

	_ = h.userSvc.UpdateLandingPageConfigs(c.Request.Context(), pCode, id, items)
	if h.ltvSvc != nil {
		_ = h.ltvSvc.CalculateLtvStatsForUserDirect(c.Request.Context(), pCode, id)
		_ = h.ltvSvc.CalculateLtvStatsForUserDirect(c.Request.Context(), "ALL", id)
	}
	response.SuccessWithMsg(c, "落地页配置保存成功！", nil)
}

// DeleteUser
func (h *AdminHandler) DeleteUser(c *gin.Context) {
	if !h.checkSuperAdmin(c) {
		return
	}
	id := parseID(c)
	u := middleware.GetCurrentUser(c)
	if u != nil && u.UserID == id {
		response.Error(c, 400, "不能删除当前登录的管理员账号")
		return
	}
	_ = h.userSvc.DeleteUser(c.Request.Context(), id)
	response.SuccessWithMsg(c, "删除用户成功", nil)
}
