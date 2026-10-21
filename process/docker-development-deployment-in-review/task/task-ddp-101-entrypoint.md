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
updated_at: 2026-10-22
---

# DDP-101：入口与容器

[汇总](../README.md) · [需求](../require/README.md) · [公共契约与计划](../plan/README.md) · [看板](README.md)

## 依据、范围与交接

采用汇总 EV-DDP-01–05 与已确认基线。按公共契约实现本任务独占 write_scope；未列入的现有业务代码只读。与其他 runtime 实现任务并行，公共入口/Compose/过程文档由 DDP-101 独占；接线需求发送给该所有者，交付到 DDP-104。

构建与测试产物使用隔离临时目录；状态变化由 /root 同步。可执行验证须覆盖相关成功、失败和边界，不把替身检查报作真实 Docker/EasyTier 端到端成功。

## 执行与验收记录

已交付 deploy.sh、Dockerfile、Compose、模板、使用文档和 ignore。15 项入口行为测试通过（真实 Bash/Git，替身 Docker）；真实 Compose 2.39.4 配置解析 2 项通过。秘密字面值、分支保护、先停后编译及失败非零均有证据。交接公共文件给 DDP-104。

## EasyNetTopo 迁移维护（2026-10-21）

依据汇总 EV-DDP-06，/root 认领原 write_scope 中的入口、模板、Compose、说明和测试夹具；独立审查 /root/migration_runtime_review 只读核对部署及构建路径，不写公共文件。修复 origin 白名单、恢复入口的 `100755` 执行权限、同步新仓库身份和默认 Compose 项目名，说明可自定义外层目录及既有 Git 根目录检查。保留已有 .env、数据、内部包名与失败不回滚语义；不新增代理配置或修改 Docker daemon。

本轮迁移已交付；Bash 语法、Python 测试夹具语法（只解析，不执行）与差异检查通过，入口执行位已恢复。/root/migration_runtime_review 的独立只读复核发现执行位问题，修正后未遗留其他迁移阻塞。验证说明写入权交给 DDP-104；按用户要求未运行测试、构建或 CI。以上 15/15 与 2/2 属于初次实现的历史验证，不作为本轮结果。

## 既有构建器配置修复（2026-10-22）

依据 EV-DDP-07，/root 重新认领入口、模板和文档，补齐 `.env` 的 `BUILDX_BUILDER` 配置。非空调用环境优先于 `.env`；二者均空时交给 Docker 当前选择。`update` / 分支切换重新执行入口前恢复原始调用环境，避免把文件配置误当作命令环境覆盖。复用现有 Compose 构建，不创建或修改共享构建器。独立审查 /root/migration_runtime_review 只读复核；本轮按用户要求仅做语法与静态核查，不运行测试或 CI。

修复已交付：`bash -n deploy.sh`、`git diff --check` 通过；独立静态复审未发现阻塞。既有 `.env` 保留，新增字段可由用户直接填写；构建前和 `init` 显示选用名称。首次更新须手动 Git 拉取，旧入口的 `update` 会先因未知字段失败。验证说明交接 DDP-104；真实 NAS 构建结果仍待回传。

## 构建日志配置补充（2026-10-22）

依据 EV-DDP-08，/root 认领入口、模板和文档，补齐 `BUILDKIT_PROGRESS`；沿用构建器项的非空调用环境覆盖和重执行恢复规则。空值默认 `auto`，支持 Docker 文档中的 `auto/plain/tty/quiet/none/rawjson`。独立审查只读复核，不运行测试或 CI。

补充已交付：Bash 语法、差异检查和 /root/migration_runtime_review 独立静态复审通过。现有 `.env` 保留，两种 Docker 控制项分别保存调用环境，重新执行时分别恢复；仅控制 BuildKit 输出，不影响后续应用日志或代理。结果交接 DDP-104，真实构建未执行。
