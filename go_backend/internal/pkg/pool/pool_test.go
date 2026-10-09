package pool

import (
	"context"
	"sync/atomic"
	"testing"
	"time"

	"go.uber.org/zap"
)

func TestSafeExecute_PanicRecovery(t *testing.T) {
	logger := zap.NewNop()
	panicked := false

	// 确认 panic 被捕获，测试进程不崩溃
	SafeExecute(logger, "test_panic_task", func() {
		panicked = true
		panic("simulated critical runtime panic!")
	})

	if !panicked {
		t.Fatal("expected task to execute and panic, but it did not")
	}
}

func TestSafeGo_PanicRecovery(t *testing.T) {
	logger := zap.NewNop()
	done := make(chan bool)

	SafeGo(logger, "test_safego_panic", func() {
		defer close(done)
		panic("simulated panic in SafeGo")
	})

	select {
	case <-done:
		// 成功退出，说明未导致主线程崩溃
	case <-time.After(500 * time.Millisecond):
		t.Fatal("SafeGo timed out")
	}
}

func TestWorkerPool_SubmitAndShutdown(t *testing.T) {
	logger := zap.NewNop()
	pool := NewWorkerPool("test_pool", 4, 16, logger)

	var counter atomic.Int32
	taskCount := 20

	for i := 0; i < taskCount; i++ {
		err := pool.Submit("increment_task", func() {
			time.Sleep(10 * time.Millisecond)
			counter.Add(1)
		})
		if err != nil {
			t.Fatalf("submit task failed: %v", err)
		}
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	if err := pool.Shutdown(ctx); err != nil {
		t.Fatalf("shutdown failed: %v", err)
	}

	if counter.Load() != int32(taskCount) {
		t.Fatalf("expected %d tasks completed, got %d", taskCount, counter.Load())
	}

	// 关闭后再提交应返回 ErrPoolClosed
	err := pool.Submit("after_close", func() {})
	if err != ErrPoolClosed {
		t.Fatalf("expected ErrPoolClosed, got: %v", err)
	}
}

func TestGlobalPools_InitAndGet(t *testing.T) {
	logger := zap.NewNop()
	InitGlobalPools(logger)

	biz := GetBizPool()
	if biz == nil || biz.Cap() != 16 {
		t.Fatalf("expected biz pool with capacity 16, got %v", biz)
	}

	thirdParty := GetThirdPartyPool()
	if thirdParty == nil || thirdParty.Cap() != 15 {
		t.Fatalf("expected third party pool with capacity 15, got %v", thirdParty)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()

	if err := ShutdownGlobal(ctx); err != nil {
		t.Fatalf("shutdown global pools failed: %v", err)
	}
}
