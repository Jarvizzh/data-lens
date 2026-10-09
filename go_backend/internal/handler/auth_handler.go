package handler

import (
	"net/http"
	"strings"

	"go_backend/internal/config"
	"go_backend/internal/pkg/crypto"
	"go_backend/internal/pkg/response"
	"go_backend/internal/service"

	"github.com/gin-gonic/gin"
)

type AuthHandler struct {
	userSvc *service.UserService
}

func NewAuthHandler(userSvc *service.UserService) *AuthHandler {
	return &AuthHandler{userSvc: userSvc}
}

type LoginRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

type LoginResponseData struct {
	Token                  string `json:"token"`
	UserID                 int64  `json:"userId"`
	Username               string `json:"username"`
	Role                   string `json:"role"`
	ExpireDays             int    `json:"expireDays"`
	IsSettlement           int    `json:"isSettlement"`
	PermPredictPayback     int    `json:"permPredictPayback"`
	PermRoiPredict         int    `json:"permRoiPredict"`
	PermGlobalDistribution int    `json:"permGlobalDistribution"`
	PermExport             int    `json:"permExport"`
	PermSettlement         int    `json:"permSettlement"`
	PermVideoGen           int    `json:"permVideoGen"`
}

func (h *AuthHandler) Login(c *gin.Context) {
	var req LoginRequest
	if err := c.ShouldBindJSON(&req); err != nil || req.Username == "" || req.Password == "" {
		response.Error(c, 400, "请输入用户名和密码")
		return
	}

	user, err := h.userSvc.FindByUsername(c.Request.Context(), strings.TrimSpace(req.Username))
	if err != nil || user == nil {
		response.Error(c, 401, "账号或密码错误")
		return
	}

	if user.Status == 0 {
		response.Error(c, 403, "该账号已被禁用")
		return
	}

	if !h.userSvc.ValidatePassword(user, req.Password) {
		response.Error(c, 401, "账号或密码错误")
		return
	}

	cfg := config.GetGlobalConfig()
	expireDays := cfg.App.Auth.TokenExpireDays
	token := crypto.GenerateToken(user.ID, user.Username, user.Role, expireDays, cfg.App.Auth.SecretKey)

	c.JSON(http.StatusOK, gin.H{
		"code":                   0,
		"msg":                    "登录成功",
		"token":                  token,
		"userId":                 user.ID,
		"username":               user.Username,
		"role":                   user.Role,
		"expireDays":             expireDays,
		"isSettlement":           user.IsSettlement,
		"permPredictPayback":     user.PermPredictPayback,
		"permRoiPredict":         user.PermRoiPredict,
		"permGlobalDistribution": user.PermGlobalDistribution,
		"permExport":             user.PermExport,
		"permSettlement":         user.PermSettlement,
		"permVideoGen":           user.PermVideoGen,
	})
}

func (h *AuthHandler) Check(c *gin.Context) {
	authHeader := c.GetHeader("Authorization")
	token := ""
	if strings.HasPrefix(authHeader, "Bearer ") {
		token = strings.TrimSpace(authHeader[7:])
	} else if authHeader != "" {
		token = strings.TrimSpace(authHeader)
	}

	tokenInfo := crypto.ParseToken(token, config.GetGlobalConfig().App.Auth.SecretKey)
	if !tokenInfo.Valid {
		c.JSON(http.StatusUnauthorized, gin.H{"code": 401, "msg": "Token 已过期或无效"})
		return
	}

	user, err := h.userSvc.FindByID(c.Request.Context(), tokenInfo.UserID)
	if err != nil || user == nil {
		c.JSON(http.StatusOK, gin.H{
			"code":     0,
			"msg":      "Token 有效",
			"userId":   tokenInfo.UserID,
			"username": tokenInfo.Username,
			"role":     tokenInfo.Role,
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":                   0,
		"msg":                    "Token 有效",
		"userId":                 user.ID,
		"username":               user.Username,
		"role":                   user.Role,
		"isSettlement":           user.IsSettlement,
		"permPredictPayback":     user.PermPredictPayback,
		"permRoiPredict":         user.PermRoiPredict,
		"permGlobalDistribution": user.PermGlobalDistribution,
		"permExport":             user.PermExport,
		"permSettlement":         user.PermSettlement,
		"permVideoGen":           user.PermVideoGen,
	})
}
