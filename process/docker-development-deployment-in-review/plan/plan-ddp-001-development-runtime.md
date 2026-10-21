---
id: PLAN-DDP-001
feature: docker-development-deployment
status: done
summary: ../README.md
requirement: ../require/README.md
task_board: ../task/README.md
depends_on: []
updated_at: 2026-10-22
---

# 部署入口与开发运行时

依据 EV-DDP-01–05，覆盖 DDP-R01–06。采用[公共契约](README.md)，不改已有正常前端构建与 Rust 业务语义。

DDP-101 拥有根入口、配置和容器接线；DDP-102 独占 Python 同步/编译/进程 watcher；DDP-103 独占前端开发配置。三者只读仓库清单与现有代码，并行组 runtime；过程文档统一由 /root 更新。各自完成针对性验证并将文件、证据和限制交给 [DDP-104](../task/task-ddp-104-verification.md)。生成缓存与测试目录隔离，不能共写安装工作区。

2026-10-21 的 EasyNetTopo 迁移维护依据 EV-DDP-06，仍由 DDP-101 统一修改入口、Compose 默认项目名、模板、说明与已有测试夹具；/root/migration_runtime_review 并行只读审查目录约束。迁移保留 Git 根目录与 .env/data 的现有限制，以及 Cargo/pnpm 内部包名。按用户要求，仅做源码、语法与差异核查，不运行测试、构建或 CI；历史验证证据单独保留。

2026-10-22 依据 EV-DDP-07，由 DDP-101 补齐既有构建器的 `.env` 配置，遵循公共契约中的覆盖顺序及重执行规则。独立审查并行只读复核；不修改 Compose 拓扑或共享 BuildKit，不扩展代理接线。语法和静态核查结果交 DDP-104，目标 NAS 的真实部署仍单独验收。
