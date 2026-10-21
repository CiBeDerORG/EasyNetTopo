---
feature: docker-development-deployment
status: in-review
baseline: feat/ecosystem-web-ui@753339df58d842ab4de36f5cd1c90788c995ea0c
updated_at: 2026-10-21
---

# Docker 开发部署：需求/计划汇总

## 目标与范围

为 `CiBeDerORG/EasyTier` 的持续魔改提供单入口 Docker Compose 开发部署。父目录可为 `/vol3/1000/software/modding/`，仓库克隆为其下的 `easytier/`；入口为仓库根 `deploy.sh`，配套文件在 `deploy/`，配置在 `.env`，全部项目缓存、生成代码与应用数据在 `data/`。前端热更新、后端增量编译后重启；编译失败停止对应服务，不回滚。

## 核查基线与现状依据

2026-10-21 核查上述完整提交，初始工作区干净；实现分支 `feat/docker-development-deploy`。

| 依据 | 类别 | 结论及来源 |
| --- | --- | --- |
| EV-DDP-01 | 用户约束 | 最新八项要求确定目录、唯一入口、分支与模板；此前明确失败不回滚。 |
| EV-DDP-02 | 已核实事实 | `rust-toolchain.toml`、`.github/actions/prepare-build/action.yml` 使用 Rust 1.95/protoc 35.1；`.github/actions/prepare-pnpm/action.yml` 使用 Node 22/pnpm 10。 |
| EV-DDP-03 | 已核实事实 | `easytier-web/src/main.rs::Cli` 支持非 embed Web API/配置服务、SQLite 与 ET_* 配置；可选空 URL 不能直接传入。 |
| EV-DDP-04 | 已核实事实 | `frontend/package.json::scripts.dev` 先构建共享库；共享库源码入口为 `src/easytier-frontend-lib.ts`，Proto 生成器写 `src/generated`。 |
| EV-DDP-05 | 拟新增方案 | 容器只读挂载源码，同步到各自 `data/workspaces/` 编译；共享库源码 alias、同源 API 与独立增量 watcher。 |

## 进度与下一步

需求、计划、任务现已建立：[需求](require/README.md)、[计划](plan/README.md)、[任务](task/README.md)。DDP-101 负责入口与接线；DDP-102 负责运行器；DDP-103 负责开发前端配置，三者按契约并行。三个实现任务已经交付，DDP-104 已完成可执行的统一复跑：Python 30/30、Node 8/8 通过，源码/文档检查通过；真实 Docker/Rust 编译验收受环境限制，总体保持 in-review。

## 问题与限制

| 编号 | 状态 | 证据、影响与处理 |
| --- | --- | --- |
| ISSUE-DDP-01 | 已核实环境限制 | 当前环境没有 Docker daemon、Cargo；已下载官方 Compose 2.39.4 完成配置解析并通过脚本/前端行为验证，真实 Docker 启动/完整 Rust 构建仍需目标主机环境。负责人 /root，影响 DDP-104。 |
| ISSUE-DDP-02 | 已核实兼容边界 | `restful/mod.rs::BrowserSecurity` 要求 HTTPS 或 loopback HTTP 管理来源；保留现有校验并记录远程访问方式。负责人 /root。 |
| ISSUE-DDP-03 | 已决策 | Docker 自身镜像层仍由 Docker data-root 管理；项目运行、构建工作区与缓存全部位于仓库 data，不调整 daemon 配置。 |

## 维护记录

- 2026-10-21：按用户最新目录要求建立汇总；保留原业务协议、账户与 UI，不把开发部署扩成镜像发布或回滚系统。

- 2026-10-21：三项实现交接并统一复跑通过；验收证据见 [verification.md](../../deploy/verification.md)，总体转入 in-review，未将 Docker/Rust 未执行项记作通过。
