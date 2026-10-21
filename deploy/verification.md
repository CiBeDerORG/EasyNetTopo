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
