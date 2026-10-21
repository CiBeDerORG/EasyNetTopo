---
id: PLAN-WUM-004
feature: web-user-management
status: done
summary: ../README.md
plan_index: README.md
requirement: ../require/README.md
task_board: ../task/README.md
baseline: feat/web-user-management@1e92296b17fc2e85dcd4521b6012232e11abed95
updated_at: 2026-10-21
---

# PLAN-WUM-004：账号、角色、会话与凭据契约

本文件拆自已交付的 PLAN-WUM，保留原节号及当时的计划/契约记录；不是新增任务或重新执行授权。计划细项编号与任务编号不一一对应。总体进度、代码基线及已知问题见[需求/计划汇总](../README.md)，执行状态见[任务看板](../task/README.md)。

- 关联任务：WUM-001、WUM-003～006。
- 现状依据：根汇总的 WUM-EV-001、WUM-EV-002；对应实现已在交付提交，不能将历史“待实施/待验证”措辞当作当前缺口。
- 验证依据：[运维验收记录](operations.md)及任务明细；本次只拆文档，未运行构建、测试或 CI。

### 8.1 角色、状态与凭据

- 角色 `admin | user` 从 `admins` 组成员关系推导；新增 `users.manage` 权限仅授予 `admins`。有效管理员为控制台访问启用且属于 `admins` 的账号；受限首次改密不移除其角色。服务每次写事务内重新读取 actor 的访问状态、权限及受限状态，不相信 HTTP 输入的能力。
- 禁止本人停用、本人降权和本人管理员密码重置，返回 `self_action_forbidden`；本人改密走既有接口。允许本人撤销全部会话，成功后退出。任何降低有效管理员数量的动作必须在同一写事务锁定、计数、更新，不允许零个有效管理员。
- `web_login_enabled=false` 只拒绝控制台登录和后续认证请求；不删除资源，不触发设备断线、Core 重启或网络冻结。恢复仅启用后续登录。访问状态或角色实际改变、密码重置、本人改密及主动撤销均增加 `auth_version`；幂等重复设置不增加版本。会话认证摘要同时覆盖当前密码哈希和版本；上线时旧摘要会话失效一次。
- 新建本地账号和重置由服务端生成 32 个十六进制字符（至少 128 位随机性）的临时密码；仅创建/重置成功响应返回一次，24 小时过期，`must_change_password=true`。数据库只保存现有 password_auth 哈希，输入仍为临时密码的 MD5 小写十六进制摘要；前端旧登录/改密摘要格式不改。新密码不可与临时密码相同。MD5 不提供传输保护。
- 受限会话仅可 GET `auth/me`、旧 `auth/check_login_status`、PUT `auth/password`、退出；所有工作区及管理接口拒绝为 403 `password_change_required`。临时凭据过期后登录和受限会话均拒绝；重新重置可恢复。本人改密成功清除期限/受限标志并退出，必须重新登录。保留旧摘要输入兼容，因此服务端只能验证摘要为 32 位十六进制；UI 新密码要求 12–128 字符，不虚称服务端能由 MD5 验证原始密码强度。
- 历史账号 `source=unknown`、`local_password_enabled=null`：保留原密码登录和成功登录后的本人改密，不向管理员开放重置。密码验证成功可确认本地凭据能力为 true，但 source 仍是 unknown。新注册/管理员创建为 local/true，新增 OIDC 为 oidc/false；设备自动建号为 device/false；历史来源不可由哈希或用户名推断。
- OIDC 使用验证过的 `(issuer, subject)` 唯一绑定；绑定后以 user_id 查找并检查停用/受限。无绑定且用户名未占用可自动创建普通用户；同名已存在但无绑定返回 `identity_conflict`，绝不自动合并，即使名字与已有管理员一致。旧 OIDC 部署升级需运维离线核实映射再绑定，绑定不改变角色、密码或访问状态；禁止仅凭 username 自动迁移。外部 webhook/设备映射仍用原资源归属，不覆盖账号状态。

- 管理写操作不新增二次因素或重新输入管理员密码：每次使用完整非受限会话、事务内权限重查、目标确认和同源保护；这是受信运维角色下的 v1 决定，不声称具备再次认证能力。
