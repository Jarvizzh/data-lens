package logger

import (
	"os"
	"strings"
	"time"

	"go_backend/internal/config"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

// customTimeEncoder 人类友好毫秒时间戳格式: 2006-01-02 15:04:05.000
func customTimeEncoder(t time.Time, enc zapcore.PrimitiveArrayEncoder) {
	enc.AppendString(t.Format("2006-01-02 15:04:05.000"))
}

// NewLogger 构建统一的企业级 Zap 日志记录器
// 支持:
// 1. console 格式 (默认): 终端彩色大写 Level、毫秒时间戳、短路径 caller、真实多行换行堆栈
// 2. json 格式: 机器采集专用标准 ISO8601 时间戳与紧凑结构化输出
func NewLogger(cfg *config.LoggerConfig) (*zap.Logger, error) {
	level := zapcore.InfoLevel
	if cfg != nil {
		switch strings.ToLower(strings.TrimSpace(cfg.Level)) {
		case "debug":
			level = zapcore.DebugLevel
		case "info":
			level = zapcore.InfoLevel
		case "warn", "warning":
			level = zapcore.WarnLevel
		case "error":
			level = zapcore.ErrorLevel
		}
	}

	format := "console"
	showCaller := true
	if cfg != nil {
		if strings.EqualFold(strings.TrimSpace(cfg.Format), "json") {
			format = "json"
		}
		showCaller = cfg.ShowCaller
	}

	encoderConfig := zapcore.EncoderConfig{
		TimeKey:        "time",
		LevelKey:       "level",
		NameKey:        "logger",
		CallerKey:      "caller",
		MessageKey:     "msg",
		StacktraceKey:  "stacktrace",
		LineEnding:     zapcore.DefaultLineEnding,
		EncodeDuration: zapcore.StringDurationEncoder,
	}

	var encoder zapcore.Encoder
	if format == "json" {
		// 生产机器采集模式 (JSON)
		encoderConfig.EncodeTime = zapcore.ISO8601TimeEncoder
		encoderConfig.EncodeLevel = zapcore.CapitalLevelEncoder
		encoderConfig.EncodeCaller = zapcore.ShortCallerEncoder
		encoder = zapcore.NewJSONEncoder(encoderConfig)
	} else {
		// 终端友好模式 (Console): 带终端彩色、人类可读时间、直观多行排版
		encoderConfig.EncodeTime = customTimeEncoder
		encoderConfig.EncodeLevel = zapcore.CapitalColorLevelEncoder
		encoderConfig.EncodeCaller = zapcore.ShortCallerEncoder
		encoder = zapcore.NewConsoleEncoder(encoderConfig)
	}

	core := zapcore.NewCore(
		encoder,
		zapcore.Lock(os.Stdout),
		level,
	)

	opts := []zap.Option{
		zap.AddStacktrace(zapcore.ErrorLevel), // 仅 Error 及以上级别打印 stacktrace
	}
	if showCaller {
		opts = append(opts, zap.AddCaller())
	}

	return zap.New(core, opts...), nil
}
