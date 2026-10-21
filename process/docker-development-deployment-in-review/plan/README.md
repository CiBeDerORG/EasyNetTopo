---
id: PLAN-DDP
feature: docker-development-deployment
status: in-review
updated_at: 2026-10-21
---

# 计划索引

[汇总](../README.md) · [需求](../require/README.md) · [任务](../task/README.md)

| 计划 | 工作包 | 顺序 |
| --- | --- | --- |
| [PLAN-DDP-001 部署实现](plan-ddp-001-development-runtime.md) | DDP-101 入口/Compose；DDP-102 运行器；DDP-103 前端热更新 | 下述契约已确定，三个写入范围并行。 |
| [PLAN-DDP-002 验证](plan-ddp-002-verification.md) | DDP-104 联调、失败验证与文档 | 三个实现交付后执行。 |

## 公共契约（唯一来源）

- 仓库根 deploy.sh，deploy/ 配套，.env 配置，data/ 所有项目状态；入口以脚本路径定位。
- Compose 服务 backend、frontend；源码只读 /source，数据 /data；每服务同步源码到 /data/workspaces/<service>，排除 .git、.env、data、node_modules、target、dist 与生成目录，不从宿主复制缓存。
- Python 入口：python3 /source/deploy/runner.py backend|frontend；health <service> 检查状态和进程；状态 /data/run/<service>.json，state 为 starting/building/running/failed/stopped，失败含 exit_code，不记录秘密。暂停标记 /data/run/pause。
- 控制 env：EASYTIER_WATCH=true，EASYTIER_POLL_INTERVAL=1，EASYTIER_DEBOUNCE=1，CARGO_BUILD_PROFILE=dev|release，CARGO_BUILD_JOBS=2。后端实际 ET_* 通过 Compose 白名单传入，可选空值省略。
- 后端在 /data/workspaces/backend 执行 cargo build --locked -p easytier-web --bin easytier-web（release 使用 --release）；CARGO_HOME=/data/cache/cargo，CARGO_TARGET_DIR=/data/cache/target；数据库 /data/web/et.db。
- 前端在 /data/workspaces/frontend 安装 pnpm --filter easytier-frontend... --frozen-lockfile --store-dir /data/cache/pnpm；复用现有 codegen；node deploy/frontend.mjs 启动 Vite，前端脚本不自行安装或生成代码。
- 前端接收 WEB_PORT=11210，API_BASE_URL=http://backend:11211（由 Compose 拼接实际端口），VITE_HMR_HOST/PORT/PROTOCOL 可空，VITE_ALLOWED_HOSTS 可空；缓存 /data/cache/vite。共享库源码 alias；/api_meta.js 返回空 api_host；代理仅 ^/api/，保留原 Host。
- 改源码只同步，前端 HMR；依赖/Proto 变化先停前端、重装/生成再启动；Rust 变化先停后端再编译，失败保持停止。构建期间再次变更必须重试最新快照，不能声称半途产物对应最新源码。
- 默认命令 up 构建并启动；update 快进拉取后 up；stop、status、logs 为辅助命令。GIT_BRANCH 为空则当前分支；不强制 reset/stash。up/update 失败返回非零。

公共写入唯一归 DDP-101：deploy.sh、Compose、Dockerfile、模板、.gitignore 和过程文档。运行器与前端不改这些文件，只提出接线需求；完成后交给 DDP-104。
