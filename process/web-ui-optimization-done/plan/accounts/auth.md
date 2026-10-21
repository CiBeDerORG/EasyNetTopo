# WUI-104：认证页面保留与针对性回归

负责人 `iteration_exec2`；分支 `feat/ecosystem-web-ui`，采用 [公共契约 v1](../contract.md) 与 [WUI-102 真实页面取证](../../require/evidence/README.md)。源码基线 `1e92296b17fc2e85dcd4521b6012232e11abed95` 的 Login/ChangePassword 及密码能力修复保留。

## 实施结论

WUI-102 已覆盖登录、注册、本人改密页面的中英文/主题/宽度前态，未记录这些页面的确定缺陷。其真实错误登录观察已确认用户名保留和错误反馈；本任务据此不为统一风格重写已有组件。

`Login.vue`、`ChangePassword.vue` **没有修改**。保留：

- 注册策略失败/关闭时不开放注册表单；登录、注册、OIDC原路由和结果处理。
- 错误与用户草稿、忙碌禁止重复提交；强制改密成功回登录。
- `localPasswordEnabled === true` 才允许本地改密；OIDC-only/未知能力显示解释，source 不替代能力判断。
- 服务端 `local_password_unavailable` 后清除密码并停止呈现可提交表单；原密码长度 12～128 和 API 兼容输入不变。
- 不新增身份客户端、轮询、重试队列或订阅，不改变 config-server 行为。

新增 `frontend/tests/web-ui-auth.test.mjs` 补两项既有覆盖缺口：策略读取失败后仅显式键盘重试才显示注册表单；强制改密长度边界、键盘提交、请求挂起时防重复，以及服务端能力撤销后清除表单。生产页面通过 WUI-103 的独立 preview/browser 支持启动，HTTP 为 mock。

## 实际验证（2026-10-21）

工作目录 `/workspace/EasyTier-iteration/easytier-web/frontend`；动态 loopback 端口，隔离浏览器 context，测试结束关闭；未修改 navigation 测试文件。

| 命令 | 实际结果 |
| --- | --- |
| `node --test --test-name-pattern='registration policy\|restricted login\|legacy default\|login distinguishes\|login sends\|local-password\|OIDC' tests/user-management-navigation.test.mjs` | **10/10 通过**；仅选中认证相关用例，不能称全文件/全链路通过 |
| `node --test tests/web-ui-auth.test.mjs` | **2/2 通过**，0 失败/跳过 |
| 本任务 `git diff --check` 与文档相对链接检查 | 通过 |

上述 10 项覆盖注册禁用、强制改密成功退出、历史默认密码非阻断提示、登录网络错误/错误凭据及用户名保留、临时账户直接强改、cn/en OIDC-only说明/菜单、未知本地能力、OIDC来源具备本地密码及服务端本地能力拒绝。

使用此前 WUI-105 已完成类型检查/生产构建的产物；本任务未改应用源码，因此没有重复构建或将历史构建声称本任务新运行。新增测试真实执行且非零测试；与 WUI-102 的真实 API 取证分别记录，不能合并说成真实 OIDC、数据库或权限联调已通过。

## 交接与限制

WUI-111 继续保持组件输入、路由、能力与会话归属；本任务无需公共文案/样式变更。真实 OIDC 提供方、真实身份绑定和后端授权由 WUI-112 按隔离环境验证。测试中的模拟策略、身份及响应只证明前端反馈；不操作真实账户、VPN或网络设置。

任务状态、看板及提交由总控更新；没有新业务功能或已知问题被静默关闭。
