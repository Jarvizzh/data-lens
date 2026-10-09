package handler

import (
	"strings"

	"go_backend/internal/config"
	"go_backend/internal/middleware"
	"go_backend/internal/pkg/response"
	"go_backend/internal/repository"
	"go_backend/internal/service/client/rocnovel"

	"github.com/gin-gonic/gin"
)

type TokenHandler struct {
	rocnovelClient *rocnovel.Client
	platformRepo   *repository.PlatformRepository
}

func NewTokenHandler(rocnovelClient *rocnovel.Client, platformRepo *repository.PlatformRepository) *TokenHandler {
	return &TokenHandler{
		rocnovelClient: rocnovelClient,
		platformRepo:   platformRepo,
	}
}

func (h *TokenHandler) GetToken(c *gin.Context) {
	u := middleware.GetCurrentUser(c)
	if !u.IsSuperAdmin() {
		response.Error(c, 403, "无权访问，API 设置仅超级管理员可见")
		return
	}

	cfg := config.GetGlobalConfig()
	auth := cfg.Order.API.Authorization
	cookie := cfg.Order.API.Cookie

	if h.rocnovelClient != nil {
		cAuth, cCookie := h.rocnovelClient.GetCredentials()
		if cAuth != "" {
			auth = cAuth
		}
		if cCookie != "" {
			cookie = cCookie
		}
	}

	if h.platformRepo != nil {
		if dbAuth, err := h.platformRepo.GetSystemConfig(c.Request.Context(), "API_AUTHORIZATION"); err == nil && dbAuth != "" {
			auth = dbAuth
		}
		if dbCookie, err := h.platformRepo.GetSystemConfig(c.Request.Context(), "API_COOKIE"); err == nil && dbCookie != "" {
			cookie = dbCookie
		}
	}

	response.Success(c, gin.H{
		"authorization": auth,
		"cookie":        cookie,
	})
}

func (h *TokenHandler) UpdateToken(c *gin.Context) {
	u := middleware.GetCurrentUser(c)
	if !u.IsSuperAdmin() {
		response.Error(c, 403, "无权访问，API 设置仅超级管理员可见")
		return
	}

	var req struct {
		Authorization string `json:"authorization"`
		Cookie        string `json:"cookie"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, 400, "参数错误")
		return
	}

	// 1. 原子更新全局配置
	config.UpdateGlobalOrderAPI(req.Authorization, req.Cookie)

	// 2. 线程安全同步已初始化的客户端
	if h.rocnovelClient != nil {
		h.rocnovelClient.UpdateCredentials(req.Authorization, req.Cookie)
	}

	// 3. 持久化落库
	if h.platformRepo != nil {
		if req.Authorization != "" {
			_ = h.platformRepo.SetSystemConfig(c.Request.Context(), "API_AUTHORIZATION", strings.TrimSpace(req.Authorization))
		}
		if req.Cookie != "" {
			_ = h.platformRepo.SetSystemConfig(c.Request.Context(), "API_COOKIE", strings.TrimSpace(req.Cookie))
		}
	}

	response.SuccessWithMsg(c, "Token 更新成功！", nil)
}
