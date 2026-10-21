# 后端架构与工程规范

适用范围：Rust workspace、原生网络运行时、Web 服务、Tauri Rust 后端、FFI/WASI 宿主，以及 Go/JavaScript SDK 与平台适配。
命名统一遵守 [命名规范](naming.md)。本文中的 MUST 为必须、SHOULD 为通常应当、MAY 为可选。

## 1. 当前架构与事实来源

以下结构已存在，新增规范用于维护这些边界，并不要求重构所有存量代码：

| 位置 | 已有职责 | 不应承担的职责 |
| --- | --- | --- |
| `easytier-proto` | protobuf/RPC 类型、描述符、代码生成与 feature 切片 | 网络策略、数据库、OS 资源管理 |
| `easytier-core` | 可移植配置、协议状态、路由、连接策略、包处理及实例生命周期 | 原生 socket/DNS/TUN/路由/服务管理系统调用 |
| `easytier` | 原生组合根、OS 资源及 Host Adapter、CLI、具体协议引擎 | 另一套 peer 图、路由决策或实例状态机 |
| `easytier-web/src/restful` | HTTP 解析、认证接入、响应与错误映射 | 重复实现核心网络策略 |
| `easytier-web/src/central_network` | 网络意图、设备注册/封禁、配置编译及 Gateway 业务 | 替代 `ClientManager` 的持久化/离线重放机制 |
| `easytier-web/src/client_manager` | 设备配置持久化、离线重放及运行时 reconcile | 依赖中央网络业务状态 |
| `easytier-web/src/db`、`migrator` | SeaORM/SQLx 数据访问与版本迁移 | HTTP 表示层或底层包转发 |
| `easytier-gui/src-tauri`、contrib/FFI | 平台组合、桥接、资源/句柄与返回值转换 | 复制核心授权或网络策略 |
| `easytier-core/src/wasi`、`easytier-js` | guest ABI 与 JavaScript Host/Adapter | 把 Host 平台差异塞入通用网络域 |

MUST 先读取 [现有核心架构](../core-architecture.md) 和 [领域上下文](../../CONTEXT.md)。
涉及凭据、socket 保护、Web 配置同步时，还需读取相应 `docs/` 专题。
[数据面计划](../data-plane-runtime-plan.md) 自述为目标架构；MUST 对照当前实现确认进度，不把计划中的接口当成已存在 API。

## 2. 依赖方向与模块边界

**BE-01 MUST：** 主依赖方向保持 `easytier → easytier-core → easytier-proto`。
展示和平台集成可消费这些 crate；core/proto 不得反向引用 Web、GUI 或原生组合根。

core 内部按现有层次组织：

```text
foundation
  ← config / packet ← socket ← host ← tunnel
  ← listener / connectivity ← peers / rpc ← gateway ← instance ← management
```

这是层次指导，非逐文件禁止表；既有局部集成边以 `core-architecture.md` 为准。

**BE-02 MUST：** 可移植策略只有一个权威 owner。原生/HTTP/FFI 层转换输入并组合能力，不缓存第二份可变 peer、route 或运行时配置真相。

**BE-03 MUST：** 新增 OS 操作通过 `host/`、`socket/` 等既有 capability seam 注入。
core 可以调度 I/O、处理超时和协议状态，但不能直接创建真实原生 socket、读取配置文件或修改系统路由。
Host 操作系统信息必须来自运行时 Host 配置；`target_os = "wasi"` 不代表实际 Host 是 WASI 操作系统。

**BE-04 SHOULD：** 新模块按业务不变量划分；`mod.rs` 声明向外暴露的接口，默认最小可见性。
新抽象应能回答“谁拥有状态、谁负责释放、删除它会重复什么策略”。不得只为套层创建无状态转发包装。

**BE-05 MUST：** 跨层新增反向依赖前，先定义较低层 capability 或更新架构决策及影响说明。
MAY 在现有拥有者内补充小型私有 helper；不因一个需求引入通用服务框架或拆分微服务。

## 3. 实例、资源与异步任务

**BE-06 MUST：** 实例经 `CoreInstance::new` / `from_toml` 的规范化路径构建。
`InstanceManager` 管集合成员；`CoreInstance` 管启动、停止及恢复。不得绕过它们增加平行生命周期。
已停止实例是 one-shot；重启需求应构建新实例。

**BE-07 MUST：** 每项长期任务说明 owner、启动点、取消信号、终止条件及回收点。
复用现有 `foundation/task.rs`、任务句柄、取消与 shutdown 机制；不得无 owner 地丢弃 `tokio::spawn` 句柄。
停止和部分启动失败必须释放 listener、socket、租约、句柄及队列中的资源。

**BE-08 MUST：** 明确背压和资源上限。新增队列、缓冲、会话表、重试循环必须说明容量、过载行为与清理规则。
SHOULD 优先有界队列；使用无界队列时需给出外部上界或消费保证。
超时、取消、断连、EOF 与操作失败是不同结果，不得以空数据或成功统一掩盖。

**BE-09 MUST：** 不持有 `std::sync::Mutex` / `parking_lot` 锁执行 `.await`。
异步锁跨 `.await` 时必须说明原子性需求、锁顺序及等待边界；避免持有集合 entry/guard 进入可重入调用。
网络/磁盘阻塞调用不得直接占用异步 worker；适用时使用受控 `spawn_blocking` 或 Host 执行上下文。

**BE-10 MUST：** `setns` 等线程局部 OS 状态不能依赖可能迁移的 async task 保持有效。
修改相关路径时需验证完整调用链，采用受控固定线程或显式上下文隔离，不扩大现有技术债。

**BE-11 SHOULD：** core 时间逻辑复用 `foundation::time`，重试设总期限/退避，避免固定短间隔忙轮询。
实例状态属于实例；`CoreProcessRuntime` 只持有允许跨实例共享的资源，不能捕获实例配置或 peer 状态。

## 4. HTTP、业务与数据持久化

**BE-12 MUST：** Web handler 按现有组织完成解析、认证上下文取得、调用业务 owner 和响应转换。
中央网络业务复用 `central_network::service`、compiler 和持久化路径；不要在每个路由复制所有权检查。
其他已有路由按相邻结构扩展，不要求一次性迁移成统一 service/repository 模板。

**BE-13 MUST：** 从登录态取得用户身份；客户端提交的 user/device/network ID 不是授权依据。
对象读取和修改均检查所有权/权限。批量操作逐项定义授权和部分失败语义。
中央网络变更沿用共享 mutation lock，避免所有权检查与 enrollment/配置写入产生竞态。

**BE-14 MUST：** 配置期望值、已持久化版本、已发布版本和设备实际运行状态必须分开处理。
HTTP 成功不等于设备已应用；离线、重放、冲突和修订版本沿用现有 reconcile 协议。
Console 模式和中央网络模式的边界按 `CONTEXT.md` 保持，不并行启用本应互斥的发布路径。

**BE-15 MUST：** 数据库结构变更新增 `migrator/` 迁移并注册；不得修改已发布迁移来假定旧库同步变化。
跨多行不变量用事务/现有串行化机制保护。定义重复请求、唯一冲突、失败回滚和并发更新结果。
SHOULD 为危险删除区分断连、删除和封禁；不能把当前删除语义误报为永久拒绝重新注册。

## 5. 协议、配置与生成代码

**BE-16 MUST：** protobuf 定义在 `easytier-proto/proto/` 修改，生成器在 `build/` 修改。
`OUT_DIR` 中生成 Rust、生成 TS 和描述符不是手改入口；修改源 schema 后执行项目生成/构建流程。
手写的 `easytier-proto/src/*.rs` 包装和 RPC runtime 不应误判为可覆盖的生成产物。

**BE-17 MUST：** 不复用 protobuf field tag；移除字段时保留其编号/名称。
新增字段采用兼容默认/可选语义，覆盖新旧节点交互；协议、配置、公开 CLI、HTTP/RPC 与 ABI 的外部契约均需兼容性说明。
路由转发/凭据过滤不得通过有损反序列化再序列化丢弃未知 protobuf 字段；复用现有 focused wire editor。

**BE-18 MUST：** TOML、API DTO、规范化 runtime snapshot 由既有转换路径衔接。
新增配置同时明确默认值、校验、Host 能力依赖、读回语义、持久化和运行时更新行为。
命名和序列化字段变化遵守 [命名规范](naming.md)，不得靠统一大小写规则破坏公开字段。

**BE-19 MUST：** 标准 runtime 对不可用能力明确报错，不静默切换协议或降低安全策略。
Compact compatibility Host 的已记录例外可保留期望值并规范化为 no-op，但不得宣称该能力已启用。

## 6. 错误、日志、安全与跨语言边界

**BE-20 SHOULD：** 领域层用可判断的类型化错误，组合边界补充上下文；沿用已有 `thiserror` / `anyhow` 用法。
HTTP/RPC 层集中映射错误与状态码，保留现有响应包络，避免前端解析错误字符串执行分支。
新增不可信输入路径 MUST 返回错误，不能依靠 `unwrap` / `expect` 导致进程退出；不变量断言需说明依据。

**BE-21 MUST：** 新日志采用结构化字段，包含必要的实例/操作/peer 上下文；避免高频逐包正常日志。
不得记录网络 secret、私钥、密码、完整 token、Cookie 或含凭据的 URL，也不得把这些内容放入错误响应。
给敏感结构派生 `Debug` 前检查字段；日志脱敏不等于取消必要的权限控制。
MUST 复用现有 process/module 级 logger 控制，不在每个实例重复初始化全局 subscriber。

**BE-22 MUST：** 授权来自认证后的身份及 credential grant，不能信任自报路由元数据。
网络 secret、ACL grant、凭据身份与生存期职责分离，按现有 credential intake 路径规范化授权。
错误、超时和不支持能力不得触发未授权 direct fallback、加密降级或更宽 ACL。

**BE-23 MUST：** FFI/ABI 新接口明确指针/长度合法性、借用或转移所有权、线程约束、取消和释放顺序。
调用者必须能释放库分配的字符串/资源；不能把 Rust 引用、布局未固定的类型或 panic 传播到外部 ABI。
`unsafe` 块最小化并写明安全前提；双重释放、过期句柄和取消后 completion 都需要定义结果。

**BE-24 MUST：** WASI/JS Adapter 遵守 guest 的驱动、唤醒及 Host operation 协议。
Browser/Cloudflare Adapter 只实现各自平台接入，不在 JS/Go 重写路由、smoltcp 或授权策略。
Go Host 拥有 wazero、WASI/Host ABI 接入、guest 驱动、completion 通知和资源关闭；JavaScript 共享 runtime 拥有 guest 生命周期与 Host 资源，Browser/Cloudflare Adapter 保留各自接入能力。
修改前 MUST 阅读 [Go SDK](../../easytier-go/README.md) 或 [JavaScript Hosts](../../easytier-js/README.md)，保持其公开 API 与内部 ABI 的边界。

## 7. Cargo feature 与平台能力

**BE-25 MUST：** feature 表达完整能力，依赖于真正使用的 core/proto 切片，不启用无关网关或管理功能。
标准/完整/禁用默认/独立 feature 的语义需清楚；只通过默认 feature 编译不能证明 feature 边界正确。

**BE-26 SHOULD：** `cfg` 放在完整 Module/Adapter 边界，共享状态机使用稳定接口。
已有细粒度 `cfg` 在修改其 owner 时渐进收敛，不为遵守文档而批量改动无关代码。
跨平台源码变更 MUST 对目标系统评估编译和能力差异；不能通过 Linux 成功宣称移动端/WASI 成功。

## 8. 验证与交付

以下 Rust 命令从仓库根目录执行，使用 `rust-toolchain.toml` 的 Rust 1.95 和 CI 固定的 protoc 35.1。
原生 Linux 使用 mold；Clang/bindgen、C 编译链按涉及的 feature 准备。Rustfmt/Clippy 需安装对应组件。
下面是按改动选择的命令，不是“已运行”声明，也不要求纯文档修改触发全量构建。

```bash
# 边界与核心逻辑变更：按影响范围选择
cargo fmt --all -- --check
cargo check --locked -p easytier-core -p easytier-proto -p easytier --features full
cargo test --locked -p easytier-core --lib

# 轻量配置与内存隧道回归，可在无 TUN 环境运行
cargo test --locked -p easytier-core --lib config::
cargo test --locked -p easytier-core --lib tunnel::ring::

# feature/目标发生变化时补充对应检查
cargo check --locked -p easytier-core --lib --no-default-features
cargo check --locked -p easytier-ffi
cargo check --locked -p easytier-core --lib --target wasm32-wasip1 \
  --features management-rpc,proxy-smoltcp-stack,ring-crypto,wasi-crypto-offload
```

完整 CI 命令与 runner 前提见 [Rust CI](../../.github/workflows/test.yml) 和 [构建准备](../../.github/actions/prepare-build/action.yml)。
`cargo clippy --all-targets --features full --all -- -D warnings`、feature matrix、nextest 全套涉及更广 workspace 和平台依赖。
TUN、namespace、BPF、UPnP、IPv6 及多节点集成需适合的 runner；MUST 如实记录未运行项，不以跳过断言或关闭检查获得“通过”。

Go/JavaScript 变更必须验证对应运行时，不能用 Cargo 检查替代。从仓库根目录按改动范围选择：

```bash
# JavaScript 独立工作区；Node 22、pnpm 10+、Rust wasm32-wasip1、protoc 35.1
pnpm --dir easytier-js install --frozen-lockfile
pnpm --dir easytier-js check

# Go CI 使用 1.25.x；TUN 示例是独立 module，需单独运行
(cd easytier-go && go test ./...)
(cd easytier-go/examples/tun && go test ./...)
```

前置条件、嵌入 WASM 的更新流程与测试范围分别以各自 README、[JS CI](../../.github/workflows/js.yml)和 [Go CI](../../.github/workflows/go.yml)为准。
需要 Web E2E 配置或真实网络能力而跳过的测试必须单独报告，不能宣称完整运行时/E2E 已验证。

提交前 MUST 回答：策略 owner 是否唯一、取消后资源是否回收、旧节点/旧配置是否兼容、是否越权/泄密、相关 feature 和错误路径是否验证。
SHOULD 为实际变化的不变量加入聚焦测试，优先模拟 Host、受控时钟及内存传输；不新增仅复述实现的测试。
涉及新能力或兼容性取舍时，MUST 更新对应架构文档和调用方说明，并报告实际执行的命令与结果。
