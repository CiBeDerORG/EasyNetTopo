# EasyNetTopo

**EasyNetTopo 是独立的通用集成管理平台**，不以任何具体业务系统为运行前提。它提供统一的 Git 应用源、主机连接、受管资源、应用生命周期和任务管理；各业务系统独立安装并通过适配器按需集成。

> **现状核对（2026-11-02，main）**：已有 Go `cmd/internal`、Vue `web/`、应用源/主机/Bundle 相关源码与 GitHub Actions 工作流；这仅证明代码和自动化入口存在，**不是 Sub2API 实际安装或 FnOS 联调已通过**。以下仍以已确认的产品方向为准；执行时必须检查目标分支最新源码与测试。

## 已确认的 V1.0 方向

1. 技术栈：**Go 后端 + Vue 3 / TypeScript / Vite 前端 + SQLite**；模块化单体，开发期前后端独立，生产可用 Go 单进程提供 API 和前端静态资源。
2. 应用商店：Git 仓库作为应用源。第一个来源是 [Deploy](https://github.com/CiBeDerORG/deploy)；首个真实示例为 `sub2api/app.yaml`，不要求补齐其他应用目录。私有 Git 访问只由控制端完成，Agent 不持有 GitHub 凭据。
3. 主机执行：通过直接可达的 **IP:port + 带指纹验证的 HTTPS** 连接 [EasyHostAgent](https://github.com/CiBeDerORG/EasyHostAgent)。一台 Agent 任一时刻只有一个有效控制中心；成功重新配对后旧授权立即失效。
4. 首个业务闭环：从 Git 刷新 Sub2API → 选择主机 → 安全配对 → 配置参数 → 固定版本包下发 → 受控安装 → 验证三个 Compose 服务健康 → 查看任务与日志 → 更新/卸载默认保留数据。
5. 扩展机制：**应用安装与专属业务集成分离**。通用平台不要求预装任何业务系统，未来新的业务 API 适配器须单独确认需求；平台自身不承担第三方产品的数据通路或专属控制逻辑。
6. 不实现任意 Shell、动态插件市场、心跳、离线任务队列、自动回滚或复杂微服务。

## 当前评审规格

- [R001 · V1.0 汇总/分级需求与验收](docs/requirements/v1-platform-scope.md)
- [T001 · EasyNetTopo V1 分级任务与状态](docs/requirements/tasks.md)
- [Go 模块化单体与集成适配器设计](docs/architecture/v1-modular-integration.md)
- [EasyNetTopo / EasyHostAgent / Deploy 的 M0 协议清单](docs/architecture/m0-cross-repo-contracts.md)

上述文件是现行产品规格与实施约束。**Bundle、持久任务及部分接口已有相关 Go 代码，但可信脚本执行、第三方 Compose 权限边界与 Sub2API 实际安装闭环仍需按当前版本验证**；不得凭原有设计文档宣布已完成。

## 工程和知识管理

实际业务代码、正式需求与技术设计属于本仓库；正式 R/T 需求与任务状态以项目 Markdown 文件为权威（`docs/requirements/`），PR、Git、CI 仅在实际存在时提供佐证，不依赖 Issues/Projects。通用 AI 协作规范与长期项目背景位于独立私有仓库 [DevKnowledge](https://github.com/CiBeDerORG/DevKnowledge)，按当前任务严格准入读取，不能假定网页端或本地 SSH 自动具有权限。

- [项目开发入口 AGENTS.md](AGENTS.md)
- [EasyNetTopo 知识入口（私有仓库）](https://github.com/CiBeDerORG/DevKnowledge/blob/main/02-%E9%A1%B9%E7%9B%AE/EasyNetTopo/INDEX.md)

本仓库不使用知识仓库 Submodule、符号链接或全量复制。未实际批准的代码初始化与部署不能以设计文档替代授权；不创建无用占位文件。