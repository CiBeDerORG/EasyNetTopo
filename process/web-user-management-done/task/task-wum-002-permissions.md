---
id: WUM-002
feature: web-user-management
summary: ../README.md
plans: [PLAN-WUM-004, PLAN-WUM-006]
status: done
owner: /root/wum002
depends_on: []
write_scope: ["easytier-web/src/restful/users.rs"]
requirements: ["UM-01","UM-06"]
acceptance: ["UA-01"]
updated_at: "2026-10-21"
---

# WUM-002：按用户查询权限

拆分后对应计划：[PLAN-WUM-004](../plan/plan-wum-004-account-contract.md)；[PLAN-WUM-006](../plan/plan-wum-006-storage-contract.md)。本任务保留原编号、状态和历史交付记录；维护入口为[需求/计划汇总](../README.md)。

需求：[Web 用户管理](../require/README.md)。计划：[PLAN-WUM](../plan/README.md)。[返回看板](README.md)。

## 目标与边界

修复现有 get_group_permissions 未按目标用户过滤的问题，并在同一文件内加入针对性回归验证；不提前实现新管理 API、状态字段或角色体系。

## 输入与输出契约

使用现有用户、组和权限表即可开展。输出为只返回目标用户权限的现有认证后端实现，以及隔离回归证据；现有函数签名和业务路由保持兼容，交付 WUM-004。

## 执行步骤

1. 读取现有查询、关系定义与测试数据库初始化方式，建立普通用户、管理员、无组用户和多组用户的权限预期表。
2. 在 users.rs 内构造能暴露跨用户权限混合的回归用例，记录修复前的失败行为；用例使用真实测试数据库。
3. 以传入用户的稳定 ID 限定查询，保留现有签名与权限集合语义，运行已注册的针对性测试。
4. 检查变更仅涉及权限作用域与其验证，记录测试入口和未覆盖范围后交接 users.rs。

## 实施与验收

- [x] 验证普通用户、管理员、无组用户以及多个组关系的查询结果；普通用户不能获得其他人的权限。
- [x] 使用真实测试数据库与彼此不同的权限关系，确保用例能发现原来的缺少过滤条件，而非仅复制实现。
- [x] 运行 easytier-web 包内实际注册的针对性测试，记录命令与命中用例；不宣称当前业务路由已有可利用的越权链路。

## 交付清单

- users.rs 中的查询修复及同文件回归用例，保持现有调用契约。
- 权限预期表、实际测试命令/命中用例/结果，以及交给 WUM-004 的文件接管说明。

## 阻塞与交接

当前状态：已处理（`done`）。前置依赖、实现、验收及下游交接均已完成，无本轮遗留阻塞。最终验证范围与未验证项目见[运维验收记录](../plan/operations.md)。

以下保留原执行阶段的依赖与写入规则，不表示当前仍待开发：

无，状态可认领但尚未开发。与 WUM-001 可并行。WUM-004 必须在本项完成后接管 users.rs；两者不可同时编辑。

## 执行记录（历史顺序）

以下记录保留各阶段当时的状态；当前结论为已处理，以文件顶部 `status: done` 和最终验收记录为准。

- 2026-10-21：仅完成任务规划，尚未认领或执行；未运行本任务的实现测试。

- 2026-10-21：由 `/root/wum002` 认领；独占原 write_scope，协调者维护任务状态和提交。基线为核实远端文档分支 `f48602e2674173be55871724f201da5982d62951`，功能分支 `feat/web-user-management`。

- 2026-10-21：按稳定用户ID过滤，select_only+InnerJoin使无组/空权限组返回空集合；保持权限去重和签名。真实SQLite用例覆盖普通/管理员/无组/空权限组/多组共5场景。
- 验证：隔离 worktree `/workspace/wum002-check` 执行 `cargo test --locked -p easytier-web group_permissions_are_scoped_to_user`，最终1 passed/0 failed/272 filtered；与主工作区users.rs逐字cmp一致。原查询红测为NULL解码错误；另仅移除ID过滤反证明确失败：普通用户错误获得 wum.manage/wum.audit/devices/sessions，恢复修复后通过。rustfmt及diff检查通过。
- 环境：Rust1.95/protoc35.1/mold，source `/workspace/.wum-tools/env`，`BINDGEN_EXTRA_CLANG_ARGS=-isystem /usr/lib/gcc/x86_64-linux-gnu/14/include`。日志 `/tmp/wum002-isolated-{red,scope-red,final-green}.log`。初次缺工具及并行模块暂态错误已解决。users.rs移交004；本结果不声称管理API授权已实现。
