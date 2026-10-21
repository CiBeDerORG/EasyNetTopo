# ECO-101：源码基线与接口能力清单

关联：[任务](../../task/task-eco-101-source-capabilities.md)、[需求](../../require/README.md)、[基线计划](../plan-eco-001-baseline.md)。供 ECO-103 契约收敛使用；本文没有定义新的后端接口。

## 核查范围与证据等级

2026-10-21，在 `/workspace/EasyTier-iteration`、`feat/ecosystem-web-ui` 核查。业务代码基线为 `1e92296b17fc2e85dcd4521b6012232e11abed95`，文档来源为 `670eb3b42800d42333356da0cb15f41d76c92f1e`；核查期间总控正在整合文档布局，未改业务代码。保留 WUM-012–014 的本地密码能力和 Cargo 测试路径修复。

以下“存在”指已读取客户端、路由、必要处理器及模型的静态事实，不代表运行验证成功。没有运行构建、测试、页面或外部接口；运行证据由 ECO-102 另交。未提供 EasyADB、家居或 Navloom 的可核实 API 来源，未调用外部服务。

源码路径均相对仓库根，关键入口位于 `easytier-web/frontend/src/modules/api.ts`（下称 ApiClient）、`easytier-web/src/restful/network.rs`（机器 API）、`easytier-web/src/restful/central_network.rs`（中央网络 API）与 `easytier-web/src/restful/rpc.rs`（RPC 代理）。所有 HTTP 路径以下均包含 `/api/v1` 前缀。

## 可复用调用链

| 用户操作/数据 | 已核实方法与 HTTP | DTO 与服务端依据 | 授权、错误和限制 |
| --- | --- | --- | --- |
| 查询控制台模式 | `get_console_info` → `GET /console-info` | `ConsoleInfo`：username、config_server_protocol/port、webhook_auth；`restful/mod.rs::handle_get_console_info` | 已登录路由；模式字段必须先用于能力判断，不改变 config-server 接入命令 |
| 列出宿主节点 | `list_machines` → `GET /machines` | `handle_list_machines` 返回 `{machines}`；`ListMachineItem`：client_url、alias?、info?、location?、networks、online、last_seen?；info 为现有 HeartbeatRequest | 常规模式 `db.list_devices(user_id)`；DB 失败 500。客户端当前是 `Array<any>`，ECO-111 应在边界明确收窄，不让 any 扩散到视图 |
| 修改宿主别名 | `update_machine_alias` → `PUT /machines/{machine-id}/alias`，`{alias}` | `handle_update_machine_alias` → `db/devices.rs::set_device_alias`，按 user_id + machine_id 更新 | 先 trim，最多 64 字符；成功 204，过长 400，无当前租户设备 404，DB 失败 500。只能证明别名保存，不能用于地点/型号或业务设备资料 |
| 网络列表 | `list_networks` → `GET /networks` | `{networks: CentralNetworkSummary[]}`；service.rs::list_networks 按 user_id 查询 networks 与 members | 只在本地中央管理模式注册；online_member_count 来源控制会话，不是业务健康或拓扑边数 |
| 所选网络成员 | `list_network_members` → `GET /networks/{network-id}/members` | `{members: CentralNetworkMember[], temporary_peers}`；service.rs::list_members 先 load(user_id, network_id)，再组装成员与临时 peer | member 含 device_id/member_id、online、running、runtime_virtual_ipv4、version、error_msg 等；临时 peer 不保证有注册宿主身份，不能按名称并入机器 |
| 网络配置已有操作 | `create_network/get_network/update_network/delete_network` → `POST /networks`、`GET/PATCH/DELETE /networks/{network-id}` | settings 与 NetworkDetail；处理器传入当前 user_id | 既有网络意图，不是业务资源 CRUD。详情含 network_secret，拓扑只需列表时不应读取或传播详情 |
| 成员已有操作 | `add_network_members/update_network_member/remove_network_member` → `POST /networks/{id}/members`、`PATCH/DELETE .../members/{device-id}` | 添加 device_ids/temporary/ttl_seconds；修改 hostname_override、virtual_ipv4、proxy_cidrs | 仅中央成员语义，不是业务设备分组或跨系统授权。后端执行租户/网络检查 |
| 成员配置 | `get_member_config/set_member_config/clear_member_config` → `GET/PUT/DELETE /networks/{id}/members/{device-id}/config` | 现有 NetworkConfig；PUT 包装 `{config}` | 可能含敏感配置；中央网络配置由中央意图所有者管理，不能绕过所有权改运行实例 |
| 所选实例路由/peer | `get_node_routes/get_node_peers` → `POST /machines/{machine-id}/proxy-rpc` | `api.instance.PeerManageRpcService` 的 list_route/list_peer；payload.instance.id 用现有 Utils.StrToUuid；响应 routes/peer_infos | `handle_proxy_rpc` 按当前 user_id + machine_id 查在线 session；无会话 404，未知 service 400，RPC 错误 500。调用失败不是整个宿主或业务设备离线 |
| 实例列表/状态/配置 | `WebRemoteClient` → `/machines/{machine-id}/networks`、`/networks/info/{inst-id}`、`/networks/config/{inst-id}` 等 | 现有 RemoteClient/NetworkTypes；机器 API `handle_list_network_instance_ids/handle_collect_one_network_info/handle_get_network_config` | ClientNotFound/NotFound 404、RPC执行错误400、超时504、其他RPC失败502；直接改中央所有配置409，code=`central_network_ownership_conflict` |
| ACL 与网络凭据 | `get/update_network_acl_policy`、`list/generate/revoke_credential` 对应已有中央网络路由 | AclPolicyInfo、NetworkCredential；credentials 包含 credential_secret | 只是网络访问控制与凭据，不证明业务资源动作授权或公网发布撤销；生态普通视图无需拉取凭据秘密 |
| 网关信息 | `get_gateway_info` → `GET /networks/gateway-info` | enabled、peer_url?、relay_data；`CentralNetworkApi::gateway_info` | 是中央网络网关配置，不能解释成浏览器可访问的公网业务入口 |

中央网络 API 的统一业务错误由 `central_network.rs::convert_error` 映射：Invalid→400、NotFound→404、Conflict→409、Database→500，响应为 Error.message（code/current_config_revision 可空）。不解析错误文案来推导离线、权限或成功；保留 HTTP 状态与明确 code。

## 模式、权限与身份边界

1. `restful/mod.rs` 在 `webhook_config.has_external_endpoint()` 为真时不注册中央网络路由。`GET /console-info` 的 webhook_auth 可用于能力判断；不能把这种模式下 /networks 的缺失误报成“没有网络”或认证失败。
2. `handle_list_machines` 的 webhook 分支仅从当前用户在线会话构造列表，alias=None、networks=[]；不具有常规模式的离线设备登记语义。此模式下不能承诺持久别名或网络归属完整。
3. 常规模式设备实体 `db/entity/devices.rs::Model` 主键为 user_id + machine_id；包含 hostname、alias、easytier_version、device_os、client_url、first_seen_time、last_seen_time。`db/devices.rs::list_devices/get_device/set_device_alias` 均按用户隔离。前端不得自己接受任意 user_id 来跨租户查询。
4. `db/entity/network_members.rs::Model` 由 user_id + network_id + id 标识，device_id 指宿主设备。成员关系不等同物理连接；peer_id 不等同机器 UUID；同一宿主可有多个成员/实例。业务设备的 source_instance_id/external_id 当前没有对应已核实的数据来源，不能复用 machine_id 或名称强行映射。
5. 浏览器路由使用现有 login_required、会话与 CSRF 保护；`auth.rs::require_password_change` 对受限会话只放行 auth/me、check_login_status、logout、PUT auth/password，否则 403 `password_change_required`。ApiClient 维持 withCredentials，401 交给已有认证失败回调；必须保留 OIDC、本地密码能力及强制改密流程。
6. 既有网络控制授权不能证明“资源控制/公网发布/导航可见”分层授权已经存在。UI 禁用只是能力提示，不能充当新资源鉴权。

## 状态与配置不能互相替代

- `ListMachineItem.online` 常规模式表示控制 session 存在，`last_seen` 是登记记录时间；既有会话 heartbeat 不授权新增页面轮询。API 缺失/请求失败必须显示未知或查询失败，只有明确布尔离线才表达控制离线。
- `Location` 只有 country、city?、region?，来源 `client_manager/mod.rs::lookup_location(client_url, geoip_db)`。私有/回环地址被归为“本地网络”；不是人工部署地点，更不是 GPS。
- `GatewayRuntimeConfig/PortForwardConfig`（`easytier-core/src/config/gateway.rs`）只有 socks5_bind 与端口规则 bind_addr/dst_addr/proto。配置原语及代理 RPC 的 PortForwardManageRpcService 不具备已核实的资源ID、资源权限、公网入口登记、端口冲突事务或活动连接撤销保证。
- 已保存配置、已应用配置、资源运行、浏览器实际可达分别需要来源证据。后端可达不证明浏览器可达；网络 secret、凭据、完整配置不写日志/路由/截图或普通资源状态。

## 能力缺口及给 ECO-103 的收敛输入

搜索范围为 `easytier-web/src`、`easytier-web/frontend/src` 的相关 route/模型/API，并搜索 EasyADB、Navloom/Navloop、home-assistant、资源端点/公网、manufacturer/model 等标识。该范围未发现以下完整能力；不据此断言外部仓库不存在。

| 缺口 | 影响 | 本轮允许的交接方式 |
| --- | --- | --- |
| 人工地点、厂商/型号及资料来源持久化 | ECO-09、ECA-09/16 | 显示已知系统信息与明确未支持字段；别名接口仅保存别名，不创建假“保存成功” |
| 服务/设备组/Android/家居业务设备注册与查询/操作 API | ECO-03/04/05、ECA-04/05/11 | 资源页面可展示未支持及缺少来源；没有真实 API 就不发虚构路径、不以 mock 数据作生产结果 |
| 资源级端点/公网发布、端口冲突、撤销活动流 API | ECO-11/12、ECA-12/13/14 | 显式缺少能力，禁止宣称端口原语已满足资源公网发布；不扩建后台平台或实际开放网络 |
| 导航登记/资源关联/删除同步与 Navloom 数据接口 | ECO-13、ECA-15 | 导航与公网能力独立；只能使用已核实的内部页面目标，不能构造未授权外链或假发布 |
| 跨应用操作审计与外部身份映射 | ECO-06/07、ECA-03/08 | 沿用已有用户与网络权限；不能声称统一身份或业务动作审计已完成 |

ECO-103 应固定模式能力、主机/成员/实例/业务设备类型、unknown/unsupported/error 与最后观察时间，以及基于现有接口的最小接入清单。适配器应只做必要 DTO 收窄与纯转换，不引入通用连接器框架。新增查询只由进入、切换、手动刷新、明确操作完成触发；不用定时器、自动重试、SSE/WebSocket、离线重放补齐缺口。

## 已有验证入口及未执行项

`frontend/package.json` 已有 build、test:browser、test:e2e、test:user-management、test:user-management:e2e；build 会写 frontend-lib/dist 与 frontend/dist，后续必须由总控隔离或串行。test:browser 使用 Playwright；dashboard.test.mjs 默认预览 127.0.0.1:5198，可提供 WEB_TEST_URL，不能与其他任务争用端口。

`tests/helpers/webTestBinary.mjs::resolveWebTestBinary` 支持 WUM_TEST_WEB_BINARY，否则从 Cargo metadata 的 target_directory 定位真实二进制，不回退旧 target/debug。user-management-e2e.test.mjs 使用临时目录、隔离数据库、回环动态端口和虚构 OIDC；这些仅为已读脚本事实，本任务没有启动它们。后续真实集成仍需对应版本二进制、前端产物、Playwright Chromium 和隔离运行条件。

本任务验证仅为路径/符号与文档链接核对；未执行应用测试，未验证 VPN、HTTPS 代理、公网访问、第三方集成或旧二进制回退。
