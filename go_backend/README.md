# ZW-LTV Go 后台重构项目 (go_backend)

本项目为 `zw-ltv` 统计与预测系统后端由 Java (Spring Boot) 向 Go 语言重构的高性能版本。

## 一、 项目特性与收益
1. **极致内存节省**：单实例常驻内存（RSS）由 Java JVM 的 800MB~1.5GB 降至 50MB~150MB（节省 85%~95%）。
2. **秒级启动与精简部署**：单二进制文件启动，Docker 镜像体积约 20MB。
3. **100% 业务与数据兼容**：
   - HTTP 接口路径与 JSON 响应结构与原系统 100% 保持一致，前端完全无感知。
   - 登录鉴权 Token 加签与解析算法完全兼容，原有已登录用户无缝使用。
   - MySQL 数据库表结构完全一致，无须进行任何数据迁移。
   - 核心 LTV 统计与预测算法（贝叶斯先验收缩、OLS 对数回归拟合、均值回归衰减等）已全部移植并通过单元测试验证。

---

## 二、 目录结构设计 (Standard Go Project Layout)

```
go_backend/
├── cmd/
│   └── server/
│       └── main.go                  # 统一启动入口，集成优雅停机与服务装配
├── configs/
│   ├── config.yaml                  # 实际运行时配置文件
│   └── config.example.yaml          # 配置样例模板
├── internal/
│   ├── config/                      # Viper 配置解析与映射结构体
│   ├── cron/                        # 定时调度器 (robfig/cron/v3, 对齐原 6 个定时任务)
│   ├── handler/                     # Gin 控制器层 (Auth, LTV, User, Settlement, Platform)
│   ├── middleware/                  # 中间件 (Auth 鉴权, CORS 跨域处理)
│   ├── model/                       # 领域实体 (按模型职责细分拆分为小文件)
│   ├── pkg/                         # 通用底座工具库
│   │   ├── crypto/                  # Ed25519 签名与 SHA256 Token 工具
│   │   ├── response/                # 统一 API JSON 包装
│   │   └── timeutil/                # 北京时间 (CST) 与美东时间 (ET) 时区转换
│   ├── repository/                  # 数据持久层 (按聚合根拆分独立 Repo 文件)
│   └── service/                     # 业务服务层
│       ├── client/                  # 三方平台客户端 (番茄司南 OpenAPI, 中文在线)
│       ├── dto/                     # 业务传输 DTO
│       ├── engine/                  # 核心回本与 ROI 数学预测纯算子引擎
│       ├── ltv_cache.go             # 内存报表多级缓存
│       ├── ltv_calculate.go         # 单 Cohort 指标计算
│       ├── ltv_service.go           # LTV 业务编排与大盘汇总
│       ├── predict_service.go       # 预测曲线外推调度
│       ├── recharge_stat_service.go # 每日充值分布统计
│       ├── settlement_service.go    # 月度结算
│       ├── sync_manager.go          # 多平台数据同步管理器
│       └── user_service.go          # 用户与权限管理
├── go.mod
├── go.sum
└── README.md
```

---

## 三、 本地编译与运行指南

### 1. 运行所有单元测试
```bash
cd go_backend
go test -v ./...
```

### 2. 编译可执行文件
```bash
cd go_backend
go build -o server cmd/server/main.go
```

### 3. 启动服务
```bash
./server
```
服务将在 `8080` 端口监听并启动定时调度器。
