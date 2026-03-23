# AIGateway Portal Backend 改造设计（GoFrame）

## 1. 目标

将当前 `Gin + database/sql` 单体实现逐步改造为 `GoFrame v2` 标准结构，保持现有前端 API 可用，同时提升以下能力：

- 结构化分层（`api/controller/service/dao/entity/do`）。
- 数据访问规范化（统一使用 GoFrame ORM，写操作使用 `do` 对象）。
- 业务可维护性（按领域拆分服务，避免 `handlers.go` 巨石化）。
- 可测试性与可观测性（服务层可单测，错误统一 `gerror` 包装）。

## 2. 当前基线（2026-03-22）

### 2.1 现状架构

- 入口：`main.go` + `app.go`。
- 路由层：`gin.Engine`，在 `buildRouter()` 中集中注册。
- 业务层：`handlers.go`（认证、计费、模型、API Key、发票全部混合）。
- 数据层：直接 `database/sql` + 手写 SQL。
- 外部依赖：可选通过 Prometheus API 拉取 usage 指标（`PORTAL_CORE_PROMETHEUS_URL`）。
- 配置：`config.go` 读取环境变量。

### 2.2 路由清单（保持兼容）

公共接口：

- `GET /api/health`

认证接口：

- `POST /api/auth/register`
- `POST /api/auth/login`
- `POST /api/auth/logout`
- `GET /api/auth/me`

计费接口（需登录）：

- `GET /api/billing/overview`
- `GET /api/billing/consumptions`
- `GET /api/billing/recharges`
- `POST /api/billing/recharges`

模型接口（需登录）：

- `GET /api/models`
- `GET /api/models/:id`

开放平台接口（需登录）：

- `GET /api/open-platform/keys`
- `POST /api/open-platform/keys`
- `PATCH /api/open-platform/keys/:id/status`
- `DELETE /api/open-platform/keys/:id`
- `GET /api/open-platform/stats`
- `GET /api/open-platform/cost-details`

发票接口（需登录）：

- `GET /api/invoices/profile`
- `PUT /api/invoices/profile`
- `GET /api/invoices/records`
- `POST /api/invoices/records`

## 3. GoFrame 目标架构

建议目录（`backend` 目录下）：

```text
cmd/
  main.go
manifest/
  config/config.yaml
  i18n/
internal/
  api/
    auth/v1/
    billing/v1/
    model/v1/
    openplatform/v1/
    invoice/v1/
  controller/
    auth/
    billing/
    model/
    openplatform/
    invoice/
    health/
  service/
    auth.go
    billing.go
    model.go
    apikey.go
    invoice.go
    session.go
    usage_sync.go
  logic/ (本项目默认不启用；业务逻辑直接在 service 实现)
  dao/      (gf gen dao 生成)
  model/
    entity/ (gf gen dao 生成)
    do/     (gf gen dao 生成)
  consts/
  middleware/
  utility/
resource/
  template/
``` 

关键约束：

- 禁止手写 `internal/dao`、`internal/model/entity`、`internal/model/do`，统一通过 `gf gen dao` 生成。
- 所有 DB 写操作使用 `do` 对象，禁止 `map` 直写。
- 服务层错误统一使用 `gerror` 包装并保留上下文。

## 4. API 设计原则

- 路由前缀保持 `/api` 不变，避免前端改动。
- 保持原响应字段命名（camelCase）和主要错误语义。
- Controller 只做参数绑定与响应输出，业务逻辑全部放入 `service`。
- 对关键写操作（注册、创建/更新 Key、开票）统一事务边界。
- 数据库配置优先使用 `PORTAL_MYSQL_*`，未设置时回退到 `PORTAL_CORE_DB_*`（兼容 `HIGRESS_PORTAL_DB_*`），便于与 core 共库部署。

示例映射：

- `POST /api/auth/register`
  - Controller: `auth.Register`
  - Service: `AuthService.Register(ctx, in)`
  - 子流程：校验邀请码 -> 本地写 user(status=disabled)/consume invite -> 不自动建 session（需管理员启用后登录）

- `POST /api/open-platform/keys`
  - Controller: `openplatform.CreateKey`
  - Service: `APIKeyService.Create(ctx, user, in)`
  - 子流程：落库 -> 返回新建 Key

- `GET /api/open-platform/stats`
  - Controller: `openplatform.GetStats`
  - Service: `UsageService.GetOpenStats(ctx, consumerName)`

## 5. 核心函数设计（服务层）

认证域：

- `Register(ctx, RegisterInput) (RegisterOutput, error)`
- `Login(ctx, LoginInput) (AuthUser, error)`
- `Logout(ctx, sessionToken string) error`
- `GetCurrentUser(ctx, consumerName string) (AuthUser, error)`

会话域：

- `CreateSession(ctx, consumerName string) (token string, expireAt gtime.Time, error)`
- `VerifySession(ctx, token string) (AuthUser, error)`
- `ClearSession(ctx, token string) error`

计费域：

- `GetBillingOverview(ctx, consumerName string) (BillingOverview, error)`
- `ListConsumptions(ctx, consumerName string) ([]ConsumptionRecord, error)`
- `ListRecharges(ctx, consumerName string) ([]RechargeRecord, error)`
- `CreateRecharge(ctx, consumerName string, in CreateRechargeInput) (RechargeRecord, error)`

模型域：

- `ListModels(ctx) ([]ModelInfo, error)`
- `GetModelDetail(ctx, modelID string) (ModelInfo, error)`

API Key 域：

- `ListKeys(ctx, consumerName string) ([]ApiKeyRecord, error)`
- `CreateKey(ctx, consumerName, department, name string) (ApiKeyRecord, error)`
- `UpdateKeyStatus(ctx, consumerName, keyID, status string) (ApiKeyRecord, error)`
- `DeleteKey(ctx, consumerName, keyID string) error`

发票域：

- `GetInvoiceProfile(ctx, consumerName string) (InvoiceProfile, error)`
- `UpdateInvoiceProfile(ctx, consumerName string, in InvoiceProfile) (InvoiceProfile, error)`
- `ListInvoiceRecords(ctx, consumerName string) ([]InvoiceRecord, error)`
- `CreateInvoice(ctx, consumerName string, in CreateInvoiceInput) (InvoiceRecord, error)`

用量同步域：

- `SyncUsageOnce(ctx) error`
- `StartUsageSync(ctx)`
- `CalculateCost(inputTokens, outputTokens, inputPrice, outputPrice) float64`

## 6. 数据模型与迁移策略

现有核心表：

- `portal_user`
- `portal_invite_code`
- `portal_session`
- `portal_api_key`
- `portal_model_catalog`
- `portal_usage_daily`
- `portal_recharge_order`
- `portal_invoice_profile`
- `portal_invoice_record`

迁移策略：

- 第一阶段：保持表结构不变，仅替换应用层为 GoFrame。
- 第二阶段：补充必要索引、唯一约束与审计字段（按业务压测结果决策）。
- 第三阶段：将初始化建表逻辑从运行时迁移为独立 migration 流程。

## 7. 分阶段实施

- Phase 1：搭建 GoFrame 项目骨架，接入配置、路由、健康检查。
- Phase 2：迁移认证与会话链路（风险最高，优先收敛）。
- Phase 3：迁移 API Key 与 Core 数据层直连逻辑。
- Phase 4：迁移计费/用量/发票接口。
- Phase 5：移除 Gin 与 `database/sql` 直连代码，补齐测试与发布文档。

## 8. 当前落地状态（2026-03-22）

- 已完成：
  - `Gin + database/sql` 到 `GoFrame` 的运行时迁移。
  - 全部既有 API 路由在 GoFrame 下完成迁移并保持路径兼容。
  - `controller + service + middleware + client + config` 分层落地。
  - 启动建表/播种、Session 鉴权、API Key 直连、Usage 定时任务迁移完成。

- 待完善：
  - `dao/entity/do` 已切换为 `gf gen dao` 自动生成版本，查询调用仍在向 `dao` API 迁移中。
  - 尚未补全接口级回归测试、压测与发布文档。

## 9. 本地开发链路补充（2026-03-23）

### 9.1 启动与端口约定

- 一键启动脚本：`./start.sh`
- 默认端口：
  - Backend：`8081`（`PORTAL_BACKEND_PORT` / `PORTAL_LISTEN_ADDR` 可覆盖）
  - Frontend：`5173`（`PORTAL_FRONTEND_PORT` 可覆盖）
- 健康检查：
  - 后端：`GET /api/health`
  - 前端：`GET /`

### 9.2 前端访问问题复盘

已修复以下会导致“前端打开无法访问”的工程问题：

- 缺少 `frontend/index.html` 导致 Vite 首页 `404`。
- 前端开发代理固定指向 `127.0.0.1:8080`，与 `start.sh` 默认后端端口 `8081` 不一致。

### 9.3 设计约束（新增）

- `frontend` 根目录必须保留入口文件 `index.html`，作为 Vite dev/build 的统一入口。
- 本地联调时，前端 `/api` 代理目标必须与启动脚本实际后端端口一致：
  - 通过 `PORTAL_DEV_API_TARGET` 统一注入（默认 `http://127.0.0.1:${BACKEND_PORT}`）。
- 修改启动端口时，优先调整环境变量，不手动硬编码多个端口配置，避免漂移。
