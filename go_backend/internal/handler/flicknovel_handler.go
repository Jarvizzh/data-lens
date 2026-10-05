package handler

import (
	"encoding/json"
	"strings"

	"go_backend/internal/middleware"
	"go_backend/internal/model"
	"go_backend/internal/pkg/response"
	"go_backend/internal/repository"
	"go_backend/internal/service"

	"github.com/gin-gonic/gin"
)

type FlicknovelHandler struct {
	syncMgr      *service.SyncManager
	platformRepo *repository.PlatformRepository
}

func NewFlicknovelHandler(
	syncMgr *service.SyncManager,
	platformRepo *repository.PlatformRepository,
) *FlicknovelHandler {
	return &FlicknovelHandler{
		syncMgr:      syncMgr,
		platformRepo: platformRepo,
	}
}

// SyncOrders 手动触发番茄司南订单同步 (/api/flicknovel/sync/orders)
func (h *FlicknovelHandler) SyncOrders(c *gin.Context) {
	var body struct {
		StartDate string `json:"startDate"`
		EndDate   string `json:"endDate"`
	}
	_ = c.ShouldBindJSON(&body)

	start := strings.TrimSpace(body.StartDate)
	if start == "" {
		start = "2026-09-16"
	}
	end := strings.TrimSpace(body.EndDate)

	count, err := h.syncMgr.SyncFlicknovelOrders(c.Request.Context(), start, end)
	if err != nil {
		response.Error(c, 500, "订单同步失败: "+err.Error())
		return
	}

	response.SuccessWithMsg(c, "番茄司南订单同步完成", gin.H{
		"syncedCount": count,
		"startDate":   start,
		"endDate":     end,
	})
}

// SyncRelations 手动触发番茄司南染色归因同步 (/api/flicknovel/sync/relations)
func (h *FlicknovelHandler) SyncRelations(c *gin.Context) {
	var body struct {
		StartDate string `json:"startDate"`
		EndDate   string `json:"endDate"`
	}
	_ = c.ShouldBindJSON(&body)

	start := strings.TrimSpace(body.StartDate)
	if start == "" {
		start = "2026-09-16"
	}
	end := strings.TrimSpace(body.EndDate)

	count, err := h.syncMgr.SyncFlicknovelRelations(c.Request.Context(), start, end)
	if err != nil {
		response.Error(c, 500, "染色归因记录同步失败: "+err.Error())
		return
	}

	response.SuccessWithMsg(c, "番茄司南染色归因记录同步完成", gin.H{
		"syncedCount": count,
		"startDate":   start,
		"endDate":     end,
	})
}

// SyncConfigs 手动触发番茄司南推广链/配置同步 (/api/flicknovel/sync/configs)
func (h *FlicknovelHandler) SyncConfigs(c *gin.Context) {
	err := h.syncMgr.SyncFlicknovelPromotionsAndTemplates(c.Request.Context())
	if err != nil {
		response.Error(c, 500, "推广链同步失败: "+err.Error())
		return
	}

	pCount, _ := h.syncMgr.GetFlicknovelRepo().CountPromotions(c.Request.Context())
	response.SuccessWithMsg(c, "番茄司南推广链接与配置同步完成", gin.H{
		"syncedConfigs": pCount,
	})
}

// SyncPromotionsAndTemplates 手动全量同步推广链接与充值模板 (/api/flicknovel/sync/promotions-and-templates)
func (h *FlicknovelHandler) SyncPromotionsAndTemplates(c *gin.Context) {
	err := h.syncMgr.SyncFlicknovelPromotionsAndTemplates(c.Request.Context())
	if err != nil {
		response.Error(c, 500, "同步推广与模板失败: "+err.Error())
		return
	}

	pCount, _ := h.syncMgr.GetFlicknovelRepo().CountPromotions(c.Request.Context())
	tCount, _ := h.syncMgr.GetFlicknovelRepo().CountTemplates(c.Request.Context())
	response.SuccessWithMsg(c, "推广链接与充值模板(v2)同步入库完成，内存字典已刷新", gin.H{
		"promotionsInCache": pCount,
		"templatesInCache":  tCount,
	})
}

// GetCacheStats 查看推广与充值模板缓存状态 (/api/flicknovel/cache/stats)
func (h *FlicknovelHandler) GetCacheStats(c *gin.Context) {
	pCount, _ := h.syncMgr.GetFlicknovelRepo().CountPromotions(c.Request.Context())
	tCount, _ := h.syncMgr.GetFlicknovelRepo().CountTemplates(c.Request.Context())
	response.Success(c, gin.H{
		"promotionsInCache": pCount,
		"templatesInCache":  tCount,
	})
}

// GetConfig 查询番茄司南配置状态（脱敏展示） (/api/flicknovel/config)
func (h *FlicknovelHandler) GetConfig(c *gin.Context) {
	fnClient := h.syncMgr.GetFlicknovelClient()
	if fnClient == nil {
		response.Success(c, gin.H{
			"configured": false,
		})
		return
	}

	baseURL, companyID, privateKey, defaultEmail := fnClient.GetConfig()
	maskedKey := "未配置"
	if len(privateKey) > 10 {
		maskedKey = privateKey[:6] + "******" + privateKey[len(privateKey)-4:]
	}

	response.Success(c, gin.H{
		"baseUrl":          baseURL,
		"companyId":        companyID,
		"privateKeyMasked": maskedKey,
		"defaultEmail":     defaultEmail,
		"configured":       strings.TrimSpace(privateKey) != "",
	})
}

// UpdateConfig 更新番茄司南配置（限超级管理员） (/api/flicknovel/config/update)
func (h *FlicknovelHandler) UpdateConfig(c *gin.Context) {
	u := middleware.GetCurrentUser(c)
	if u == nil || !strings.EqualFold(u.Role, "SUPER_ADMIN") {
		response.Error(c, 403, "仅超级管理员可修改番茄司南配置")
		return
	}

	var body struct {
		CompanyID    string `json:"companyId"`
		PrivateKey   string `json:"privateKey"`
		DefaultEmail string `json:"defaultEmail"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		response.Error(c, 400, "参数错误")
		return
	}

	fnClient := h.syncMgr.GetFlicknovelClient()
	if fnClient != nil {
		fnClient.UpdateCredentials(body.CompanyID, body.PrivateKey, body.DefaultEmail)
	}

	// 持久化到 platform_config
	if h.platformRepo != nil {
		cfg, _ := h.platformRepo.FindByCode(c.Request.Context(), "flicknovel")
		if cfg == nil {
			cfg = &model.PlatformConfig{
				PlatformCode: "flicknovel",
				PlatformName: "番茄司南",
				AuthType:     "ED25519_KEY",
				Status:       1,
			}
		}
		credMap := map[string]string{
			"companyId":    body.CompanyID,
			"privateKey":   body.PrivateKey,
			"defaultEmail": body.DefaultEmail,
		}
		credJSON, _ := json.Marshal(credMap)
		cfg.AuthCredentials = string(credJSON)
		_ = h.platformRepo.Save(c.Request.Context(), cfg)
	}

	response.SuccessWithMsg(c, "番茄司南配置已成功更新！", nil)
}
