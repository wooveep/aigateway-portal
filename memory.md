# Memory - Portal Backend 改造记录

## 2026-03-22

### 本次目标

- 启动 `backend` 的 GoFrame 改造工作。
- 建立可持续更新的改造文档体系（设计、待办、变更记录）。

### 本次改动

- 新增 [`project.md`](./project.md)
  - 输出当前基线架构（Gin + database/sql）与 GoFrame 目标架构。
  - 梳理完整 API 清单（`/api/*`）。
  - 设计服务层核心函数（认证、会话、计费、模型、API Key、发票、用量同步）。
  - 定义分阶段迁移策略（Phase 1~5）。

- 新增 [`TODO.md`](./TODO.md)
  - 按优先级拆分改造任务（P0~P3）。
  - 明确本轮已完成项和后续施工路径。

- 新增 [`memory.md`](./memory.md)
  - 建立改造日志入口，后续每次迭代追加记录。

### 决策记录

- 先保证 API 兼容，再替换框架实现，避免前端联调中断。
- 严格遵循 GoFrame 规范：
  - 业务逻辑放 `service`。
  - `dao/entity/do` 使用生成代码，不手写。
  - DB 写操作统一使用 `do` 对象。

### 当前状态

- 代码层尚未进行 GoFrame 迁移实现（本次为改造准备阶段）。
- 下一步进入 P0：创建 GoFrame 骨架并先迁移 `GET /api/health`。

## 2026-03-22（全量改造实施）

### 本次目标

- 按 `project.md` 方案完成后端从 `Gin + database/sql` 到 `GoFrame` 的全量迁移。
- 保持现有 ` /api/* ` 路由兼容并通过编译验证。

### 本次改动

- 重构入口与工程结构：
  - 新入口：[`main.go`](./backend/main.go) + [`internal/cmd/cmd.go`](./backend/internal/cmd/cmd.go)。
  - 新增 `internal` 分层：`config`、`controller`、`middleware`、`service`、`client`、`model`、`httpx`。
  - 新增 [`manifest/config/config.yaml`](./backend/manifest/config/config.yaml)。

- 全量迁移 API（保持路径不变）：
  - 认证：`/api/auth/register|login|logout|me`。
  - 计费：`/api/billing/*`。
  - 模型：`/api/models`、`/api/models/:id`。
  - 开放平台：`/api/open-platform/*`。
  - 发票：`/api/invoices/*`。
  - 健康检查：`GET /api/health`。

- 全量迁移核心能力：
  - 会话鉴权中间件（Cookie + DB Session 校验）。
  - 启动迁移建表与基础数据播种（模型目录、初始邀请码）。
  - Higress Console Consumer/Usage 对接客户端。
  - 用量定时同步任务（`StartUsageSync/syncUsageOnce`）。
  - 前端静态资源挂载与 SPA 回退。

- 依赖与构建更新：
  - `go.mod` 切换到 GoFrame 依赖（`github.com/gogf/gf/v2`）。
  - [`Dockerfile`](./backend/Dockerfile) 后端构建镜像调整为 `golang:1.23-alpine`（匹配 GoFrame v2.10.0 的 Go 版本要求）。
  - 原实现迁移到 `./backend/.legacy-src/` 备份目录。

### 验证结果

- 执行：`go test ./...`
- 结果：通过（所有包可编译，无测试用例失败）。

### 当前遗留

- 尚未使用 `gf gen dao` 生成 `dao/entity/do`（当前 `do` 为手工定义）。
- 尚未完成 API 回归测试、压测和发布文档。

## 2026-03-22（继续下一步：DAO 生成）

### 本次目标

- 完成 `gf gen dao`，将 `dao/entity/do` 切换为 GoFrame CLI 生成代码。

### 本次改动

- 生成代码目录：
  - `backend/internal/dao/*`
  - `backend/internal/dao/internal/*`
  - `backend/internal/table/*`
  - `backend/internal/model/entity/*`
  - `backend/internal/model/do/*`

- 为避免冲突，将手写 `do` 备份到：
  - `./backend/.legacy-src/model-do/do.manual.go`

- 兼容生成代码命名差异（例如 `PortalApiKey.KeyId`、`PortalRechargeOrder.OrderId`、`PortalInvoiceRecord.InvoiceId`），已同步调整 service 层引用。

### 验证结果

- 执行：`go test ./...`
- 结果：通过。

### 当前遗留

- 查询逻辑仍大量使用手写 SQL，待逐步替换为 `dao` 调用。
- 接口回归测试、压测和发布文档仍待完成。

## 2026-03-22（console 对接与共库）

### 本次目标

- 强化 `portal` 与 `higress-console` 的对接能力，支持与 console 使用同一个 Portal 数据库配置。

### 本次改动

- 后端配置增强：
  - [`backend/internal/config/config.go`](./backend/internal/config/config.go) 新增 `HIGRESS_PORTAL_DB_URL / HIGRESS_PORTAL_DB_USERNAME / HIGRESS_PORTAL_DB_PASSWORD` 回退逻辑。
  - 当未显式设置 `PORTAL_MYSQL_*` 时，自动解析 JDBC URL（`jdbc:mysql://...`）并转换为 Go MySQL DSN 参数。

- Helm 注入增强：
  - [`helm/templates/backend-deployment.yaml`](./helm/templates/backend-deployment.yaml) 增加 `HIGRESS_PORTAL_DB_*` 环境变量注入，默认与 portal 当前 DB Secret/Host 对齐。
  - [`helm/values.yaml`](./helm/values.yaml) 新增 `database.jdbcParams`，用于构建 `HIGRESS_PORTAL_DB_URL`。

- 文档更新：
  - [`README.md`](./README.md) 增加共库说明与新环境变量说明。

### 验证结果

- 执行：`go test ./...`（backend）
- 结果：通过。

## 2026-03-22（本地启动脚本）

### 本次目标

- 提供仅本地进程的一键启动方式，便于开发测试环境快速验证启动。
- 明确不在脚本中启动任何外部依赖（容器/MySQL/Console）。

### 本次改动

- 新增 [`start.sh`](./start.sh)：
  - 一键启动 `backend` 与 `frontend` 本地进程。
  - 启动后自动校验：
    - 后端健康检查 `GET /api/health`
    - 前端开发服务地址可访问
  - 默认关闭 `PORTAL_USAGE_SYNC_ENABLED`，减少本地启动时对 console 的依赖。
  - 统一输出日志到 `.logs/backend-dev.log` 与 `.logs/frontend-dev.log`。

- 更新 [`README.md`](./README.md)：
  - 增加 `./start.sh` 本地启动说明。
  - 明确脚本不会启动外部依赖，数据库需提前可用。

### 验证结果

- 执行：`bash -n ./start.sh`
- 结果：通过（脚本语法检查通过）。

## 2026-03-23（portal 去除 console 依赖，直连 core）

### 本次目标

- Portal 不再依赖 console。
- 改为直接对接 core 的数据库与指标服务。

### 本次改动

- 后端解耦 console：
  - 移除 console client 依赖。
  - 注册与 API Key 流程不再调用 console API。
- Usage 同步调整：
  - 改为直接查询 Prometheus（`PORTAL_CORE_PROMETHEUS_URL`）。
- 配置收敛：
  - 新增 `PORTAL_CORE_DB_*` 配置入口。
  - 兼容旧变量 `HIGRESS_PORTAL_DB_*` 回退逻辑。
- Helm 调整：
  - 去除 `HIGRESS_CONSOLE_*` 与 `console-secret` 注入。
  - 改为注入 `PORTAL_CORE_PROMETHEUS_URL`。
- 文档同步：
  - `README.md`、`project.md`、`TODO.md` 更新为“直连 core”口径。

### 验证结果

- 执行：`cd backend && go test ./...`
- 结果：通过。
- 执行：`cd helm && helm template portal .`
- 结果：通过。
- 执行：`cd ../higress/helm/higress && helm template aigateway . -f values-local-minikube.yaml`
- 结果：通过。
- 执行：`cd ../higress/helm/higress && helm template aigateway . -f values-production-gray.yaml`
- 结果：通过。
- 执行：`bash -n ./start.sh`
- 结果：通过。

### 当前状态

- Portal 运行链路已不依赖 console。
- Usage 同步在未配置 `PORTAL_CORE_PROMETHEUS_URL` 时自动跳过。

## 2026-03-23（前端无法访问修复）

### 本次目标

- 修复 `frontend` 开发环境“页面无法访问”的问题。
- 消除本地启动脚本与前端代理端口不一致导致的 API 联调故障。

### 问题定位

- `Vite` 启动后访问 `http://127.0.0.1:5173/` 返回 `404`。
- 根因 1：`frontend` 根目录缺少 `index.html`，Vite 无法提供入口页。
- 根因 2：`vite.config.ts` 中 `/api` 代理固定到 `127.0.0.1:8080`，而 `start.sh` 默认后端端口是 `8081`。

### 本次改动

- 新增前端入口文件：
  - [`frontend/index.html`](./frontend/index.html)
- 调整开发代理目标可配置并默认对齐本地后端端口：
  - [`frontend/vite.config.ts`](./frontend/vite.config.ts)
  - 新增 `PORTAL_DEV_API_TARGET`（默认 `http://127.0.0.1:8081`）
- 启动脚本注入代理目标，避免手工改端口：
  - [`start.sh`](./start.sh)
  - 导出 `PORTAL_DEV_API_TARGET="${PORTAL_DEV_API_TARGET:-http://127.0.0.1:${BACKEND_PORT}}"`

### 验证结果

- 执行：`curl -i http://127.0.0.1:5173/`
- 结果：`200 OK`（首页可访问）。
- 执行：`curl -i http://127.0.0.1:5173/api/health`
- 结果：`200 OK`（请求正确代理到本地 backend）。
- 执行：`cd frontend && npm run build`
- 结果：通过。

### 当前状态

- 本地开发链路（`start.sh` -> `backend + frontend`）可直接访问和联调。
- 前端 dev 代理与后端端口默认保持一致，可通过 `PORTAL_DEV_API_TARGET` 覆盖。

## 2026-03-23（注册流程与启用策略调整）

### 本次目标

- 修复登录/注册页表单校验提示不随输入消除的问题。
- 明确邀请码来源为 Console 端「组织架构」管理员生成。
- 调整注册后权限策略：默认禁用，管理员启用后方可登录。

### 本次改动

- 前端表单校验修复：
  - `LoginPage.vue`、`RegisterPage.vue` 的 `a-form` 补充 `:model="form"`，修复“请输入”提示不自动消除问题。
- 注册页交互调整：
  - 邀请码输入项增加说明：邀请码由 Console「组织架构」菜单管理员生成。
  - 注册成功后不再“注册并登录”，改为提示“联系管理员启用后登录”并跳转登录页。
- 后端注册流程调整：
  - `portal_user.status` 注册写入由 `active` 改为 `disabled`。
  - 注册接口仅在用户为 `active` 时创建会话；新注册用户不会自动登录。
  - 注册响应不再下发默认 API Key（`defaultApiKey` 置空）。
- 引导邀请码来源收敛：
  - `PORTAL_BOOTSTRAP_INVITE_CODE` 默认值改为空。
  - 启动播种逻辑仅在该配置非空时才写入预置邀请码。

### 验证结果

- 执行：`cd backend && go test ./...`
- 结果：通过。
- 执行：`cd frontend && npm run build`
- 结果：通过（存在构建体积 warning，不影响功能）。

### 当前状态

- Portal 注册用户默认处于禁用状态，需管理员在 Console「组织架构」中启用后才可登录。
- Portal 不再默认内置固定邀请码（除非显式配置 `PORTAL_BOOTSTRAP_INVITE_CODE`）。
