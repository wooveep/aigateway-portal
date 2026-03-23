# TODO - Portal Backend GoFrame 改造

## P0（当前迭代）

- [x] 建立改造文档基线：`project.md`、`memory.md`、`TODO.md`。
- [x] 初始化 GoFrame 工程骨架（`cmd`、`manifest`、`internal` 目录）。
- [x] 建立配置加载与环境变量映射（覆盖现有 `PORTAL_*` 与 `HIGRESS_*`）。
- [x] 落地健康检查接口 `GET /api/health`（GoFrame 路由版本）。
- [x] 建立统一错误返回结构与 `gerror` 包装规范。
- [x] 新增本地一键启动脚本 `./start.sh`（仅启动项目本地进程，不启动外部依赖）。

## P1（认证与会话）

- [x] 迁移 `register/login/logout/me` 到 `controller + service`。
- [x] 会话能力迁移为 GoFrame 中间件，替换当前 Gin cookie 校验流程。
- [x] 保持 Cookie 名称、TTL、Secure/SameSite 行为兼容。
- [x] 注册流程事务化：用户（默认禁用）与邀请码消耗写入一致性。

## P1（API Key 与 Core 对齐）

- [x] 迁移 API Key 增删改查接口。
- [x] Portal API Key 全流程改为直连 Portal/Core 数据层，不再依赖 console API。
- [x] 清理 Console Client 依赖与配置注入。

## P2（计费、模型、发票、用量）

- [x] 迁移 billing 概览/消费/充值接口。
- [x] 迁移 model 列表/详情接口。
- [x] 迁移 invoice profile/record 接口。
- [x] 迁移 `startUsageSync/syncUsageOnce` 周期任务。
- [ ] 将成本计算与价格查询下沉服务层并补充单测（代码已迁移，单测待补）。

## P2（数据层与工程化）

- [x] 使用 `gf gen dao` 生成 `dao/entity/do`。
- [x] 增加共库配置回退（支持 `PORTAL_CORE_DB_*` 与兼容 `HIGRESS_PORTAL_DB_*`）。
- [ ] 将查询调用逐步替换为 `dao` API（当前仍有手写 SQL 查询）。
- [x] 所有写操作改为 `do` 对象入库。
- [ ] 为关键表补充索引评估与迁移脚本。
- [ ] 接入基础日志字段（trace id、consumerName、request id）。

## P3（收尾发布）

- [x] 清理 Gin 和 `database/sql` 依赖。
- [ ] 回归测试全量 API（至少覆盖认证、API Key、发票、计费）。
- [ ] 编写部署/回滚说明与配置清单。
- [ ] 发布前压测并记录性能对比结果。

## P3（前端与本地开发链路）

- [x] 修复前端入口缺失导致的 dev 首页 404（补充 `frontend/index.html`）。
- [x] 修复前端 `/api` 代理端口与本地 backend 端口不一致问题（`vite.config.ts` + `start.sh`）。
- [x] 验证本地联调链路：`/` 与 `/api/health` 可访问。
- [ ] 优化前端构建产物体积（当前主 chunk > 500k，考虑路由级拆包与手动分包策略）。
