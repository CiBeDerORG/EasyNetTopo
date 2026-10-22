# 开发部署验证记录

日期：2026-10-21。实现基线：`feat/ecosystem-web-ui@753339df58d842ab4de36f5cd1c90788c995ea0c`；部署变更位于 `feat/docker-development-deploy`。

## 已执行

| 检查 | 结果 | 实际边界 |
| --- | --- | --- |
| `bash -n deploy.sh`、`git diff --check` | 通过 | Bash 语法与变更空白检查。 |
| 入口测试 | 15/15 通过 | 真实 Bash、真实临时 Git 仓库、替身 Docker；验证父目录/空格路径、模板与已有配置、秘密字面值、端口校验、分支切换、快进与分叉、停止保留数据、失败非零且不启动旧服务。 |
| Compose 配置测试 | 2/2 通过 | 官方 Docker Compose 2.39.4 真实解析；验证 build context、source/data bind、UID/GID、端口、ws→tcp、前端密钥隔离及无自动重试。未连接 Docker daemon。 |
| 运行器生命周期测试 | 13/13 通过 | 真实临时文件、rsync、Linux 进程组与随机本机 TCP 端口；Cargo/pnpm/Vite 使用替身，验证失败停服、修正恢复、编译期间变更、新容器不复用旧成功、暂停/停止、启动与运行退出、进程组清理、前端普通变化不重启。 |
| Node/Vite 集成 | 8/8 通过，无跳过 | Node 22.23.3、pnpm 10.34.7、protoc 35.1、项目锁定 Vite 5.4.21；真实 HTTP、Vue/TS/YAML/Proto/CSS 转换及 WebSocket HMR。 |
| 前端 CLI 终止 | 通过 | 启动最终 CLI、请求 api_meta.js 后发送 SIGTERM，3 秒内退出 0。 |

协调者统一复跑：Python 30 项全部通过（14.278 秒）；Node 8 项全部通过（4.660 秒）。前端安装与代码生成在独立的 `data/verification/frontend` 工作区执行，原源码目录没有新增 node_modules 或 generated 输出。

### 前端实测内容

- `/api_meta.js` 始终返回空 `api_host`；仅 `/api/` 转发后端，保留浏览器 Host/Origin。
- 禁止读取前端 `.env`；data 私有文件与未允许 Host 的请求被拒绝。
- Web 入口直接使用 frontend-lib 源码，Vue 解析一致；两边模板进入 Tailwind 扫描，嵌套 CSS 被实际转换。
- 修改共享 Vue 组件后，同一个 Vite 进程发出真实 WebSocket `update`，再次请求取得更新后的代码。

### 已解决的验证问题

- 默认沙箱不允许本机 socket；仅为上述隔离测试获准启用 loopback 监听后重跑成功。
- `docker compose config` 在输出规范化配置时会将 `$` 转义成 `$$`。已核对 Docker Compose v2.39.4 的 `cmd/compose/config.go::escapeDollarSign`，测试按序列化后的字面值校验，不把输出转义误判为容器环境变量变化。
- Vite 5 的关闭测试需等待预优化和待处理模块请求结束；修正测试时序后完整退出。最终 CLI 的正常 SIGTERM 也已单独验证。

## 复跑命令

Python 行为测试需要宿主 Python 3、Git、Bash、flock、rsync；这是测试环境要求，正常部署宿主只需 README 中的 Docker 工具。

```bash
COMPOSE_BIN="$PWD/data/verification/tools/docker-compose" \
PYTHONDONTWRITEBYTECODE=1 \
python3 -m unittest discover -s deploy/tests -p 'test_*.py' -v
```

`COMPOSE_BIN` 指向官方 Compose v2 可执行文件。未设置时仅 Compose 真实解析测试被明确跳过；其他测试继续运行。

前端集成需一个已用锁文件安装 frontend/lib 依赖、并执行原 Proto codegen 的可丢弃工作区：

```bash
EASYTIER_FRONTEND_TEST_WORKSPACE="$PWD/data/verification/frontend" \
npm --cache "$PWD/data/verification/npm-cache" exec \
  --yes --package=node@22 -- \
  node --test --test-reporter=tap deploy/tests/frontend.test.mjs
```

未设置工作区时，测试明确跳过真实 Vite 集成，不把轻量测试当作完整验证。准备工作区时不要复制原仓库的 `.env`、`.git` 或 `data/`。

## 尚未执行

当前环境没有 Docker daemon 和 Rust 编译环境，因此以下项目仍需在目标 Linux/Docker 主机验证，不能将上面的替身测试或配置解析视为替代：

1. 开发镜像实际构建，amd64/arm64 各架构拉取及工具链运行。
2. `cargo build --locked -p easytier-web --bin easytier-web` 的完整 Rust 编译。
3. Compose 整套启动、实际 SQLite 初始化、账号登录、配置服务与真实节点连接。
4. 目标 NAS 文件系统上的监听时延、主机重启后的手动启动，以及 HTTPS 反向代理部署。

浏览器完整页面与真实后端账户流程也未执行；本次没有修改这些业务流程。功能实现已交付，需求总体保留 `in-review`，等待目标主机的 Docker/Rust 验收证据。

## EasyNetTopo 仓库迁移核查（2026-10-21）

迁移基线：`CiBeDerORG/EasyNetTopo:main@a3e35a2a01ca87a87f356763a69fc49a38cb6258`。上面的 Python 30/30、Node 8/8 等结果属于旧仓库的初次实现，本轮没有复跑，也不据此宣称新仓库的 Docker 部署通过。

### 本轮已执行

| 核查 | 结果与边界 |
| --- | --- |
| 仓库快照 | 1526 个文件逐 blob 核对，树 `a2a421b44cd29d6795fd28632a76af06c072bbaa` 与远端基线一致；仅在该基线上修改部署相关文件。 |
| `bash -n deploy.sh`、`git diff --check` | 通过，覆盖入口语法和变更空白。 |
| Python `ast.parse` | 迁移后的入口测试夹具语法通过；没有执行测试。 |
| 入口执行权限 | 新仓库原模式为 `100644`；已恢复 `100755`，直接调用无需另加 `bash`。 |
| 迁移配置静态复核 | origin 接受新仓库的 HTTPS、SCP 风格 SSH、`ssh://` 地址；入口、模板、Compose 默认项目名一致；已有 `.env` 创建/保留逻辑不变。 |
| 独立目录审查 | /root/migration_runtime_review 核对部署入口、运行器、Cargo/build.rs、pnpm、Vite 和 Proto 生成器：当前 Linux Docker 链路没有外层目录名或宿主固定绝对路径校验。 |

### 主目录约束结论

- `deploy.sh` 根据脚本目录和 `pwd -P` 定位；`up/update` 要求该目录就是 Git 仓库根。Git、Compose、构建子进程使用明确的工作目录，不依赖调用时的 cwd。
- `.env` 必须是普通文件，`data/` 不能是符号链接；路径不能包含 CR/LF。仓库外的单文件入口符号链接不在现有支持范围内。
- 容器中的 `/source`、`/data` 是挂载约定；内部 `easytier-*` 目录、Cargo 包名、pnpm 成员和生成器相对路径应保留。
- `.cargo/config.toml` 中 OHOS 专用绝对 SDK 路径属于其他目标平台，不参与本次 Linux amd64/arm64 Docker 构建。

### 本轮未执行

按用户对简单改动的要求，未运行测试、安装依赖、构建镜像或触发 GitHub Actions。未连接用户 NAS，真实 Docker/Rust 启动验收仍待目标主机完成；这些静态结果只证明迁移改动与目录约束的审查范围。

## 已有 Buildx 构建器配置修复（2026-10-22）

基线：`CiBeDerORG/EasyNetTopo:main@d986fc68800a0c66df99b268aacece1aadeb2bd0`。用户 NAS 日志明确报告 `.env` 中 `BUILDX_BUILDER` 被白名单拒绝；本轮增加该配置，复用现有 Compose 构建调用。模板留空，用户可设置已有的 `easyadb-proxy`。

### 本轮已执行

- `bash -n deploy.sh`、`git diff --check` 通过。
- /root/migration_runtime_review 独立静态复审通过：非空调用环境优先于 `.env`，均空时交给 Docker；重执行入口前恢复原始调用环境，避免把 `.env` 值提升成命令覆盖；统一导出传给原 Compose 调用。
- 模板与使用说明同步；已有 `.env` 的首次创建/保留规则不变，入口继续使用 `100755`。

### 本轮未执行与边界

按用户对简单改动的要求，未增加或执行测试，未构建镜像、安装依赖或触发 GitHub Actions。未连接 NAS 验证新入口；不将本轮静态结果记作镜像拉取或应用启动成功。此前日志中的镜像站 502、宿主端认证请求、Dockerfile 下载和运行容器依赖下载的代理接线均不在此配置修复范围内。

## BuildKit 日志配置补充（2026-10-22）

基线：`main@2b2372a6ee7b6daa7323e71b95f5cb3de38e924d`。用户随后反馈 `.env` 第 59 行 `BUILDKIT_PROGRESS` 被拒绝。本轮补齐解析、默认值、合法日志格式校验、模板和说明；默认 `auto`，支持 `plain` 排错。

- `bash -n deploy.sh`、`git diff --check` 通过。
- /root/migration_runtime_review 独立静态复审通过：先取非空调用环境、文件值或默认 auto，再校验；重新执行前分别恢复 builder 与 progress 的原始调用环境。
- 未新增或执行测试，未构建镜像、安装依赖或触发 CI；没有连接 NAS。此项只解除配置解析阻塞，不代表代理下载或部署运行通过。

## easyadb-proxy 镜像认证修复（2026-10-22）

用户在 FnOS 的 easyadb-proxy BuildKit 模式下遇到 `auth.docker.io/token` 连接超时。本轮依据 EasyADB 已有 `scripts/build-with-mihomo.sh`、`proxy-host.sh`、`proxy-config.sh`，增加 `deploy/build-mihomo.sh`，在已明确指定 `BUILDX_BUILDER=easyadb-proxy` 时分别配置宿主 Buildx 认证代理与 Dockerfile RUN 代理，生成镜像导入本机后保持原 Compose 部署。

共享 BuildKit、Mihomo、Docker daemon 均不修改。代理地址只从已有 BuildKit 容器读取，敏感值不写入仓库与常规日志；非 easyadb-proxy 构建路径不变。该修复未在用户 NAS 实测，镜像代理和运行时 Cargo/pnpm 下载待目标机验证。

## Debian APT 源经 Mihomo 返回 502（2026-10-22）

用户在 `easyadb-proxy` 成功进入工具链镜像构建后，`apt-get update` 通过 `172.19.0.2:7890` 请求 `http://deb.debian.org/debian/dists/bookworm/InRelease` 返回 `502 Bad Gateway`，随后 APT 报 `repository is not signed`。这是索引未取得导致的连带错误，不能关闭 apt-secure 或放宽签名检查。

本轮仅修改 `deploy/Dockerfile`：将 Debian 官方 HTTP 源替换为 HTTPS，兼容 deb822 `debian.sources` 和旧 `sources.list`，并为 APT update/install 指定 `Acquire::Retries=3`。采用临时 Bookworm 来源文件核对了两种格式的替换，`Signed-By` 未改；没有 Docker daemon 或目标 FnOS 的网络可供真实构建。**HTTPS 是否能通过现有 Mihomo 上游还须目标 NAS 验证。**
