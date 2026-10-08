# 预测回本模型（Payback Prediction Model）详解

本文档详细说明 `data-lens` 统计与预测系统针对 **海外 H5 小说投放（Overseas H5 Web Novel Ad Campaigns）** 业务场景下的 **预测回本模型（Payback Prediction Model）** 的业务背景、数学原理、算法推导步骤、模块解耦架构设计与边界防守机制。

---

## 一、 业务背景与核心目标 (Business Context & Objectives)

### 1. 业务背景
在海外 H5 小说（Web Novel）广告投放中，团队主要通过 **Meta (Facebook)、TikTok、Google** 等渠道投放小说落地页（H5 Landing Page）进行买量获取读者（Cohort 批次）。

海外 H5 小说业务的变现与充值具有显著的行业特征：
1. **多元化变现模式**：
   - **单次/金币充值 (Coin Packs)**：读者在关键卡点章节（Paywall Chapter）购买金币解锁后续章节。
   - **VIP 订阅 (Weekly/Monthly Subscriptions)**：周订（Period = 7）、月订（Period = 30）等自动续费订阅模式。
2. **充值回弹与长尾追更 (Long-tail Repeat Purchases)**：
   - **首日爆发**：落地页诱导章节卡点触发读者首日冲动消费（D1 充值占比较高）。
   - **完本追更与续扣脉冲**：忠实读者在随后的第 7 天、14 天、30 天继续追更完本，或触发周卡/月卡自动续扣，产生长尾复购。
3. **关键决策需求**：
   优化师需要根据前 **3~7 天的早期充值与续扣数据**，精准预测当前落地页/广告账户批次在未来 365 天内的 LTV 增长曲线与回本周期（Payback Period），据此决定**账户消耗（Spend）加倍还是关停**。

### 2. 核心目标
- **高早期准确率**：在 D3 / D5 / D7 观察窗口下，平均绝对误差 MAE ≤ 2.5 天，±3 天命中率 > 85%。
- **混合变现建模**：完美适配单次金币充值复购与周订（Period = 7）、月订（Period = 30）等多订阅周期的划扣脉冲。
- **健壮的防守机制**：包含贝叶斯先验收缩、均值回归衰减、追更平盘熔断以及 OLS 对数拟合兜底。

---

## 二、 解耦架构设计 (Refactored Decoupled Architecture)

系统采用 **3 层解耦架构（编排层 - 独立引擎层 - 纯算子与助手工具层）** 与 **自下而上（Bottom-Up）独立 Cohort 叠加** 模式，彻底消除了类与类之间的双向/逆向依赖：

```mermaid
graph TD
    subgraph "1. 业务编排与门面层 (Orchestration & Facade)"
        A["LtvPredictService<br/>(负责Spring依赖注入、维度数据查询与服务编排)"]
        B["LtvPredictFacade<br/>(门面类，组装单Cohort及大盘PredictionResult DTO)"]
    end

    subgraph "2. 独立预测引擎层 (Engines)"
        C["PaybackPredictEngine<br/>(专职负责单Cohort及大盘交叠回本天数计算)"]
        D["RoiPredictEngine<br/>(专职负责D30/D60/D90里程碑充值与ROI预测)"]
    end

    subgraph "3. 纯算法算子与数据助手层 (Operators, Helpers & Constants)"
        E["CohortCurveExtrapolator<br/>(推导D1~D365每日充值曲线 & 贝叶斯放缩算子)"]
        F["CohortStatHelper<br/>(Cohort数据提取 & 追更/平盘停滞判定助手)"]
        G["PredictAlgorithmConstants<br/>(算法超参数与熔断区间集中管理常量类)"]
    end

    A -->|"1. 查询数据并调度外推"| E
    A -->|"2. 传入365天充值曲线"| B
    B --> C
    B --> D
    
    E -.-> F
    E -.-> G
    C -.-> F
    D -.-> F
    D -.-> G
```

---

## 三、 核心算法与推导步骤 (Core Algorithms & Mathematical Steps)

### Step 1: 建立或匹配小说基准曲线 (Benchmark Baseline Matching)

针对 Cohort 中解析出的订阅周期 P（单次/日订 P = 1, 周订 P = 7, 月订 P = 30 等）：
1. **订阅套餐配置动态匹配 (Subscription Config Versions)**：
   优先根据当前 Cohort 的上线时间 `launchTime` 从 `SubscriptionConfigVersion` 匹配当时的生效套餐：
   - 首充价格：`configFirstUsd = firstPriceCent / 100.0`
   - 续费价格：`configRenewUsd = renewPriceCent / 100.0`
2. **历史大盘真实基准线匹配 (Empirical Baseline)**：
   从数据库匹配该小说落地页/渠道维度（如维度类型 `USER` 或 `ALL`）由历史成熟 Cohort（Age ≥ 14 天）萃取出的网文基准线：
   - `baseRet[d]`：第 d 天的基准留存/续订率；
   - `baseArpu[d]`：第 d 天的基准单客 ARPU（含金币复购与自动续扣）。
3. **合成基准线兜底 (Synthetic Standard Benchmark Fallback)**：
   若无历史基准数据，优先采用套餐配置价；若全为空，启用默认兜底价格（**周订 $19.99 / 日订 $9.99**）：
   - **周订 / 月订**：在划扣节点 d = 1, P+1, 2P+1 ... 按衰减指数建模：
     `baseRet[d] = 0.55 ^ (cycleIndex - 1)`
   - **单次/日订**：按小说读者流失与追更规律的幂律衰减建模：
     `baseRet[d] = 1 / (d ^ 0.75)`
4. **90 天后小说完本尾部长尾衰减**：
   `baseRet[d] = baseRet[90] × (90 / d) ^ 1.2`  (d > 90)

---

### Step 2: 动态客单萃取与放缩因子计算 (`CohortCurveExtrapolator.computeOptimalScaleFactor`)

根据已知观察天数 maxDays = min(daysElapsed, 60)，计算当前小说批次的实际累计 ROI 与基准预期 ROI 总和：

`actualRoi = actualRecharge / spend`

`baseRoiSum = Σ (baseRet[d] × unitPrice_d × userCount) / spend`

#### 1. 单客充值力 (Realized ARPU) 贝叶斯动态萃取
对于大额金币充值批次，单客真实充值力可能远超单次订阅价格。系统通过贝叶斯加权吸收当前批次的真实单客充值水平：
`RealizedARPU = actualRecharge / (userCount × Σ baseRet[d])`
`EffectiveARPU = w_arpu × clamp(0.5 × BaseARPU, 8.0 × BaseARPU, RealizedARPU) + (1 - w_arpu) × BaseARPU`
其中时间置信度权重 `w_arpu = timeWeight(maxDays) × (userCount / (userCount + 5.0))`（7 天以内 `w_arpu = 0` 保证早期风控，14 天后平滑提升至 0.85）。

#### 2. 放缩因子贝叶斯先验收缩与小样本自适应松绑
- **极早期（maxDays ≤ 7 天）**：
  加入消耗金额加权先验收缩，防止早期低消耗大充值导致预测过度乐观：
  `α = (actualRoi + priorWeight) / (baseRoiSum + priorWeight)`
  限制 α ∈ [`EARLY_STAGE_MIN_ALPHA`, `EARLY_STAGE_MAX_ALPHA`]（即 `[0.80, 1.25]`），贝叶斯权重 `w = 0.15 + 0.10 × (maxDays / 7)`。
- **成熟期常规批次（maxDays > 7 天）**：
  限制 α ∈ [`MATURE_STAGE_MIN_ALPHA`, `MATURE_STAGE_MAX_ALPHA`]（即 `[0.60, 2.00]`），成熟期权重 `w = 0.85 + 0.10 × (min(46, maxDays - 14) / 46)`。
- **小样本活跃大户批次（N ≤ 5 且 C_7 ≥ 0.70 且 ROI ≥ 0.40）**：
  识别到土豪持续充值特征后，自适应松绑放缩上限至 `3.50`，远期衰减指数放宽至 `0.15`。

最终放缩因子：
`scaleFactor = w × α + (1.0 - w) × 1.0`

---

### Step 3: 未来 365 天充值曲线推导与双轨系综融合 (Curve Extrapolation & Ensemble)

从 t = maxDays + 1 至 365 天进行逐日外推：

1. **轨道 A：留存衰减基准外推曲线推导**：
   - 放缩系数向 1.0 平滑回归：`scaleDecay(t) = (maxDays / t) ^ SCALE_DECAY_EXPONENT`
   - 有效放缩因子：`effectiveScaleFactor(t) = 1.0 + (scaleFactor - 1.0) × scaleDecay(t)`
   - 周期续订自然衰减：`cycleDecay(t) = (7 / t) ^ CYCLE_DECAY_EXPONENT`  (t > 7)
   - 每日预测充值收入：`ΔR(t) = baseRet[t] × effectiveScaleFactor(t) × cycleDecay(t) × EffectiveARPU × userCount`
   - 若命中活跃大户特征，施加近 7 天真实充值日速度下界保护：`ΔR(t) = max(ΔR(t), DailyVelocity × (maxDays/t)^0.15)`
2. **轨道 B：成熟期 (D14+) 双轨 OLS 动量动态系综融合**：
   - 当 `maxDays ≥ 14` 且对数回归拟合优度 `R^2 ≥ 0.85` 时，根据 `R^2` 和观察天数动态计算融合权重 `λ ∈ [0, 0.45]`：
     `cumRecharge[t] = (1 - λ) × cumRecharge_A[t] + λ × (spend × (a × ln(t) + b))`

---

## 四、 关键边界条件与防守熔断机制 (Safeguards & Edge Cases)

```mermaid
flowchart TD
    Start["开始判定小说 Cohort 回本天数"] --> Check1{"历史已实现 ROI >= 1.0 ?"}
    Check1 -- "是" --> ReturnActual["返回历史首次回本天数 d (已回本)"]
    Check1 -- "否" --> Check2{"观察窗口 < 3 天 ?"}
    Check2 -- "是" --> ReturnNull["返回 null (数据不足，暂不预测)"]
    Check2 -- "否" --> Check3{"判定小说追更/续扣平盘 (isSubscriptionStagnant) ?"}
    Check3 -- "是" --> ReturnStagnant["返回 -1 (回本停滞)"]
    Check3 -- "否" --> Check4{"推导充值曲线在 365 天内到达消耗 ?"}
    Check4 -- "是" --> ReturnFutureDay["返回预测回本天数 t (1~365)"]
    Check4 -- "否" --> Check5{"365天充值仍在微幅增长 ?"}
    Check5 -- "是" --> ReturnOver365["返回 366 (>365天回本)"]
    Check5 -- "否" --> ReturnStagnant
```

### 1. 通用网文订阅/复购平盘停滞判定 (`CohortStatHelper.isSubscriptionStagnant`)
根据小说 Cohort 中解析出的最长订阅/复购周期 P_max，动态计算平盘观察窗口：

requiredFlatDays = max(`MIN_FLAT_DAYS`, P_max × `PERIOD_FLAT_MULTIPLIER`) （即 `max(6, P_max × 2)`）

- **单次/日订** (P ≤ 3)：连续 6 天充值增长 ≤ $0.01，表明读者弃书/完本停读，判为回本停滞。
- **周卡/周订** (P = 7)：连续 14 天（2 周）充值增长 ≤ $0.01，表明读者连续 2 期未续费，判为回本停滞。
- **月卡/月订** (P = 30)：连续 60 天（2 个月）充值增长 ≤ $0.01，表明读者连续 2 个月未续费，判为回本停滞。

若触发停滞，未来预测充值曲线平盘，回本天数返回 `-1`。

### 2. 成熟期 (D14+) 双轨 OLS 动量动态系综 (Ensemble)
当观察天数 $t \ge 14$ 且历史数据对数拟合优度 $R^2 \ge 0.85$ 时，系统自动激活双轨动态融合机制：
- **轨道 A**：留存衰减基准外推曲线（大盘宏观规律）；
- **轨道 B**：历史充值对数回归拟合曲线 $\text{ROI}(t) = a \cdot \ln(t) + b$（微观增长动量）；
- **融合加权**：$\text{FinalCurve}(t) = (1 - \lambda) \times \text{Curve}_A(t) + \lambda \times \text{Curve}_B(t)$，兼顾大盘宏观留存规律与当前批次的强增长惯性。

### 3. 小样本 (N ≤ 5) 活跃大户自适应松绑与单客 ARPU 贝叶斯萃取
针对小样本高客单批次（如仅有 2 名超级土豪读者）：
- **连续性指数 $C_7$ 识别**：若近 7 天内连续充值天数占比 $C_7 \ge 0.70$ 且实际 ROI $\ge 0.40$，动态松绑放缩上限至 3.5 并柔化远期衰减；
- **Realized ARPU 贝叶斯修正**：结合样本量与成熟期时间置信度，动态吸收单客高充值力，攻克极端小样本回本预测钝化问题。

### 4. 兜底线性对数拟合方法 (`CohortCurveExtrapolator.predictCohortOlsFallback`)
当缺少有效网文历史基准线时，利用历史充值点通过最小二乘法对数拟合 R(t) = a × ln(t) + b，当 a > 0.0001 时求得解析解：

t_payback = exp((1.0 - b) / a)

### 5. ROI 里程碑预测与月度指标汇总同步优化 (`RoiPredictEngine.calculateCohortRoiTrend`)
系统为 **单 Cohort 批次** 以及 **自然月月度汇总（Monthly Summary）** 提供高精度的 D30 / D60 / D90 ROI 预测：
1. **分阶段分权治理 (Phase-Decoupled Control)**：
   - **极早期 ($D3 \sim D7$)**：微观曲线易受 D1 首充脉冲扰动，结合网文历史经验增长倍率先验（$M_{\text{prior}}(3 \to 30)=2.65, M(7 \to 30)=1.80$）与样本量置信度进行贝叶斯平滑融合；
   - **成熟期 ($D14 \sim D30$)**：底层外推曲线具备充分样本特异性与 OLS 动量，$100\%$ 完全信任底层微观外推曲线，杜绝宏观先验稀释；
   - **到达自适应修正**：当天数达到 $30\text{D} / 60\text{D} / 90\text{D}$ 时，预测值直接精确修正为实际已发生的真实值，消除任何数据断层。
2. **首充冲动型断崖衰竭识别 (Impulse Dropoff Detection)**：
   - 在 $D3 \sim D5$ 窗口，检测读者是否在 D1 大额充值后连续停滞（$\text{Recharge}(D3) \le \text{Recharge}(D1) \times 1.05$）；
   - 若命中读者流失特征，自动施加 $0.70$ 的流失折价保护，杜绝早期乐观误判。
3. **时间自适应弹性上下界 (Elastic Dynamic Bounds)**：
   - 彻底废除静态硬编码天花板，基于当前达成 ROI 与剩余时间动态计算上限 $\text{MaxROI}_{30}(t) = \text{ActualROI}(t) \times (1.0 + 2.2 \sqrt{\frac{30-t}{30}}) + 0.08 \times \frac{30-t}{30}$。
4. **月度指标汇总 (Monthly Summary) 100% 自动继承**：
   - `LtvStatService.getSingleMonthSummary` 汇总上月指标时，逐日调用底层单批次预测算子并累加充值（$\frac{\sum \text{PredRecharge}}{\sum \text{Spend}}$），全量自动继承底层 ROI 引擎的所有优化红利。

### 6. 大盘整体回本天数 (`PaybackPredictEngine.calculateOverallPaybackDays`)
对于广告主/整体小说平台视角，采用 **自下而上（Bottom-Up）自然日历对齐算法**：
- 按各个 H5 落地页/广告账户 Cohort 真实的 `launchDate` 在自然日历上逐日求和大盘预测充值曲线；
- 解决不同批次小说 Cohort 处于不同生命周期阶段（如老广告组与新测试广告组）的交叠累加问题；
- 若大盘预测充值曲线在未来某天达到总消耗金额，计算相对于今天的剩余天数。

---

## 五、 源码类与方法映射表 (Source Code Mapping)

| 包名 / 类名 | 关键方法 / 字段 | 职责说明 |
| :--- | :--- | :--- |
| `com.ltv.stat.service.LtvPredictService` | `predictCohortDailyRechargeCurve` | 编排层，实现 Realized ARPU 贝叶斯萃取、双轨系综融合并生成 D1~D365 预测充值曲线 |
| `com.ltv.stat.service.engine.LtvPredictFacade` | `assembleCohortPrediction` / `assembleOverallPrediction` | 门面类，分发调度引擎并组装单 Cohort 及大盘 `PredictionResult` DTO |
| `com.ltv.stat.service.engine.PaybackPredictEngine` | `calculateCohortPaybackDays` / `calculateOverallPaybackDays` | 引擎层，计算单 Cohort 真实已回本天数、未来交叉回本点及大盘交叠回本 |
| `com.ltv.stat.service.engine.RoiPredictEngine` | `calculateCohortRoiTrend` | 引擎层，计算 D30/D60/D90 里程碑预测，实现历史倍率先验融合、首充冲动折价与到达真实修正 |
| `com.ltv.stat.service.engine.CohortCurveExtrapolator` | `computeOptimalScaleFactor` / `computeOlsFit` / `computeOlsEnsembleWeight` | **纯算子引擎**，计算贝叶斯放缩因子、OLS 拟合优度及动态系综权重 |
| `com.ltv.stat.util.CohortStatHelper` | `getRechargeForDay` / `isSubscriptionStagnant` / `getRechargeContinuityRatio` | **数据与状态助手**，提供充值提取、连续性指数与平盘停滞判定 |
| `com.ltv.stat.service.engine.PredictAlgorithmConstants` | `SCALE_DECAY_EXPONENT` / `getEmpiricalMultiplierTo30` 等 | **常量管理**，集中存放所有算法超参数、P2 动态系综阈值与先验配置 |
| `com.ltv.stat.service.LtvBenchmarkService` | `getBenchmarkCurve` / `recalculateAllBenchmarks` | 查询与萃取网文历史成熟 Cohort 的留存与 ARPU 基准线 |

---

## 六、 总结

预测回本模型在完美结合 **海外 H5 小说变现特征（金币卡点解锁 + VIP 周/月卡续订）** 的基础上，实现了**代码结构与数学算法的高度解耦**。通过 **数据助手 + 纯算子 + 超参数集中管理 + 独立预测引擎** 的架构设计，既保障了优化师在买量早期（D3~D7）预测极高的准确度与决策稳定性，又极大提升了系统的可扩展性与代码可维护性。
