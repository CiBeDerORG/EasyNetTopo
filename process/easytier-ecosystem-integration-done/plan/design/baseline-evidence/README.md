# ECO-102：页面与请求基线证据

记录日期：2026-10-21 UTC。负责人：`iteration_exec2`。关联 [ECO-102](../../../task/task-eco-102-page-baseline.md)、[需求](../../../require/README.md)。

## 采用基线与证据等级

- 实现基线：`1e92296b17fc2e85dcd4521b6012232e11abed95`，包含用户管理、密码能力判断及测试二进制定位修复。
- 文档基线：`670eb3b42800d42333356da0cb15f41d76c92f1e`；总控整合到 `feat/ecosystem-web-ui`，工作区 `/workspace/EasyTier-iteration`。
- 以下源码事实来自上述实现基线。本记录不将源码推断、历史测试结果或模拟接口当作本次真实页面验证。
- 本任务不修改业务源码，不新建生态接口，不改变既有查询周期、账号权限或 config-server 行为。

## 入口与身份边界（源码已核实）

`easytier-web/frontend/src/main.ts` 使用 hash 路由；以下路径应在浏览器 URL 的 `#` 后使用。

| 路径 | 现有入口与边界 |
| --- | --- |
| `/auth`、`/auth/login`、`/auth/register` | `Login.vue`，登录/注册复用同一组件；可用方式由真实认证策略决定 |
| `/h`、`/h/dashboard` | `MainPage.vue` 壳层与 `Dashboard.vue` |
| `/h/users` | 用户管理，`MainPage.canManage` 检查 `users.manage` 且非强制改密；服务端仍负责授权 |
| `/h/password` | 本地改密；菜单仅 `local_password_enabled === true` 可见，`ChangePassword.vue` 接收能力与来源 |
| `/h/deviceList` | 设备宿主列表及接入命令；此处 device 不是生态下挂业务设备 |
| `/h/deviceList/device/:deviceId/:instanceId?` | 宿主/实例管理抽屉路径，不能用 peer ID 替换稳定宿主身份 |
| `/h/networks`、`/h/networks/:networkId` | 中央网络列表/详情；外部 Console 模式由壳层返回仪表盘 |

`MainPage.refreshIdentity` 获取当前身份，强制改密时跳转改密并清除临时凭据；401 结束会话。生态入口必须保留这些边界，不能仅因加入菜单而获得权限。

## 请求来源（静态事实，运行频率另行取证）

API 路径相对 `/api/v1`；请求封装位于 `frontend/src/modules/api.ts` 和 `userManagement.ts`。

| 所有者/入口 | 请求与触发 | 生命周期与复用限制 |
| --- | --- | --- |
| `MainPage.loadNavigation` | 挂载即执行、之后 `setInterval(..., 10_000)`；身份 `/auth/me`，首次 `/console-info`，中央模式 `/networks` 与 `/blocked-devices` | 卸载清理 interval。新增生态资源查询不能接入该定时器 |
| `DeviceList.loadDevices` | `/machines`；`Utils.PeriodicTask(..., 1000)` 启动时立即查询，异步任务结束后递归 timeout；另有手动刷新 | 不是固定 1 秒 interval；卸载 stop。直接嵌入整个列表会携带旧周期副作用 |
| `DeviceList` 挂载 | `/console-info`，供空态接入命令使用 | 不应把查询失败解释成未登记或离线 |
| `NetworkDetail.openNodeDetail/loadNodeDetail` | 打开节点后查询 routes、peers、ACL，随后 5 秒 interval | 通过 `/machines/:machine_id/proxy-rpc` 调用 `list_route`、`list_peer`、`get_acl_stats`；关闭详情清理定时器。不得直接作为新按需面板的生命周期 |
| `MainPage` 用户页面 | 路由/权限变化、显式动作后读取用户和事件 | 有请求序号与临时凭据清理；不要复制会话所有权到生态组件 |

`NetworkDetail` 的 routes/peers/ACL 使用 `Promise.allSettled`；失败可保留已有结果，不能据此推导节点离线或资源健康。以上请求清单只覆盖本任务读到的入口，不宣称全仓无其他周期请求。

## config-server 兼容基线

接入命令格式保持 `easytier-core --config-server ${proto}://${host}:${port}/${username}`，协议、端口、用户名来自 `/console-info`。`MainPage` 的 host 来自 `new URL(getApiBase(), window.location.href).hostname`；`DeviceList` 来自 `window.location.hostname`。两者当前来源不同，跨源部署下不能未经契约核实就合并为同一实现。本任务只记录，不执行命令、不接入真实设备、不修改 VPN/路由/防火墙。

## 页面取证环境与当前限制

- 新工作区最初没有依赖或构建产物；旧工作区的 dist/二进制不能冒充当前页面产物。
- 已将既有 pnpm 依赖复制到新工作区，当前共享库完成 protobuf 生成、`vue-tsc -b` 与 Vite 构建；生成源码无差异。旧工作区产物未修改。
- 原 `pnpm ... build` 入口触发自动依赖检查，因 `/home/agent/.local/share/pnpm` 不存在失败；使用清单中相同工具逐项执行。protoc 自动下载解析失败后使用既有本地归档，不新增下载。
- `/usr/bin/chromium` 可用；未复用残留浏览器用户数据或会话。
- 旧二进制对应后端源码与本基线比较无差异，但不能据此声称重新编译验证。后续如复用，仅用于真实 API；页面必须来自本工作区当前源码。
- `main.rs:get_config_server_listeners` 固定通配监听，缺少单独 loopback 地址参数；临时端口 0 不等于 loopback 绑定。经总控确认，沿用现有 E2E 隔离容器模式：API `127.0.0.1:38237`，Vite `127.0.0.1:43021`；config-server TCP 端口 0、无客户端，不发布端口或修改网络设置。WUI-102 共享取证完成后已统一关闭 API/Vite，并核实两个进程均不存在；临时 SQLite 目录已删除。

## 真实浏览器取证（已执行）

2026-10-21 08:58 UTC，系统 Chromium、独立浏览器 context、新建 SQLite、虚构管理员；当前前端源码 + 当前生成的共享库，真实后端 API，无接口 mock。WUI-102 在同一隔离数据库于本次采证前后（08:57–08:58 UTC；未单独记录创建秒数）建立了一个无成员的虚构网络，故网络列表截图包含该记录；不声称全程空网络。没有设备或 VPN 实例。

| 证据 | 实际观察 |
| --- | --- |
| [auth-1440.png](auth-1440.png) | 未登录页面；真实策略返回注册禁用 |
| [device-list-1440.png](device-list-1440.png) | 设备为 0，显示空态与 config-server 接入命令；命令端口 0 是测试配置，不可作为实际接入地址 |
| [network-list-1440.png](network-list-1440.png)、[network-list-360.png](network-list-360.png) | 真实虚构网络列表，桌面表格与窄屏卡片；0/0 成员，不宣称设备连接 |
| [runtime-observations.json](runtime-observations.json) | 24 次页面 API 响应摘要，无请求体、Cookie 或令牌；此范围未捕获 pageerror |
| [network-detail-empty-1440.png](network-detail-empty-1440.png) | 无成员网络的详情入口，不替代真实节点详情 |
| [network-detail-missing-1440.png](network-detail-missing-1440.png)、[detail-observations.json](detail-observations.json) | 不存在网络的真实接口失败场景；敏感配置展示区域被遮蔽 |

设备页空闲观察 11 秒，仅作为旧周期出现的基线证据，不是新功能“两周期无轮询”验收。实际 `/machines` 连续响应间隔约 1.01 秒；同时观察到壳层第二次 `/auth/me`、`/blocked-devices`、`/networks`。页面进入时两个 `/console-info` 分别来自壳层和设备页。切换网络列表后设备查询停止，列表读取 `/networks/gateway-info` 与 `/networks`。不存在网络的真实请求返回 404，页面显示“Unable to refresh data”与 Retry，而非离线；相关 12 次响应见 detail-observations.json，未捕获 pageerror。此记录是旧页面前态，不允许新增生态页面继承这些周期查询。

未开展 WUI 全页面/语言/主题矩阵，也未运行全量测试；共享库生成、类型检查、构建通过仅证明采证依赖可用。

## 尚未验证及下游交接

| 项目 | 当前证据/缺失 | 对 ECO-103 的影响 |
| --- | --- | --- |
| 登录、权限、设备空态、网络页面真实渲染 | 已取得上述最小页面证据；权限角色矩阵未测试 | 可据截图确定既有入口，不能扩张为完整权限验收 |
| 实际请求顺序和空闲频率 | 设备/壳层周期已有网络摘要；真实节点详情缺设备 | 按需契约必须显式排除旧 timer 副作用 |
| 真实宿主、所选成员、节点 routes/peers/ACL | 没有隔离的已连接设备/实例 | 仅确认接口和入口，不能宣称真实拓扑、直连/中继、ACL 有效 |
| 业务资源与公网/导航入口 | 未提供真实来源/API/公网环境 | 交 ECO-101 能力清单；缺失能力显示未支持，不编造资源成功状态 |
| config-server 实际连接/认证/下发 | 仅静态命令与身份来源核查 | 保持既有语义，后续集成必须区分测试设备与真实网络验证 |

未取得的证据不阻塞已核实静态契约，但禁止下游将其作为已验证能力。2026-10-21 UTC：ECO-102/WUI-102 浏览器均已关闭；API/Vite 进程退出已核实，临时 SQLite 与服务信息文件已删除；依赖与隔离构建产物留给后续实施使用，未纳入提交。未复制数据库或浏览器会话到版本库。
