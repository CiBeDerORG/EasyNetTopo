# 统一命名规范

## 原则与兼容性优先级

本规范约束新增和实际修改的命名。`MUST` 为必须，`SHOULD` 为应遵循且例外需说明，`MAY` 为可选。

1. **MUST** 先保持公开契约兼容，再遵守当前语言习惯；线协议、TOML、JSON、CLI、数据库、ABI、公开 SDK 的已有名字不能当作纯内部符号重命名。
2. **MUST** 在同一模块使用同一领域概念与名称，按含义命名，不能用 `data2`、`tempManager`、`newService` 掩盖职责。
3. **SHOULD** 新代码遵守下表。现有目录的统一风格、框架约定和生成规则优先于新增的文件风格建议；禁止因本规范进行全库批量重命名。
4. **MUST** 内部重命名同步所有调用、测试、文档和导出；公开契约重命名需要显式兼容/迁移方案。

## 文件、模块与包

| 对象 | 新增命名约定 | 示例或例外 |
| --- | --- | --- |
| Rust crate / Cargo package | `kebab-case`，复用项目 `easytier-` 前缀 | `easytier-core`；Rust 导入使用 `easytier_core` |
| Rust 模块、文件、目录 | `snake_case` | `runtime_reconcile.rs`、`peer_manager/`；遵守现有 `mod.rs` 布局 |
| Vue 业务组件 | `PascalCase.vue`，使用领域名+职责 | `NetworkDetail.vue`、`AclRuleDialog.vue` |
| 路由/框架入口 | 服从路由器与构建工具约定 | `pages/index.vue`、`App.vue`、`main.ts`、`index.ts` 不重命名 |
| 新增 TS/JS 非组件文件 | **SHOULD** `camelCase.ts` / `.js` | `networkStatus.ts`；GUI 既有 `mobile_vpn.ts` 等目录局部 `snake_case` 继续保持 |
| 测试文件 | 使用所在包现有 runner 的发现模式 | `*.test.ts`、`*.test.mjs`、Rust `tests.rs`/`*_tests.rs`；不自行统一扩展名 |
| JS 包 | 已有 workspace 命名及 scope | `easytier-frontend-lib`、`@easytier/runtime`；scope 不混作目录名 |
| 设计文档 | `kebab-case.md`，必要时加日期区分历史决策 | `product-design.md`；保留已有 `README.md`/`CONTEXT.md` |
| 数据库迁移 | 沿用已有时间戳+序号+动作模式 | `m20261001_000010_member_ip_allocation.rs`；已应用迁移不因命名修改 |
| 生成文件 | 严格服从生成器 | `src/generated/`、`auto-imports.d.ts`、`typed-router.d.ts`；禁止手改 |

不得创建 `common2`、`utils-new`、`final_v2` 等没有明确领域或版本契约的模块。现有 `utils`/`common` 不要求迁移；新逻辑优先放到有清晰所有权的领域模块。

### 需求、计划与任务命名

- 同一需求使用稳定的英文 `kebab-case` 主题名，例如 `web-user-management`，统一目录为 `process/<feature>-<status>/`，如 `process/web-user-management-done/`。目录内固定使用 `require/`、`plan/`、`task/`，各自的 `README.md` 作为入口；需求目录根 `README.md` 是先于明细编写的需求/计划汇总，保存总体元数据、进度、问题、代码依据与三类文档链接，不另设第二份汇总。
- 目录后缀允许 `draft`、`ready`、`in-progress`、`blocked`、`in-review`、`done`、`cancelled`。它表示该需求的总体状态，不等于某一任务的状态；任务元数据继续按协作规则使用自身 `status`，已有 `in_progress`、`in_review` 不因目录命名被重写。
- 需求状态变化时整体重命名目录，并同步总索引、根元数据和所有受影响链接；稳定 `feature`、需求/验收/计划/任务编号不变。不通过复制一个新目录留下两套活动需求，也不能因文档写完将需求改为 `done`。
- 不同需求独立命名、独立目录、独立计划。用户管理为 `web-user-management`，拓扑集成为 `easytier-ecosystem-integration`，现有 Web 交互优化与美化为 `web-ui-optimization`；跨需求只关联依赖和兼容边界，不混合任务归属。
- 需求和验收编号保持稳定，例如 `UM-01`、`UA-01`；计划索引使用 `PLAN-<领域缩写>`，计划明细使用 `PLAN-<领域缩写>-<三位序号>`，任务使用 `<领域缩写>-<三位序号>`，例如 `PLAN-WUM`、`PLAN-WUM-001`、`WUM-001`。计划与任务编号属于不同命名空间，不默认一一对应，不同主题不复用编号。
- 计划明细文件名为 `plan-<编号>-<名称>.md`，例如 `plan-wui-001-baseline.md`；任务明细为 `task-<任务编号>-<任务名称>.md`，例如 `task-wum-001-contract.md`。编号/名称转为小写 `kebab-case`；文件名不包含负责人或执行状态，以免状态变化破坏链接。既有任务迁移文件名仍保留原 ID 与完成证据。
- 全局索引为 `process/README.md`，单需求汇总为 `process/<feature>-<status>/README.md`；`plan/README.md` 是计划索引，`task/README.md` 是任务看板，具体内容拆成各自目录内的小文件。模板统一位于 `process/templates/`：`summary.md`、`require.md`、`plan-index.md`、`plan.md`、`task-board.md`、`task.md`；字段、状态与链接遵循 [协作规则](../../process/README.md)。

## 代码符号

| 语言/对象 | 约定 | 示例 |
| --- | --- | --- |
| Rust 类型、trait、enum 及 variant | `PascalCase` | `CoreInstance`、`SocketContext`、`ConfigSource::Web` |
| Rust 函数、变量、字段、方法 | `snake_case` | `list_peers`、`instance_id`、`network_secret` |
| Rust 常量与 static | `SCREAMING_SNAKE_CASE` | `MAX_PENDING_OPERATIONS` |
| TS/JS 本地变量、函数、普通方法 | `camelCase` | `selectedNetworkId`、`loadNetworkConfig` |
| TS 类型、interface、class、Vue 组件 | `PascalCase`；不加无意义的 `I` 前缀 | `RemoteClient`、`NetworkConfig` |
| TS 模块级固定配置常量 | **SHOULD** `SCREAMING_SNAKE_CASE` | `DEFAULT_POLL_INTERVAL_MS`；普通 `const` 局部值仍用 `camelCase` |
| Vue 本地 props、computed、handlers | `camelCase`；模板属性/自定义事件优先 `kebab-case` | `networkId` → `:network-id`、`config-saved`；`update:modelValue` 等框架契约保持原样 |
| Composable / 组件回调 | `use`+领域职责 / `handle`+动作 | `useNetworkStatus`、`handleSaveConfig`；已有 API 不强制改名 |
| Go / Kotlin / Swift / C ABI | 遵循语言、所在 SDK 及既有导出/符号前缀 | Go 导出使用 `PascalCase`；既有 C/JNI 导出禁止按 TS 风格重写 |

前端 `Api.RemoteClient` 已有 `run_network` 等 `snake_case` 方法；**MUST** 保留该接口及适配实现的一致性。生成 DTO 的字段名也不是本地变量风格问题，不能强行转换。

### 语义规则

- **SHOULD** 查询命名区分副作用：`get`/`find`/`list` 表达读取，`create`/`update`/`delete`/`start`/`stop` 表达动作；名字与实际行为不一致时优先澄清契约。
- **SHOULD** 布尔命名可读为条件：TS `isConnected`、`hasCredential`、`canEdit`；Rust `is_connected`、`has_credential`。配置中的既有 `enable_*`、`disable_*`、`no_tun` 保持兼容。
- **MUST** 区分“用户意图”和“实际状态”：`desiredRevision` 与 `appliedRevision`、`isSaving` 与 `isRunning` 不得共用一个布尔值。
- **SHOULD** 集合用复数，映射表达键：`peers`、`peerById` / `peer_by_id`；`id` 必须说明所属对象，跨域时使用 `network_id`、`device_id`、`instance_id`。
- **MUST** 非类型化数值标明单位，避免时间与速率混淆：`timeout_ms` / `timeoutMs`、`rx_bytes` / `rxBytes`、`instance_recv_bps_limit`。遵循现有契约定义，不擅自把 bit/s 改成 byte/s；Rust 内部优先 `Duration` 等类型。
- **SHOULD** 状态使用语义明确的 enum/联合类型，如 `connecting`、`connected`、`failed`，避免 `status = 1` 和组合不合法的多个布尔值；序列化取值服从已有契约。
- **MUST** 区分配置默认值、字段缺省、显式 `false`、空列表与清空动作，不使用模糊名字或 `||` 默认处理隐藏差异。

## 跨层契约与代码生成

| 边界 | 规则 |
| --- | --- |
| Protobuf | 新 message/service 采用 `PascalCase`，字段采用 `snake_case`，新增 enum/variant 跟随相邻 schema；保留已有 `PublicServer` 等值。字段编号和 enum 数值是契约，禁止重新分配或复用删除编号 |
| Proto → TS | `frontend-lib/scripts/codegen-proto.mjs` 使用 `use_proto_field_name`，生成 TS 保留 `snake_case`；不得编辑生成文件制造 camelCase 分叉 |
| HTTP JSON / RPC | 已有 EasyTier DTO 普遍保留 `snake_case`。**MUST** 沿用对应接口、schema 和序列化约定，不能全局增加 camelCase 转换器 |
| TOML / 配置键 | 保持 Core 的权威模型与已公布键名；新增字段同步 schema/注解、Core 规范化和相关消费端，不在 UI 自造别名写入持久化配置 |
| CLI / 环境变量 | CLI 新长选项优先 `--kebab-case`，环境变量 `SCREAMING_SNAKE_CASE`；Core 已有 `ET_*` 映射通过现有注解/生成路径扩展，不为所有程序强加同一前缀 |
| REST 路径、DB 列、Tauri command、ABI | 遵循相邻接口的现有风格和版本契约，禁止自动“美化”公共路径、command 名、表/列名或导出符号 |

确需转换时，**MUST** 在已有适配边界集中转换并覆盖缺省、空值、未知字段等语义，明确区分 UI view model 与契约 DTO。不得让每个组件自行猜测字段映射。

## 产品术语

代码使用已有英文领域术语，界面通过现有本地化资源展示中文。下表用于避免混用概念，不要求改写所有历史文案。

| 概念 | 代码/英文术语 | 中文含义与边界 |
| --- | --- | --- |
| 网络 | `network` | 用户配置的逻辑网络；显示名与协议身份名不同 |
| 设备 | `device` | 中央管理登记的设备，可承载多个实例；不等同 peer |
| 实例 | `instance` | 某次配置对应的网络运行实例；不能拿 device ID 替代 instance ID |
| 对等节点 | `peer` | 网络协议参与者；同一 peer 可有多条隧道 |
| 隧道 | `tunnel` | 一条 EasyTier 传输连接；不等同 peer 或整个网络 |
| 凭据 / 授权约束 | `credential` / `grant` | 身份凭据与其权限约束不同；grant 不拥有密钥和身份生命周期 |
| 平台宿主 / 适配器 | `Host` / `Adapter` | Host 拥有平台资源，Adapter 实现能力接口；不要混作远端网络节点 |
| 连接状态 / 运行状态 | `online` / `running` | 可管理性、实例运行和数据面可达性分别表达，不能互相推断 |
| 配置来源 / 编排方式 | `ConfigSource` / `NetworkingMethod` | 来源、网络连接方式和运行状态是不同维度；遵循实际 enum，不把 UI 标签直接当线协议值 |

对使用相应资源的模块，**MUST** 遵循现有 `cn.yaml` / `en.yaml` 和 Rust locale 资源的命名与嵌套风格；不因通用语言标签建议擅改 `cn` 为 `zh-CN`。新 key **SHOULD** 按领域和用途命名，例如 `network.save_failed`，避免 `text1`；沿用相邻资源的大小写与组织方式。独立子项目不因本规范被要求引入共享国际化依赖。

## 命名评审清单

- 名称能否区分配置、运行状态、用户显示名与协议身份？
- 是否遵循所在目录和语言习惯，保留了生成器与框架约定？
- 是否误改了公共字段、枚举值、编号、路径、命令或 ABI？
- 数字的单位、ID 的所属对象、布尔值的语义是否清晰？
- 如有例外或迁移，是否解释范围并验证旧消费者行为？
