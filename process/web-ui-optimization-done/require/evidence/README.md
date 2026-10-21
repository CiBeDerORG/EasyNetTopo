# WUI-102：真实页面前态与问题证据

采集日期：2026-10-21；交接版本 `WUI-EVIDENCE-1`；输入为 [WUI-BASELINE-1](../baseline.md)。关联：[任务](../../task/task-wui-102-page-evidence.md)、[需求验收](../README.md)。

## 实际环境及范围

本次已进入用户授权的页面取证任务，不是为文档排版运行测试。使用 `feat/ecosystem-web-ui` 的源码基线 `1e92296b17fc2e85dcd4521b6012232e11abed95`。执行 agent 2 独占准备依赖、共享库构建和服务，执行 agent 3 不另写 dist，也没有运行完整测试套件。

采集时 Vite 位于 `http://127.0.0.1:43021`，代理到隔离 Web API `http://127.0.0.1:38237`；SQLite 为 `/tmp/eco102-isolated-sPK10H/isolated.sqlite`。服务和数据库由 ECO-102 管理，服务关闭后地址不再作为长期可访问入口。服务建立与二进制来源见[同环境记录](../../../easytier-ecosystem-integration-done/plan/design/baseline-evidence/README.md)。所有会话使用独立浏览器 context 和虚构账户；没有真实设备、Core/VPN进程或生产网络操作。

本次创建一条虚构空网络 `fictional-wui-long-network-name-for-responsive-layout`（ID `b4efa55f-d5df-453d-bf1a-c8674c27fa55`）用于长名称和空成员页，不含运行实例。未更改共用测试管理员密码或权限。没有将临时密码、Cookie、密钥、完整配置或认证响应写入证据。

## 覆盖与观察结果

[page-matrix.json](page-matrix.json) 保存 **96项**实际采集记录：登录、关闭注册、本人改密、用户列表、工作台、空设备列表、网络列表、空成员详情共8页面，分别覆盖 cn/en、light/dark、360/768/1440宽度。结果为 **整页横向溢出0、pageerror 0**。这是这些空/少量数据页面的观测，不能外推到全部数据或功能；局部表格滚动不算整页溢出。

16张代表截图为每页的英文浅色1440与中文深色360组合；其余组合保留尺寸/错误/标题记录，没有声称全部96项都有截图。截图从实际浏览器渲染取得，矩阵请求连接真实隔离API，没有mock响应。

| 已观察流程 | 方法与实际结果 | 证据 |
| --- | --- | --- |
| 用户创建失败及恢复 | 真实页面，拦截POST并在400ms后abort，未向后端创建账户；只发1次写请求、忙碌按钮禁用、非敏感用户名保留、Escape后焦点回创建按钮 | [交互记录](interaction-observations.json)、[失败截图](user-create-failure-en-light-360.png) |
| 错误登录与键盘提交 | 虚构无效账号在真实API被拒绝；Enter提交、用户名保留；截图前清空密码 | [交互记录](interaction-observations.json)、[失败截图](login-failure-en-light-360.png) |
| 用户列表迟到响应 | 获取真实GET响应后延迟700ms交付，立即离开到改密页；改密表单仍显示，用户列表未恢复 | [交互记录](interaction-observations.json)；延迟注入不是后端自然故障 |
| 移动导航关闭 | 键盘Enter打开、Escape关闭，等待隐藏后再等500ms，连续3次焦点落BODY而非打开按钮 | [独立复核](mobile-focus-observations.json) |
| 设备接入命令复制 | 空设备页复制内容逐字等于页面code内容，协议/地址/端口/用户名结构符合现有生成逻辑 | [布局及复制记录](layout-observations.json)；未执行命令、未连接config-server |

创建流程只验证忙碌期间禁用和单次显式提交；没有强制对禁用按钮注入第二个事件，不能据此宣称覆盖所有网络重放情形。

## 已复现问题与有限改动建议

优先级依据实际键盘/触控影响和可读性，不以任务数量要求每页重写。

| 编号 | 页面、动作与实际结果 | 影响/优先级 | 所有者与建议 |
| --- | --- | --- | --- |
| WUI-OBS-001 | 360宽度用户管理页，键盘打开导航再Escape关闭；3次等待500ms后activeElement均为BODY | 键盘用户失去原位置，需要重新遍历页面；P2 | **WUI-111**：宿主保存导航触发控件引用并在关闭后恢复焦点，兼容导航选择后的内容焦点；不修改PrimeVue共享实现 |
| WUI-OBS-002 | 360中文用户表：用户名单元格宽32.72px；4/5字符账号行高约124px，19字符虚构账号行高292.72px；用户名、角色和表头被挤成竖列 | 长账号难以快速比较，列表高度过大；P2 | **WUI-105**：现有局部滚动容器内设置合理表格/列最小宽度，保留字段与操作、筛选及分页。不要删列或扩大整页宽度 |
| WUI-OBS-003 | 360中文长网络名详情：topbar高147px、菜单按钮实测宽9.41px，名称宽78.27px | 导航触控目标被挤压，顶部信息占用过高；P1 | **WUI-103/111串行**：基础样式保护操作按钮不收缩，标题在受限区域截断并保留完整名称可访问方式。页面正文仍显示完整网络名，不重命名数据 |

尺寸证据：[layout-observations.json](layout-observations.json)。视觉证据：[用户列表](users-cn-dark-360.png)、[长网络名详情](network-detail-cn-dark-360.png)。在英文浅色1440、其他已采页面中没有据此发现必须整体重做布局的问题。

登录、改密、工作台、空设备、网络列表已有基本结构可用；用户创建失败/焦点与迟到列表保护保持不变。WUI-104/106/107/108/109/110 后续可以在本任务覆盖边界内交付针对性回归及保留说明，不能为了完成编号制造无依据改动。

## 未覆盖及交接限制

- 没有真实设备/运行实例，未验证设备详情有数据、实例配置/启停、节点路由/peer/ACL统计和VPN通信；空成员页面不替代这些证据。涉及这些状态的新改动需补独立fixture或隔离设备证据，不能借空页面结果验收。
- 注册服务策略关闭，未运行开放注册验证码流程；未在本任务执行真实OIDC流程、强制改密成功、密码重置或会话撤销。保留已有WUM行为与测试入口，未来修改需相应回归。
- 矩阵主要观察加载后页面与布局，没有测量所有状态的空闲请求数量；现有周期来源以WUI-101为准，ECO-102的空闲观测仅证明其记录范围。
- fault injection范围只有用户创建请求abort和用户列表响应延迟；其余结果不能泛称全故障覆盖。
- 此处截图不是视觉认证，未进行全组件对比度量测、读屏器审计或真实触屏设备测试。桌面Chromium窄viewport不是物理手机。
- 只使用现有接口和本地临时数据，不新增后台机制。共享文件修改必须由总控交接，监工据此审查WUI-103契约与后续联调准入。

## 复现步骤与脚本

可复现脚本保存在 [scripts/](scripts/)，并非只有 `/tmp` 日志。脚本使用 Playwright 与当前项目现有依赖，从仓库根执行；先由环境所有者建立**隔离**Web/SQLite与Vite代理并提供虚构管理员。运行环境变量：`WUI_EVIDENCE_BASE`、`WUI_EVIDENCE_USERNAME`、`WUI_EVIDENCE_PASSWORD`，布局脚本还需 `WUI_EVIDENCE_NETWORK_ID`。不得使用生产地址或账户。脚本不启动VPN或执行接入命令。

```sh
node process/web-ui-optimization-done/require/evidence/scripts/capture.mjs
node process/web-ui-optimization-done/require/evidence/scripts/interactions.mjs
node process/web-ui-optimization-done/require/evidence/scripts/focus.mjs
node process/web-ui-optimization-done/require/evidence/scripts/layout.mjs
```

capture 会在隔离DB创建或复用上述前缀的空网络，并覆盖本目录矩阵与代表截图；其他脚本复用同环境，layout需使用capture输出的networkId。账户秘密只通过环境传入，不存入证据；密码、响应和Cookie不打印。正式再次取证前保存旧证据并记录对应版本，不能把新版输出混称旧基线。

本次实际执行的是这些脚本的 `/tmp/wui102-*.mjs` 初始形式；归档时仅将路径和虚构登录参数改为环境变量并格式化，四份归档脚本已做语法解析，**没有把归档版本未重新运行称为再次通过**。脚本不是生产实现，也不替代后续自动化回归。
