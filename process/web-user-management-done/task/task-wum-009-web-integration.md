---
id: WUM-009
feature: web-user-management
summary: ../README.md
plans: [PLAN-WUM-002, PLAN-WUM-007]
status: done
owner: /root/wum009
depends_on: ["WUM-007","WUM-008"]
write_scope: ["easytier-web/frontend/src/main.ts","easytier-web/frontend/src/components/MainPage.vue","easytier-web/frontend/src/components/Login.vue","easytier-web/frontend/src/components/ChangePassword.vue","easytier-web/frontend/tests/user-management-navigation.test.mjs","easytier-web/frontend-lib/src/locales/en.yaml (username_help only)","easytier-web/frontend-lib/src/locales/cn.yaml (username_help only)"]
requirements: ["UM-01","UM-02","UM-03","UM-04","UM-05","UM-06","UM-07","UM-08","UM-09"]
acceptance: ["UA-01","UA-03","UA-05","UA-10","UA-11"]
updated_at: "2026-10-21"
---

# WUM-009：前端导航与身份流程接线

拆分后对应计划：[PLAN-WUM-002](../plan/plan-wum-002-parallel-delivery.md)；[PLAN-WUM-007](../plan/plan-wum-007-ui-contract.md)。本任务保留原编号、状态和历史交付记录；维护入口为[需求/计划汇总](../README.md)。

需求：[Web 用户管理](../require/README.md)。计划：[PLAN-WUM](../plan/README.md)。[返回看板](README.md)。

## 目标与边界

将 WUM-007 的 HTTP 客户端与 WUM-008 的组件接入 Web 工作区，完成用户管理入口、能力变化、注册策略和强制改密的页面流程。

## 输入与输出契约

依赖客户端、组件和文案已交付；使用 WUM-001 契约组合。输出可运行 Web 构建、页面路径与可复现交互步骤，供 WUM-010 与真实后端联调。

## 执行步骤

1. 核对 WUM-007 的调用接口与 WUM-008 的组件输入/事件，列出当前身份、用户页面和登录流程之间的数据与导航对应关系；接口缺口回交原任务。
2. 在 main.ts/MainPage.vue 接入路由、菜单和组件数据流，将用户操作连接到 HTTP 客户端，并以服务端返回能力驱动工作区导航。
3. 在 Login.vue/ChangePassword.vue 接入注册策略、受限改密与登录失效反馈，使用已交付的文案键；缺失组件或文案仍由其所有者补齐。
4. 运行现有前端构建与本项导航检查，记录可复现的页面路径、账号状态和操作步骤，将真实服务端验证输入交给 WUM-010。

## 实施与验收

- [x] 管理菜单按服务端能力显示，直接访问受限页面有明确反馈；前端控制不能替代服务器授权。
- [x] 注册关闭、受限改密、退出/权限撤销后的导航与草稿恢复一致；401、403、网络故障分别反馈。
- [x] 完成组件数据/事件与 HTTP 适配接线；全局状态不复制第二套账号真相。
- [x] 执行前端构建与导航/表单针对性检查；此阶段可用契约 mock，必须说明真实授权留给 WUM-010。

## 交付清单

- 已接线的路由、导航和身份流程，附各页面使用的客户端方法与组件说明。
- 导航/表单检查文件及实际构建、验证结果，明确哪些结果使用了契约 mock。
- WUM-010 可复现的页面访问和账号状态准备步骤，以及尚需真实后端确认的行为清单。

## 阻塞与交接

当前状态：已处理（`done`）。前置依赖、实现、验收及下游交接均已完成，无本轮遗留阻塞。最终验证范围与未验证项目见[运维验收记录](../plan/operations.md)。

以下保留原执行阶段的依赖与写入规则，不表示当前仍待开发：

等待 WUM-007/008；可与后端 WUM-004/005 或已就绪的 WUM-006 并行。客户端/组件/文案缺陷回派原任务，不自行跨范围并行修改。退出条件是前端完整流程可运行且验证记录可复现。

## 执行记录（历史顺序）

以下记录保留各阶段当时的状态；当前结论为已处理，以文件顶部 `status: done` 和最终验收记录为准。

- 2026-10-21：仅完成任务规划，尚未认领或执行；未运行本任务的实现测试。

- 2026-10-21：007/008已验收后认领，使用v1.1、api.userManagement与UserManagement组件；用户选择A的默认密码提示需非阻断显示。

- 2026-10-21：完成 `#/h/users` / `#/h/password`、能力导航、注册策略、受限改密与默认密码非阻断提示。MainPage单一持有me能力状态；403刷新、401退出、网络错误保留草稿，序号阻止迟到响应，导航/退出清除凭据。
- 验证：`pnpm --filter easytier-frontend build`通过（覆盖007/008回派补丁）；`node --test easytier-web/frontend/tests/user-management-navigation.test.mjs` 9/9通过（系统Chromium154+契约mock）；`git diff --check`通过。首轮重复文案定位问题已修正复验。真实后端授权仍交010。

- 2026-10-21：010真实embed浏览器发现创建按钮只呈现空遮罩、未渲染Dialog内容，无pageerror；重新打开并回派009诊断production构建与接线。现有mock测试通过不替代该真实失败。

- 2026-10-21：009定位为两份username_help文案裸@触发production vue-i18n linked-message解析错误（SyntaxError10），Vue捕获后Dialog插槽渲染注释。移交009独占en/cn两行转义修复，显示文本保持不变，并增加production preview回归；无须改组件或构建配置。

- 2026-10-21：两行文案修复后pnpm build通过；导航测试默认改production preview，实际10/10通过（含中英文创建弹窗和字面量@断言），日志/tmp/wum009-production-navigation.log。已移除诊断日志，等待010真实embed复验后恢复done。

- 2026-10-21：010统一runner真实embed浏览器管理全链已通过（创建/停用/恢复/角色/撤会话/重置/审计/离线草稿），生产Dialog缺陷关闭，009恢复done。010另一个测试路由定位修正仍在复验，不影响此已验证修复。
