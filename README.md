# AIGateway Portal

独立用户门户项目（前后端分离，不并入 `higress-console`）：
- 后端：Go + GoFrame
- 前端：Vue 3 + Vite + Ant Design Vue
- 部署：独立镜像、独立 Helm 子 Chart

## 核心能力

1. 个人账单：充值、消费记录、充值记录
2. 模型广场：模型清单、模型详情（token 单价/调用方式）
3. 开放平台：API Key 管理、调用统计、费用明细
4. 发票管理：开票信息与开票记录
5. 认证：邀请码注册（管理员在 Console「组织架构」生成）、登录、登出、会话管理（新注册默认禁用，需管理员启用）

## 与 core 集成

- Portal 用户、API Key、账单、发票等数据直接写入 Portal/Core 共用数据库。
- Usage 同步可通过 `PORTAL_CORE_PROMETHEUS_URL` 直接查询 core 的 Prometheus 指标。
- 项目标准为与 core 组件共用同一套 PostgreSQL Portal 数据库：
  - `portal_user`
  - `portal_invite_code`
  - `portal_api_key`
  - `portal_model_catalog`
  - `portal_usage_daily`
  - `portal_recharge_order`
  - `portal_invoice_profile`
  - `portal_invoice_record`

## 后端关键环境变量

- `PORTAL_DB_DRIVER`（固定为 `postgres`）
- `PORTAL_DB_HOST` / `PORTAL_DB_PORT` / `PORTAL_DB_USER` / `PORTAL_DB_PASSWORD` / `PORTAL_DB_NAME`
- `PORTAL_DB_PARAMS`（推荐 `sslmode=disable`）
- `PORTAL_DB_AUTO_MIGRATE`
- `PORTAL_SESSION_COOKIE_NAME`
- `PORTAL_SESSION_SECRET`
- `PORTAL_BOOTSTRAP_INVITE_CODE`（可选，默认空；仅用于预置单个邀请码）
- `PORTAL_CORE_PROMETHEUS_URL`（可选，用于 usage 同步）
- `PORTAL_CORE_DB_URL` / `PORTAL_CORE_DB_USERNAME` / `PORTAL_CORE_DB_PASSWORD`（可选，用于共享库场景）

## API 概览

认证：
- `POST /api/auth/register`
- `POST /api/auth/login`
- `POST /api/auth/logout`
- `GET /api/auth/me`

业务：
- `GET /api/billing/overview`
- `GET /api/billing/consumptions`
- `GET /api/billing/recharges`
- `POST /api/billing/recharges`
- `GET /api/models`
- `GET /api/models/:id`
- `GET /api/open-platform/keys`
- `POST /api/open-platform/keys`
- `PATCH /api/open-platform/keys/:id/status`
- `DELETE /api/open-platform/keys/:id`
- `GET /api/open-platform/stats`
- `GET /api/open-platform/cost-details`
- `GET /api/invoices/profile`
- `PUT /api/invoices/profile`
- `GET /api/invoices/records`
- `POST /api/invoices/records`

## 注册与启用流程

1. 用户在 Portal 注册页填写管理员生成的邀请码完成注册。
2. 注册成功后，Portal 用户默认为 `disabled`，且不会自动登录。
3. 管理员在 Console「组织架构」中启用该用户后，用户才可登录 Portal。

## 本地开发

推荐先在仓库根目录启动核心依赖，再本地运行 Portal：

```bash
cd /path/to/aigateway-group
python3 ./scripts/aigateway-dev.py minikube-dev --core-only
```

这样会在集群里保留 `postgresql / redis / controller / gateway / plugin-server / prometheus` 等核心服务，
但不会启动集群版 `console` / `portal`，便于你直接修改本地源码做验证。

一键启动（推荐）：
```bash
./start.sh
```

说明：
- `start.sh` 仅启动本地 `backend + frontend` 进程，不会拉起容器，也不会启动 PostgreSQL/Prometheus 等外部依赖。
- 脚本会校验后端 `GET /api/health` 和前端地址是否可访问。
- 需提前确保后端 PostgreSQL 连接可用（如本机端口转发到共享库，或你已配置可访问的共享 PostgreSQL）。
- 脚本默认设置 `PORTAL_USAGE_SYNC_ENABLED=false`，避免本地启动时强依赖 core 监控服务。

后端：
```bash
cd backend
go mod tidy
go run main.go
```

前端：
```bash
cd frontend
npm install
npm run dev
```

## 镜像与 Helm

Portal 采用单镜像部署：前端构建产物会打包进后端镜像并随同发布。
默认镜像名：`aigateway/portal:0.2.0`。

构建镜像：
```bash
./build-images.sh
```

Helm Chart 目录：
- `./helm`（Portal 子 Chart，数据库依赖走共享 PostgreSQL 配置）

父 Chart 集成位置：
- `./helm/higress`
