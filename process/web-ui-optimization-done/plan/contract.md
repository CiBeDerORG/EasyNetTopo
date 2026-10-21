# WUI 公共契约 v1

负责人：`iteration_exec2`（WUI-103）；采用代码基线 `1e92296b17fc2e85dcd4521b6012232e11abed95`，本轮分支 `feat/ecosystem-web-ui`。

输入：[WUI-101 基线](../require/baseline.md)、[WUI-102 页面证据](../require/evidence/README.md)。本契约只组织既有页面改进；不新增后端、业务对象、轮询、自动重试、订阅或通用框架。公开 API、路由、DTO、存储键及 config-server 行为保持兼容。

## 证据驱动的本轮改动

WUI-102 的 96 个真实页面/语言/主题/宽度组合未发现整页横向溢出或 pageerror；已有失败草稿、忙碌去重等观察符合预期，不能据此重写这些页面。

| 实际问题 | 唯一修改者与交接 |
| --- | --- |
| 360px 长网络标题使 topbar 高 147px，菜单按钮宽约 9.41px | WUI-103 调整公共 flex 空间、保留按钮宽度并视觉省略标题；完整标题保留 DOM。WUI-111 维护宿主标记与可访问名称 |
| 窄屏用户表用户名列约 32.72px、长名称行高约 292.72px | WUI-105 在 UserManagement 局部设置可读最小列宽/受控表格滚动；不得用全局 body 横向隐藏掩盖 |
| 移动导航 Escape 关闭后焦点落到 BODY，重复 3 次 | WUI-111 由宿主拥有触发元素及返回焦点；不在公共 CSS 或页面子组件复制焦点所有权 |

其余任务按已有验收范围验证保留，不为显示进度强行改动已满足要求的代码。新问题须附真实复现条件与证据，再交对应文件所有者。

## 公共视觉与命名

- 沿用 Aura/PrimeVue、`theme.ts` 和 `console-theme-mode` 的 light/dark/system；不新建 token 或主题切换系统。只使用已有 `--p-*`、`--console-*`、共享兼容变量。
- 公共容器仍为 `console-page`、`page-heading`、`console-panel`、`list-toolbar` 等现有类。局部样式用所属页面语义，不创建 common2、万能 utility 或全局不带范围的按钮覆盖。
- topbar 约定：第一个直接 div 为面包屑/当前标题，第二个为操作区；前者伸缩、后者不收缩，前者最后一个 span 是完整页标题。当前结构已由生产组件测试覆盖。这些结构选择器仅为兼容当前宿主的过渡。WUI-111 必须增加明确的 `console-breadcrumb`、`console-page-title`、`console-topbar-actions` 类，并提供标题 `title` 属性及正文完整名称；随后由 WUI-103 串行替换为语义选择器，不由 WUI-111 越界改 CSS。
- 标题省略仅是视觉呈现：DOM 文本和页面 h1 保留完整名称。禁止删改字符串来缩短标题；WUI-111 补可视用户可发现的完整名称方式后，才闭合 OBS-003 的全部交接。菜单及操作按钮不得被长名称挤到不可点击。
- Vue 组件 PascalCase、内部 TS camelCase；API/协议已有 snake_case 不重命名。区分设备宿主、运行实例、网络成员、peer 与生态业务设备；不把“保存”“应用”“运行”“可达”合为成功。
- cn/en 文案由本任务唯一写入。优先复用已有 `web.console`、`web.common`、`user_management` 键；新键须同时给两种语言、插值参数与使用者。当前公共修复没有新增文案，不制造同义键。

## 页面输入、输出与归属

| 任务/页面 | 必须保留的输入输出及职责 |
| --- | --- |
| WUI-104 Login / ChangePassword | Login 的 `isRegistering`、原登录/注册/OIDC路由；ChangePassword 的 api/required/localPasswordEnabled/source。只有明确本地密码能力 true 展示可用表单；不从账号来源推断 |
| WUI-105 UserManagement | current-user/users/events/selected-user/loading/busy/stale/error/credential 等 props；query/select/create/set-access/set-role/reset-password/revoke-sessions/query-events/dismiss-credential/retry 事件。展示组件不自行建 HTTP 客户端，不复制宿主的凭据/权限状态 |
| WUI-106 Dashboard | api/centralEnabled 输入、现有数据源与导航；只调整本页面证实问题，不能把接口失败转换为设备 0 或离线 |
| WUI-107 DeviceList / DeviceDetails | api/centralEnabled、device/compact/containerClass；保留设备路由及别名/删除语义。DeviceDetails 为纯展示，不新增查询 |
| WUI-108 DeviceManagement | api/deviceList、原 deviceId/instanceId 路由；继续给共享 RemoteManagement 提供 RemoteClient，不重写共享实例管理或配置逻辑 |
| WUI-109 NetworkList | api、既有网络创建输入/规范化/成功返回 network_id 后跳转；错误保留草稿，不能把提交受理显示成运行成功 |
| WUI-110 NetworkDetail / AclPolicyTab | 原 networkId 路由及 api/networkId/members 契约；详情与 ACL 同一所有者。宿主使用 networkId key 重建，不能假定缺 watch 就是已复现串数据 |
| WUI-111 MainPage / main.ts | 会话身份、权限、临时凭据、导航与焦点所有权；整合页面事件、保持深链与强改约束；公共 CSS 变动回派 WUI-103 |

上述明细来自 [源码基线](../require/baseline.md)。不建立第二份接口类型定义；类型来源仍是现有 API 模块和共享库。类型或 props/events 变化必须先说明生产者、消费者、错误路径与兼容性，经总控/监工审查后改契约。

## 状态、表单及异步交互

1. 读取区分初始加载、成功空数据、成功有数据、失败、保留的旧数据；未知与明确离线分开。部分失败不报告全成功。
2. 页面拥有表单草稿；提交期间禁重复动作并提供忙碌反馈。失败保留非敏感草稿、给出可操作错误；密码和临时凭据按现有生命周期清除，不新增持久化。
3. 确认弹窗显示明确对象和后果；取消不触发写入。打开时聚焦可用控件，关闭返回触发者；若触发者已卸载则移至当前任务的合理目标。WUI-111 处理导航 Drawer 的已证实焦点缺陷。
4. API 仍经 ApiClient/UserManagementClient；保留 401 结束会话、403 刷新或拒绝、冲突及网络错误差异。禁止渲染原始敏感响应或 HTML。
5. 选中对象或路由变化后，旧结果不得恢复错误对象或敏感凭据。复用已有序号/清理，不建立多个同等状态所有者；需要新增守卫时明确生命周期和测试。
6. 原周期请求见 WUI-101，仅保留既有机制；不借共享 helper 添加 timer、自动重试、离线重放、WebSocket/SSE。状态刷新入口只沿既有契约。
7. config-server 接入命令的各自 host 来源、协议/端口/用户名与复制内容保持原样；本任务不统一 MainPage 与 DeviceList 的不同 host 推导。

## 验证与共享写入交接

- 公共 CSS/theme/locales/package.json、`tests/web-ui-support/` 由 WUI-103 唯一写入；页面不能抢写。ECO-104/105 获总控交接后分别使用 locale/package，后续变动再排队。
- `tests/web-ui-support/webUiBrowser.mjs` 仅拥有生产 preview 与浏览器的启动/关闭；调用者拥有独立 context、fixture 和断言。无全局 fixture 状态，不承担业务策略。
- `test:web-ui:foundation` 先按既有脚本构建，再运行生产组件布局回归。HTTP 为明确 mock，不能称作真实后端联调。每个后续任务仍运行自身针对性验证；不得把此12组合测试当作WUI-102的96组合真实取证。
- 页面视觉验证至少按修改影响覆盖 cn/en、light/dark、360/768/1440、长名称与键盘；通过、失败、未运行分别记录。DOM整体不溢出不能替代可读列宽和可点击控件验证。
- 公共修复通过构建/针对性测试和监工审查后才能供页面任务消费。集中跨页面联调 WUI-112 在 WUI-111 后执行；最终验收 WUI-113 再核对证据和未验证项。
- 真实宿主、实例、ACL运行、VPN/公网及真实OIDC服务仍需相应隔离环境。空成员、mock 或生产构建不证明这些能力。

执行与验证结果见 [WUI-103 交付记录](shared/foundation.md)。
