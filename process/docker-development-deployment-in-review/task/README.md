# 任务看板

[汇总](../README.md) · [需求](../require/README.md) · [计划](../plan/README.md)

| 任务 | 状态 | 负责人 | 前置 |
| --- | --- | --- | --- |
| [DDP-101 入口与容器](task-ddp-101-entrypoint.md) | done；已适配 EasyNetTopo，支持已有 Buildx 构建器配置 | /root | 公共契约 |
| [DDP-102 运行器](task-ddp-102-runner.md) | done | /root/deploy_design_review | 公共契约 |
| [DDP-103 前端开发配置](task-ddp-103-frontend.md) | done | /root/easytier_docker_review | 公共契约 |
| [DDP-104 联调](task-ddp-104-verification.md) | in_review | /root | DDP-101、102、103 |

文件写入权由各任务独占；过程文档及公共接线 /root 唯一写入。无并发运行的共同构建输出，交接依照公共契约；未运行的 Docker/Rust 检查不计为通过。
