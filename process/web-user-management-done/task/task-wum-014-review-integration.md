---
id: WUM-014
feature: web-user-management
summary: ../README.md
plans: [PLAN-WUM-003, PLAN-WUM-010]
status: done
owner: /root
depends_on: ["WUM-012", "WUM-013"]
write_scope: ["easytier-web/frontend/tests/user-management-e2e.test.mjs (after WUM-013 handoff)", "process/web-user-management-done/require/README.md", "process/web-user-management-done/plan/operations.md", "process/web-user-management-done/plan/README.md", "process/README.md", "process/web-user-management-done/task/README.md", "process/web-user-management-done/task/README.md", "process/web-user-management-done/task/task-wum-012-password-capability.md", "process/web-user-management-done/task/task-wum-013-e2e-binary.md", "process/web-user-management-done/task/task-wum-014-review-integration.md"]
requirements: ["UM-11", "UM-12"]
acceptance: ["UA-15", "UA-16"]
updated_at: "2026-10-21"
---

# WUM-014：修复联调与交付

拆分后对应计划：[PLAN-WUM-003](../plan/plan-wum-003-verification-and-change.md)；[PLAN-WUM-010](../plan/plan-wum-010-review-fixes.md)。本任务保留原编号、状态和历史交付记录；维护入口为[需求/计划汇总](../README.md)。

需求：[审核修复](../require/README.md)。计划：[PLAN-WUM 第10节](../plan/README.md)。[看板](README.md)。

## 目标与边界

对 WUM-012/013 的真实组合执行生产构建、完整用户管理测试与真实 OIDC 页面回归，保持 config-server 全链路源码不变。只用本地临时服务、虚构账号和 SQLite；不涉及生产部署。

## 执行与验收

- [x] 两项独立修复及其验证交付后再开始联调。
- [x] 移除旧 target 软链接，在自定义 Cargo 目录运行统一入口，区分模拟接口与真实 E2E。
- [x] 真实 OIDC 登录后无本地改密表单，本地/首次改密原场景继续通过。
- [x] 独立审阅、diff/文档链接检查，确认后端及 config-server 未修改。
- [x] 更新证据并提交，正常 push 到 `origin/feat/web-user-management`，核对远程提交。

## 阻塞与记录

- 2026-10-21：等待 WUM-012/013 交付。协调者维护全部文档与看板；修复失败回派原所有者。用户已授权提交推送，无需再次确认。

- 2026-10-21：前置012/013已交付，解除阻塞。接收E2E写入权，增加真实OIDC本人改密入口/直接URL/后端拒绝验证，开始完整联调。

- 2026-10-21：统一入口43/43通过（路径6、客户端mock6、组件mock7、真实E2E9、生产导航mock15），前端与Rust embed构建通过。旧路径软链接已移除，自定义Cargo目录自动解析成功；设备认证兼容回归1/1通过。独立代码复核、文档链接和差异检查通过；无Rust/config-server源码变化。日志及未完成的旧Dashboard回归详见[运维记录](../plan/operations.md)。
- 交付：本次修复随代码提交至 `feat/web-user-management` 并正常推送，远程提交以Git记录为准；不合并main。原工作区未提交的生态规划文档保持原样。没有生产部署。
