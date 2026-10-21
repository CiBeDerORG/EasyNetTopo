# ECO-107：宿主详情与资源类型交接

采用 [ECO-103 v1](../contract.md)，组件 `frontend/src/components/ecosystem/nodes/EcosystemNodeDetails.vue` 消费 EcosystemNodeDetailsProps，仅发出 `selectResourceKind`；无HTTP、定时器、表单保存或额外身份状态。基于104/105已交付组件与预览，未直接引用其他未交付的业务资源组件。

## 行为与边界

- 宿主别名/主机名、系统类型/版本/发行版、EasyTier版本、最近出现时间只展示已返回字段。机器标识、网络标识、运行实例标识分别呈现，不能互换或根据名称猜测。
- GeoIP明确为连接IP估计位置；人工部署地点、厂商、型号、架构缺少契约字段时显示未知，人工资料保存显示未支持。不从hostname推断架构，不创建浏览器“保存”假持久化。原设备别名编辑继续留在既有页面。
- 未选宿主只显示选择提示；切换为null立即移除旧资料和类型操作。未知会话不显示离线，控制在线不代表业务健康/公网可达。空实例列表只称“未上报”，不声称所有实例已停止。
- webhook网络归属显示未支持，不能把其空数组当“没有所属网络”。中央模式按返回的网络引用只读展示，同一宿主多网络仍一份资料。
- 服务、设备组、设备三个按钮是查看能力的导航意图，明确这些是宿主下的业务分类，不把宿主自身变成业务设备；无配对/创建/控制操作。缺失资源API的说明保持可见。
- 请求失败、无权限与上次查询时间仍由ECO-112/104查询层显示；本组件没有自行查询，因此不伪造组件内部的加载/保存失败流程。

所有字段用文本绑定，长文本换行；PrimeVue按钮有可见名称并可键盘激活。类型转换只有局部hostFields与estimatedLocation派生，不新增通用框架。locale由104唯一写入者配对追加node下additional_metadata/deployment_location/manufacturer/model/architecture/no_networks_reported/no_instances_reported/resource_boundary八个键；另按ECO-109消费者请求补access.supported_note一对，能力可用不表示授权或服务已启动。

## 实际验证（2026-10-21）

- `node tests/ecosystem-support/checkComponents.mjs src/components/ecosystem/nodes/EcosystemNodeDetails.vue`：退出0，真实Vue组件与公共fixtures无emit类型检查通过。
- `node --test tests/ecosystem-nodes.test.mjs`：**5/5通过**，0失败/跳过，约12.4秒。真实Chromium加载组件，数据为开发fixture，不是真实节点。
- 覆盖缺失字段/未知/无假保存、OS与GeoIP/人工位置分离、实例与网络区分、null切换清理、三个键盘类型事件且不改props、webhook不误报空态；所有页面检查无API请求/pageerror。
- CN/EN × light/dark × 360/768/1440共12组合，长名称、网络、实例与`<img ...>`恶意形状文本安全显示、不创建img/input、无横向溢出、按钮可聚焦。测试期间109申请的新增语言键触发Vite语言资源热更新，最终所有断言仍通过；没有删减或放宽断言。

未执行后端集成、真实宿主、VPN、公网或生产构建。人工资料写入与业务资源控制按契约未支持，不以本任务通过宣称这些业务验收完成。测试服务使用105随机回环端口/临时cache并已关闭，不改共享dist。
