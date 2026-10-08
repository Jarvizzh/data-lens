# DataLens · 全域 LTV 与投流分析中台 (data-lens)

**DataLens** 是一套面向海外短剧与网文多平台买量投放（Overseas Short Drama & Web Novel Ad Campaigns）的企业级商业化数据分析中台。系统聚焦于**全域订单自动化同步清洗**、**实时 Cohort（同幼群）LTV 矩阵计算**、**新老客充值留存深度分析**，以及**基于数学模型的远期 ROI / 回本周期（Payback Period）预测**，为海外投放优化师与商业化团队提供实时、严谨的买量决策支撑。

---

> [!WARNING]
> ### ⚠️ 后台维护与架构演进特别说明 (Architecture & Maintenance Notice)
> 
> - **Java 后端 (`backend/`) 状态：已停止维护（Deprecated / Frozen）**  
>   原 Java (Spring Boot) 后端已全面冻结，仅保留作为历史代码归档与基线对照。**后续所有新功能研发、性能优化、渠道接入与线上生产部署均不再维护 Java 版本，后续将正式下线**。
> - **主流服务 (`go_backend/`) 状态：唯一活跃主力维护版本（Active Production）**  
>   基于 Go 语言构建的 `data-lens-server` 已全面通过线上全量业务与财务指标的 100% 严格一致性比对验证，具备常驻内存小（50MB~150MB）、高吞吐低延迟、并发拉取与自适应数学预测等显著优势，**为本项目唯一的长期迭代与生产交付版本**。

---

## 一、 项目工程结构

```
data-lens/
├── go_backend/      # 【核心主力】Go 后台服务 (data-lens-server) - 唯一持续迭代维护
├── frontend/        # 【前端系统】React 18 + Vite + Tailwind 响应式中台控制台
├── backend/         # 【已停止维护】原 Java Spring Boot 后台 (仅做历史归档与对照)
└── README.md        # 项目主总览文档 (当前文件)
```

---

## 二、 核心子系统指南

### 1. Go 后端核心服务 (`go_backend/`)
作为平台唯一核心服务端，提供 RESTful API 接口、多数据源同步引擎、定时调度及回本预测算法。
- **服务名称**：`data-lens-server`
- **主要技术栈**：Go 1.22+、Gin Web Framework、GORM、Shopspring Decimal、Robfig Cron v3、Uber Zap
- **核心文档**：
  - [go_backend/README.md](go_backend/README.md)：Go 后台完整技术架构、工程结构与本地开发指南。
  - [go_backend/DEPLOY.md](go_backend/DEPLOY.md)：生产环境（Alibaba Cloud Linux 3）静态编译打包、Docker 容器化与 Nginx 运维部署指南。
  - [go_backend/PAYBACK_MODEL.md](go_backend/PAYBACK_MODEL.md)：预测回本模型（Payback Prediction Model）数学推导、贝叶斯收缩算子与风控熔断机制技术白皮书。

### 2. 前端控制台系统 (`frontend/`)
为投放运营与管理者提供沉浸式的全域分析与交互决策界面。
- **系统名称**：`data-lens-frontend`
- **主要技术栈**：React 18、Vite、Lucide Icons、ECharts、Modern CSS Variables
- **核心功能**：
  - **LTV 核心报表**：Day1 ~ Day60 留存充值与实时 ROI 矩阵、主导订阅周期与留存率、预估回本天数展示；
  - **每日充值分析**：新老客充值金额与人数分层、ARPU、复购率大盘统计；
  - **平台充值汇总**：跨平台（中文在线 rocnovel、番茄司南 flicknovel 等）全量订单汇总大盘；
  - **财务月度结算**：按落地页分配与无主落地页维度的月度结算流水。

---

## 三、 本地快速启动指南

### 1. 启动 Go 后台服务 (`go_backend`)
```bash
cd go_backend

# 复制并配置本地运行配置
cp configs/config.example.yaml configs/config.yaml

# 启动服务 (默认监听 8080 端口)
go run cmd/server/main.go
```

### 2. 启动前端界面 (`frontend`)
```bash
cd frontend

# 安装依赖
npm install

# 启动本地开发服务 (支持代理转至 8080 后台)
npm run dev
```
本地访问：`http://localhost:5173`。

---

## 四、 生产构建与交付概览

生产环境采用**前端静态资源托管 + Go 静态二进制轻量容器化**的标准生产方案：

```bash
# 1. 前端生产打包 (生成静态资源 dist/)
cd frontend
npm run build

# 2. Go 后端静态交叉编译 (生成 data-lens-server-linux-amd64.tar.gz 发布包)
cd ../go_backend
./build_linux.sh amd64
```

详细的服务器部署、环境参数与 Docker 运行流程请查阅 [go_backend/DEPLOY.md](go_backend/DEPLOY.md)。
