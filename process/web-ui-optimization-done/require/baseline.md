# WUI-101：源码、路由与请求基线

交接版本：`WUI-BASELINE-1`，核查日期：2026-10-21。关联：[需求](README.md)、[任务](../task/task-wui-101-source-baseline.md)、[下一步页面取证](../task/task-wui-102-page-evidence.md)。

## 采用基线与证据性质

- 实现基线为 `feat/web-user-management@1e92296b17fc2e85dcd4521b6012232e11abed95`；文档来源为 `docs/architecture-and-user-management-plan@670eb3b42800d42333356da0cb15f41d76c92f1e`。
- 本轮工作区 `/workspace/EasyTier-iteration`，分支 `feat/ecosystem-web-ui`。核查时 HEAD 为上述实现提交，文档整合与目录迁移由总控进行；`git diff --name-only 1e92296b -- easytier-web/frontend/src easytier-web/frontend-lib/src easytier-web/frontend/package.json` 输出为空。因此以下源码事实采用该固定提交，不把尚未提交的文档当作新代码版本。
- 以下文件路径均相对仓库根目录。`F` 表示 `easytier-web/frontend`，`L` 表示 `easytier-web/frontend-lib`。核查是源码与脚本阅读，没有启动应用、安装依赖、构建、测试或 CI；不证明实际页面外观、接口可达或运行成功。

## 路由、页面及输入输出

路由权威入口为 `F/src/main.ts` 的 `routes` 与 `createWebHashHistory`；下表路径均为 hash 内路径。`/auth/login` 是登录别名，`/h/dashboard` 是工作台别名，未匹配路径重定向 `dashboard`。

| 入口 | 组件、输入与输出 | 请求与边界依据 |
| --- | --- | --- |
| `/auth`、`/auth/login`、`/auth/register` | `Login.vue` 接收 `isRegistering`，保留用户名、密码、验证码草稿；提交后导航 | `loadPolicy/onSubmit/onRegister/onSsoLogin`：读取注册/OIDC 策略；登录后读取身份，强制改密账户直接去 `changePassword`；注册受策略限制；SSO 跳转既有入口 |
| `/h`、`/h/dashboard` | `MainPage.vue` 宿主；`Dashboard.vue` 接收 `api/centralEnabled` | 宿主拥有身份、权限、导航、临时凭据和用户管理请求；`Dashboard.loadSummary` 读取 summary、设备、条件性网络列表 |
| `/h/users` | `users/UserManagement.vue` 接收 current-user/users/events/selected-user/loading/busy/stale/error/credential；发出 query/select/create/set-access/set-role/reset-password/revoke-sessions/query-events/dismiss-credential/retry | 组件不自行发 HTTP；`MainPage.loadUsers/selectUser/loadEvents/mutate` 经 `UserManagementClient` 发请求。宿主负责错误后身份刷新、列表/详情序号和敏感凭据清除 |
| `/h/password` | `ChangePassword.vue` 接收 api/required/localPasswordEnabled/source | `canChangePassword` 只接受明确 `true`，不从 source 推断本地密码；`changePassword` 成功清除密码并回登录。OIDC-only、未知能力、服务端 local_password_unavailable 均保持解释性限制 |
| `/h/deviceList` | `DeviceList.vue` 接收 api/centralEnabled；表格/卡片/筛选和设备管理抽屉；`DeviceDetails.vue` 为 device/compact/containerClass 的纯展示 | `loadDevices`、`saveAlias`、`confirmDeleteDevice`；详情无独立请求；删除可携带 block 参数，不能美化成仅隐藏条目 |
| `/h/deviceList/device/:deviceId/:instanceId?` | `DeviceManagement.vue` 从路由取得对象，接收 api/deviceList，给共享 `RemoteManagement` 注入 remoteClient、instance-id、new-config-generator | `get_remote_client` 返回 Web HTTP 适配器；实例列表/配置/启停等语义仍由共享组件与 `RemoteClient` 契约拥有。更改所选实例更新原路由 |
| `/h/networks` | `NetworkList.vue` 接收 api；创建草稿包含网络名、CIDR、安全模式及高级节点 | `loadNetworks/openCreate/createNetwork`、网关读取；创建成功再跳转返回 network_id 的详情。保留 Gateway/Manual/Standalone 等原有语义和规范化 |
| `/h/networks/:networkId` | `NetworkDetail.vue` 以 route.params.networkId 驱动网络、成员、临时节点、配置、凭据、节点抽屉和 ACL | `loadAll`、成员增删改、设置保存、成员配置读取/保存/重置、凭据生成/撤销、节点路由/peer/ACL统计、日志级别与配置导出、删除网络；ACL 子组件接收 api/networkId/members |

`MainPage` 在 `must_change_password` 时限制工作区，`users.manage` 能力决定管理入口；`centralEnabled` 由 console-info 的 `webhook_auth === false` 决定，外部认证控制台访问中央网络路由会返回工作台。其 RouterView 为 networkDetail 使用 networkId 作为 key，切换网络会重建详情组件，不能仅因详情没有 watch(networkId) 就断言路由切换失效。页面隐藏不能替代后端授权。

## 请求触发、周期与清理

| 所有者与关键符号 | 初始/手动/操作完成触发 | 既有周期与清理 |
| --- | --- | --- |
| `MainPage.loadNavigation` | mounted；身份错误重试；身份成功且不需强改后读取 console-info（未缓存时）、网络和阻止设备；解除阻止后重新加载阻止设备 | `setInterval` 10000ms；unmounted 清 interval 并 `disposeRequests`，递增用户管理相关序号及清除 credential |
| `MainPage.loadUsers/selectUser/loadEvents/mutate` | route.name/canManage watch、查询/详情/审计事件、显式写入后刷新 | 本组没有新增独立 timer；序号守卫抑制部分迟到结果，离开用户页/权限变化清临时凭据；完整竞态仍由取证/回归确认 |
| `Login.loadPolicy` | mounted、策略失败手动重试、注册业务失败后重读策略；登录/注册/SSO均用户触发 | 未发现该组件周期任务；captcha 是图片 URL，浏览器图片加载独立于 Axios |
| `ChangePassword.changePassword` | 用户提交，校验本地能力、长度和 busy | 未发现该组件周期任务；成功/本地能力拒绝清密码 |
| `Dashboard.loadSummary` | mounted 的 PeriodicTask；centralEnabled watch；手动刷新 | `PeriodicTask(2000)`；unmounted stop；requestInFlight 防同函数重入，三个数据源用 allSettled 处理部分失败 |
| `DeviceList.loadDevices` | mounted 的 PeriodicTask；刷新/重试；别名/删除成功；子路由 update；mounted 另读一次 console-info | `PeriodicTask(1000)`；unmounted stop；requestInFlight 防重入 |
| `NetworkList.loadNetworks` | mounted 的 PeriodicTask；刷新/重试；创建后；mounted 另读网关 | `PeriodicTask(2000)`；unmounted stop；refreshing 防重入 |
| `NetworkDetail.loadAll` | mounted 与网关并行加载；错误重试；成员/设置/配置操作后 | `PeriodicTask(2000)`，顺序读取 network/members，安全网络且凭据已加载时再触发 credentials；unmounted stop 并递增 configRequest |
| `NetworkDetail.openNodeDetail/loadNodeDetail` | 打开节点抽屉立即读取 routes/peers/acl_stats，并另读日志级别；日志修改、导出配置由显式动作触发 | 抽屉 `setInterval(5000)`；打开前清旧 interval，关闭/unmounted 清理 |
| `NetworkDetail` 配置/凭据/设置 | 编辑成员读取接口IP与成员配置；首次相应页签读取凭据/设置；显式生成/撤销后刷新；网关有已加载检查 | `loadMemberConfigForm` 的 configRequest 防迟到覆盖，关闭编辑框/unmounted 递增；不意味着所有请求均有同样防护 |
| `AclPolicyTab.loadPolicy/save` | `watch(networkId, immediate: true)` 读取；用户 save 更新 policy | 该组件无独立周期任务；读取/保存无请求序号，实际切换行为需取证 |
| `L/src/components/RemoteManagement.vue` | 实例选择 watch、配置/启停/删除/导入导出；元数据缓存补读 | 共享 `PeriodicTask(1000)` 调用 loadNetworkInstanceIds/loadCurrentNetworkInfo，pauseAutoRefresh 时跳过；unmounted stop 并移除 resize 监听。属于共享库只读边界 |

`L/src/modules/utils.ts:PeriodicTask` 使用递归 `setTimeout`，start 首次延迟 0，任务 await 完成后再等待配置间隔；因此 1000/2000ms 是任务结束后的间隔，不应误称固定每秒请求数。stop 清 timeout 并移除 task 引用，不取消已经发出的 HTTP 请求。MainPage 与节点抽屉使用的是原生 interval，机制不同。

已在 `F/src` 搜索 `setInterval/setTimeout/PeriodicTask/WebSocket/EventSource`，命中周期入口为上表 Web 页面；该搜索不证明整个仓库不存在其他机制。主题 `F/src/modules/theme.ts:initThemeMode` 的 matchMedia change listener 为应用级主题资源，不是网络订阅。

## API、账号与接入命令兼容点

`F/src/modules/api.ts:ApiClient` 统一创建带 `/api/v1` 的 Axios、withCredentials、响应 data 解包与 401 回调；`F/src/modules/userManagement.ts:UserManagementClient` 使用同一 transport、固定 DTO、写入意图头及安全错误映射。保留已有 snake_case API、MD5 输入兼容、未知身份字段与 401/403/冲突/网络错误差异。此轮 UI 不能新增并行 HTTP 客户端或自动重试写入。

接入命令在两个已存在的位置生成：

```text
easytier-core --config-server ${proto}://${host}:${port}/${username}
```

- `MainPage.enrollCommand`：host 来自 `new URL(getApiBase(), window.location.href).hostname`。
- `DeviceList.enrollCommand`：host 来自 `window.location.hostname`。
- 两者 proto/port/username 均来自 console-info，复制函数直接复制 computed 字符串。跨域部署下 host 可能不同是现有源码事实，本任务不改写或判定为运行缺陷。后续排版保持各自原生成与复制内容，若要统一需单独核实兼容影响。
- `F/src/modules/api-host.ts:getApiBase` 从部署注入的 window.apiMeta.api_host 读取并去掉末尾斜杠，不由普通用户输入覆盖。
- 不修改 config-server 连接、设备认证、配置下发或后端网络行为；保存 API 成功不能转换成已验证 VPN/公网连通的提示。

## 现有验证入口及证据边界

| 文件/脚本 | 实际定义的覆盖与运行前提 | 本次状态 |
| --- | --- | --- |
| `F/package.json:build` | 先构建 frontend-lib，再 vue-tsc、Vite；产物为共享库与宿主 dist，需串行或隔离 | 只读，未运行 |
| `test:browser` → `tests/dashboard.test.mjs` | pnpm build 后 Playwright + Vite preview，默认5198端口；拦截 API，覆盖工作台、设备、网络、ACL、布局等，是 mock 浏览器回归 | 只读，未运行；旧记录不证明当前可用 |
| `tests/user-management-navigation.test.mjs` | 默认 built preview，可 WEB_TEST_DEV=1；动态端口，真实组件/客户端、mock HTTP，覆盖宿主权限/导航/强改/OIDC本地能力等 | 只读，未运行 |
| `tests/user-management-components.test.mjs` | Vite fixture + Playwright，组件 props/events 模拟，覆盖列表/分页/确认/凭据/键盘 | 只读，未运行；不验证后端权限 |
| `tests/user-management-client.test.mjs` | 客户端 DTO、写入头、分页/错误/敏感响应，隔离 mock transport/server | 只读，未运行 |
| `test:user-management` | build、cargo build --locked -p easytier-web --features embed，再串行运行 user-management-*.test.mjs | 只读，未运行 |
| `test:user-management:e2e` / `tests/user-management-e2e.test.mjs` | 隔离临时 SQLite、loopback Web 与测试 OIDC、虚构账户、Playwright；不启动 core/VPN，清理进程和临时数据 | 只读，未运行；真实 Web/数据库与模拟身份提供方须分别表述 |
| `tests/helpers/webTestBinary.mjs`、`user-management-binary.test.mjs` | 使用 WUM_TEST_WEB_BINARY 或 Cargo metadata 的 target_directory，支持 CARGO_TARGET_DIR；保留1e92296b修复 | 只读，未运行 |
| `test:e2e` / `central-e2e.test.mjs` | 另需 rust Docker 容器、核心进程及既有路径假设；不能作为本任务的无副作用检查 | 只读，未运行，不启动网络环境 |

Playwright 脚本采用 WEB_TEST_CHROMIUM 或 `/usr/bin/chromium` 的入口已核实；本任务没有验证浏览器二进制、依赖、构建产物或服务当前可用性。后续先确认版本与隔离资源，不混用其他会话的 dist。

## WUI-102 待取证与交接限制

1. 按路由表取得真实渲染前态，区分隔离真实 Web 与 mock API；记录账号能力、语言、主题、360/768/1440宽度、长名称、焦点和失败/重试动作。未取证页面不预填视觉缺陷。
2. 优先核实对象切换、抽屉关闭、迟到响应及重复提交；源码中局部序号保护不能当成全页面竞态已解决，也不能仅凭缺少序号判定已复现缺陷。
3. 实测空闲请求以本表计时器、挂载页面、当前模式和打开抽屉状态为基线，不新增周期请求，不把状态读取失败标为业务离线。
4. 主页面身份与账号生命周期由 WUI-111 修改；页面 agent 不越界。WUI-103 独占主题/CSS/locales/package.json；WUI-112 独占集中 dashboard 与真实用户 E2E；API 客户端归 ECO-111。所有共享写入由总控按 WUI 优先规则交接。
5. 本产物只交付源码事实，不含运行验收结论。最终文档状态、根汇总与看板由总控根据监工审查更新。
