package handler

import (
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"go_backend/internal/middleware"
	"go_backend/internal/pkg/response"
	"go_backend/internal/service"
	"go_backend/internal/service/dto"

	"github.com/gin-gonic/gin"
)

type LtvHandler struct {
	ltvSvc     *service.LtvService
	distSvc    *service.DailyDistributionService
	permSvc    *service.UserPermissionService
	syncMgr    *service.SyncManager
	predictSvc *service.PredictService
}

func NewLtvHandler(
	ltvSvc *service.LtvService,
	distSvc *service.DailyDistributionService,
	permSvc *service.UserPermissionService,
	syncMgr *service.SyncManager,
	predictSvc *service.PredictService,
) *LtvHandler {
	return &LtvHandler{
		ltvSvc:     ltvSvc,
		distSvc:    distSvc,
		permSvc:    permSvc,
		syncMgr:    syncMgr,
		predictSvc: predictSvc,
	}
}

func (h *LtvHandler) resolveTargetUserID(c *gin.Context) int64 {
	targetUIDStr := c.Query("targetUserId")
	if targetUIDStr == "" {
		targetUIDStr = c.Query("userId")
	}

	u := middleware.GetCurrentUser(c)
	currentUID := int64(1)
	currentRole := "USER"
	if u != nil {
		currentUID = u.UserID
		currentRole = u.Role
	}

	if targetUIDStr == "" {
		return currentUID
	}

	targetUID, err := strconv.ParseInt(targetUIDStr, 10, 64)
	if err != nil || targetUID <= 0 {
		return currentUID
	}

	if h.permSvc != nil && !h.permSvc.CanUserViewTarget(c.Request.Context(), currentUID, currentRole, targetUID) {
		return currentUID
	}
	return targetUID
}

// GetLtvList 获取 LTV 报表 (兼容 /api/ltv/list 与 /api/ltv/daily-stats)
func (h *LtvHandler) GetLtvList(c *gin.Context) {
	platformCode := c.DefaultQuery("platformCode", "ALL")
	targetUID := h.resolveTargetUserID(c)

	resp, err := h.ltvSvc.GetLtvListResponse(c.Request.Context(), platformCode, targetUID)
	if err != nil {
		response.Error(c, 500, "查询 LTV 统计失败: "+err.Error())
		return
	}

	c.JSON(http.StatusOK, resp)
}

// GetDailyDistribution 获取每日充值分布统计
func (h *LtvHandler) GetDailyDistribution(c *gin.Context) {
	platformCode := c.DefaultQuery("platformCode", "ALL")
	targetUID := h.resolveTargetUserID(c)

	resp, err := h.distSvc.GetDailyDistributionResponse(c.Request.Context(), platformCode, targetUID)
	if err != nil {
		response.Error(c, 500, "查询每日充值分布失败: "+err.Error())
		return
	}

	c.JSON(http.StatusOK, resp)
}

// GetGlobalDailyDistribution 获取平台汇总充值分布 (仅有权限用户可访问)
func (h *LtvHandler) GetGlobalDailyDistribution(c *gin.Context) {
	platformCode := c.DefaultQuery("platformCode", "ALL")
	resp, err := h.distSvc.GetGlobalDailyDistributionResponse(c.Request.Context(), platformCode)
	if err != nil {
		response.Error(c, 500, "查询全盘充值分布失败: "+err.Error())
		return
	}

	c.JSON(http.StatusOK, resp)
}

// SaveLaunchConfig 保存投放消耗与备注 (/api/ltv/config)
func (h *LtvHandler) SaveLaunchConfig(c *gin.Context) {
	var req dto.SaveLaunchConfigRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, 400, "参数错误: "+err.Error())
		return
	}

	if strings.EqualFold(req.PlatformCode, "ALL") {
		response.Error(c, 400, "大盘数据不可直接编辑，请先切换至具体平台")
		return
	}

	targetUID := int64(1)
	if req.TargetUserID != nil && *req.TargetUserID > 0 {
		targetUID = *req.TargetUserID
	} else if req.UserID > 0 {
		targetUID = req.UserID
	} else if u := middleware.GetCurrentUser(c); u != nil {
		targetUID = u.UserID
	}

	req.UserID = targetUID
	if err := h.ltvSvc.SaveLaunchConfig(c.Request.Context(), req); err != nil {
		response.Error(c, 500, "保存配置失败: "+err.Error())
		return
	}

	response.SuccessWithMsg(c, "消耗与备注更新成功！", req)
}

// BatchSpend 批量导入消耗 (/api/ltv/batch-spend)
func (h *LtvHandler) BatchSpend(c *gin.Context) {
	var req dto.BatchSpendRequest
	if err := c.ShouldBindJSON(&req); err != nil || len(req.Items) == 0 {
		response.Error(c, 400, "导入数据不能为空")
		return
	}

	if strings.EqualFold(req.PlatformCode, "ALL") {
		response.Error(c, 400, "大盘数据不可直接导入，请先切换至具体平台")
		return
	}

	targetUID := int64(1)
	if req.TargetUserID != nil && *req.TargetUserID > 0 {
		targetUID = *req.TargetUserID
	} else if u := middleware.GetCurrentUser(c); u != nil {
		targetUID = u.UserID
	}

	count, err := h.ltvSvc.BatchSaveLaunchConfig(c.Request.Context(), req.PlatformCode, targetUID, req.Items)
	if err != nil {
		response.Error(c, 500, "批量导入失败: "+err.Error())
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":  0,
		"msg":   "批量导入账户消耗成功！",
		"count": count,
	})
}

// Recalculate 重新计算 LTV 报表 (/api/ltv/recalculate)
func (h *LtvHandler) Recalculate(c *gin.Context) {
	platformCode := c.DefaultQuery("platformCode", "ALL")
	targetUID := h.resolveTargetUserID(c)

	_ = h.ltvSvc.CalculateLtvStatsForUserDirect(c.Request.Context(), platformCode, targetUID)
	_ = h.ltvSvc.CalculateLtvStatsForUserDirect(c.Request.Context(), "ALL", targetUID)

	resp, err := h.ltvSvc.GetLtvListResponse(c.Request.Context(), platformCode, targetUID)
	if err != nil {
		response.Error(c, 500, "重算 LTV 失败: "+err.Error())
		return
	}
	resp.Msg = "重算 LTV 完成"
	c.JSON(http.StatusOK, resp)
}

// SyncOrders 同步订单 (/api/ltv/sync-orders)
func (h *LtvHandler) SyncOrders(c *gin.Context) {
	var req struct {
		StartTime    string `json:"startTime"`
		EndTime      string `json:"endTime"`
		PlatformCode string `json:"platformCode"`
	}
	_ = c.ShouldBindJSON(&req)

	platformCode := c.Query("platformCode")
	if platformCode == "" {
		platformCode = req.PlatformCode
	}
	if platformCode == "" {
		platformCode = "ALL"
	}

	totalSynced := 0
	if h.syncMgr != nil {
		var err error
		totalSynced, err = h.syncMgr.SyncOrdersForPlatform(c.Request.Context(), platformCode, req.StartTime, req.EndTime)
		if err != nil {
			if strings.Contains(err.Error(), "TOKEN_EXPIRED") {
				c.JSON(http.StatusOK, gin.H{
					"code": 4002,
					"msg":  "订单接口 Authorization 登录 Token 已过期，请在页面更新最新 Token",
				})
				return
			}
			response.Error(c, 500, "同步订单失败: "+err.Error())
			return
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"code":              0,
		"msg":               fmt.Sprintf("订单同步完成，共抓取/更新 %d 笔订单！", totalSynced),
		"totalSyncedOrders": totalSynced,
	})
}

// SyncAndCalc 同步订单并重新计算 (/api/ltv/sync-and-calc)
func (h *LtvHandler) SyncAndCalc(c *gin.Context) {
	var req struct {
		StartTime    string `json:"startTime"`
		EndTime      string `json:"endTime"`
		PlatformCode string `json:"platformCode"`
	}
	_ = c.ShouldBindJSON(&req)

	platformCode := c.Query("platformCode")
	if platformCode == "" {
		platformCode = req.PlatformCode
	}
	if platformCode == "" {
		platformCode = "ALL"
	}

	totalSynced := 0
	if h.syncMgr != nil {
		var err error
		totalSynced, err = h.syncMgr.SyncOrdersForPlatform(c.Request.Context(), platformCode, req.StartTime, req.EndTime)
		if err != nil && strings.Contains(err.Error(), "TOKEN_EXPIRED") {
			c.JSON(http.StatusOK, gin.H{
				"code": 4002,
				"msg":  "订单接口 Authorization 登录 Token 已过期，请在页面更新最新 Token",
			})
			return
		}
	}
	_ = h.ltvSvc.CalculateAllLtvStats(c.Request.Context())
	c.JSON(http.StatusOK, gin.H{
		"code":              0,
		"msg":               fmt.Sprintf("数据同步与重新计算完成，共同步 %d 笔订单", totalSynced),
		"totalSyncedOrders": totalSynced,
	})
}

// RecalculateDailyDistribution 仅重算每日充值分布 (/api/ltv/recalculate-daily-distribution)
func (h *LtvHandler) RecalculateDailyDistribution(c *gin.Context) {
	platformCode := c.DefaultQuery("platformCode", "ALL")
	targetUID := h.resolveTargetUserID(c)

	resp, err := h.distSvc.RecalculateDailyDistribution(c.Request.Context(), platformCode, targetUID)
	if err != nil {
		response.Error(c, 500, "重算每日充值分析失败: "+err.Error())
		return
	}
	resp.Msg = "重算每日充值分析完成！"
	c.JSON(http.StatusOK, resp)
}

// GetBenchmark 获取 LTV 预测基准数据曲线 (/api/ltv/benchmark)
func (h *LtvHandler) GetBenchmark(c *gin.Context) {
	platformCode := c.Query("platformCode")
	dimensionType := c.DefaultQuery("dimensionType", "ALL")
	dimensionValue := c.DefaultQuery("dimensionValue", "DEFAULT")
	subPeriodDays, _ := strconv.Atoi(c.DefaultQuery("subPeriodDays", "1"))

	curve, err := h.predictSvc.GetBenchmarkCurve(c.Request.Context(), platformCode, dimensionType, dimensionValue, subPeriodDays)
	if err != nil {
		response.Error(c, 500, "查询预测基准数据失败: "+err.Error())
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":  0,
		"data":  curve,
		"total": len(curve),
	})
}

// RecalculateBenchmark 手动重算预测基准库 (/api/ltv/recalculate-benchmark)
func (h *LtvHandler) RecalculateBenchmark(c *gin.Context) {
	if err := h.predictSvc.RecalculateAllBenchmarks(c.Request.Context()); err != nil {
		response.Error(c, 500, "重算基准库失败: "+err.Error())
		return
	}
	response.SuccessWithMsg(c, "LTV 预测基准库重新计算完成", nil)
}

// SyncSubscribeConfigs 手动触发拉取订阅配置版本库 (/api/ltv/sync-subscribe-configs)
func (h *LtvHandler) SyncSubscribeConfigs(c *gin.Context) {
	platformCode := c.DefaultQuery("platformCode", "rocnovel")
	count := 0
	if h.syncMgr != nil {
		var err error
		count, err = h.syncMgr.SyncConfigsForPlatform(c.Request.Context(), platformCode)
		if err != nil {
			response.Error(c, 500, "同步订阅配置失败: "+err.Error())
			return
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"code":              0,
		"msg":               "平台落地页与订阅配置版本数据同步完成",
		"savedVersionCount": count,
	})
}
