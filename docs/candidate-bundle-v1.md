# M3-03A：固定 Git Commit 的 ZIP v1 候选部署包

> 当前状态：仅支持控制端内部的 **候选包构建**。此产物没有通过 Deploy 完整 Schema + 语义校验，也没有发布方签名或 Agent 主机本地批准。**禁止据此启用安装按钮、执行脚本或 Compose。**

## 1. 构建输入与不可变性

`catalog.Service.BuildCandidateBundle(ctx, source_id, app_id)` 仅从 SQLite 当前成功快照读取 `source_id + app_id + revision + manifest_sha256`，并要求二者属于同一来源/固定 Git Commit。

- Git 缓存要求已有 `refs/easynettopo/catalog/<commit>`，其解析必须精确等于 SQLite 中的 Commit SHA-1。缺少固定引用时拒绝构建，不重新 fetch、checkout，也不读取移动中的分支。
- 从该 Commit 的 Git 树中读取 `<app_id>/app.yaml` 和 `package.files` 指定的 Blob，重新核对 Manifest SHA-256，拒绝符号链接、缺少文件、非普通文件。
- 控制端从 Git 缓存获取的文件不执行、不复制到应用安装目录，也不借用其他工作区内容。
- 缺少完整 Schema/语义验证前，该方法输出只供内部测试、格式评审使用；尚未接入公开 HTTP API 与 Agent 上传客户端。

## 2. ZIP v1 候选格式

构建器使用 Go 标准库 ZIP Store 方法、固定 1980-01-01 UTC 时间戳和按 ZIP 目标路径字典序排序，避免运行时间、源文件修改时间或 Manifest 文件排列顺序改变 ZIP 字节。

归档仅包含：

- `bundle-manifest.json`：`format_version=1`、`source_id`、`application_id`、固定 `git_revision`、`manifest_sha256`、所有明示资源的源/目标路径、权限、字节数和 SHA-256。
- `<app_id>/app.yaml`：来自固定 Git Commit 的 Manifest 原始字节。
- `package.files[*].destination`：严格按 Manifest 指定的源 Blob 映射到目标路径。当前共享库只允许 `lib/* → deploy/lib/*` 精确映射，其余文件限应用自身目录。

只允许常规文件权限 `0644` / `0755`、1–128 个资源、每个文件最多 5 MiB、资源总和最多 24 MiB，最终 ZIP 不得超过 Agent 的 64 MiB 限制。拒绝路径穿越、绝对/隐藏路径、密钥/数据/运行时目录、大小写碰撞及文件与子目录碰撞。**不打包 .env、.secrets 或应用运行数据。**

`SHA-256(ZIP)` 为上传字节完整性标识；它既不是 `Git Commit`，也不是可信发布身份。

## 3. 待通过的独立准入

1. 与 Deploy `schema/app-manifest-v1.schema.json` 及 `tools/validate_app_manifest.py` 完成完整 Schema + 语义一致性测试，包括 Compose、健康状态、配置/端口和数据保留。当前 `previewManifest` 只做商店预览检查，**不可替代**。
2. 明确 Git 来源授权、签名发布流程、受信任公钥/主机本地批准，以及发布元信息签名的规范化字节。不能由 `source_id`、Manifest 自称官方或用户操作直接授权 Shell。
3. 衔接经过审核的构建结果与 `hosts.Client.HasBundle` / `UploadBundle`，核验 Agent 回执；**不得**未经完整校验上传第三方可执行资源并进入任务路径。
4. Agent `POST /v1/tasks` 持久化及最小权限执行门禁、冲突检查、数据保留、安全日志与实际 fnOS/Linux/Sub2API 安装验证，属于后续独立任务。

## 4. 测试

- ZIP 字节确定性、归档时间戳、Manifest 与资源 SHA-256、Unix 权限。
- 非法来源/Commit/Manifest、路径穿越、敏感目录、重复/大小写碰撞、非法权限、文件尺寸限制。
- 用真实本地 Git Commit + 裸仓库固定引用，从 SQLite 目录快照构建；修改工作区不影响候选包，删除固定引用必须拒绝构建。

本阶段不修改 Deploy 或 EasyHostAgent，不引入 SSH 密钥，不执行任何安装命令。
