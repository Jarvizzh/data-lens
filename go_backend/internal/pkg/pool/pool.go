package pool

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"go.uber.org/zap"
)

const (
	// DefaultSubmitTimeout 默认入队背压等待超时，杜绝通道满时永久死锁阻塞
	DefaultSubmitTimeout = 5 * time.Second
)

var (
	ErrPoolClosed   = errors.New("worker pool is closed")
	ErrQueueTimeout = errors.New("submit task timed out: worker queue is full")
)

// WorkerPool 具有并发硬上限与缓冲队列的高性能安全协程池
type WorkerPool struct {
	name           string
	capacity       int
	queue          chan func()
	defaultTimeout time.Duration
	wg             sync.WaitGroup
	closed         atomic.Bool
	running        atomic.Int32
	logger         *zap.Logger
	closeOnce      sync.Once
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
		name:           name,
		capacity:       capacity,
		queue:          make(chan func(), queueSize),
		defaultTimeout: DefaultSubmitTimeout,
		logger:         logger,
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

// SetDefaultTimeout 设置默认入队超时时间
func (p *WorkerPool) SetDefaultTimeout(timeout time.Duration) {
	if timeout > 0 {
		p.defaultTimeout = timeout
	}
}

// Submit 提交异步任务，若队列满则在 defaultTimeout 时间内尝试入队；
// 超时未入队则返回 ErrQueueTimeout，彻底消除队列满时的死锁挂起隐患
func (p *WorkerPool) Submit(taskName string, fn func()) error {
	ctx, cancel := context.WithTimeout(context.Background(), p.defaultTimeout)
	defer cancel()
	return p.SubmitWithContext(ctx, taskName, fn)
}

// TrySubmit 非阻塞快速入队，若队列已满或已关闭立即返回 false，零等待
func (p *WorkerPool) TrySubmit(taskName string, fn func()) bool {
	if fn == nil || p.closed.Load() {
		return false
	}
	wrapped := func() {
		SafeExecute(p.logger, fmt.Sprintf("%s:%s", p.name, taskName), fn)
	}
	select {
	case p.queue <- wrapped:
		return true
	default:
		return false
	}
}

// SubmitWithFallback 带有调用方兜底（Caller-Runs）策略的提交方法
// 若成功入队则异步执行；若队列已满或入队超时，自动降级在当前 Goroutine 同步执行，确保任务绝不丢失
func (p *WorkerPool) SubmitWithFallback(taskName string, fn func()) {
	if fn == nil {
		return
	}
	if err := p.Submit(taskName, fn); err != nil {
		p.logger.Warn("Worker queue saturated, falling back to synchronous execution (Caller-Runs)",
			zap.String("pool", p.name),
			zap.String("task", taskName),
			zap.Error(err),
		)
		SafeExecute(p.logger, fmt.Sprintf("%s:%s[caller_fallback]", p.name, taskName), fn)
	}
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
	InitGlobalPoolsWithCap(16, 15, logger)
}

// InitGlobalPoolsWithCap 支持指定容量初始化全局池
func InitGlobalPoolsWithCap(bizCap, tpCap int, logger *zap.Logger) {
	globalMu.Lock()
	defer globalMu.Unlock()

	if bizCap <= 0 {
		bizCap = 16
	}
	if tpCap <= 0 {
		tpCap = 15
	}

	// 1. 业务异步协程池：处理主账号级联重算、缓存异步失效等内部重计算 (默认容量 16, 队列 1024)
	globalBizPool = NewWorkerPool("BizPool", bizCap, 1024, logger)

	// 2. 三方调用协程池：处理中文在线/番茄司南外部 API 并发拉取 (容量 15, 队列 1024)
	// 并发 15 既能跑满网络吞吐，又极其安全平稳防范三方网关限流
	globalThirdPartyPool = NewWorkerPool("ThirdPartyPool", tpCap, 1024, logger)

	logger.Info("Global worker pools initialized successfully",
		zap.Int("biz_pool_cap", bizCap),
		zap.Int("third_party_pool_cap", tpCap),
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
