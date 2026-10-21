---
id: PLAN-WUM-005
feature: web-user-management
status: done
summary: ../README.md
plan_index: README.md
requirement: ../require/README.md
task_board: ../task/README.md
baseline: feat/web-user-management@1e92296b17fc2e85dcd4521b6012232e11abed95
updated_at: 2026-10-21
---

# PLAN-WUM-005：HTTP、DTO、分页与错误契约

本文件拆自已交付的 PLAN-WUM，保留原节号及当时的计划/契约记录；不是新增任务或重新执行授权。计划细项编号与任务编号不一一对应。总体进度、代码基线及已知问题见[需求/计划汇总](../README.md)，执行状态见[任务看板](../task/README.md)。

- 关联任务：WUM-006～009。
- 现状依据：根汇总的 WUM-EV-001、WUM-EV-003；对应实现已在交付提交，不能将历史“待实施/待验证”措辞当作当前缺口。
- 验证依据：[运维验收记录](operations.md)及任务明细；本次只拆文档，未运行构建、测试或 CI。

### 8.2 HTTP 与 DTO（WUM-006/007/008/009）

所有路径基于 `/api/v1`，JSON 使用 snake_case；ID 为正 i32，日期为 RFC3339 UTC 字符串或 null。用户摘要 `UserSummary`：

```ts
type UserSummary = {
  id: number; username: string; role: 'admin' | 'user';
  web_login_enabled: boolean; source: 'unknown' | 'local' | 'oidc' | 'device';
  local_password_enabled: boolean | null; must_change_password: boolean;
  created_at: string | null; last_login_at: string | null;
};
type Page<T> = { items: T[]; total: number; page: number; page_size: number };
type CurrentUser = UserSummary & { capabilities: string[]; default_password_warning: boolean };
type CredentialDelivery = { user: UserSummary; temporary_password: string; expires_at: string };
type UserEvent = { id: number; actor_id: number | null; target_id: number | null;
  action: string; occurred_at: string; result: 'success' | 'denied' | 'failure' };
```

`capabilities` 当前只暴露 `users.manage`（满足访问/权限且非受限时），不得暴露设备 token、密码哈希、auth_version 或 OIDC subject。审计 action 固定为 `create/access/role/password_reset/revoke_sessions/bootstrap/recover/identity_bind`，不含请求体或凭据。未知时间返回 null，UI 显示未知。

| 方法路径 | 请求 | 成功响应 |
| --- | --- | --- |
| GET `/auth/me` | 无 | 200 CurrentUser |
| GET `/auth/policy` | 无 | 200 `{registration_enabled:boolean,oidc_enabled:boolean}` |
| GET `/admin/users` | `search?`, `role?`, `web_login_enabled?`, `page=1`, `page_size=20` | 200 Page<UserSummary> |
| GET `/admin/users/{id}` | 无 | 200 UserSummary |
| POST `/admin/users` | `{username:string}`，默认普通用户 | 201 CredentialDelivery |
| PUT `/admin/users/{id}/access` | `{web_login_enabled:boolean}` | 200 UserSummary |
| PUT `/admin/users/{id}/role` | `{role:'admin'|'user'}` | 200 UserSummary |
| POST `/admin/users/{id}/password-reset` | `{}` | 200 CredentialDelivery |
| POST `/admin/users/{id}/revoke-sessions` | `{}` | 200 UserSummary |
| GET `/admin/user-events` | `target_id?`, `page=1`, `page_size=20` | 200 Page<UserEvent> |

新管理员创建用户名采用 `^[A-Za-z0-9_.@-]{3,64}$`、大小写敏感、不自动 trim；旧用户名保持可读可登录，不据此重新校验历史记录。列表 search 为用户名大小写不敏感字面子串，SQL LIKE 元字符转义；空 search 等于无筛选。角色/状态条件与 search AND 组合，用户按 id ASC，审计按 id DESC；page>=1、1<=page_size<=100，非法值 400，超出末页为空 items 且 total 保留。稳定排序不保证跨并发修改的快照游标。

错误保持现有 `{message:string,code?:string}` 包络，新接口必须填写稳定 code：400 `invalid_request`；401 `unauthenticated`（包括停用/失效会话）；403 `forbidden`、`password_change_required`、`self_action_forbidden`、`local_password_unavailable`、`csrf_rejected`；404 `user_not_found`；409 `username_conflict`、`last_admin`、`identity_conflict`；500 `internal_error`。登录统一 401 `invalid_credentials`，不泄露是否存在/停用。临时密码过期归此登录失败。HTTP 403 不等于未登录；无响应的网络错误单独表示。

角色/访问 PUT 为幂等设置，重复设置仍授权校验但不撤销会话/重复成功审计。创建/重置/撤销不自动重试；创建唯一约束冲突 409，重置成功但响应丢失需用户明确再次重置，旧临时凭据随即失效。不引入持久缓存凭据以实现幂等重放。
