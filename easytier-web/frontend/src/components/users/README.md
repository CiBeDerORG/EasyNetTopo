# 用户管理组件交接

`UserManagement.vue` 依据 PLAN-WUM v1，通过 `types.ts` 中的结构化 props 和事件工作，不发 HTTP 请求。父页面负责唯一的请求状态、响应序号和账号数据；这不提供服务端授权。

- 输入：`currentUser`、`users`、`events`、`selectedUser`、`loading`、`busy`、`stale`、`error`、`credential`，字段与计划第8节一致。
- 输出：`query`、`select`、`create`、`set-access`、`set-role`、`reset-password`、`revoke-sessions`、`query-events`、`dismiss-credential`、`retry`。类型签名见组件 `defineEmits`。
- 变更发起前清空上次 `error`，整个操作期间设置 `busy=true`，最后设置 `busy=false`。成功后重读数据；失败提供稳定 `error.code`，组件保留非敏感草稿，不乐观改动账号状态。
- `select` 同时发出目标账号第一页的 `query-events`。列表和审计分别分页；父页面须丢弃迟到响应，避免旧详情或查询覆盖新选择。
- 创建和重置成功后传入 `credential`；仅对话框显示且默认遮蔽。收到 `dismiss-credential` 或导航/退出时立即将父级凭据设为 null；不得写入 toast、URL、存储或日志。
- 组件对本人停用/降权/管理员重置和未知本地凭据做提示；服务器仍需重新授权及执行最后管理员保护。

全部文案在 `user_management.*`。WUM-009 还可使用 `registration_disabled`、`password_change_required`、`password_policy`、`password_changed`、`session_expired`、`policy_failed` 和 `default_password_warning`（提示立即改密但不锁定历史账号）。

验证入口（frontend 目录）：`node --test tests/user-management-components.test.mjs`。测试用 Vite 挂载真实 Vue/PrimeVue 组件，以虚构 props/events 驱动 Chromium；`WEB_TEST_CHROMIUM` 可指定浏览器，默认 `/usr/bin/chromium`。这类测试不证明真实数据库、HTTP 授权或完整业务通过；真实联调归 WUM-010。
