# ECO-114 真实接口联调

2026-10-21，负责人 `iteration_exec2`。ECO-113 经监工验收后开始，采用提交 `80622a37ba382f19f4318755a9f589ac894ab216`，契约为 [design/contract.md](../contract.md)。

## 环境与证据边界

- 当前工作区 `/workspace/EasyTier-iteration` 的生产前端：`index-No5yjWNX.js` SHA256 `ed035866825a24eb84fe4c0aed58e22f9838281756bbebaaa5e177cc757473c0`。Vite preview 动态 loopback 端口代理真实 API，不使用旧二进制内嵌页面，无 HTTP route mock。
- 通过 `resolveWebTestBinary` 及显式 `WUM_TEST_WEB_BINARY=/workspace/EasyTier/target/debug/easytier-web` 复用旧后端。后端来源提交 `0326521d77a7bc69881f48b2e0a29d00be854084`；本次实际 `git diff --quiet 0326521d -- easytier-web/src easytier-web/Cargo.toml easytier-web/build.rs` 返回 0。不是新后端编译验证；既有 embed 前端未用于本专项。
- 每模式独立临时目录及 SQLite、虚构管理员；ordinary 账号只在测试数据库创建。API/preview/webhook接收器均 `127.0.0.1` 动态端口；config-server TCP 使用端口0且无客户端，既有实现通配监听限制沿用总控批准的容器模式，无对外发布端口。测试结束关闭进程/浏览器并删除 SQLite。
- webhook URL 指向隔离本地 404 接收器，仅启用真实外部模式，不伪造设备认证成功；本专项没有外部设备请求或 VPN。
- WUI-112 使用其冻结 dist 和独立数据库，本任务不构建、不改这些产物。ECO-113 的 protobuf codegen 缺 protoc/DNS 限制仍保留，schema 未改，不能将本测试算作 codegen 通过。

## 按需请求证据

central 与 webhook 各在生态页面完成初始查询后空闲 **22 秒**，超过已知最大相关旧周期 10 秒的两倍。真实观察如下：

| 模式 | 初始生态 hosts | 22 秒空闲请求 | 解释 |
| --- | --- | --- | --- |
| central | `/machines` 1 次；选择真实无成员网络后读取对应 `/members` | `/auth/me`、`/networks`、`/blocked-devices` 各 2 次 | 旧 MainPage 10 秒刷新；无新 `/machines` 或 `/members` 请求 |
| webhook | `/machines` 1 次 | `/auth/me` 2 次 | 旧身份刷新；整个观察没有中央 `/networks` 查询 |

静态检查生态组件和 ecosystemApi 未出现 setInterval/setTimeout/PeriodicTask/WebSocket/EventSource/visibilitychange；`online` 仅为数据字段。旧周期未被隐去，也未因为观察有限而声称永远不可能有新请求。请求摘要不记录密码、Cookie、响应体或真实密钥。

## 实际执行与失败记录

首轮真实专项 0/2：两种模式均已完成页面与22秒观察，末尾登出误用 POST 得到真实405。核实 `restful/auth.rs` 原接口为 GET 后仅改测试，未修改业务；失败不能记为通过。

`WUM_TEST_WEB_BINARY=/workspace/EasyTier/target/debug/easytier-web node tests/ecosystem-support/runTests.mjs all` 实际 **75/75 通过，0失败、0跳过**，耗时147.2秒。11个已登记文件均存在并执行，包含 support/shared/topology/nodes/resources/access/navigation/api/composition/host/integration；[完整输出](runner-all.log)保留各实际测试名称。该轮 integration 包含3项真实场景，其余72项为组件/适配器/宿主fixture或工具验证，不能合并宣称75项真实后端。

追加第4项真实删除网络恢复时，完整integration复跑3/4通过；删除实际成功204，测试误期望200而失败。修正真实DELETE契约并使该项拥有独立SQLite后，执行 `WUM_TEST_WEB_BINARY=/workspace/EasyTier/target/debug/easytier-web node --test --test-name-pattern='real deleted network' tests/ecosystem-integration.test.mjs`，**1/1通过**，见[输出](deleted-network-recovery.log)。实际删除后旧选择请求404，页面显示对象不存在；显式Refresh恢复最新列表并清除失效选择。不是all76同轮通过，最终文件4个真实用例证据是all中的3项加后续定向1项。

普通账号真实证据：临时密码账号直达生态被送到强改，未请求machines；修改测试密码再登录后，管理员管理API返回403，另一用户网络成员请求403或404且不返回成员；真实普通生态页面无管理员入口且没有管理员网络。本轮未更改生产代码，因此未重复构建；沿用已核对SHA的113产物。`git diff --check`通过。

## 逐项验收与明确限制

| ECA | 本次证据等级及结论 |
| --- | --- |
| 01 | 已验证真实身份进入生态入口；端点远程访问/设备动作无设备环境，未验证 |
| 02 | 组件 fixture 覆盖网络与应用未知状态区分；没有真实服务停止事件，不证明业务健康 |
| 03 | 真实普通账号管理接口拒绝与跨用户网络成员拒绝由专项验证；资源级动作没有API，未支持 |
| 04、05 | Android/家居来源未支持，无真实配对或执行验证 |
| 06 | 真实删除网络后查询404及用户显式刷新恢复由专项验证；其他错误/迟到覆盖属于组件fixture |
| 07、08 | 不改本地自动化；自动化连续性、跨应用动作记录没有真实环境/API，未验证/未支持 |
| 09 | 人工节点资料保存API未支持，不能宣称持久化通过 |
| 10 | 多网络对象隔离有组件fixture；真实无成员网络只能证明网络ID查询路径，真实同宿主多网未验证 |
| 11、12 | 类型及缺失能力提示有组件/页面验证；业务设备、受控访问端点API未支持 |
| 13、14、15 | 映射、撤销、导航发布API未支持；不伪造生命周期成功，仅验证能力说明组件 |
| 16 | 待接入资料保存/绑定API未支持 |
| 17 | deploy后置，本轮未执行 |
| 18 | 当前实现沿用户管理基线，旧导航17项在ECO113阶段通过；本次真实身份、强改及普通权限另验证 |
| 19 | 两模式22秒实际观察及静态检查支持无新增生态周期查询；旧壳层请求明确保留 |
| 20 | 当前实际空态/网络ID/404查询，其他切换、超时、旧响应归属由组件fixture覆盖；真实节点离线未验证 |
| 21 | 本轮只读生态适配，缺少动作API显式未支持，无受理后轮询或重放机制 |

公网/HTTPS代理、真实VPN/设备连接、配置下发、生产部署、旧二进制回退仍未验证。以上缺口按既定契约保留，不将未支持能力改成“全部外部功能完成”。
