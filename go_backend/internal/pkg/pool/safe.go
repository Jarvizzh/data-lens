package pool

import (
	"runtime/debug"

	"go.uber.org/zap"
)

// SafeExecute 安全执行函数，内部捕获任何 panic 并记录包含堆栈的 ERROR 日志，杜绝单点崩溃
func SafeExecute(logger *zap.Logger, taskName string, fn func()) {
	if fn == nil {
		return
	}
	defer func() {
		if r := recover(); r != nil {
			stack := debug.Stack()
			if logger != nil {
				logger.Error("PANIC_RECOVERED: Background task panicked",
					zap.String("task_name", taskName),
					zap.Any("panic_reason", r),
					zap.ByteString("stack_trace", stack),
				)
			}
		}
	}()
	fn()
}

// SafeGo 启动独立受保护的 Goroutine，保证内部未捕获的异常绝不拖垮宿主进程
func SafeGo(logger *zap.Logger, taskName string, fn func()) {
	if fn == nil {
		return
	}
	go func() {
		SafeExecute(logger, taskName, fn)
	}()
}
