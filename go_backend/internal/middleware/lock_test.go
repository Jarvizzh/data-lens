package middleware

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"go_backend/internal/pkg/locker"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

func TestExclusiveLockMiddleware(t *testing.T) {
	gin.SetMode(gin.TestMode)
	l := locker.NewTaskLocker(nil, zap.NewNop())

	r := gin.New()
	r.POST("/task", ExclusiveLock(l, "middleware_key", 5*time.Second, "任务执行中"), func(c *gin.Context) {
		// 模拟耗时任务
		time.Sleep(100 * time.Millisecond)
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})

	// 第一个请求异步执行
	doneCh := make(chan struct{})
	go func() {
		w1 := httptest.NewRecorder()
		req1 := httptest.NewRequest(http.MethodPost, "/task", nil)
		r.ServeHTTP(w1, req1)
		close(doneCh)
	}()

	// 稍等 10ms 确保第一个请求已加锁
	time.Sleep(10 * time.Millisecond)

	// 第二个并发请求立即进入，应被拦截并返回 409
	w2 := httptest.NewRecorder()
	req2 := httptest.NewRequest(http.MethodPost, "/task", nil)
	r.ServeHTTP(w2, req2)

	var resp struct {
		Code int    `json:"code"`
		Msg  string `json:"msg"`
	}
	_ = json.Unmarshal(w2.Body.Bytes(), &resp)
	if resp.Code != http.StatusConflict {
		t.Fatalf("expected code 409 conflict, got %d", resp.Code)
	}

	<-doneCh

	// 第一个任务释放后，再次请求应成功
	w3 := httptest.NewRecorder()
	req3 := httptest.NewRequest(http.MethodPost, "/task", nil)
	r.ServeHTTP(w3, req3)
	if w3.Code != http.StatusOK {
		t.Fatalf("expected 200 OK after lock release, got %d", w3.Code)
	}
}
