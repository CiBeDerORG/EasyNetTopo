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

## Debian APT HTTP 502 的局部修复（2026-10-22）

按 FNOS 构建日志定位到 `deploy/Dockerfile` 的 HTTP Debian 官方源被 Mihomo 代理返回 502；由于 InRelease 没有下载成功引发的 `not signed` 不属于真实仓库签名校验失败。本轮在 Dockerfile 内将 `deb.debian.org`、`security.debian.org` 官方源改成 HTTPS，保留 apt-secure 和原来源的 Signed-By，并添加 `Acquire::Retries=3`。不修改 Buildx/Mihomo 共享配置、Node/Rust 版本及运行容器网络；已使用临时两种格式来源作定向转换核查，完整 FnOS 构建未执行，交 DDP-104 验收。

## 定时更新接入（2026-10-22）

新增 `deploy/scheduled-deploy.sh` 辅助脚本，根入口 `schedule` 委派 start/status/logs/stop，`check` 提供有条件 Git fetch/快进/部署。状态与成功 Git 树保存在 data/run，不复制业务数据；cron 精确标记、Git 原用户、部署锁和失败标记确保不覆盖本地改动，也不误把未成功构建记为成功。实际 FnOS 上 Cron / root UID / Docker 联调交给 DDP-104。

## 工具链构建阶段拆分（2026-10-22）

针对用户仅有复合 RUN 退出码 1 的日志，在 `deploy/Dockerfile` 将 Node/npm 软链接与校验、Debian APT 安装、pnpm 安装和 Rust 工具链检查独立成层，确保失败能落到具体步骤并让已成功层被缓存。基础 `rust:1.95-bookworm` 已安装目标 Rust 工具链，因此优先复用已有 Rust，只有版本名缺失时才补装 Rustup。不修改认证代理、版本、签名校验或服务运行规则；待目标 FnOS 验证实际构建。

## 定时任务 root Git 身份修复（2026-10-22）

将 `deploy.sh` 中散落的 root `git` 调用统一为在部署入口最早位置定义的 `git_as_owner`，按 `EASYNETTOPO_GIT_UID` 或 `SUDO_UID` 切换原登录用户执行 Git，仅 Docker/Compose 继续由 root 运行。修复 `safe.directory` 导致定时任务在根目录预检阶段失败；工作区状态失败不视为干净，保留快进、dirty 保护、无变更跳过和失败重试语义。不新增 root 全局 safe.directory 豁免。增加 root 执行非 root 自有仓库的回归场景，FnOS 实际执行交 DDP-104。

## protoc 安装下载源容错（2026-10-22）

在 `deploy/Dockerfile` 接入可检查的 `deploy/install-protoc.sh`，解决 GitHub Release 重定向到 `release-assets.githubusercontent.com` 后 TLS 连接失败。下载先走官方 Release，失败再尝试 SourceForge exact mirror；必须匹配 35.1 对应架构的官方 SHA-256。维护 amd64/arm64 和版本固定契约，未知版本无校验值拒绝；不改变安装目标、BuildKit 或运行时配置。隔离模拟 4 个场景通过，FnOS 端到端待确认。
