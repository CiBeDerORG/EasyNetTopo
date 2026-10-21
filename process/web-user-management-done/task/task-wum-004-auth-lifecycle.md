---
id: WUM-004
feature: web-user-management
summary: ../README.md
plans: [PLAN-WUM-004, PLAN-WUM-006, PLAN-WUM-008]
status: done
owner: /root/wum004
depends_on: ["WUM-002","WUM-003"]
write_scope: ["easytier-web/src/restful/users.rs","easytier-web/src/restful/auth.rs","easytier-web/src/restful/oidc.rs"]
requirements: ["UM-01","UM-04","UM-05","UM-06","UM-07","UM-08"]
acceptance: ["UA-01","UA-03","UA-04","UA-05","UA-06","UA-09","UA-10"]
updated_at: "2026-10-21"
---

# WUM-004：认证与会话生命周期

拆分后对应计划：[PLAN-WUM-004](../plan/plan-wum-004-account-contract.md)；[PLAN-WUM-006](../plan/plan-wum-006-storage-contract.md)；[PLAN-WUM-008](../plan/plan-wum-008-deployment-compatibility.md)。本任务保留原编号、状态和历史交付记录；维护入口为[需求/计划汇总](../README.md)。

需求：[Web 用户管理](../require/README.md)。计划：[PLAN-WUM](../plan/README.md)。[返回看板](README.md)。

## 目标与边界

在现有认证入口落实账号访问状态、会话撤销、受限改密及适用 OIDC 行为；提供当前身份与公开注册策略接口。仅控制契约定义的 Web 访问，不把账号状态扩散为设备停机行为。

## 输入与输出契约

输入为 WUM-002 权限修复、WUM-003 存储接口和 WUM-001 契约。输出认证策略、auth 路由及会话验证规则，交付 WUM-006/009/010。与 WUM-005 共同使用存储契约，不依赖对方未实现的业务模块。

## 执行步骤

1. 接收 WUM-002 的 users.rs 交接与 WUM-003 存储接口，按契约列出密码登录、用户重载、OIDC、改密及退出的状态转换。
2. 在既有认证模块落实访问状态、会话有效性及受限改密行为；仅调用约定 DB 接口，不并行修改存储模块或依赖未完成的账号服务。
3. 实现当前身份和公开注册策略处理，保留旧入口兼容性；将需要的全局路由/Session 接线说明交给 WUM-006。
4. 使用真实数据库和已注册认证测试验证多会话与各适用身份路径，记录运行结果、所支持模式及全局接线后的复验项。

## 实施与验收

- [x] 密码登录、用户重载、适用 OIDC 登录遵守访问状态；恢复账号不复活旧会话。
- [x] 密码与会话版本一致参与认证有效性；首次改密会话只能访问必要身份/改密/退出接口。
- [x] 当前身份包含必要能力，注册策略不泄露秘密；网页注册与设备/OIDC 自动建号分别处理。
- [x] 启用 OIDC 的范围落实稳定身份与同名冲突策略；旧数据不被猜测为本地账号。
- [x] 使用真实数据库验证多会话、停用/恢复、改密、角色撤销与已支持登录模式；保留原有认证兼容性。

## 交付清单

- 认证模块实现及状态转换说明，包含旧会话失效、恢复与受限改密的可观察行为。
- 当前身份/注册策略接口产物、全局接线清单和适用身份模式说明，供 WUM-006/009 消费。
- 真实数据库认证验证记录与剩余复验项，供 WUM-010 联调使用。

## 阻塞与交接

当前状态：已处理（`done`）。前置依赖、实现、验收及下游交接均已完成，无本轮遗留阻塞。最终验证范围与未验证项目见[运维验收记录](../plan/operations.md)。

以下保留原执行阶段的依赖与写入规则，不表示当前仍待开发：

等待 WUM-002 和 WUM-003 完成。users.rs 从 WUM-002 串行接管。可与 WUM-005 和前端任务并行；restful/mod.rs 的全局接线交由 WUM-006，若测试需要新共享接缝先协调登记，不能提交未编译的模块。

## 执行记录（历史顺序）

以下记录保留各阶段当时的状态；当前结论为已处理，以文件顶部 `status: done` 和最终验收记录为准。

- 2026-10-21：仅完成任务规划，尚未认领或执行；未运行本任务的实现测试。

- 2026-10-21：002/003实际验收后认领，users.rs已串行移交；DB完整access保存需锁事务，用户选择A保留默认密码登录并返回仅本人警告。

- 2026-10-21：User新增认证元数据需要串行适配 `restful/network.rs` 唯一测试构造点；该文件由协调者独占，只将fixture改用User::from_record，不改变网络业务逻辑。

- 2026-10-21：完成密码哈希+auth_version会话摘要、登录/重载停用与临时期限校验、受限改密、稳定OIDC绑定及同名拒绝、me/policy、仅本人默认密码警告。认证更新在DB写锁事务内重读，不覆盖并发撤销。
- 验证：`cargo test --locked -p easytier-web auth_lifecycle` 5/5通过，含真实SQLite与Cookie会话router（独立撤销/多会话/恢复不复活/受限改密/policy/历史默认密码提示）、OIDC绑定和禁用；rustfmt与diff检查通过。完整真实IdP HTTP尚未运行，留010。
- 006接线：将 `auth::require_password_change` 放在合并browser API router中且AuthManagerLayer内，覆盖工作区/RPC/config解析生成；保持内部token路由独立。auth::router已注册me/policy，安全错误helper可复用。

- 2026-10-21：WUM-010前置复核发现OIDC错误Debug可包含完整token响应体及外部回调文本，重新打开；004独占oidc.rs最小脱敏修复，010补真实失败响应日志回归。

- 2026-10-21：004已将OIDC错误日志改为固定阶段文本；010增量embed构建通过，并通过真实IdP畸形200/400 token响应及callback error_description虚构marker回归，日志与callback响应均无marker。该缺陷关闭，浏览器联调其他问题仍由010跟踪。
