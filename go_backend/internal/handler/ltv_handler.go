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

type LtvHandler struct {
	ltvSvc           *service.LtvService
	rechargeDistRepo *repository.RechargeDistributionRepository
}

func NewLtvHandler(ltvSvc *service.LtvService, rechargeDistRepo *repository.RechargeDistributionRepository) *LtvHandler {
	return &LtvHandler{
		ltvSvc:           ltvSvc,
		rechargeDistRepo: rechargeDistRepo,
	}
}

// GetDailyStats 获取 LTV 报表数据
func (h *LtvHandler) GetDailyStats(c *gin.Context) {
	platformCode := c.DefaultQuery("platformCode", "ALL")
	userIDStr := c.Query("userId")

	targetUserID := int64(1)
	if userIDStr != "" {
		if uid, err := strconv.ParseInt(userIDStr, 10, 64); err == nil && uid > 0 {
			targetUserID = uid
		}
	} else if u := middleware.GetCurrentUser(c); u != nil {
		targetUserID = u.UserID
	}

	resp, err := h.ltvSvc.GetLtvDailyStats(c.Request.Context(), platformCode, targetUserID)
	if err != nil {
		response.Error(c, 500, "查询 LTV 统计失败: "+err.Error())
		return
	}

	response.Success(c, resp)
}

// Recalculate 触发重新计算 LTV 报表
func (h *LtvHandler) Recalculate(c *gin.Context) {
	platformCode := c.DefaultQuery("platformCode", "ALL")
	userIDStr := c.Query("userId")

	targetUserID := int64(1)
	if userIDStr != "" {
		if uid, err := strconv.ParseInt(userIDStr, 10, 64); err == nil && uid > 0 {
			targetUserID = uid
		}
	}

	if err := h.ltvSvc.CalculateLtvStatsForUserDirect(c.Request.Context(), platformCode, targetUserID); err != nil {
		response.Error(c, 500, "重算 LTV 失败: "+err.Error())
		return
	}

	response.SuccessWithMsg(c, "重算完成", nil)
}

// SaveLaunchConfig 保存投放消耗与备注
func (h *LtvHandler) SaveLaunchConfig(c *gin.Context) {
	var req dto.SaveLaunchConfigRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, 400, "参数错误: "+err.Error())
		return
	}

	if req.UserID <= 0 {
		if u := middleware.GetCurrentUser(c); u != nil {
			req.UserID = u.UserID
		} else {
			req.UserID = 1
		}
	}

	if err := h.ltvSvc.SaveLaunchConfig(c.Request.Context(), req); err != nil {
		response.Error(c, 500, "保存配置失败: "+err.Error())
		return
	}

	response.SuccessWithMsg(c, "保存成功", nil)
}

// GetDailyDistribution 获取每日充值分布统计
func (h *LtvHandler) GetDailyDistribution(c *gin.Context) {
	platformCode := strings.ToLower(c.DefaultQuery("platformCode", "ALL"))
	userIDStr := c.Query("userId")
	startDate := c.Query("startDate")
	endDate := c.Query("endDate")

	targetUserID := int64(1)
	if userIDStr != "" {
		if uid, err := strconv.ParseInt(userIDStr, 10, 64); err == nil && uid > 0 {
			targetUserID = uid
		}
	}

	list, err := h.rechargeDistRepo.FindByFilter(c.Request.Context(), platformCode, []int64{targetUserID}, startDate, endDate)
	if err != nil {
		response.Error(c, 500, "查询充值分布失败: "+err.Error())
		return
	}

	response.Success(c, list)
}
