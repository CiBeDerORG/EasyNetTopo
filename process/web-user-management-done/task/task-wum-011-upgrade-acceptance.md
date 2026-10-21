---
id: WUM-011
feature: web-user-management
summary: ../README.md
plans: [PLAN-WUM-003, PLAN-WUM-008]
status: done
owner: /root/wum011
depends_on: ["WUM-010"]
write_scope: ["process/web-user-management-done/plan/operations.md","easytier-web/src/user_management/upgrade_tests.rs","easytier-web/src/user_management/mod.rs (test module registration only)","easytier-web/src/restful/auth.rs (tests only, /root/wum004)"]
requirements: ["UM-04","UM-05","UM-06","UM-07","UM-09","UM-10"]
acceptance: ["UA-08","UA-12","UA-13","UA-14"]
updated_at: "2026-10-21"
---

# WUM-011：旧库升级与交付验收

拆分后对应计划：[PLAN-WUM-003](../plan/plan-wum-003-verification-and-change.md)；[PLAN-WUM-008](../plan/plan-wum-008-deployment-compatibility.md)。本任务保留原编号、状态和历史交付记录；维护入口为[需求/计划汇总](../README.md)。

需求：[Web 用户管理](../require/README.md)。计划：[PLAN-WUM](../plan/README.md)。[返回看板](README.md)。

## 目标与边界

确认已有部署可升级、初始化/恢复可用，形成运维说明与最终验收结果。路径为建议新增；本任务的验收不自动执行生产部署。

## 输入与输出契约

输入 WUM-010 的联调结果、WUM-003 迁移、WUM-005 管理员恢复方式和部署契约。输出 docs/operations 下的运维说明、升级/回退边界与本任务中的真实验证证据。

## 执行步骤

1. 使用隔离测试环境准备旧版本数据库样本，记录升级前的账号 ID、组关系、凭据可用性和资源归属，并保存可恢复副本。
2. 运行实际迁移和新版本登录/账号操作，对比升级前后的关键数据；新增测试时先完成其模块注册范围的串行交接。
3. 验证管理员初始化、遗失后的恢复以及目标部署的身份模式；在测试副本上检查备份恢复与降级后的访问控制边界。
4. 汇总 WUM-010 的运行时结果与本项升级证据，编写运维说明、已知限制和最终验收结论，交给协调者更新计划。

## 实施与验收

- [x] 用包含旧用户、组、设备/网络归属的测试库升级，验证 ID/凭据兼容、未知字段、账号访问与原资源保留。
- [x] 验证已有管理员、首次初始化、管理员遗失后的恢复；不使普通公开注册用户获得管理权限。
- [x] 明确备份/恢复及旧二进制不能强制新访问状态的回退限制；验证支持部署的 Cookie/身份行为。
- [x] 若新增升级测试，先由协调者移交必要模块注册范围，使其进入实际 runner；不得把未编译测试作为验收证据。
- [x] 汇总所有 UA 场景结果、适用部署和限制；说明尚未支持的范围，链接实际验证步骤。由协调者据证据更新任务及计划状态。

## 交付清单

- 升级前后数据与行为对照、管理员恢复及备份/回退验证记录，不包含真实凭据。
- `process/web-user-management-done/plan/operations.md` 中可复现的升级、初始化、恢复操作和适用部署说明。
- 覆盖全部关联验收的最终结论与限制；未解决问题明确回派，不以运维文档替代实现修复。

## 阻塞与交接

当前状态：已处理（`done`）。前置依赖、实现、验收及下游交接均已完成，无本轮遗留阻塞。最终验证范围与未验证项目见[运维验收记录](../plan/operations.md)。

以下保留原执行阶段的依赖与写入规则，不表示当前仍待开发：

等待 WUM-010。业务缺陷回派原任务；涉及 user_management/ 的注册或修复与 WUM-005 串行交接。退出条件为目标部署升级/恢复验证完成、文档可复现、关联验收无未处理阻塞。

## 执行记录（历史顺序）

以下记录保留各阶段当时的状态；当前结论为已处理，以文件顶部 `status: done` 和最终验收记录为准。

- 2026-10-21：仅完成任务规划，尚未认领或执行；未运行本任务的实现测试。

- 2026-10-21：010通过后认领；由005串行移交mod.rs仅测试模块注册行。使用隔离旧库、虚构凭据、真实CLI进程验证升级/恢复/备份，禁止实际生产或VPN操作。首次初始化文档须明确旧迁移预置admin/user仍存在，显式恢复admin不会清理user，自选新管理员也不会替换它们。

- 2026-10-21：`cargo test --locked -p easytier-web user_management_upgrade_`实际3/3通过（2组新增进程/HTTP验收+1项既有迁移回归），/tmp/wum011-upgrade.log。真实旧库为截至编号000010的8份迁移，编号不是数量。追加004独占auth.rs测试区验证无管理员时普通注册仍不提权，不扩大生产模块可见性。

- 2026-10-21：最终显式入口`cargo test --locked -p easytier-web user_management_upgrade_ -- --include-ignored`实际3/3通过、0 ignored，日志/tmp/wum011-upgrade.log。两项进程测试需要已构建的本次Web二进制，普通Cargo/nextest归档CI默认忽略，不能把默认跳过算验收。
- 2026-10-21：004补充`auth_lifecycle_registration_without_admin_does_not_promote`实际1/1通过（/tmp/wum004-registration-no-admin.log），验证零管理员成员时注册仍普通用户、无users.manage且可认证。
- 2026-10-21：交付[升级与运维说明](../plan/operations.md)，UA-01至14证据及真实/mock/未测范围齐全，011完成。未运行旧二进制、HTTPS代理部署或真实VPN数据面；没有生产变更或剩余实施阻塞。
