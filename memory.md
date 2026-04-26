# Memory - Portal Backend 改造记录

> 本文件只保留 Portal 子项目稳定事实。跨项目数据库、发布和计费主链决策看根目录 `Memory.md`。

## 当前基线

- Portal 后端已从 `Gin + database/sql` 迁移到 GoFrame。
- 当前数据库标准固定为 PostgreSQL-only，推荐配置为 `PORTAL_DB_*`。
- 历史 `PORTAL_MYSQL_*`、`jdbc:mysql://`、MySQL 表述只作为迁移记录，不代表当前规范。
- Portal 不再依赖 Console API 完成注册、API Key、账单主链；账务与用户数据直接读写 Portal/Core 数据层。
- Prometheus 不再是账务主链，只作为可选对账和观测来源。

## GoFrame 迁移事实

- 新入口为 `backend/main.go` 与 `backend/internal/cmd/cmd.go`。
- 工程分层固定为 `config`、`controller`、`middleware`、`service`、`client`、`model`、`httpx`。
- `manifest/config/config.yaml` 是 GoFrame 配置入口。
- 已迁移并保持路径兼容的 API：认证、计费、模型、开放平台、发票、健康检查。
- 会话鉴权使用 Cookie + DB Session 中间件。
- 启动期负责建表与基础数据播种，注册流程以事务保证用户创建和邀请码消耗一致。
- `gf gen dao` 已生成 `dao/entity/do`，手写 `do` 备份到 `backend/.legacy-src/model-do/do.manual.go`。
- 仍有部分查询路径使用手写 SQL，后续逐步替换为 DAO。

## 配置与本地开发

- 支持 `PORTAL_DB_*`，并保留 `PORTAL_CORE_DB_*` 与历史 `HIGRESS_PORTAL_DB_*` 回退。
- 本地 `start.sh` 只启动 backend / frontend 本地进程，不启动外部依赖。
- 本地默认关闭 `PORTAL_USAGE_SYNC_ENABLED`，减少对外部服务依赖。
- 前端 dev 入口已补 `frontend/index.html`。
- `frontend/vite.config.ts` 的 `/api` 代理默认对齐本地后端端口 `8081`，可通过 `PORTAL_DEV_API_TARGET` 覆盖。

## 注册与账号策略

- Portal 注册用户默认写入 `disabled`，注册成功后不自动登录。
- 注册响应不再下发默认 API Key。
- 邀请码来源收敛到 Console 组织架构管理员生成。
- `PORTAL_BOOTSTRAP_INVITE_CODE` 默认空；只有显式配置时才播种预置邀请码。
- 登录 / 注册页 `a-form` 必须绑定 `:model="form"`，否则校验提示不会随输入消除。

## 验证记录

- GoFrame 全量迁移后，`cd backend && go test ./...` 通过。
- DAO 生成切换后，`cd backend && go test ./...` 通过。
- Portal 去 Console API 依赖后，`cd backend && go test ./...`、Portal Helm template、父 Chart template 通过。
- 前端入口与代理修复后，`curl http://127.0.0.1:5173/`、`curl http://127.0.0.1:5173/api/health`、`cd frontend && npm run build` 通过。
- 注册策略调整后，`cd backend && go test ./...`、`cd frontend && npm run build` 通过。
