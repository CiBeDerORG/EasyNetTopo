# ECO-111：只读适配交接 v1

依据 [ECO-103契约](../contract.md)、[真实源码能力清单](../baseline.md)。本轮基线 `feat/ecosystem-web-ui@50e9edb4d8814028a4c35b46e290bf826b47ede2`；不修改后端、协议或config-server。

## 所有权与实际调用

`src/modules/ecosystemApi.ts:createEcosystemReader(api, currentUserId, {timeoutMs?})` 只接受已验证宿主身份，返回契约的loadOverview/loadMembers。身份不会作为查询参数发送；没有auth/me、自建会话、计时循环、重试或订阅。

- overview按序读取console-info、machines，只有明确webhook_auth=false才读取networks；任一步失败均拒绝整个新快照，不返回伪空成功。
- members只允许最近成功overview列出的中央网络；尚未加载、刷新在途/失败、webhook和未知网络分别拒绝，不发猜测路径。
- overview每次开始失效适配器的成员读取许可；代次检查阻止迟到overview覆盖许可或继续发后续中央请求。旧请求本身没有声称已取消。页面仍须按ECO-112契约丢弃旧结果、管理snapshot和previous。
- loadMembers独立返回所请求networkId，不写全局“当前成员”；同时读不同网络不会共用返回数据。页面负责选择改变、卸载与身份变化时的显示代次。
- 四个既有ApiClient读方法仅增加可选ApiReadOptions(timeout毫秒、signal)。无参数旧调用保持原路径、响应解包、失败、withCredentials和401回调。旧null envelope仍抛错；reader在调用边界将这类TypeError归为invalid_response，不修改旧页面处理。
- 新reader每个HTTP请求默认15000ms超时，可注入有限正数用于针对性验证。超时停止该轮，无自动后续调用；这不是刷新计时器。最大overview为至多三个有限请求，不承诺15秒为整个overview总时长。

## DTO依据与收窄

实际读取 `restful/network.rs:ListMachineItem/handle_list_machines`、`db/entity/devices.rs`、`db/entity/network_members.rs`、`proto/web.proto:DeviceOsInfo/HeartbeatRequest` 与ApiClient中央网络DTO：

- 正常离线宿主由后端登记记录构造HeartbeatRequest，machine_id为现有四个uint32字段，running_network_instances为空；OS字段是os_type/version/distribution。在线webhook记录info可以为null；该合法传输形态缺失可用主机身份，本契约明确报invalid_response并保留页面旧快照，不能编造零UUID或按client_url认主机。本期不宣称所有机器记录都能显示。
- UUID对象先检查四字段整数范围，再调用既有Utils.UuidToStr；成员device_id为UUID字符串并规范小写。网络/member标识检查非空安全标识，不能把路径片段当ID。
- `parseHostNode/parseNetworkSummary/parseMembers` 单独承担领域转换；小型record/list/optionalBoolean/count仅做必要形状收窄，不引入通用schema框架。
- 未知可选文本归null，缺少online归unknown，明确false为offline；缺人数为null，真实0保留。非法布尔/计数/关键ID/数组拒绝，不过滤坏记录成成功空列表。
- 临时peer只有networkId/peerId/hostname/ipv4，不赋machineId。网络归属不是P2P边，running实例不由networkId推断。
- 明确投影字段，不保留client_url、user_token、network_secret、credential_secret、完整配置或原始错误。未知业务能力保持各自unsupported原因。

## 错误与消费者

`EcosystemReadError`只含安全kind/status与固定通用message；`toEcosystemQueryError(unknown)`提供消费者统一安全映射。401沿旧ApiClient回调；403 password_change_required与其他403分离；404、HTTP408/504或Axios超时、无响应网络错误、结构错误、其他失败分别处理。错误不保留Axios response/config/cause或服务端message。

ECO-112应使用该转换函数，不能把错误对象直接显示或从message猜测。身份变更重新创建reader并清页面数据；overview更新失败后只能手动刷新重新取得成员读取许可，旧snapshot可继续显示但不是新查询成功。

## 实际验证

`tests/ecosystem-api.test.mjs`通过现有esbuild编译真实ApiClient/adapter，保留真实Axios/interceptors与共享UUID工具，仅隔离无关NetworkTypes导出；使用本机回环HTTP fixture服务。**它不是真实Web/数据库联调**。

覆盖：准确GET调用清单、注入身份不外发、central/webhook分支、unknown/false/0、空数组、无身份/坏UUID/坏计数/坏布尔、secret投影、401回调、403/404/超时/断连、无重试、迟到overview、并发成员、旧无options调用。实际Axios25ms超时通过故意延迟100ms的fixture验证，等待后仍只有一次请求。

已执行：

- `node tests/ecosystem-support/checkComponents.mjs src/modules/ecosystemApi.ts`：严格类型检查退出0。
- `node tests/ecosystem-support/runTests.mjs api`：初版9/9通过；后续代次修订的最终结果在下方记录。
- `node --test tests/user-management-client.test.mjs`：6/6通过，保留旧账号transport、MD5输入、错误与敏感信息行为。

运行有现有依赖的Node url.parse弃用提示，未升级依赖。首次测试文件写入命令因工作目录错误未创建文件，runner明确拒绝缺失suite；纠正路径后完成上述实际验证，不计零测试为通过。

未执行真实后端空环境联调、真实节点/设备、VPN、公网、HTTPS或完整生产构建；由ECO-114及最终验收分别记录。没有安装依赖、写production dist或修改其他任务文件。

2026-10-21最终交接：代次修订后严格类型检查退出0，API专项 **9/9通过、0跳过**；此前旧用户客户端 **6/6通过**。任务实现和测试已交监工，不自行修改看板状态。
