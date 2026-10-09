package locker

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

func TestTaskLocker_MemoryLockAndRelease(t *testing.T) {
	l := NewTaskLocker(nil, zap.NewNop())
	ctx := context.Background()
	key := "test_lock_key"

	unlock1, acquired1, err := l.TryLock(ctx, key, 5*time.Second)
	if err != nil || !acquired1 {
		t.Fatalf("first lock should succeed, got acquired: %v, err: %v", acquired1, err)
	}

	// 并发重复加锁应失败
	_, acquired2, err := l.TryLock(ctx, key, 5*time.Second)
	if err != nil {
		t.Fatalf("second lock should return no error, got: %v", err)
	}
	if acquired2 {
		t.Fatalf("second lock should fail because first lock is active")
	}

	// 释放锁
	unlock1()

	// 再次获取应成功
	unlock3, acquired3, err := l.TryLock(ctx, key, 5*time.Second)
	if err != nil || !acquired3 {
		t.Fatalf("lock after release should succeed, got acquired: %v, err: %v", acquired3, err)
	}
	unlock3()
}

func TestTaskLocker_TTLExpiration(t *testing.T) {
	l := NewTaskLocker(nil, zap.NewNop())
	ctx := context.Background()
	key := "test_ttl_key"

	// 加锁，设置 50ms TTL
	_, acquired1, err := l.TryLock(ctx, key, 50*time.Millisecond)
	if err != nil || !acquired1 {
		t.Fatalf("initial lock failed: %v", err)
	}

	// 立即再次加锁应失败
	_, acquired2, _ := l.TryLock(ctx, key, 50*time.Millisecond)
	if acquired2 {
		t.Fatal("lock should be busy")
	}

	// 等待过期
	time.Sleep(60 * time.Millisecond)

	// 过期后重新加锁应成功
	unlock3, acquired3, _ := l.TryLock(ctx, key, 50*time.Millisecond)
	if !acquired3 {
		t.Fatal("lock should be acquired after TTL expiration")
	}
	unlock3()
}

func TestTaskLocker_TryLockMulti(t *testing.T) {
	l := NewTaskLocker(nil, zap.NewNop())
	ctx := context.Background()

	// 1. 获取 multi lock
	unlock1, acquired1, err := l.TryLockMulti(ctx, 5*time.Second, "key_a", "key_b")
	if err != nil || !acquired1 {
		t.Fatalf("first multi lock should succeed: %v, %v", acquired1, err)
	}

	// 2. 尝试获取其中已被锁定的 key_b，应失败
	_, acquired2, _ := l.TryLock(ctx, "key_b", 5*time.Second)
	if acquired2 {
		t.Fatal("key_b should be locked")
	}

	// 3. 尝试获取 multi lock 其中包含被锁定的 key，应全部回滚并返回失败
	_, acquired3, _ := l.TryLockMulti(ctx, 5*time.Second, "key_c", "key_a")
	if acquired3 {
		t.Fatal("multi lock containing key_a should fail")
	}

	// 验证 key_c 是否被正确回滚释放
	unlockC, acquiredC, _ := l.TryLock(ctx, "key_c", 5*time.Second)
	if !acquiredC {
		t.Fatal("key_c should have been rolled back and available")
	}
	unlockC()

	// 4. 释放第一个 multi lock
	unlock1()

	// 5. 再次获取 key_a 和 key_b 应成功
	unlock4, acquired4, err := l.TryLockMulti(ctx, 5*time.Second, "key_a", "key_b")
	if err != nil || !acquired4 {
		t.Fatalf("second multi lock after unlock should succeed: %v, %v", acquired4, err)
	}
	unlock4()
}

func TestTaskLocker_GuardGin(t *testing.T) {
	gin.SetMode(gin.TestMode)
	l := NewTaskLocker(nil, zap.NewNop())

	w1 := httptest.NewRecorder()
	c1, _ := gin.CreateTestContext(w1)
	c1.Request = httptest.NewRequest(http.MethodPost, "/test", nil)

	unlock1, ok1 := l.GuardGin(c1, "gin_key", 5*time.Second, "任务执行中")
	if !ok1 || unlock1 == nil {
		t.Fatal("first GuardGin should succeed")
	}

	w2 := httptest.NewRecorder()
	c2, _ := gin.CreateTestContext(w2)
	c2.Request = httptest.NewRequest(http.MethodPost, "/test", nil)

	unlock2, ok2 := l.GuardGin(c2, "gin_key", 5*time.Second, "任务执行中")
	if ok2 || unlock2 != nil {
		t.Fatal("second GuardGin should fail")
	}
	var resp struct {
		Code int    `json:"code"`
		Msg  string `json:"msg"`
	}
	_ = json.Unmarshal(w2.Body.Bytes(), &resp)
	if resp.Code != http.StatusConflict {
		t.Fatalf("expected JSON code 409 Conflict, got %d", resp.Code)
	}

	unlock1()

	w3 := httptest.NewRecorder()
	c3, _ := gin.CreateTestContext(w3)
	c3.Request = httptest.NewRequest(http.MethodPost, "/test", nil)

	unlock3, ok3 := l.GuardGin(c3, "gin_key", 5*time.Second, "任务执行中")
	if !ok3 || unlock3 == nil {
		t.Fatal("GuardGin after release should succeed")
	}
	unlock3()
}

func TestTaskLocker_AcquireOrSkip(t *testing.T) {
	l := NewTaskLocker(nil, zap.NewNop())
	ctx := context.Background()

	unlock1, ok1 := l.AcquireOrSkip(ctx, "cron_key", 5*time.Second, "Cron Job A")
	if !ok1 || unlock1 == nil {
		t.Fatal("first AcquireOrSkip should succeed")
	}

	unlock2, ok2 := l.AcquireOrSkip(ctx, "cron_key", 5*time.Second, "Cron Job A")
	if ok2 || unlock2 != nil {
		t.Fatal("second AcquireOrSkip should return false")
	}

	unlock1()

	unlock3, ok3 := l.AcquireOrSkip(ctx, "cron_key", 5*time.Second, "Cron Job A")
	if !ok3 || unlock3 == nil {
		t.Fatal("AcquireOrSkip after unlock should succeed")
	}
	unlock3()
}


