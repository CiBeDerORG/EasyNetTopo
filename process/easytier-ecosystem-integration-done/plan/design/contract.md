# ECO-103：页面与按需查询契约 v1

依据：[源码基线](baseline.md)、[实际页面证据](baseline-evidence/README.md)、[任务](../../task/task-eco-103-domain-contract.md)、[公共计划](../plan-eco-002-contract.md)。代码基线 `1e92296b17fc2e85dcd4521b6012232e11abed95`；ECO-101 产物已随 `af28b81e` 提交，ECO-102 已获监工验收。公共类型唯一来源为 `easytier-web/frontend/src/modules/ecosystemTypes.ts`，本文件规定其语义；不新增服务端 schema，不更名现有 snake_case DTO。

## 本期已冻结范围

生产页面复用现有账号与布局，新增网络归属视图、宿主资料、业务资源类型入口、访问能力与导航能力说明。实际读取仅为当前用户的宿主、中央网络摘要及所选网络成员；业务资源、人工资料保存、公网发布、端口映射服务和导航发布 **没有已核实 API，固定为未支持**。该边界来自实施前 ECO-101/102，不是联调失败后的临时降级。

不新增注册业务资源、配对手机、执行家居动作、保存人工地点型号、发布/撤销公网、保存导航或跨系统审计方法；没有无效表单、模拟保存成功或 localStorage 假持久化。已存在别名修改继续使用原设备页，本期生态页只读，不重复引入编辑工作流。ECO-09 等缺少后端的验收项保留未实现说明；完成页面任务不等于这些业务能力已交付。

ECO-102 已取得真实登录、设备空态、一个无成员网络、404及360/1440视口证据；没有连接宿主、实际 peer 链路或业务资源。不得将样例/mock成员视为真实拓扑验收。现有设备页约1秒、MainPage约10秒的周期已取证；新页面不复用包含此副作用的 DeviceList/NetworkDetail 或 Utils.PeriodicTask。

## 最小可执行接口与模式

ECO-111 实现 `EcosystemReader`；ECO-112 消费，不从展示组件发 HTTP。工厂建议命名 `createEcosystemReader(api, userId)`，userId 来自宿主已验证的 CurrentUser.id（ECO-113 交给 EcosystemPage 的 `currentUserId` 属性），不新增身份存储、不重复轮询 auth/me，也不把 userId 发作租户覆盖参数。

| 方法 | 唯一调用清单与返回 | 结束/失败语义 |
| --- | --- | --- |
| loadOverview | 先 `get_console_info()`；随后 `list_machines()`；仅 webhook_auth=false 时 `list_networks()`。返回 EcosystemOverview | 单轮请求全部成功才形成新快照；任一失败保留该对象上次成功快照并显式报错，不将失败转为空列表。不得请求网络详情或凭据秘密 |
| loadMembers(networkId) | 当前中央模式、已列出的所选网络调用 `list_network_members(networkId)`；返回 EcosystemMembers，包含同一networkId | 切换后旧响应丢弃；404标对象不存在，可手动回列表，不推断节点离线。webhook模式不调用此方法 |

模式为 `central` 或 `webhook`，来自 console-info.webhook_auth，不能只根据404猜测。在 webhook 模式 networks capability=`unsupported/external_console`，networks=[]只表示不读取，不展示“0个中央网络”的成功空态；仍可显示当前会话的宿主快照。未知/未完成模式查询时只展示加载或错误，不能默认启用中央 API。

能力矩阵：networks仅中央模式supported；nodeMetadata=`node_metadata_api_missing`、resources=`resource_api_missing`、publicAccess=`public_access_api_missing`、portMappings=`port_mapping_api_missing`、navigation=`navigation_api_missing`，均unsupported。nodeMetadata专指人工字段保存，已返回OS/别名/GeoIP仍正常展示。三种访问能力独立字段，不因其中一项存在而推断另两项许可。`supported`仅为接口能力，不代替服务端授权或实际运行验证。

ECO-111 不用新增 /resources、/navigation、/navloom 路由；也不从网络配置、port forward RPC 或 gateway-info 推导资源公网入口。此版本无写入 API，因此不需要虚构 accepted/applied 操作状态或通用任务框架；后续真实能力增加必须回到103修订并重新评审。

## 身份、关系与数据收窄

- `EcosystemNodeRef {userId,machineId}` 是当前会话内宿主身份；比较两项均相等。接口的 machine UUID 用既有 Utils UUID 转换，但先验证形状，缺失/无效ID不能成为空UUID节点。禁止按hostname、IP、alias、peerId合并；会话切换清空全部选择与数据。
- 节点唯一记录位于 overview.nodes。`EcosystemNetworkRef` 与节点的 networks 只表达中央网络归属；`EcosystemMembership` 带 networkId/memberId/node，不能复制为第二份节点资料。成员主机不在此次机器列表时保留成员行与其身份，显示宿主资料未知，不能制造系统信息。
- `EcosystemTemporaryPeer` 仅networkId/peerId/hostname/ipv4，不捏造machineId或业务设备ID。不得把peerId作为跨网络唯一键。
- runningInstanceIds仅为源接口列出的运行实例，不把 networkId 当 instanceId。未读取routes/peers，界面必须称“网络成员/归属”，不得画成P2P直连、中继、延迟或可访问边。
- `EcosystemResourceKind` 区分service/device_group/device。v1没有已核实业务资源记录，所以不定义假resourceId、source_instance_id、external_id、endpoint或默认IP。无资源数据来源时展示未支持原因，而非成功空列表。未来来源身份须另行契约评审。
- wire DTO→view model仅在ECO-111转换。未知字符串为null，明确在线/离线才转换为online/offline，缺失布尔为unknown；人数缺失为null，不填0。OS来自info.device_os的os_type/version/distribution，不从hostname或字符串猜架构/厂商。
- geoLocation来自已有Location，明确标“连接IP估计位置”；与人工地点分开，人工字段显示未支持。lastSeenAt来自last_seen，version来自easytier_version，缺失均null。适配器不保留client_url、user_token、network_secret或credential_secret到视图。
- 必须验证顶层数组、关键ID与布尔/计数等字段；非法结构报invalid_response，不悄悄过滤为“全部正常”。适配只做必要结构收窄，不另造通用schema框架。

## 查询、错误与生命周期

`EcosystemQueryState<T>` 使用idle/loading/ready/error四态；loading/error可保存同一对象的previous快照。快照queriedAt为成功读取时ISO时间，不是设备上报时间或持续健康保证。显示固定时间，不新增每秒更新文案的定时器。能力unsupported和请求forbidden/not_found分离。

| 情况 | EcosystemQueryError.kind | 行为 |
| --- | --- | --- |
| 401 | unauthenticated | 保留现有ApiClient身份失效回调；宿主处理登出，不在子页创建另一登录流程 |
| 403 password_change_required | password_change_required | 交宿主身份/强制改密处理，不自动重试 |
| 其他403（含CSRF拒绝） | forbidden | 显示访问被拒绝，不假定资源离线或自动提升权限 |
| 404 | not_found | 所选对象未找到，不清空其他对象并称成功 |
| 明确超时（HTTP408/504或客户端超时码） | timeout | 停止本轮，支持用户手动重查 |
| 无HTTP响应的网络失败 | network | 未知结果，保留可识别的旧快照 |
| DTO结构不符合约定 | invalid_response | 显示查询失败，不填空/零 |
| 其他HTTP/未知失败 | request_failed | 安全通用文案，不展示原始Axios对象或服务端body |

错误只保留kind和status；原始响应可能含敏感配置，不放日志、页面、截图或query参数。ECO-104使用i18n键映射安全文案。QueryState的error不包含任意message。

ECO-112独占overview/members查询状态、当前选择和请求代次。进入页执行一次overview；用户选择网络执行一次members；选择已有节点仅展示已加载数据。手动刷新overview，完成后最多读取一次仍存在的当前网络成员。相同查询在途时合并/禁用重复刷新；切换网络时旧代次失效（不能因为另一对象在途就丢弃新对象请求）。清空选择、卸载、身份变更时失效所有旧请求。不同网络不能复用previous成员快照。

不由watchEffect叠加onMounted产生重复请求；不得在错误catch里重试。页面空闲、回到标签、重连不触发请求。现有ApiClient可增加可选取消/timeout参数但须保留旧消费者；若使用请求代次忽略迟到响应，不声称底层请求已取消。普通请求超时不等于轮询；ECO-111应保证实际请求有有限超时，失败后无自动后续调用。

## 公共组件输入输出与唯一所有者

以下名称/路径为本轮拟新增组件，公共props/events类型已在ecosystemTypes.ts声明。子页只读props、发明确事件，不导入ApiClient、其他并行页或尚未交付实现。类型内事件名沿用camelCase，Vue模板监听使用kebab-case（如`@select-network`、`@back-to-node`）。

| 任务/组件 | props类型 | events与职责 |
| --- | --- | --- |
| ECO-106 `ecosystem/topology/EcosystemTopology.vue` | EcosystemTopologyProps | refresh、selectNetwork(networkId或null)、selectNode(nodeRef)。网络归属列表/关系视图，区分空态、未知、临时peer；无伪造连线 |
| ECO-107 `ecosystem/nodes/EcosystemNodeDetails.vue` | EcosystemNodeDetailsProps | selectResourceKind(kind)。展示唯一宿主已知资料及人工资料缺口，服务/设备组/设备入口不伪造数量 |
| ECO-108 `ecosystem/resources/EcosystemResourceDetails.vue` | EcosystemResourceDetailsProps | 无写入事件；展示当前节点和选中类型的未支持原因，不创造资源记录或动作 |
| ECO-109 `ecosystem/access/EcosystemAccessSettings.vue` | EcosystemAccessSettingsProps | 无写入事件；分开呈现公网、映射、导航能力缺口，没有假的提交开关 |
| ECO-110 `ecosystem/navigation/EcosystemNavigation.vue` | EcosystemNavigationProps | backToNode(nodeRef)，只回当前宿主上下文；无第三方URL、无发布操作，不把内部回链冒充资源导航集成 |
| ECO-112 `EcosystemPage.vue` | api: ApiClient、currentUserId: number | 组合上述页，调用reader，维护选择及查询代次；不修改其他子组件 |
| ECO-113 宿主路由 | 现有会话的currentUser.id作为currentUserId | 新路由建议`/h/ecosystem`，name=ecosystem；只在已加载身份且非强制改密时渲染，身份变更重置页；保留原有路由、命令、OIDC及菜单权限 |

ECO-104提供`ecosystem/shared/EcosystemQueryNotice.vue`（props: status为QueryState的status、error可空、queriedAt可空；emit refresh）和`EcosystemCapabilityNotice.vue`（props: capability），可选纯状态标签但不得引入请求。QueryNotice在loading时禁用刷新，父层仍负责在途去重；capability notice的unsupported必须展示原因，supported可不渲染。共享文案固定在`web.ecosystem`前缀，至少包含mode、query/error种类、unsupported原因、connection状态、network membership语义、各页标题与unknown/last queried文本；cn/en成对由104唯一写入。组件不能自带定时器或根据渲染触发请求。

ECO-105预览使用明确的开发fixture而非生产默认数据，按上述类型覆盖central/webhook、空态、带宿主的样例成员、unknown、loading、error、unsupported。各页测试必须能证明选择事件与无隐式请求；sample network不得当作真实peer链路。

## 下游验证与本任务证据

ECO-111验证真实路径调用清单、webhook跳过central、DTO收窄、敏感字段不进入视图、错误与超时；ECO-112验证快速切换/迟到响应/重复刷新/卸载/身份变化。ECO-114才做真实API联调与空闲观察：记录壳层旧周期和生态自身请求来源，不能把旧周期掩盖成新功能无请求，也不能把mock当真实节点。ECO-115逐项保留上述业务缺口和真实VPN/公网未验证范围。

本任务仅新增静态公共类型与契约，无运行实现。实际类型检查与监工结论在交接时记录；不把本轮类型检查表述为页面或端到端通过。

### ECO-103 实际检查记录（2026-10-21）

- 在 `easytier-web/frontend` 执行 `node node_modules/typescript/bin/tsc --noEmit --strict --skipLibCheck --target ES2020 --module ESNext --moduleResolution Bundler src/modules/ecosystemTypes.ts`：退出0，公共类型通过严格检查，无输出产物。
- 使用现有 `vue/compiler-sfc` 在内存解析/compileScript 七组消费者声明（Topology、NodeDetails、ResourceDetails、AccessSettings、Navigation、QueryNotice、CapabilityNotice）：七组均通过，包含导入props接口和tuple式defineEmits接口。没有创建测试组件文件或改构建入口。初次从未直接暴露的 `@vue/compiler-sfc` 导入失败，改用已安装Vue公开入口后完成；未安装依赖。
- Markdown相对链接、公共导出与契约对应关系检查；未运行完整Web构建、浏览器或后端测试。消费者行为与真实API仍由后续任务验证。
