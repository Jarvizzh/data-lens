package handler

import (
	"strconv"

	"go_backend/internal/pkg/response"
	"go_backend/internal/service"

	"github.com/gin-gonic/gin"
	"github.com/shopspring/decimal"
)

type SettlementHandler struct {
	settleSvc *service.SettlementService
}

func NewSettlementHandler(settleSvc *service.SettlementService) *SettlementHandler {
	return &SettlementHandler{settleSvc: settleSvc}
}

type SaveSettlementRequest struct {
	SettlementType           string          `json:"settlementType"`
	TargetUserID             *int64          `json:"targetUserId"`
	MonthStr                 string          `json:"monthStr"`
	SettledRefundAmount      decimal.Decimal `json:"settledRefundAmount"`
	MonthSettledRefundAmount decimal.Decimal `json:"monthSettledRefundAmount"`
	CrossPeriodRefundAmount  decimal.Decimal `json:"crossPeriodRefundAmount"`
	ShareRatio               decimal.Decimal `json:"shareRatio"`
	ChannelFeeRate           decimal.Decimal `json:"channelFeeRate"`
	Remark                   string          `json:"remark"`
}

func (h *SettlementHandler) GetConfigs(c *gin.Context) {
	settlementType := c.Query("settlementType")
	monthStr := c.Query("monthStr")
	var targetUserID *int64
	if uidStr := c.Query("targetUserId"); uidStr != "" {
		if uid, err := strconv.ParseInt(uidStr, 10, 64); err == nil {
			targetUserID = &uid
		}
	}

	configs, err := h.settleSvc.GetSettlementConfigs(c.Request.Context(), settlementType, targetUserID, monthStr)
	if err != nil {
		response.Error(c, 500, "查询结算配置失败: "+err.Error())
		return
	}
	response.Success(c, configs)
}

func (h *SettlementHandler) SaveConfig(c *gin.Context) {
	var req SaveSettlementRequest
	if err := c.ShouldBindJSON(&req); err != nil || req.MonthStr == "" {
		response.Error(c, 400, "参数错误")
		return
	}

	err := h.settleSvc.SaveSettlementConfig(c.Request.Context(), service.SaveSettlementParam{
		SettlementType:           req.SettlementType,
		TargetUserID:             req.TargetUserID,
		MonthStr:                 req.MonthStr,
		SettledRefundAmount:      req.SettledRefundAmount,
		MonthSettledRefundAmount: req.MonthSettledRefundAmount,
		CrossPeriodRefundAmount:  req.CrossPeriodRefundAmount,
		ShareRatio:               req.ShareRatio,
		ChannelFeeRate:           req.ChannelFeeRate,
		Remark:                   req.Remark,
	})
	if err != nil {
		response.Error(c, 500, "保存结算配置失败: "+err.Error())
		return
	}

	response.SuccessWithMsg(c, "保存成功", nil)
}
