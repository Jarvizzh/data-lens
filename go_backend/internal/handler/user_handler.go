package handler

import (
	"strconv"
	"strings"

	"go_backend/internal/middleware"
	"go_backend/internal/pkg/response"
	"go_backend/internal/service"

	"github.com/gin-gonic/gin"
)

type UserHandler struct {
	userSvc *service.UserService
}

func NewUserHandler(userSvc *service.UserService) *UserHandler {
	return &UserHandler{userSvc: userSvc}
}

type LandingPageUpdateRequest struct {
	PlatformCode   string   `json:"platformCode"`
	UserID         int64    `json:"userId"`
	LandingPageIDs []string `json:"landingPageIds"`
}

func (h *UserHandler) GetLandingPages(c *gin.Context) {
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

	pages, err := h.userSvc.GetLandingPages(c.Request.Context(), platformCode, targetUserID)
	if err != nil {
		response.Error(c, 500, "查询落地页失败: "+err.Error())
		return
	}

	response.Success(c, pages)
}

func (h *UserHandler) UpdateLandingPages(c *gin.Context) {
	var req LandingPageUpdateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, 400, "参数错误: "+err.Error())
		return
	}

	if req.UserID <= 0 {
		if u := middleware.GetCurrentUser(c); u != nil {
			req.UserID = u.UserID
		}
	}

	if err := h.userSvc.ReplaceLandingPages(c.Request.Context(), req.PlatformCode, req.UserID, req.LandingPageIDs); err != nil {
		response.Error(c, 500, "更新落地页配置失败: "+err.Error())
		return
	}

	response.SuccessWithMsg(c, "更新成功", nil)
}

// ListUsers 管理员查询所有用户列表
func (h *UserHandler) ListUsers(c *gin.Context) {
	users, err := h.userSvc.ListAllUsers(c.Request.Context())
	if err != nil {
		response.Error(c, 500, "查询用户列表失败: "+err.Error())
		return
	}
	response.Success(c, users)
}

type CreateUserRequest struct {
	Username               string `json:"username"`
	Password               string `json:"password"`
	Role                   string `json:"role"`
	IsMaster               int    `json:"isMaster"`
	IsSettlement           int    `json:"isSettlement"`
	PermPredictPayback     int    `json:"permPredictPayback"`
	PermRoiPredict         int    `json:"permRoiPredict"`
	PermGlobalDistribution int    `json:"permGlobalDistribution"`
	PermExport             int    `json:"permExport"`
	PermSettlement         int    `json:"permSettlement"`
	PermVideoGen           int    `json:"permVideoGen"`
	AllowedPlatforms       string `json:"allowedPlatforms"`
}

func (h *UserHandler) CreateUser(c *gin.Context) {
	var req CreateUserRequest
	if err := c.ShouldBindJSON(&req); err != nil || strings.TrimSpace(req.Username) == "" || req.Password == "" {
		response.Error(c, 400, "请输入用户名和密码")
		return
	}

	user, err := h.userSvc.CreateUser(c.Request.Context(), service.CreateUserParam{
		Username:               strings.TrimSpace(req.Username),
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
		response.Error(c, 400, "创建用户失败: "+err.Error())
		return
	}

	response.Success(c, user)
}
