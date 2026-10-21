# ECO-108：业务资源能力说明交接

采用 [ECO-103 v1](../contract.md)，组件 `frontend/src/components/ecosystem/resources/EcosystemResourceDetails.vue` 只消费 EcosystemResourceDetailsProps。此版本没有已核实的业务资源records、外部来源ID或访问端点，因此没有资源列表、控制事件、HTTP、表单或保存方法。

## 已实现边界

- 展示所选service/device_group/device分类，以及明确标注的宿主名称和宿主标识；这些是查看上下文，不作为业务资源身份。节点本身不会被包装成Android/家居设备，也不创建虚构资源数量或默认端口。
- 缺少资源能力时复用104未支持提示，另解释暂无已核实记录/端点。即使传入supported标志，单一能力位也不能制造记录、地址、可执行操作或成功状态。
- 宿主切换直接更新props展示；清空宿主移除旧类型上下文。不存在持久化草稿或跨节点挂载/同名合并机制。
- 普通文本绑定处理宿主名称；无链接、按钮或输入，因此不会新增键盘焦点陷阱，也不为展示未支持状态制造不可执行控件。
- 请求失败/无权限/观察时间由112查询所有者和104提示负责；当前外部配对、家居动作、审计、端点访问均未接入，不能把没有API标成请求成功或未配对。

locale由104唯一所有者新增resource.host_context/availability_note两对，同时集中处理110申请的navigation.host_context；与110执行者确认冻结之后才运行本轮消费者测试，测试期间未继续修改locale。

## 实际验证（2026-10-21）

- `node tests/ecosystem-support/checkComponents.mjs src/components/ecosystem/resources/EcosystemResourceDetails.vue`：退出0，Vue组件及公共fixture无emit类型检查通过。
- `node --test tests/ecosystem-resources.test.mjs`：**4/4通过**，0失败/跳过，约9.6秒。数据为开发fixture，真实Chromium渲染，不是后端资源证据。
- 覆盖三种分类、明确宿主上下文、无虚构资源记录/操作/端点；切换宿主与取消选择去旧资料；无localStorage；supported标志不产生假功能；无API请求/pageerror。
- CN/EN × light/dark × 360/768/1440共12组合，长且含HTML形状的宿主文本安全输出，无img、无横向溢出，无缺失语言键。并行导航组件源码写入触发一次Vite页面重载，测试最终断言均通过，未改变或放宽断言。

测试采用105隔离端口/缓存并已关闭，不写dist。未运行真实EasyADB/家居、VPN、公网或生产构建；相应ECA-04/05/08等业务能力继续记录为未实现/未验证，本任务只完成契约允许的诚实能力呈现。
