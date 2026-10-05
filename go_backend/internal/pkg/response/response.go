package response

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

type APIResponse struct {
	Code int         `json:"code"`
	Msg  string      `json:"msg"`
	Data interface{} `json:"data"`
}

func Success(c *gin.Context, data interface{}) {
	c.JSON(http.StatusOK, APIResponse{
		Code: 0,
		Msg:  "success",
		Data: data,
	})
}

func SuccessWithMsg(c *gin.Context, msg string, data interface{}) {
	c.JSON(http.StatusOK, APIResponse{
		Code: 0,
		Msg:  msg,
		Data: data,
	})
}

func Error(c *gin.Context, code int, msg string) {
	c.JSON(http.StatusOK, APIResponse{
		Code: code,
		Msg:  msg,
		Data: nil,
	})
}

func Unauthorized(c *gin.Context, msg string) {
	c.JSON(http.StatusUnauthorized, APIResponse{
		Code: 401,
		Msg:  msg,
		Data: nil,
	})
}
