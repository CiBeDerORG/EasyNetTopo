---
id: PLAN-WUM-007
feature: web-user-management
status: done
summary: ../README.md
plan_index: README.md
requirement: ../require/README.md
task_board: ../task/README.md
baseline: feat/web-user-management@1e92296b17fc2e85dcd4521b6012232e11abed95
updated_at: 2026-10-21
---

# PLAN-WUM-007：用户管理组件与客户端契约

本文件拆自已交付的 PLAN-WUM，保留原节号及当时的计划/契约记录；不是新增任务或重新执行授权。计划细项编号与任务编号不一一对应。总体进度、代码基线及已知问题见[需求/计划汇总](../README.md)，执行状态见[任务看板](../task/README.md)。

- 关联任务：WUM-007～009。
- 现状依据：根汇总的 WUM-EV-002、WUM-EV-003；对应实现已在交付提交，不能将历史“待实施/待验证”措辞当作当前缺口。
- 验证依据：[运维验收记录](operations.md)及任务明细；本次只拆文档，未运行构建、测试或 CI。

### 8.4 UI 边界

WUM-008 输出 `components/users/UserManagement.vue`（及其内部组件），结构化 props 使用[HTTP 契约](plan-wum-005-http-contract.md)中 DTO 的等价本地类型，不 import 尚未交付客户端。props：`currentUser`, `users:Page<UserSummary>`, `events:Page<UserEvent>`, `selectedUser:UserSummary|null`, `loading:boolean`, `busy:boolean`, `stale:boolean`, `error:{code:string,message:string}|null`, `credential:CredentialDelivery|null`。events：`query(UserListFilter)`、`select(id)`、`create({username})`、`set-access({id,web_login_enabled})`、`set-role({id,role})`、`reset-password(id)`、`revoke-sessions(id)`、`query-events({target_id?,page,page_size})`、`dismiss-credential()`、`retry()`。

WUM-009 唯一持有 HTTP 状态和请求序号，忽略旧查询响应；组件维持非敏感表单草稿，busy 禁止重复提交。成功之后重新读取列表/身份，失败不乐观改变账号。credential 默认遮蔽，关闭/导航/退出立即清空，不存缓存和 toast。组件负责目标/影响确认、键盘焦点、空/错误/过期反馈和窄屏；009 负责 401 退出、403 明确反馈并刷新能力、网络失败保留草稿。WUM-007 提供 `UserManagementClient`（复用 ApiClient 的 axios 实例或等价注入），方法与服务同名但省略 actor_id，另有 `getMe/getPolicy`，错误对象至少有 `status:number|null`、`code:string`、`message:string`。007 删除完整 Axios 响应日志。008 统一提供 `user_management.*` 双语键及强制改密/注册关闭文案，009 向其请求缺失键。
