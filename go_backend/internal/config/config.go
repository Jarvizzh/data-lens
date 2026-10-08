package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/spf13/viper"
)

type Config struct {
	Server     ServerConfig     `mapstructure:"server"`
	Database   DatabaseConfig   `mapstructure:"database"`
	Order      OrderAPIConfig   `mapstructure:"order"`
	Flicknovel FlicknovelConfig `mapstructure:"flicknovel"`
	App        AppConfig        `mapstructure:"app"`
	Logger     LoggerConfig     `mapstructure:"logger"`
}

type ServerConfig struct {
	Port int `mapstructure:"port"`
}

type DatabaseConfig struct {
	MySQL MySQLConfig `mapstructure:"mysql"`
}

type MySQLConfig struct {
	DSN                string `mapstructure:"dsn"`
	MaxIdleConns       int    `mapstructure:"max_idle_conns"`
	MaxOpenConns       int    `mapstructure:"max_open_conns"`
	ConnMaxLifetimeMin int    `mapstructure:"conn_max_lifetime_min"`
}

type OrderAPIConfig struct {
	API RocnovelAPI `mapstructure:"api"`
}

type RocnovelAPI struct {
	URL           string `mapstructure:"url"`
	Authorization string `mapstructure:"authorization"`
	Cookie        string `mapstructure:"cookie"`
	ClientGroupID string `mapstructure:"client_group_id"`
}

type FlicknovelConfig struct {
	API FlicknovelAPI `mapstructure:"api"`
}

type FlicknovelAPI struct {
	BaseURL          string `mapstructure:"base_url"`
	CompanyID        string `mapstructure:"company_id"`
	PrivateKey       string `mapstructure:"private_key"`
	DefaultEmail     string `mapstructure:"default_email"`
	DefaultDistAppID int64  `mapstructure:"default_dist_app_id"`
}

type AppConfig struct {
	Auth AuthConfig `mapstructure:"auth"`
}

type AuthConfig struct {
	Username        string `mapstructure:"username"`
	Password        string `mapstructure:"password"`
	TokenExpireDays int    `mapstructure:"token_expire_days"`
	SecretKey       string `mapstructure:"secret_key"`
}

type LoggerConfig struct {
	Level      string `mapstructure:"level"`
	Format     string `mapstructure:"format"`
	ShowCaller bool   `mapstructure:"show_caller"`
}

var GlobalConfig Config

func LoadConfig(configPath string) (*Config, error) {
	v := viper.New()
	v.SetConfigFile(configPath)
	v.SetConfigType("yaml")
	v.AutomaticEnv()
	v.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))

	if err := v.ReadInConfig(); err != nil {
		return nil, fmt.Errorf("read config file failed: %w", err)
	}

	var cfg Config
	if err := v.Unmarshal(&cfg); err != nil {
		return nil, fmt.Errorf("unmarshal config failed: %w", err)
	}

	if portEnv := os.Getenv("SERVER_PORT"); portEnv != "" {
		if p, err := strconv.Atoi(portEnv); err == nil && p > 0 {
			cfg.Server.Port = p
		}
	} else if portEnv := os.Getenv("PORT"); portEnv != "" {
		if p, err := strconv.Atoi(portEnv); err == nil && p > 0 {
			cfg.Server.Port = p
		}
	}

	if dsnEnv := os.Getenv("MYSQL_DSN"); dsnEnv != "" {
		cfg.Database.MySQL.DSN = dsnEnv
	}

	if cfg.Server.Port == 0 {
		cfg.Server.Port = 8080
	}
	if strings.TrimSpace(cfg.App.Auth.Username) == "" || strings.TrimSpace(cfg.App.Auth.Password) == "" || strings.TrimSpace(cfg.App.Auth.SecretKey) == "" {
		return nil, fmt.Errorf("app.auth (username, password, secret_key) must be explicitly configured")
	}
	if cfg.App.Auth.TokenExpireDays == 0 {
		cfg.App.Auth.TokenExpireDays = 7
	}
	if cfg.Logger.Level == "" {
		cfg.Logger.Level = "info"
	}
	if cfg.Logger.Format == "" {
		cfg.Logger.Format = "console"
	}
	if !v.IsSet("logger.show_caller") {
		cfg.Logger.ShowCaller = true
	}

	GlobalConfig = cfg
	return &cfg, nil
}
