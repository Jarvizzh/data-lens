package locker

import (
	"context"
	"database/sql"
	"fmt"
	"net/http"
	"sync"
	"time"

	"go_backend/internal/pkg/response"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

const (
	LockKeySyncOrders    = "sync:orders"
	LockKeyCalcLtv       = "calc:ltv"
	LockKeyCalcDist      = "calc:dist"
	LockKeySyncConfigs   = "sync:configs"
	LockKeyCalcBenchmark = "calc:benchmark"
)

type lockEntry struct {
	expiresAt time.Time
}

// TaskLocker 具有内存与 MySQL 命名锁双层防护的排他任务协调器
type TaskLocker struct {
	db          *gorm.DB
	logger      *zap.Logger
	memoryLocks sync.Map // key -> lockEntry
	mu          sync.Mutex
}

// NewTaskLocker 创建任务排他锁管理器
func NewTaskLocker(db *gorm.DB, logger *zap.Logger) *TaskLocker {
	if logger == nil {
		logger = zap.NewNop()
	}
	return &TaskLocker{
		db:     db,
		logger: logger,
	}
}

// TryLock 尝试以非阻塞方式获取指定 Key 的排他锁
// ttl 为防死锁保底超时时间（通常推荐 5~15 分钟）
// 返回的 unlock 闭包保证可安全、幂等地释放锁
func (l *TaskLocker) TryLock(ctx context.Context, key string, ttl time.Duration) (unlock func(), acquired bool, err error) {
	if ttl <= 0 {
		ttl = 10 * time.Minute
	}
	now := time.Now()

	// 1. 本地内存锁预检与加锁 (进程内快速拒绝并发踩踏)
	l.mu.Lock()
	if val, loaded := l.memoryLocks.Load(key); loaded {
		entry := val.(lockEntry)
		if now.Before(entry.expiresAt) {
			l.mu.Unlock()
			return nil, false, nil
		}
		// 已过期则允许覆盖
	}
	l.memoryLocks.Store(key, lockEntry{expiresAt: now.Add(ttl)})
	l.mu.Unlock()

	// 2. MySQL 分布式会话锁 (SELECT GET_LOCK(key, 0))
	var dbConn *sql.Conn
	var dbUnlocked sync.Once
	if l.db != nil && l.db.Dialector != nil && l.db.Dialector.Name() == "mysql" {
		sqlDB, dbErr := l.db.DB()
		if dbErr == nil {
			// 获取独立专用连接以确保 GET_LOCK 与 RELEASE_LOCK 在同一连接会话中执行
			conn, connErr := sqlDB.Conn(ctx)
			if connErr == nil {
				dbLockName := fmt.Sprintf("datalens:%s", key)
				var lockRes int
				// 0 表示非阻塞尝试
				qErr := conn.QueryRowContext(ctx, "SELECT GET_LOCK(?, 0)", dbLockName).Scan(&lockRes)
				if qErr != nil || lockRes != 1 {
					// 数据库级锁获取失败
					_ = conn.Close()
					l.memoryLocks.Delete(key)
					return nil, false, qErr
				}
				dbConn = conn
			}
		}
	}

	var once sync.Once
	unlockFunc := func() {
		once.Do(func() {
			// 释放内存锁
			l.memoryLocks.Delete(key)

			// 释放数据库锁并归还连接
			if dbConn != nil {
				dbUnlocked.Do(func() {
					dbLockName := fmt.Sprintf("datalens:%s", key)
					var res int
					_ = dbConn.QueryRowContext(context.Background(), "SELECT RELEASE_LOCK(?)", dbLockName).Scan(&res)
					_ = dbConn.Close()
				})
			}
		})
	}

	return unlockFunc, true, nil
}

// TryLockMulti 顺序尝试获取多个锁，若其中任一锁获取失败，自动按逆序释放已获取的锁
func (l *TaskLocker) TryLockMulti(ctx context.Context, ttl time.Duration, keys ...string) (unlock func(), acquired bool, err error) {
	if len(keys) == 0 {
		return func() {}, true, nil
	}
	if len(keys) == 1 {
		return l.TryLock(ctx, keys[0], ttl)
	}

	unlocks := make([]func(), 0, len(keys))
	for _, key := range keys {
		u, ok, err := l.TryLock(ctx, key, ttl)
		if err != nil || !ok {
			// 释放已获取的所有锁（逆序释放）
			for i := len(unlocks) - 1; i >= 0; i-- {
				unlocks[i]()
			}
			return nil, false, err
		}
		unlocks = append(unlocks, u)
	}

	var once sync.Once
	return func() {
		once.Do(func() {
			for i := len(unlocks) - 1; i >= 0; i-- {
				unlocks[i]()
			}
		})
	}, true, nil
}

// GuardGin 针对 Gin HTTP 控制器的快速排他守卫
// 若获取到排他锁，返回有效 unlock 闭包与 acquired=true；
// 若锁已被占用或失败，自动向客户端写入 HTTP 409 冲突响应，并返回 (nil, false)；
// 若 l 为 nil（未注入排他锁或测试环境），安全放行返回 (noop, true)。
func (l *TaskLocker) GuardGin(c *gin.Context, key string, ttl time.Duration, busyMsg string) (unlock func(), acquired bool) {
	if l == nil {
		return func() {}, true
	}
	u, ok, err := l.TryLock(c.Request.Context(), key, ttl)
	if err != nil || !ok {
		response.Error(c, http.StatusConflict, busyMsg)
		return nil, false
	}
	return u, true
}

// GuardGinMulti 针对 Gin HTTP 控制器的复合多键排他守卫
func (l *TaskLocker) GuardGinMulti(c *gin.Context, ttl time.Duration, busyMsg string, keys ...string) (unlock func(), acquired bool) {
	if l == nil {
		return func() {}, true
	}
	u, ok, err := l.TryLockMulti(c.Request.Context(), ttl, keys...)
	if err != nil || !ok {
		response.Error(c, http.StatusConflict, busyMsg)
		return nil, false
	}
	return u, true
}

// AcquireOrSkip 定时调度/异步工作协程专用的排他获取方法
// 若成功获取锁，返回有效 unlock 闭包与 acquired=true；
// 若已被占用，自动打印 Warn 日志并返回 (nil, false)，便于调用方直接 return 跳过。
func (l *TaskLocker) AcquireOrSkip(ctx context.Context, key string, ttl time.Duration, taskName string) (unlock func(), acquired bool) {
	if l == nil {
		return func() {}, true
	}
	u, ok, err := l.TryLock(ctx, key, ttl)
	if err != nil || !ok {
		l.logger.Warn(fmt.Sprintf("%s skipped: lock held by another task", taskName), zap.String("key", key))
		return nil, false
	}
	return u, true
}

// RunExclusive 闭包式排他任务执行器
// 若成功获取锁则执行 task 并在完成时自动释放；若未获取锁且 onBusy != nil 则调用 onBusy
func (l *TaskLocker) RunExclusive(ctx context.Context, key string, ttl time.Duration, task func(), onBusy func()) bool {
	if l == nil {
		task()
		return true
	}
	u, ok, err := l.TryLock(ctx, key, ttl)
	if err != nil || !ok {
		if onBusy != nil {
			onBusy()
		}
		return false
	}
	defer u()
	task()
	return true
}


