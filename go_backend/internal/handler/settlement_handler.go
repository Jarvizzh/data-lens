package handler

import (
	"strconv"
	"strings"

	"go_backend/internal/middleware"
	"go_backend/internal/pkg/response"
	"go_backend/internal/repository"
	"go_backend/internal/service"
	"go_backend/internal/service/dto"

	"github.com/gin-gonic/gin"
)

type SettlementHandler struct {
	settleSvc *service.SettlementService
	permSvc   *service.UserPermissionService
	userRepo  *repository.UserRepository
}

func NewSettlementHandler(
	settleSvc *service.SettlementService,
	permSvc *service.UserPermissionService,
	userRepo *repository.UserRepository,
) *SettlementHandler {
	return &SettlementHandler{
		settleSvc: settleSvc,
		permSvc:   permSvc,
		userRepo:  userRepo,
	}
}

func (h *SettlementHandler) checkPermission(c *gin.Context) bool {
	u := middleware.GetCurrentUser(c)
	if u == nil {
		response.Unauthorized(c, "未登录")
		return false
	}
	if u.IsSuperAdmin() {
		return true
	}
	if h.userRepo != nil {
		user, err := h.userRepo.FindByID(c.Request.Context(), u.UserID)
		if err == nil && user != nil && user.PermSettlement == 1 {
			return true
		}
	}
	response.Error(c, 403, "无权访问，您未开通月份结算权限")
	return false
}

// GetAccounts 获取参与结算的账号列表 (/api/settlement/accounts)
func (h *SettlementHandler) GetAccounts(c *gin.Context) {
	if !h.checkPermission(c) {
		return
	}

	u := middleware.GetCurrentUser(c)
	accounts, err := h.permSvc.GetSettlementAccountsForUser(c.Request.Context(), u.UserID)
	if err != nil {
		response.Error(c, 500, "获取结算账号失败: "+err.Error())
		return
	}

	response.Success(c, accounts)
}

// GetList 获取月份结算明细列表 (/api/settlement/list)
func (h *SettlementHandler) GetList(c *gin.Context) {
	if !h.checkPermission(c) {
		return
	}

	u := middleware.GetCurrentUser(c)
	platformCode := c.Query("platformCode")
	settlementType := c.DefaultQuery("settlementType", "PLATFORM_ALL")
	var targetUserID *int64

	isAdmin := u.IsAdmin()
	if !isAdmin {
		settlementType = "USER_ACCOUNT"
		targetUserID = &u.UserID
	} else if strings.EqualFold(settlementType, "USER_ACCOUNT") {
		if uidStr := c.Query("targetUserId"); uidStr != "" {
			if uid, err := strconv.ParseInt(uidStr, 10, 64); err == nil && uid > 0 {
				targetUserID = &uid
			}
		}
		if targetUserID == nil {
			targetUserID = &u.UserID
		}
	}

	list, err := h.settleSvc.GetMonthlySettlementList(c.Request.Context(), platformCode, settlementType, targetUserID)
	if err != nil {
		response.Error(c, 500, "查询结算数据失败: "+err.Error())
		return
	}

	response.Success(c, list)
}

// SaveConfig 保存结算参数配置 (/api/settlement/save)
func (h *SettlementHandler) SaveConfig(c *gin.Context) {
	if !h.checkPermission(c) {
		return
	}

	u := middleware.GetCurrentUser(c)
	var req dto.MonthlySettlementSaveRequestDto
	if err := c.ShouldBindJSON(&req); err != nil || req.MonthStr == "" {
		response.Error(c, 400, "参数错误")
		return
	}

	isAdmin := u.IsAdmin()
	if !isAdmin {
		req.SettlementType = "USER_ACCOUNT"
		req.TargetUserID = &u.UserID
	} else if strings.EqualFold(req.SettlementType, "USER_ACCOUNT") && req.TargetUserID == nil {
		req.TargetUserID = &u.UserID
	}

	saved, err := h.settleSvc.SaveSettlementConfig(c.Request.Context(), req)
	if err != nil {
		response.Error(c, 500, "保存结算配置失败: "+err.Error())
		return
	}

	response.SuccessWithMsg(c, "结算参数与配置保存成功！", saved)
}
