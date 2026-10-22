---
feature: docker-development-deployment
status: in-review
baseline: main@2b2372a6ee7b6daa7323e71b95f5cb3de38e924d
initial_baseline: CiBeDerORG/EasyTier:feat/ecosystem-web-ui@753339df58d842ab4de36f5cd1c90788c995ea0c
updated_at: 2026-10-22
---

# Docker 开发部署：需求/计划汇总

## 目标与范围

为 `CiBeDerORG/EasyNetTopo` 的持续魔改提供单入口 Docker Compose 开发部署。父目录可为 `/vol3/1000/software/modding/`，外层仓库目录名可自定义，文档以 `EasyNetTopo/` 为例；入口为仓库根 `deploy.sh`，配套文件在 `deploy/`，配置在 `.env`，全部项目缓存、生成代码与应用数据在 `data/`。前端热更新、后端增量编译后重启；编译失败停止对应服务，不回滚。

## 核查基线与现状依据

初次实现基于上面的 `initial_baseline`，实现分支为旧仓库的 `feat/docker-development-deploy`。2026-10-21 迁移复核基于新仓库 `main@a3e35a2a01ca87a87f356763a69fc49a38cb6258`；工作快照的全部 1526 个文件和 Git tree 均与该远端提交一致，修改前工作区干净。历史验证结果仍只对应初次实现，不作为本轮复跑结果。

| 依据 | 类别 | 结论及来源 |
| --- | --- | --- |
| EV-DDP-01 | 用户约束 | 最新八项要求确定目录、唯一入口、分支与模板；此前明确失败不回滚。 |
| EV-DDP-02 | 已核实事实 | `rust-toolchain.toml`、`.github/actions/prepare-build/action.yml` 使用 Rust 1.95/protoc 35.1；`.github/actions/prepare-pnpm/action.yml` 使用 Node 22/pnpm 10。 |
| EV-DDP-03 | 已核实事实 | `easytier-web/src/main.rs::Cli` 支持非 embed Web API/配置服务、SQLite 与 ET_* 配置；可选空 URL 不能直接传入。 |
| EV-DDP-04 | 已核实事实 | `frontend/package.json::scripts.dev` 先构建共享库；共享库源码入口为 `src/easytier-frontend-lib.ts`，Proto 生成器写 `src/generated`。 |
| EV-DDP-05 | 拟新增方案 | 容器只读挂载源码，同步到各自 `data/workspaces/` 编译；共享库源码 alias、同源 API 与独立增量 watcher。 |
| EV-DDP-06 | 已核实事实 | 新仓库 `deploy.sh` 的 origin 校验仍指向旧仓库，Git 模式为 `100644`，不能按说明直接执行；入口按脚本位置定位并校验 Git 根目录，Cargo/pnpm/生成器依赖内部相对布局，不校验外层目录名。DDP-101 负责迁移修复，/root/migration_runtime_review 只读核查目录约束与差异。 |
| EV-DDP-07 | 已核实事实 | `main@d986fc68800a0c66df99b268aacece1aadeb2bd0` 的 `deploy.sh::ENV_KEYS` 不接受 `BUILDX_BUILDER`；用户在 NAS 的 `.env` 配置此项后被拒绝。已有 EasyADB 构建器为 `easyadb-proxy`，当前入口仅接受从调用环境继承它。DDP-101 负责补齐配置契约，独立审查只读复核优先级与重新执行行为。 |
| EV-DDP-08 | 已核实事实 | `main@2b2372a6ee7b6daa7323e71b95f5cb3de38e924d` 已接受构建器名称，但 `ENV_KEYS` 仍未接受 `BUILDKIT_PROGRESS`；用户的 `.env` 第 59 行因此阻塞。DDP-101 补齐日志格式配置，沿用非空调用环境覆盖与重执行来源规则。 |

## 进度与下一步

需求、计划、任务现已建立：[需求](require/README.md)、[计划](plan/README.md)、[任务](task/README.md)。DDP-101 负责入口与接线；DDP-102 负责运行器；DDP-103 负责开发前端配置，三者按契约并行。三个实现任务已经交付，DDP-104 已完成可执行的统一复跑：Python 30/30、Node 8/8 通过，源码/文档检查通过；真实 Docker/Rust 编译验收受环境限制，总体保持 in-review。

本轮 DDP-101 已完成入口、执行权限、模板、Compose 默认项目名、使用说明和原有测试夹具的迁移维护；目录约束由独立只读审查并行核对。Bash/Python 语法与差异检查通过，证据交给 DDP-104 记录于 [verification.md](../../deploy/verification.md)。按用户对简单改动的要求，本轮未运行测试、构建或 GitHub Actions；总体仍等待目标 NAS 的完整部署验收。

2026-10-22：DDP-101 已修复 `.env` 对 `BUILDX_BUILDER` 的支持；保留命令环境覆盖、默认构建器与现有失败语义。/root 唯一写入入口、模板和过程文档，/root/migration_runtime_review 的独立只读审查通过；本轮 Bash 语法和差异检查通过，未运行测试、构建或 CI。DDP-104 继续等待目标机完整部署结果。

2026-10-22：DDP-101 已补齐 `.env` 的 `BUILDKIT_PROGRESS` 支持；/root 唯一写入，/root/migration_runtime_review 独立静态复审通过。Bash 语法与差异检查通过，未运行测试、构建或 CI；目标机部署验收仍由 DDP-104 跟进。

## 问题与限制

| 编号 | 状态 | 证据、影响与处理 |
| --- | --- | --- |
| ISSUE-DDP-01 | 已核实环境限制 | 当前环境没有 Docker daemon、Cargo；已下载官方 Compose 2.39.4 完成配置解析并通过脚本/前端行为验证，真实 Docker 启动/完整 Rust 构建仍需目标主机环境。负责人 /root，影响 DDP-104。 |
| ISSUE-DDP-02 | 已核实兼容边界 | `restful/mod.rs::BrowserSecurity` 要求 HTTPS 或 loopback HTTP 管理来源；保留现有校验并记录远程访问方式。负责人 /root。 |
| ISSUE-DDP-03 | 已决策 | Docker 自身镜像层仍由 Docker data-root 管理；项目运行、构建工作区与缓存全部位于仓库 data，不调整 daemon 配置。 |
| ISSUE-DDP-04 | 已解决 | DDP-101 已将 origin 白名单迁移到新仓库，恢复入口 `100755`，同步文档和模板，并明确外层目录自由、入口必须位于 Git 根目录及 .env/data 的符号链接限制。验证范围见本轮记录。 |
| ISSUE-DDP-05 | 配置修复已交付 | `.env` 中的 `BUILDX_BUILDER` 被白名单拒绝，阻塞复用已有构建器。DDP-101 已补齐解析、模板与使用说明；此修复不包含镜像认证、Dockerfile 下载或运行时依赖的代理接线，目标机完整部署仍待验证。 |
| ISSUE-DDP-06 | 配置修复已交付 | `.env` 中的 `BUILDKIT_PROGRESS` 未被接受；DDP-101 已增加日志格式项、模板与说明，修复不改变构建器或下载代理。 |

## 维护记录

- 2026-10-21：按用户最新目录要求建立汇总；保留原业务协议、账户与 UI，不把开发部署扩成镜像发布或回滚系统。

- 2026-10-21：三项实现交接并统一复跑通过；验收证据见 [verification.md](../../deploy/verification.md)，总体转入 in-review，未将 Docker/Rust 未执行项记作通过。

- 2026-10-21：完成 EasyNetTopo 仓库迁移与主目录约束复核；修复 origin 白名单与入口执行位，内部包名和源码相对布局保持兼容。仅执行本轮静态核查，保留原项目历史测试记录。

- 2026-10-22：补齐 `.env` 的已有 Buildx 构建器选择，保留命令环境覆盖并修正重执行时的来源区分；模板和说明同步，仅执行语法与静态审阅。

- 2026-10-22：继续补齐 `.env` 的 `BUILDKIT_PROGRESS`，默认 auto，示例 plain；沿用相同覆盖和重执行规则，仅执行语法与静态审阅。

## 2026-10-22：已配置构建器的镜像认证超时

用户提供真实 FNOS 日志：在 `easyadb-proxy` 选择后拉取基础镜像仍于 `auth.docker.io/token` 超时。核对 EasyADB 原构建脚本后确定原流程还为宿主 Buildx CLI 的令牌请求注入 Mihomo 代理，并通过 Buildx 参数单独为 Dockerfile RUN 下载提供代理。DDP-101 本轮增加 `deploy/build-mihomo.sh`，仅选中 `easyadb-proxy` 时复用其现有配置，不新建或修改共享构建器；其余默认 Compose 构建路径保留。仍待 NAS 首次真实镜像构建、Cargo/pnpm 运行网络的验收，DDP-104 保持 in_review。

## 2026-10-22：Debian 软件源经 Mihomo 返回 HTTP 502

用户 FNOS 日志确认基础镜像认证已通过，下一失败发生在 `deploy/Dockerfile` 的 `apt-get update`：`http://deb.debian.org/debian/dists/bookworm/InRelease` 经 `172.19.0.2:7890` 代理返回 502。DDP-101 将现有 Debian 官方 APT 源转为 HTTPS，并对索引、软件包下载配置有限重试；不改共享 BuildKit / Docker daemon、不关闭 Debian 仓库签名检查。已在临时 deb822/legacy 来源上核对替换与 Signed-By 保留，未运行真正镜像构建，DDP-104 继续等待 FNOS 验证；如 HTTPS 仍 502，应检查 Mihomo 上游网络与规则。

## 2026-10-22：定时分支更新

参照已交付 EasyADB 的 cron 定时更新接口新增 EasyNetTopo 根入口 `deploy.sh schedule start <间隔>/status/logs/stop`；所有辅助实现在 `deploy/scheduled-deploy.sh`，状态/日志放入 `data/run/`。新增 `deploy.sh check` 先 fetch/快进检查远端 Git 树，只有树变化、部署尚无成功记录或上轮失败才构建，未变化不碰 Docker。cron 登记每分钟轮询与用户设定频率的到期节流，Git 操作沿用原 sudo 登录用户身份，复用应用部署锁。本次不改变已知编译失败不回滚原则；NAS cron/构建整体验收待实机测试。

## 2026-10-22：工具链 RUN 退出码 1

用户在源 HTTPS 修复后回传 `deploy/Dockerfile` 的整段工具链安装 RUN 以退出码 1 失败，未包含失败命令前面的日志。确认现有 Dockerfile 将 APT/npm/rustup 联网步骤混在同一个 RUN 中，且 `rust:1.95-bookworm` 已有 Rust 工具链；DDP-101 本轮拆分缓存层以便定位，并将无条件执行的 `rustup toolchain install` 改为版本名缺失时才执行，保留既定版本及安全边界。该修改已审阅但尚未在目标 FnOS 上真实构建，DDP-104 继续根据下一次具体日志验收。

## 2026-10-22：修复 cron Git 根目录校验的 root 身份错误

用户实际 cron 日志显示每分钟检查在 `deploy.sh` 的 Git 根目录预检报错。经代码核对，root 直接执行 `git -C` 触发 Git 对登录用户所拥有仓库的 dubious ownership 防护， stderr 被原代码重定向，错误误报为目录错误。DDP-101 已统一全部 Git 操作使用用户 UID，含预检、更新、快进、HEAD/工作区状态与部署成功标记；不关闭安全检查、不调整共享 BuildKit 或 root cron 身份。DDP-104 继续等待 NAS 实测。
