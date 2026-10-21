---
id: WUM-008
feature: web-user-management
summary: ../README.md
plans: [PLAN-WUM-005, PLAN-WUM-007]
status: done
owner: /root/wum008
depends_on: ["WUM-001"]
write_scope: ["easytier-web/frontend/src/components/users/","easytier-web/frontend-lib/src/locales/cn.yaml","easytier-web/frontend-lib/src/locales/en.yaml","easytier-web/frontend/tests/user-management-components.test.mjs"]
requirements: ["UM-02","UM-03","UM-04","UM-05","UM-06","UM-07","UM-08","UM-09"]
acceptance: ["UA-02","UA-04","UA-05","UA-06","UA-07","UA-08","UA-10","UA-11","UA-13"]
updated_at: "2026-10-21"
---

# WUM-008：用户管理组件与文案

拆分后对应计划：[PLAN-WUM-005](../plan/plan-wum-005-http-contract.md)；[PLAN-WUM-007](../plan/plan-wum-007-ui-contract.md)。本任务保留原编号、状态和历史交付记录；维护入口为[需求/计划汇总](../README.md)。

需求：[Web 用户管理](../require/README.md)。计划：[PLAN-WUM](../plan/README.md)。[返回看板](README.md)。

## 目标与边界

开发可独立交付的用户列表/详情、创建、停用恢复、角色、重置、会话撤销和审计组件；补齐中英文文案。users/ 为建议新增目录。通过 props/events 与 UI view model 定义边界，不直接依赖尚未实现的客户端文件。

## 输入与输出契约

输入 WUM-001 的页面状态、交互和 DTO/组件契约。输出给 WUM-009 的组件、事件、所需数据形状及反馈状态；提供登录/注册/强制改密相关文案给 WUM-009 使用。

## 执行步骤

1. 按 WUM-001 契约整理组件清单、props/events 与状态矩阵，复用现有 UI 组件；用明确的输入数据表达列表、操作结果和权限，不导入尚未落地的 HTTP 客户端。
2. 在 users/ 范围内实现列表、详情及各操作组件，并通过本任务测试中的模拟输入驱动交互；保持全局路由和真实数据调用由 WUM-009 接入。
3. 统一补充 cn.yaml/en.yaml 中的用户管理及公共登录流程文案，将键名与适用场景交给 WUM-009；其他任务的文案需求通过本任务串行写入。
4. 运行现有前端构建及可执行的组件检查，覆盖交互状态、键盘和布局；记录模拟输入范围及组件挂载方式，供后续接线复用。

## 实施与验收

- [x] 覆盖分页搜索、加载/空/失败/过期、长用户名、迟到响应、重复提交；未知来源和时间不造假。
- [x] 危险动作准确显示目标与影响；重置适用性、最后管理员限制以服务端结果为准；不宣称停用等于设备断网。
- [x] 凭据仅在受控交付流程临时展示，默认遮蔽；不放进列表、toast 或持久缓存。
- [x] 验证中英文、键盘、对话框焦点、窄屏与相关交互失败场景；新组件参与类型检查并可通过现有工具独立验证。
- [x] 记录使用模拟数据的范围；组件测试通过不代表真实鉴权或完整业务已通过。

## 交付清单

- 可挂载的组件及 props/events 说明，包含虚构输入样例和提交/成功/失败状态的接入方式。
- 中英文文案键清单，标明 WUM-009 登录、注册和强制改密流程需要使用的键。
- 类型/构建与组件交互的实际验证记录、测试入口和留给 WUM-010 的真实后端验证事项。

## 阻塞与交接

当前状态：已处理（`done`）。前置依赖、实现、验收及下游交接均已完成，无本轮遗留阻塞。最终验证范围与未验证项目见[运维验收记录](../plan/operations.md)。

以下保留原执行阶段的依赖与写入规则，不表示当前仍待开发：

等待 WUM-001。可与 WUM-003/007 并行。两份 locale 文件由本项统一写入；WUM-009 的新增文案通过交接请求处理，不并行改相同资源。全局页面/路由由 WUM-009 接入。

## 执行记录（历史顺序）

以下记录保留各阶段当时的状态；当前结论为已处理，以文件顶部 `status: done` 和最终验收记录为准。

- 2026-10-21：仅完成任务规划，尚未认领或执行；未运行本任务的实现测试。

- 2026-10-21：基于 WUM-001 v1 独立契约认领；D-01 未决，不实施自动停用旧预置账号。独占原 write_scope；协调者维护状态与提交。

- 2026-10-21：交付 `components/users/UserManagement.vue`、本地等价types及 `user_management.*` 双语资源；契约props/events接入，敏感凭据默认遮蔽，仅内存展示。公开登录文案含 registration_disabled/password_change_required/password_policy/password_changed/session_expired/policy_failed/default_password_warning。
- 验证：`pnpm --filter easytier-frontend build` 通过；`node --test easytier-web/frontend/tests/user-management-components.test.mjs` 7/7通过（系统Chromium154，真实组件+模拟props/events，含中英文/375px/键盘焦点/错误草稿/重复提交/各动作确认/审计分页/凭据清除）。首轮fixture与定位器失败已修正并复验。真实后端与迟到HTTP响应由009/010验证，不以组件mock代替授权。

- 2026-10-21：009接线反馈回派补充：007保留旧登录/注册响应的兼容可选code/status；008补invalid_credentials双语文案。原验收证据保留，本次补充完成后复验再关闭。

- 2026-10-21：回派补充已交付：007登录/注册兼容code/status、安全错误及避免重复MD5，客户端6/6通过；008补invalid_credentials双语，92键一致。补充集成构建由009持锁执行，本轮未重复全构建。
