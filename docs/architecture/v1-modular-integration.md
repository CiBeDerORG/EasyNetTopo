# EasyNetTopo V1.0 — 模块化单体与适配器架构（设计草案）

> 本文是目标设计，非现有代码。技术选择已在产品讨论中确认：Go API + Vue 3/TypeScript/Vite + SQLite；不因为选择 Go 而移除前端 TypeScript。
> 历史设计草案：对应的旧版 R001/T001 需求与任务文件已撤销；本文不是当前第一期正式需求或开发任务依据。
> 原则：**平台与业务解耦、应用安装与业务集成分离、最小实现先行**。

## 1. 进程边界

- **EasyNetTopo**：一个 Go 后端进程，提供用户 Web/API，生产构建可 embed Vue 产物；一个 SQLite 文件作为平台持久化。无业务适配器时也能启动。
- **EasyHostAgent**：每台受管 Linux/FnOS 主机上独立 Go/systemd 服务，基于直接 IP:port + TLS 1.3 接受配对控制端请求，持有执行状态和 Docker 真相。
- **Deploy 或第三方 Git**：仅是版本化静态应用源，控制端按需 Fetch/Clone，Agent 不拉取，也不需要私有 Git 凭据。
- **业务服务**：Sub2API、EasyTier 等独立运行。未来专属业务集成走已有授权 API，不强制应用必须由平台安装。

浏览器只调用 EasyNetTopo 的 API。Git/Agent/业务系统凭据留在服务端。

## 2. Go 模块边界（实现时按需建立，禁止先造空目录）

| 逻辑模块 | 职责 | 不做的事 |
|---|---|---|
| platform | 登录、授权、配置、密钥引用、审计 | 业务系统专属字段 |
| integrations | 静态注册适配器元数据与能力集，后续管理独立业务连接 | Go .so 动态加载/第三方代码执行 |
| catalog | Git 源记录、手动同步、manifest 验证与不可变目录快照 | 远程 Docker 执行 |
| hosts | 主机 ID、IP 端点、指纹/Token 引用、按需主机与 Docker 状态 | 覆盖外部容器的业务逻辑 |
| operations | 业务操作审计、请求校验、任务索引、状态恢复 | 持有实际 Docker Engine |
| adapters/git | 可控 Git CLI、版本化只读工作副本、源认证 | 把 Git 凭据转交 Agent |
| adapters/easyhostagent | 配对、带指纹认证的 HTTPS、bundles、tasks、inventory 客户端 | 自行持久化执行中的远程任务 |
| httpapi | 路由、权限、验证、HTTP 错误映射 | 私有业务规则 |
| storage | SQLite migration 与仓储实现 | 引入额外数据库服务 |

规则：消费模块定义所需最小接口，由适配器实现；不要让 platform 反向依赖 EasyHostAgent、EasyTier、Docker 等第三方结构。模块间调用优先普通 Go 方法，不引入事件总线或服务发现系统。

### 最小接口草案（不是当前已提交 Go 代码）

~~~go
type CatalogProvider interface {
    Refresh(ctx context.Context, source Source) (CatalogSnapshot, error)
}

type HostProvider interface {
    GetHost(ctx context.Context, host Host) (HostSnapshot, error)
    Discover(ctx context.Context, host Host) (ResourceSnapshot, error)
}

type TaskExecutor interface {
    Submit(ctx context.Context, host Host, req OperationRequest) (TaskReceipt, error)
    GetTask(ctx context.Context, host Host, taskID string) (TaskSnapshot, error)
}

// 未来业务集成只有需要时注册；不是所有应用必需实现。
type IntegrationProvider interface {
    Kind() string
    Capabilities() []string
    Check(ctx context.Context, conn IntegrationConnection) error
}
~~~

不要设计万能 Plugin 接口或强迫 Git/主机/业务 API 共用一份资源 JSON。只有资源通用身份、状态、时间戳和能力声明需要共享。

## 3. SQLite 最小实体与约束（逻辑草案，不是可直接执行的迁移）

| 表 | 核心字段 | 重要约束 |
|---|---|---|
| system_settings | key、value | controller_id 是稳定 UUID；首次建立后不得静默重置 |
| app_sources | id、name、git_url、branch、credential_ref、trust_policy、last_success_commit | 来源 ID 由平台签发，不信 manifest 自报的官方身份 |
| catalog_snapshots | source_id、commit_sha、created_at、manifest_digest | 只有所有约束校验通过才切换当前快照 |
| catalog_entries | source_id、commit_sha、app_id、manifest_json | UNIQUE(source_id,commit_sha,app_id) |
| hosts | id、name、endpoint、agent_id、last_observed_at | 主机的 ID 不随 IP 变化 |
| agent_bindings | host_id、agent_id、spki_sha256、encrypted_token_ref、paired_at | 主机和 Agent 身份双向检查，当前仅一个控制端 |
| installations | id、host_id、source_id、app_id、commit_sha、bundle_sha256、runtime_kind、runtime_key、status、last_observed_at | UNIQUE(host_id,runtime_kind,runtime_key)；卸载后数据归属记录需留审计 |
| operation_tasks | id、host_id、agent_id、installation_id、action、status、error、updated_at | task_id 由控制端生成；HTTP 超时保留 outcome_unknown |
| audit_events | id、actor_id、action、resource_type、resource_id、created_at、outcome | 不记录 Token/明文密码 |

integration_connections 在未来实际加入业务 API 集成时创建，不为“以后可能用”预建迁移；当前业务只需适配器注册元数据，不需要动态插件配置表。

保持**平台期望记录**与 **Agent 实际运行信息**的区分。Agent `complete=false`、离线或超时不触发删除。数据库仅作上次快照缓存，读取页面要标明采集时间。

## 4. Git 源与目录原子性

1. 添加来源时收集 Git URL、ref（分支）、凭据引用；服务端限制可用协议、凭据访问范围及执行 Git 命令的工作目录，不接受用户输入作为任意 shell 字符串。
2. **只有用户点击刷新**才 fetch/clone；对本次获得的 commit SHA 做固定快照。禁止刷新时把旧成功快照清空。
3. 使用控制端内置 Schema + 语义验证器，而不是信任 Git 源自带 Schema。无效目录记录错误但不冒充可安装应用；刷新失败沿用上次成功状态。
4. 同一来源下使用 app_id，跨来源必须增加 source_id。不同来源宣告相同 Compose project 时在**目标主机安装预检**阻断，不能用商店身份不同绕过。
5. 安装读取固定 commit 的文件白名单并生成 SHA-256 固定包。source/commit/manifest digest/包 digest 一起进入平台审计及 Agent 任务元数据。
6. 删除 Git 源不删除已有的任务、应用实例或其被批准的原部署资源；安装实例持有必要的不可变来源证据。

## 5. 主机、权限与 TLS

- 用户只在后端提交 host_id，不直接控制 Agent 的路径、命令或 bearer；后端依据绑定的 Agent ID、端点、固定 SPKI 与加密 Token 发起 HTTPS。
- 首次配对前通过**主机本地可信终端**人工核对指纹。不能因为 GET /v1/identity 返回某指纹就自行信任该指纹。
- 现有 Agent `controller_id` 是稳定 UUID，旧 `device_id` 在 V1.0 语义映射为 EasyNetTopo `host_id`，继续使用 UUID 格式。
- 一台 Agent 一个有效控制端，成功新配对立即使先前 Token 失效；后端必须能识别授权丢失并提示重新配对，而不是自动重试旧凭据。
- IP:port 直连，不假设 EasyTier 网络存在。入站防火墙限定可信控制端，不发布 Agent 管理口到公网。
- Go 后端对浏览器/API 执行用户授权与审计；Agent 对每项管理动作做控制端授权、包/参数校验与受控执行。控制端 admin 权限本身不能自动提升成用户任意 Shell 能力。

## 6. 任务执行与状态归属

- 控制端 UUID 作 task_id，保存对应 host_id/agent_id。Agent **持久化后**再返回 HTTP 202。
- Agent 上的任务状态权威：accepted / running / cancelling / succeeded / failed / cancelled / interrupted。控制端专用 outcome_unknown 表示请求响应丢失，先查同一任务。
- 所有 Docker start/stop/restart、安装/更新/卸载沿用 Agent 的 POST /v1/tasks；不另建同步控制接口。
- 同一应用/Compose 资源互斥，涉及共享 .env/.secrets 的修改也必须串行化；不同资源任务的并行遵守实际共享依赖边界。
- 不自动回滚，不自动覆盖用户数据；执行失败保留现场、退出码（如有）与有权限的日志。页面按需轮询，不建立常驻 WebSocket、心跳、离线排队。

## 7. 首次发布、目录与构建

建议在**真正初始化时**按已采用模块创建最小目录：

~~~text
cmd/easynettopo/
internal/platform/
internal/catalog/
internal/hosts/
internal/operations/
internal/adapters/git/
internal/adapters/easyhostagent/
internal/httpapi/
internal/storage/
web/
docs/requirements/
docs/architecture/
~~~

技术基线：Go（具体受支持版本在初始化时锁定）、net/http 标准库优先、SQLite（驱动按依赖/跨平台实测确定）、slog、Vue 3 / TypeScript / Vite。生产构建可用 Go embed 打包静态资源，实现一个进程 + 数据目录；开发期可以分别运行前后端。

未确认的驱动、前端组件库和 Go 具体版本不得因本文示例自动入库。

## 8. 明确延后

- EasyTier 组网/拓扑业务接口；仅保留以后“业务集成连接”不依赖安装记录的架构边界。
- 插件热加载、插件沙箱、多控制端并发授权、跨主机自动编排、微服务、分布式任务队列。
- 通用资源全字段模型与未来所有系统的数据库实体。

## 9. 设计验证

先以 Sub2API 做一次包含 Git 克隆、配对、授权、受控安装、健康检查、页面关闭与日志恢复、默认保留数据卸载的端到端测试。只有通过该闭环，才扩展第三方 Compose 或官方原生应用适配器。