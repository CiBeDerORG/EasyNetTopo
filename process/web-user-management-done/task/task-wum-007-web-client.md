---
id: WUM-007
feature: web-user-management
summary: ../README.md
plans: [PLAN-WUM-005, PLAN-WUM-007]
status: done
owner: /root/wum007
depends_on: ["WUM-001"]
write_scope: ["easytier-web/frontend/src/modules/api.ts","easytier-web/frontend/src/modules/userManagement.ts","easytier-web/frontend/tests/user-management-client.test.mjs"]
requirements: ["UM-01","UM-02","UM-03","UM-04","UM-05","UM-06","UM-07","UM-08","UM-09"]
acceptance: ["UA-02","UA-03","UA-10","UA-11","UA-13"]
updated_at: "2026-10-21"
---

# WUM-007：Web HTTP 客户端与错误语义

拆分后对应计划：[PLAN-WUM-005](../plan/plan-wum-005-http-contract.md)；[PLAN-WUM-007](../plan/plan-wum-007-ui-contract.md)。本任务保留原编号、状态和历史交付记录；维护入口为[需求/计划汇总](../README.md)。

需求：[Web 用户管理](../require/README.md)。计划：[PLAN-WUM](../plan/README.md)。[返回看板](README.md)。

## 目标与边界

实现用户管理 HTTP/DTO、当前身份和注册策略访问；修正相关错误表达与敏感响应日志。新增路径为建议；不改路由或页面，不加入网络 RemoteClient。

## 输入与输出契约

输入为 WUM-001 HTTP 与状态契约。输出类型化请求/响应、安全错误和调用方法供 WUM-009 组合；与 WUM-008 仅共享已定义契约，无需等待其组件文件。

## 执行步骤

1. 对照 WUM-001 契约梳理现有 ApiClient 的认证、密码输入和错误处理路径，逐项列出需新增或复用的 HTTP 方法、DTO 与可空字段。
2. 在所属模块中实现调用与错误映射，集中处理凭据响应和日志；向 WUM-009 提供稳定的调用接口，不等待 WUM-008 的组件实现。
3. 在本任务测试文件中准备脱敏的成功、冲突、拒绝访问和网络失败响应，验证请求与响应适配；新增测试使用实际可执行的独立入口，公共脚本变更交由 WUM-010 汇总。
4. 在 frontend 目录执行现有 `pnpm build`，运行本项针对性检查，并记录给页面接线方的用法、错误分支和兼容限制。

## 实施与验收

- [x] DTO 保持 snake_case 契约及旧密码输入兼容；401、403、网络失败可区分，不把所有失败归为未登录。
- [x] 创建/重置凭据不进入完整 Axios 日志、URL、localStorage 或通用提示；不改变其他业务接口的无关行为。
- [x] 通过类型/构建检查和可执行的客户端针对性验证，覆盖错误映射、请求参数与敏感日志；明确 mock 不能证明后端授权。
- [x] 新测试可用明确命令独立运行；固定公共 runner 的扩展由 WUM-010 汇总，不以未运行的文件作为测试证据。

## 交付清单

- HTTP 方法与 DTO 对照表、契约版本和 WUM-009 可使用的错误处理说明。
- 所属客户端实现与针对性测试文件；测试响应使用虚构数据，不附带真实凭据。
- 实际构建/测试命令、结果和未覆盖范围，以及交给 WUM-010 的测试入口接入说明。

## 阻塞与交接

当前状态：已处理（`done`）。前置依赖、实现、验收及下游交接均已完成，无本轮遗留阻塞。最终验证范围与未验证项目见[运维验收记录](../plan/operations.md)。

以下保留原执行阶段的依赖与写入规则，不表示当前仍待开发：

等待 WUM-001。可与 WUM-003/008 并行。api.ts 与新 userManagement.ts 由本项独占；全局前端入口由 WUM-009 修改。

## 执行记录（历史顺序）

以下记录保留各阶段当时的状态；当前结论为已处理，以文件顶部 `status: done` 和最终验收记录为准。

- 2026-10-21：仅完成任务规划，尚未认领或执行；未运行本任务的实现测试。

- 2026-10-21：基于 WUM-001 v1 独立契约认领；D-01 未决，不实施自动停用旧预置账号。独占原 write_scope；协调者维护状态与提交。

- 2026-10-21：交付 `api.userManagement` 及 v1.1 DTO（含 `default_password_warning`），安全错误 `UserManagementError/toUserManagementError`；移除完整 Axios/认证响应日志。方法为 getMe/getPolicy/listUsers/getUser/createUser/setAccess/setRole/resetPassword/revokeSessions/listEvents。
- 验证：`pnpm --filter easytier-frontend build` 通过（共享库、vue-tsc、Vite）；`node --test easytier-web/frontend/tests/user-management-client.test.mjs` 5/5 通过；`git diff --check` 通过。使用实际 Axios + 隔离模拟 HTTP 服务，不证明后端授权；真实后端由 WUM-010 验证。

- 2026-10-21：009接线反馈回派补充：007保留旧登录/注册响应的兼容可选code/status；008补invalid_credentials双语文案。原验收证据保留，本次补充完成后复验再关闭。

- 2026-10-21：回派补充已交付：007登录/注册兼容code/status、安全错误及避免重复MD5，客户端6/6通过；008补invalid_credentials双语，92键一致。补充集成构建由009持锁执行，本轮未重复全构建。
