# EasyNetTopo

**EasyNetTopo 是独立的通用集成管理平台**，不以任何具体业务系统为运行前提。它提供统一的 Git 应用源、主机连接、受管资源、应用生命周期和任务管理；各业务系统独立安装并通过适配器按需集成。

> **现状核对（2026-11-02，main）**：已有 Go `cmd/internal`、Vue `web/`、应用源/主机/Bundle 相关源码与 GitHub Actions 工作流；这仅证明代码和自动化入口存在，**不是 Sub2API 实际安装或 FnOS 联调已通过**。以下仍以已确认的产品方向为准；执行时必须检查目标分支最新源码与测试。

## 当前代码基线

- Go 后端、Vue 3/TypeScript 前端与 SQLite 工程已存在。
- 已有 Git 应用源/商店只读查询、`app.yaml` 校验、固定版本部署包构建与 Agent 缓存传输能力。
- 已有 EasyHostAgent 配对和 Docker/Compose 状态查询能力；**安装、卸载执行闭环尚未实现**。
- **历史 R001/T001 需求及任务规划已撤销**。此处不再将旧版 Sub2API 专项闭环、M4 任务预检或更新流程当作已确认的第一期开发范围。新需求及任务须重新确认后建立。

## 技术参考

- [Go 模块化单体与集成适配器设计（历史设计草案）](docs/architecture/v1-modular-integration.md)
- [EasyNetTopo / EasyHostAgent / Deploy 的 M0 协议清单](docs/architecture/m0-cross-repo-contracts.md)

上述设计与协议文件供核对已有工程边界；不能代替重新确认的需求，也不能证明未实现功能已完成。

## 工程和知识管理

实际业务代码和技术设计属于本仓库。旧版需求/任务文件已删除；后续重新确认的需求和任务在本仓库独立建档，Git/PR/CI 仅提供真实发生的开发证据，不替代业务验收。通用 AI 协作规范与长期项目背景位于独立私有仓库 [DevKnowledge](https://github.com/CiBeDerORG/DevKnowledge)，按当前任务严格准入读取，不能假定网页端或本地 SSH 自动具有权限。

- [项目开发入口 AGENTS.md](AGENTS.md)
- [EasyNetTopo 知识入口（私有仓库）](https://github.com/CiBeDerORG/DevKnowledge/blob/main/02-%E9%A1%B9%E7%9B%AE/EasyNetTopo/INDEX.md)

本仓库不使用知识仓库 Submodule、符号链接或全量复制。未实际批准的代码初始化与部署不能以设计文档替代授权；不创建无用占位文件。