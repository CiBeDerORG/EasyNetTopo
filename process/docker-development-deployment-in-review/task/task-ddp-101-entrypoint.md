---
id: DDP-101
feature: docker-development-deployment
summary: ../README.md
plans: [PLAN-DDP-001]
status: done
owner: /root
depends_on: []
read_scope: [easytier-web/src, Cargo.toml, rust-toolchain.toml, .github/actions]
write_scope: [deploy.sh, deploy/compose.yaml, deploy/Dockerfile, deploy/.env.example, deploy/README.md, deploy/tests/test_entrypoint.py, deploy/tests/test_compose.py, .gitignore, process/README.md, process/docker-development-deployment-in-review]
parallel_group: runtime
requirements: [DDP-R01, DDP-R02, DDP-R03, DDP-R04, DDP-R05, DDP-R06]
acceptance: [DDP-A01, DDP-A02, DDP-A03, DDP-A04, DDP-A05, DDP-A06]
updated_at: 2026-10-21
---

# DDP-101：入口与容器

[汇总](../README.md) · [需求](../require/README.md) · [公共契约与计划](../plan/README.md) · [看板](README.md)

## 依据、范围与交接

采用汇总 EV-DDP-01–05 与已确认基线。按公共契约实现本任务独占 write_scope；未列入的现有业务代码只读。与其他 runtime 实现任务并行，公共入口/Compose/过程文档由 DDP-101 独占；接线需求发送给该所有者，交付到 DDP-104。

构建与测试产物使用隔离临时目录；状态变化由 /root 同步。可执行验证须覆盖相关成功、失败和边界，不把替身检查报作真实 Docker/EasyTier 端到端成功。

## 执行与验收记录

已交付 deploy.sh、Dockerfile、Compose、模板、使用文档和 ignore。15 项入口行为测试通过（真实 Bash/Git，替身 Docker）；真实 Compose 2.39.4 配置解析 2 项通过。秘密字面值、分支保护、先停后编译及失败非零均有证据。交接公共文件给 DDP-104。
