package pool

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"sync/atomic"

	"go.uber.org/zap"
)

var (
	ErrPoolClosed   = errors.New("worker pool is closed")
	ErrQueueTimeout = errors.New("submit task timed out: worker queue is full")
)

// WorkerPool 具有并发硬上限与缓冲队列的高性能安全协程池
type WorkerPool struct {
	name      string
	capacity  int
	queue     chan func()
	wg        sync.WaitGroup
	closed    atomic.Bool
	running   atomic.Int32
	logger    *zap.Logger
	closeOnce sync.Once
}

// NewWorkerPool 创建指定名称、容量与队列深度的协程池
func NewWorkerPool(name string, capacity int, queueSize int, logger *zap.Logger) *WorkerPool {
	if capacity <= 0 {
		capacity = 8
	}
	if queueSize <= 0 {
		queueSize = 256
	}
	if logger == nil {
		logger = zap.NewNop()
	}

	p := &WorkerPool{
		name:     name,
		capacity: capacity,
		queue:    make(chan func(), queueSize),
		logger:   logger,
	}

	// 预先拉起固定数量的常驻 Worker 协程
	for i := 0; i < capacity; i++ {
		p.wg.Add(1)
		go func(workerID int) {
			defer p.wg.Done()
			for task := range p.queue {
				p.running.Add(1)
				task()
				p.running.Add(-1)
			}
		}(i)
	}

	return p
}

// Submit 提交异步任务，若队列满则阻塞直到有空闲槽位，内部自动注入 Panic 恢复保护
func (p *WorkerPool) Submit(taskName string, fn func()) error {
	return p.SubmitWithContext(context.Background(), taskName, fn)
}

// SubmitWithContext 带超时/取消上下文的提交任务，内部杜绝向已关闭通道发送数据引发的 Panic 竞态
func (p *WorkerPool) SubmitWithContext(ctx context.Context, taskName string, fn func()) (err error) {
	if fn == nil {
		return nil
	}
	if p.closed.Load() {
		return ErrPoolClosed
	}

	defer func() {
		if r := recover(); r != nil {
			err = ErrPoolClosed
		}
	}()

	wrapped := func() {
		SafeExecute(p.logger, fmt.Sprintf("%s:%s", p.name, taskName), fn)
	}

	select {
	case p.queue <- wrapped:
		return nil
	case <-ctx.Done():
		return fmt.Errorf("%w: %s", ErrQueueTimeout, ctx.Err().Error())
	}
}

// Running 返回当前正在执行任务的 Worker 数量
func (p *WorkerPool) Running() int32 {
	return p.running.Load()
}

// Waiting 返回队列中排队等待执行的任务数量
func (p *WorkerPool) Waiting() int {
	return len(p.queue)
}

// Cap 返回协程池的最大 Worker 并发容量
func (p *WorkerPool) Cap() int {
	return p.capacity
}

// Shutdown 优雅关闭协程池：拒绝新任务提交，并等待队列中已有在途任务执行完成
func (p *WorkerPool) Shutdown(ctx context.Context) error {
	p.closeOnce.Do(func() {
		p.closed.Store(true)
		close(p.queue)
	})

	done := make(chan struct{})
	go func() {
		p.wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		p.logger.Info("Worker pool stopped cleanly",
			zap.String("pool_name", p.name),
		)
		return nil
	case <-ctx.Done():
		p.logger.Warn("Worker pool shutdown timed out before workers finished",
			zap.String("pool_name", p.name),
			zap.Error(ctx.Err()),
		)
		return ctx.Err()
	}
}

// ==============================================================================
// 全局单例协程池管理 (业务协程池 + 三方调用协程池 物理舱壁隔离)
// ==============================================================================

var (
	globalMu             sync.RWMutex
	globalBizPool        *WorkerPool
	globalThirdPartyPool *WorkerPool
)

// InitGlobalPools 初始化全局业务协程池与三方 API 协程池
func InitGlobalPools(logger *zap.Logger) {
	globalMu.Lock()
	defer globalMu.Unlock()

	// 1. 业务异步协程池：处理主账号级联重算、缓存异步失效等内部重计算 (容量 16, 队列 1024)
	globalBizPool = NewWorkerPool("BizPool", 16, 1024, logger)

	// 2. 三方调用协程池：处理中文在线/番茄司南外部 API 并发拉取 (容量 8, 队列 1024)
	// 严格限制最大并发为 8，遵守三方接口风控配额，防爆三方网关
	globalThirdPartyPool = NewWorkerPool("ThirdPartyPool", 8, 1024, logger)

	logger.Info("Global worker pools initialized successfully",
		zap.Int("biz_pool_cap", 16),
		zap.Int("third_party_pool_cap", 8),
	)
}

// GetBizPool 获取全局业务协程池
func GetBizPool() *WorkerPool {
	globalMu.RLock()
	defer globalMu.RUnlock()
	return globalBizPool
}

// GetThirdPartyPool 获取全局三方调用协程池
func GetThirdPartyPool() *WorkerPool {
	globalMu.RLock()
	defer globalMu.RUnlock()
	return globalThirdPartyPool
}

// ShutdownGlobal 并发优雅关闭全局双协程池，使两个物理隔离池共享完整的停机 Context 超时
func ShutdownGlobal(ctx context.Context) error {
	globalMu.RLock()
	biz := globalBizPool
	tp := globalThirdPartyPool
	globalMu.RUnlock()

	var wg sync.WaitGroup
	var errMu sync.Mutex
	var errs []error

	if biz != nil {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if err := biz.Shutdown(ctx); err != nil {
				errMu.Lock()
				errs = append(errs, err)
				errMu.Unlock()
			}
		}()
	}

	if tp != nil {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if err := tp.Shutdown(ctx); err != nil {
				errMu.Lock()
				errs = append(errs, err)
				errMu.Unlock()
			}
		}()
	}

	wg.Wait()
	return errors.Join(errs...)
}
