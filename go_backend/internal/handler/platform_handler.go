package handler

import (
	"go_backend/internal/pkg/response"
	"go_backend/internal/pkg/timeutil"
	"go_backend/internal/repository"
	"go_backend/internal/service"

	"github.com/gin-gonic/gin"
)

type PlatformHandler struct {
	platformRepo *repository.PlatformRepository
	syncMgr      *service.SyncManager
}

func NewPlatformHandler(platformRepo *repository.PlatformRepository, syncMgr *service.SyncManager) *PlatformHandler {
	return &PlatformHandler{
		platformRepo: platformRepo,
		syncMgr:      syncMgr,
	}
}

func (h *PlatformHandler) ListPlatforms(c *gin.Context) {
	list, err := h.platformRepo.FindAll(c.Request.Context())
	if err != nil {
		response.Error(c, 500, "查询平台失败: "+err.Error())
		return
	}
	response.Success(c, list)
}

func (h *PlatformHandler) TriggerSync(c *gin.Context) {
	startTime := c.DefaultQuery("startTime", "2026-07-10")
	endTime := c.DefaultQuery("endTime", timeutil.GetTodayCst())

	err := h.syncMgr.SyncOrdersAllPlatforms(c.Request.Context(), startTime, endTime)
	if err != nil {
		response.Error(c, 500, "触发同步失败: "+err.Error())
		return
	}
	response.SuccessWithMsg(c, "同步触发成功", nil)
}
