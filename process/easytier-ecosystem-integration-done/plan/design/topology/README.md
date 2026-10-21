# ECO-106：网络归属视图交接

采用 [契约v1](../contract.md)、[源码基线](../baseline.md) 与 [ECO-102真实旧页面证据](../baseline-evidence/README.md)。105公共预览和104展示组件已验收后开始实现；本组件位于 `frontend/src/components/ecosystem/topology/EcosystemTopology.vue`，只消费 EcosystemTopologyProps、发出 refresh/selectNetwork/selectNode，不执行HTTP或拥有会话。

## 展示和所有权

- 页面称“网络归属”，以文本卡片表达宿主/所选网络成员，临时peer单列；没有已核实连接数据，所以不绘制P2P/中继/延迟边，也不把成员归属画成实测路径。
- 宿主以userId+machineId匹配，多网络归属不复制节点记录，同名宿主不会合并；成员找不到同租户宿主时标资料未知并禁用详情按钮，不借用其他租户同machineId资料。
- 网络选择由父层处理，组件只发事件；选择已有宿主同样只发稳定身份。地点筛选是本地筛选已返回的GeoIP文本，明确称连接IP估计位置；人工部署地点仍未支持。
- 初次loading/error不显示成功空态；失败可保留同一对象上次成功快照及时间。成员快照networkId与当前选择不符时不显示旧行，ready会降为“尚未查询”而非假称新网络已加载。
- webhook模式明确中央网络归属不可用，不渲染中央网络选择或“0网络”成功断言；仍显示当前宿主快照。
- 纯派生转换集中在局部computed与有明确语义的小函数；无HTTP客户端、轮询、持久化、订阅或跨组件直接调用。不修改既有config-server/用户权限/认证行为。

ECO-104唯一文案所有者在本次交接追加`web.ecosystem.topology.all_locations/no_matching_hosts`两对cn/en键，保留其他任务的文案，不新增另一份locale事实源。ECO-112组合页负责请求去重、身份变化清空、所选网络切换请求；组件事件模板监听继续用kebab-case。

## 实际验证（2026-10-21）

- `node tests/ecosystem-support/checkComponents.mjs src/components/ecosystem/topology/EcosystemTopology.vue`：最终退出0，包含公共fixture和导入组件的Vue严格无emit类型检查。
- `node --test tests/ecosystem-topology.test.mjs`：最终 **7/7通过**，0失败/跳过，约15.4秒。真实浏览器加载Vue组件，输入是开发fixture而非真实宿主/API。
- 覆盖键盘稳定身份选择、网络事件不越权改props、同名/多网络宿主、GeoIP筛选、webhook、loading/403/空态/未知、同machineId不同用户隔离、跨网络旧快照、失败保留时间。
- CN/EN × light/dark × 360/768/1440共12组合，长主机名/网络名下控件可见、可聚焦、无横向溢出；所有测试检查没有组件发出的API请求或pageerror。此结果不代替真实VPN或整个页面的空闲请求验收。
- 初次类型检查发现局部派生变量与props同名，已重命名；初次浏览器7项中3项暴露select嵌套label将选项混入可访问名称，已分离label和select后复跑通过，未放宽断言。
- [桌面英文浅色](fixture-en-light-1440.png)、[窄屏中文深色](fixture-cn-dark-360.png)为公共harness的开发样例截图，含明确Development sample提示，不是实网拓扑。无真实账户或凭据。

验证服务使用105的临时cache和随机回环端口，结束后关闭；未写dist、未安装依赖、未升级锁文件、未执行生产构建或后端联调。公网、业务资源与真实链路仍按契约标未支持/未验证；本任务只完成网络归属展示与选择。

监工预审后补充：旧网络的error+previous也不得污染新选择，已将不匹配旧快照的ready/error展示为idle（新loading仍保留loading）。针对修改执行同一105组件类型检查及 `node --test --test-name-pattern='unknown data' tests/ecosystem-topology.test.mjs`：类型退出0，目标场景 **1/1通过**；此前完整7/7与此定向结果分别记录，没有将本次过滤执行称为7项全跑。测试断言旧ready/error均无旧宿主行或旧错误提示。

截图最初暴露105预览HTML缺charset导致样例横幅乱码，已由105所有者修复后重采；未修改生产页面编码或越权改公共harness。
