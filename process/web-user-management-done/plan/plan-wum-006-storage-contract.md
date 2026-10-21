---
id: PLAN-WUM-006
feature: web-user-management
status: done
summary: ../README.md
plan_index: README.md
requirement: ../require/README.md
task_board: ../task/README.md
baseline: feat/web-user-management@1e92296b17fc2e85dcd4521b6012232e11abed95
updated_at: 2026-10-21
---

# PLAN-WUM-006：存储、事务与服务契约

本文件拆自已交付的 PLAN-WUM，保留原节号及当时的计划/契约记录；不是新增任务或重新执行授权。计划细项编号与任务编号不一一对应。总体进度、代码基线及已知问题见[需求/计划汇总](../README.md)，执行状态见[任务看板](../task/README.md)。

- 关联任务：WUM-003～006。
- 现状依据：根汇总的 WUM-EV-001；对应实现已在交付提交，不能将历史“待实施/待验证”措辞当作当前缺口。
- 验证依据：[运维验收记录](operations.md)及任务明细；本次只拆文档，未运行构建、测试或 CI。

### 8.3 存储与服务边界（WUM-003/004/005）

采用新增 `user_access` 侧表（user_id 主键且外键 users.id），避免直接修改现有标为生成的 users 实体及改变设备端 users::Model 构造路径。字段为 `web_login_enabled`（默认 true）、`auth_version` i64（默认 0）、`must_change_password`（默认 false）、`source`（默认 unknown）、`local_password_enabled` nullable bool、`created_at`/`last_login_at`/`temporary_password_expires_at` nullable UTC 时间。迁移为已有用户建立 unknown 行；后续无侧表的旧自动建号路径读取为同样默认值，写入时幂等补行，不推断来源。

新增 `user_identities`：id、user_id 外键、issuer、subject，唯一 `(issuer,subject)`；新增 `user_events`：[HTTP 契约](plan-wum-005-http-contract.md)中的 UserEvent 字段。新增单行写锁表 `user_management_lock`；事务首先 UPDATE 该锁行后再读取 actor/target/管理员数量，保证 SQLite 写入序列化，不用进程 mutex 代替数据库一致性。迁移添加 users.manage 并幂等关联 admins。历史默认凭据问题见[部署与历史账号兼容](plan-wum-008-deployment-compatibility.md)。

WUM-003 在 `db/user_management.rs` 暴露以下共享 Rust 契约，SeaORM 类型/连接复用已有依赖，不依赖 restful 或业务模块；允许泛型 `C: ConnectionTrait` 供事务和普通连接复用：

```rust
// UserAccess 的字段与侧表一致； UserRecord { user: users::Model, access: UserAccess,
// role: String } 仅供服务端使用，禁止直接序列化到 HTTP。
async fn get_user_record<C: ConnectionTrait>(conn: &C, user_id: i32)
    -> Result<Option<UserRecord>, DbErr>;
async fn ensure_user_access<C: ConnectionTrait>(conn: &C, user_id: i32)
    -> Result<UserAccess, DbErr>;
async fn save_user_access<C: ConnectionTrait>(conn: &C, access: &UserAccess)
    -> Result<(), DbErr>;
async fn set_user_role<C: ConnectionTrait>(conn: &C, user_id: i32, role: &str)
    -> Result<(), DbErr>; // 幂等维护 admins/users 关系，不删除其他既有组
async fn count_enabled_admins<C: ConnectionTrait>(conn: &C) -> Result<u64, DbErr>;
async fn has_manage_permission<C: ConnectionTrait>(conn: &C, user_id: i32)
    -> Result<bool, DbErr>; // 按用户关联的真实 users.manage 权限查询，供事务内重新授权
async fn lock_user_management(txn: &DatabaseTransaction) -> Result<(), DbErr>;
async fn append_user_event<C: ConnectionTrait>(conn: &C, event: NewUserEvent)
    -> Result<(), DbErr>;
async fn find_identity<C: ConnectionTrait>(conn: &C, issuer: &str, subject: &str)
    -> Result<Option<i32>, DbErr>;
async fn bind_identity<C: ConnectionTrait>(conn: &C, user_id: i32, issuer: &str, subject: &str)
    -> Result<(), DbErr>;
```

WUM-003 同时提供 `list_user_records(conn, filter: UserListFilter) -> Result<(Vec<UserRecord>, u64), DbErr>`、`list_user_events(conn, target_id: Option<i32>, page: u64, page_size: u64) -> Result<(Vec<UserEvent>,u64), DbErr>`；UserListFilter 含 search/role/web_login_enabled 可选项及 page/page_size。具体时间类型统一 chrono UTC，存储与 JSON 间集中转换。接口若需调整由 WUM-003 交付前回报协调者并同步本节，004/005 不自行分叉。

WUM-004 自主使用 db 契约实现认证、成功登录时间/凭据能力更新、OIDC 和本人改密事务；WUM-005 自主使用相同 db 契约实现 `UserManagementService::new(Db)`，方法 `list_users(actor_id,filter)`、`get_user(actor_id,target_id)`、`create_user(actor_id,username)`、`set_access(actor_id,target_id,enabled)`、`set_role(actor_id,target_id,role)`、`reset_password(actor_id,target_id)`、`revoke_sessions(actor_id,target_id)`、`list_events(actor_id,target_id,page,page_size)`，返回[HTTP 契约](plan-wum-005-http-contract.md)中的安全 DTO/稳定业务错误；WUM-006 将 HTTP 映射到这些方法。两者不互相调用未落地模块。

每个管理写动作事务顺序：取得 DB 写锁→重新授权→读取目标及不变量→更新账号/版本/关系→成功审计→commit；任一步失败全部回滚。通过身份验证但被业务规则拒绝的管理写动作在回滚后单独记录 denied（actor/可确认 target/action），基础设施错误记录 failure；审计写失败返回 internal_error，不能宣称动作成功。路由层匿名、未获管理权限、受限会话及 CSRF 拒绝不执行账号动作，不写账号业务审计；服务层已进入管理动作后违反目标/自身/最后管理员等业务规则的拒绝记录 denied。只读操作不写管理审计。这不是全量 HTTP 安全事件日志，不宣称每个被拒请求均有审计。
