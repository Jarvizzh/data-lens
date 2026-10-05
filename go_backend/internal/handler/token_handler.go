package handler

import (
	"strings"

	"go_backend/internal/config"
	"go_backend/internal/middleware"
	"go_backend/internal/pkg/response"
	"go_backend/internal/service/client/rocnovel"

	"github.com/gin-gonic/gin"
)

type TokenHandler struct {
	rocnovelClient *rocnovel.Client
}

func NewTokenHandler(rocnovelClient *rocnovel.Client) *TokenHandler {
	return &TokenHandler{rocnovelClient: rocnovelClient}
}

func (h *TokenHandler) GetToken(c *gin.Context) {
	u := middleware.GetCurrentUser(c)
	if u == nil || !strings.EqualFold(u.Role, "SUPER_ADMIN") {
		response.Error(c, 403, "无权访问，API 设置仅超级管理员可见")
		return
	}

	response.Success(c, gin.H{
		"authorization": config.GlobalConfig.Order.API.Authorization,
		"cookie":        config.GlobalConfig.Order.API.Cookie,
	})
}

func (h *TokenHandler) UpdateToken(c *gin.Context) {
	u := middleware.GetCurrentUser(c)
	if u == nil || !strings.EqualFold(u.Role, "SUPER_ADMIN") {
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

	if req.Authorization != "" {
		config.GlobalConfig.Order.API.Authorization = req.Authorization
	}
	if req.Cookie != "" {
		config.GlobalConfig.Order.API.Cookie = req.Cookie
	}

	response.SuccessWithMsg(c, "Token 更新成功！", nil)
}
