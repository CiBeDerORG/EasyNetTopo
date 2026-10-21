---
id: PLAN-DDP-001
feature: docker-development-deployment
status: done
summary: ../README.md
requirement: ../require/README.md
task_board: ../task/README.md
depends_on: []
updated_at: 2026-10-21
---

# 部署入口与开发运行时

依据 EV-DDP-01–05，覆盖 DDP-R01–06。采用[公共契约](README.md)，不改已有正常前端构建与 Rust 业务语义。

DDP-101 拥有根入口、配置和容器接线；DDP-102 独占 Python 同步/编译/进程 watcher；DDP-103 独占前端开发配置。三者只读仓库清单与现有代码，并行组 runtime；过程文档统一由 /root 更新。各自完成针对性验证并将文件、证据和限制交给 [DDP-104](../task/task-ddp-104-verification.md)。生成缓存与测试目录隔离，不能共写安装工作区。
