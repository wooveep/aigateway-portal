# AIGateway Portal TODO

> 本文件只记录 Portal 子项目自身轻量待办。跨项目账单、模型、发布和组织任务以根目录 `TODO.md` 为准。

## 已完成

- [x] 建立 GoFrame 改造文档基线：`project.md`、`memory.md`、`TODO.md`。
- [x] 完成 GoFrame 工程骨架、配置加载、健康检查、统一错误结构、本地 `start.sh`。
- [x] 迁移认证与会话：`register/login/logout/me`、Cookie 中间件、注册事务化。
- [x] 迁移 API Key 增删改查，并改为直连 Portal/Core 数据层，不再依赖 Console API。
- [x] 迁移 billing、model、invoice、open-platform、usage sync 主接口。
- [x] 使用 `gf gen dao` 生成 `dao/entity/do`，写操作已改为 `do` 对象入库。
- [x] 支持 `PORTAL_DB_*`，并兼容 `PORTAL_CORE_DB_*`、历史 `HIGRESS_PORTAL_DB_*`。
- [x] 清理 Gin 和 `database/sql` 主依赖。
- [x] 修复前端 dev 首页 404、`/api` 代理端口不一致和本地联调链路。
- [x] 注册用户默认 `disabled`，邀请码默认不再内置，需 Console 管理员启用后登录。

## 待完成

- [ ] 将成本计算与价格查询进一步下沉服务层并补单测。
- [ ] 将剩余手写 SQL 查询逐步替换为 `dao` API。
- [ ] 为关键表补充索引评估与迁移脚本。
- [ ] 接入基础日志字段：trace id、consumerName、request id。
- [ ] 全量回归 Portal API，至少覆盖认证、API Key、发票、计费。
- [ ] 编写部署 / 回滚说明与配置清单。
- [ ] 发布前压测并记录性能对比结果。
- [ ] 优化前端构建产物体积，当前主 chunk 仍偏大。
