# DataLens 后端服务 (data-lens-server)

`data-lens-server` 是 **DataLens | 全域 LTV 与投流分析中台** 的核心后端服务，基于 Go 语言开发。服务面向海外短剧与网文多平台投流业务，提供高吞吐的订单数据同步清洗、实时同幼群（Cohort）LTV 矩阵计算、每日充值分布统计、基于数学模型的远期 ROI / 回本周期预测，以及月度财务结算等核心能力。

---

## 一、 系统架构与核心特性

1. **高性能与低资源消耗**：
   - 纯 Go 原生高并发协程架构，服务单实例常驻内存（RSS）稳定在 50MB ~ 150MB，高频并发计算无 GC 停顿抖动。
   - 单二进制无依赖运行，启动速度达毫秒级，生产容器镜像（基于 Alpine）仅约 30MB。
2. **多平台统一接入与扩展驱动**：
   - 内置统一的 `PlatformSyncer` 驱动引擎，目前已原生支持**中文在线（rocnovel）** 与 **番茄司南（flicknovel）**。
   - 支持多平台并发拉取、增量订单幂等落库（GORM OnConflict 批量合流）与并发错误聚合（`errors.Join`）。
3. **精准时区与金融级数值计算**：
   - 原生支持跨时区智能切分（国内平台基于北京时间 CST，海外平台基于协调世界时 UTC 与美东时间 ET）。
   - 全链路使用高精度定点数（`shopspring/decimal`）进行金额与费率计算，杜绝浮点数精度损耗。
4. **数学预测与推演引擎**：
   - 内置贝叶斯先验收缩、OLS 对数曲线拟合与均值回归衰减算法，自适应测算投放 Day1~Day90 累充走势与预估回本天数（Payback Days）。
5. **企业级权限与安全控制**：
   - 基于 SHA-256 与 Base64 加签的无状态 Token 鉴权，支持超级管理员（SUPER_ADMIN）、普通管理员（ADMIN）及多用户数据视图隔离。

---

## 二、 工程目录结构 (Standard Go Project Layout)

```
go_backend/
├── cmd/
│   └── server/
│       └── main.go                  # 服务启动主入口，装配配置、数据库连接、路由与优雅停机
├── configs/
│   ├── config.yaml                  # 本地/运行期配置文件
│   ├── config.example.yaml          # 配置模板样例
│   └── config.production.yaml       # 生产环境配置模板
├── internal/
│   ├── config/                      # Viper 配置映射结构体与环境覆盖逻辑
│   ├── cron/                        # 后台定时调度器 (平台数据同步与大盘重算)
│   ├── handler/                     # Gin Web API 控制器层 (Auth, LTV, User, Settlement, Platform)
│   ├── middleware/                  # 鉴权中间件、CORS 跨域与请求日志
│   ├── model/                       # 数据库实体与平台/用户枚举常量定义
│   ├── pkg/                         # 通用工具底座
│   │   ├── crypto/                  # Ed25519 签名与 Token 工具
│   │   ├── response/                # 统一 HTTP JSON 返回包装器
│   │   └── timeutil/                # CST / UTC / ET 时区转换与统一时间格式化
│   ├── repository/                  # GORM 数据访问与持久化层
│   └── service/                     # 核心业务服务层
│       ├── client/                  # 第三方数据源 OpenAPI 客户端 (番茄司南、中文在线)
│       ├── dto/                     # API 数据传输对象 (Data Transfer Objects)
│       ├── engine/                  # 纯数学计算与曲线拟合预测引擎
│       ├── daily_distribution_service.go # 每日充值分布与大盘汇总
│       ├── ltv_cache.go             # 内存报表多级缓存
│       ├── ltv_calculate.go         # 单 Cohort 指标与实测充值计算
│       ├── ltv_service.go           # LTV 核心业务编排
│       ├── predict_service.go       # 预测曲线外推与回本分析
│       ├── recharge_stat_service.go # 充值分布底层计算
│       ├── settlement_service.go    # 月度财务结算单
│       ├── sync_manager.go          # 多平台同步驱动管理中心
│       └── user_service.go          # 用户、角色与数据权限服务
├── build_linux.sh                   # Linux 生产交叉编译与打包脚本
├── Dockerfile                       # 生产环境容器构建配置
├── docker-compose.yaml              # 容器编排部署文件
├── DEPLOY.md                        # 生产部署与维护运维指引
├── go.mod
├── go.sum
└── README.md
```

---

## 三、 本地开发与运行

### 1. 环境依赖
- Go 1.22+
- MySQL 8.0+

### 2. 配置文件配置
在 `configs/config.yaml` 中配置数据库连接与第三方平台鉴权信息：
```yaml
server:
  port: 8080

database:
  mysql:
    dsn: "root:123456@tcp(127.0.0.1:3306)/data_lens?charset=utf8mb4&parseTime=True&loc=Asia%2FShanghai"
```

### 3. 运行测试
```bash
go test -v ./...
```

### 4. 启动服务
```bash
# 编译并运行
go run cmd/server/main.go

# 或直接构建二进制运行
go build -o server cmd/server/main.go
./server
```
服务启动后将在 `8080` 端口监听，并自动初始化后台定时同步与重算任务。

---

## 四、 生产构建与容器化部署

本项目采用纯静态编译构建（`CGO_ENABLED=0`），目标生产系统为 **Alibaba Cloud Linux 3 / CentOS / Ubuntu**。

### 1. 本地交叉编译打包
```bash
# 默认构建 Linux amd64 生产发布包 (若需 ARM 架构可传入 arm64)
./build_linux.sh amd64
```
构建产物将统一收拢至 `dist/` 目录：
- 可执行二进制：`dist/server`（约 24MB，静态去符号编译）
- 线上发布包：`dist/data-lens-server-linux-amd64.tar.gz`（约 8.9MB，含二进制、依赖配置与 Dockerfile）

### 2. 线上 Docker 容器一键部署
```bash
# 解压至服务器
mkdir -p /data/data-lens-server
tar -zxvf data-lens-server-linux-amd64.tar.gz -C /data/data-lens-server
cd /data/data-lens-server

# 构建镜像
docker build -t data-lens-server:latest .

# 使用 Docker Compose 启动 (生产环境默认监听 8098 端口)
docker compose up -d
```

> - 详细的生产上线清单、参数覆盖、Nginx 反代配置及故障回滚指引，请直接阅读 [DEPLOY.md](DEPLOY.md)。
> - 核心预测算法推导、贝叶斯收缩算子与风控熔断机制，请参阅 [PAYBACK_MODEL.md](PAYBACK_MODEL.md)。
