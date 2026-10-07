package logger

import (
	"testing"

	"go_backend/internal/config"

	"go.uber.org/zap"
)

func TestNewLogger_Console(t *testing.T) {
	cfg := &config.LoggerConfig{
		Level:      "info",
		Format:     "console",
		ShowCaller: true,
	}

	log, err := NewLogger(cfg)
	if err != nil {
		t.Fatalf("failed to create console logger: %v", err)
	}
	defer log.Sync()

	log.Info("Test console log message", zap.String("module", "test"), zap.Int("code", 200))
}

func TestNewLogger_JSON(t *testing.T) {
	cfg := &config.LoggerConfig{
		Level:      "debug",
		Format:     "json",
		ShowCaller: true,
	}

	log, err := NewLogger(cfg)
	if err != nil {
		t.Fatalf("failed to create json logger: %v", err)
	}
	defer log.Sync()

	log.Debug("Test json log message", zap.String("trace_id", "xyz123"))
}
