package repository

import (
	"fmt"
	"log"
	"os"
	"time"

	"go_backend/internal/config"
	"go_backend/internal/model"

	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	gormlogger "gorm.io/gorm/logger"
)

var DB *gorm.DB

func InitDB(cfg *config.MySQLConfig) (*gorm.DB, error) {
	newLogger := gormlogger.New(
		log.New(os.Stdout, "\r\n", log.LstdFlags),
		gormlogger.Config{
			SlowThreshold:             500 * time.Millisecond,
			LogLevel:                  gormlogger.Warn,
			IgnoreRecordNotFoundError: true,
			Colorful:                  false,
		},
	)

	db, err := gorm.Open(mysql.Open(cfg.DSN), &gorm.Config{
		Logger:                 newLogger,
		SkipDefaultTransaction: true,
		PrepareStmt:            true,
	})
	if err != nil {
		return nil, fmt.Errorf("connect to mysql failed: %w", err)
	}

	sqlDB, err := db.DB()
	if err != nil {
		return nil, fmt.Errorf("get sql.DB failed: %w", err)
	}

	maxIdle := cfg.MaxIdleConns
	if maxIdle <= 0 {
		maxIdle = 10
	}
	maxOpen := cfg.MaxOpenConns
	if maxOpen <= 0 {
		maxOpen = 50
	}
	maxLifetime := cfg.ConnMaxLifetimeMin
	if maxLifetime <= 0 {
		maxLifetime = 30
	}

	sqlDB.SetMaxIdleConns(maxIdle)
	sqlDB.SetMaxOpenConns(maxOpen)
	sqlDB.SetConnMaxLifetime(time.Duration(maxLifetime) * time.Minute)

	// 自动安全同步缺失列（如新增的 day2/day3 留存统计字段，仅增加列不影响现有数据）
	_ = db.AutoMigrate(&model.LtvDailyStat{})

	DB = db
	return db, nil
}
