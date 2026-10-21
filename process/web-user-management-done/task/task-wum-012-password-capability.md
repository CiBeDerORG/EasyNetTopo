---
id: WUM-012
feature: web-user-management
summary: ../README.md
plans: [PLAN-WUM-010]
status: done
owner: /root
depends_on: ["WUM-009"]
write_scope: ["easytier-web/frontend/src/components/MainPage.vue", "easytier-web/frontend/src/components/ChangePassword.vue", "easytier-web/frontend-lib/src/locales/en.yaml", "easytier-web/frontend-lib/src/locales/cn.yaml", "easytier-web/frontend/tests/user-management-navigation.test.mjs"]
requirements: ["UM-11"]
acceptance: ["UA-15"]
updated_at: "2026-10-21"
---

# WUM-012：改密能力与反馈

拆分后对应计划：[PLAN-WUM-010](../plan/plan-wum-010-review-fixes.md)。本任务保留原编号、状态和历史交付记录；维护入口为[需求/计划汇总](../README.md)。

需求：[UM-11](../require/README.md)。计划：[PLAN-WUM 第10节](../plan/README.md)。[看板](README.md)。

## 目标与契约

仅明确支持本地密码时提供本人改密，直接路由展示不可用原因；保留 `local_password_unavailable`，提示 OIDC-only 用户到身份认证平台操作。以能力字段判断，保留本地/双身份及强制改密。不能改变 config-server 或后端协议。

## 执行与验收

- [x] 调整菜单、表单能力与中英文反馈，不增加后台请求或计时器。
- [x] 生产构建及浏览器覆盖 OIDC-only、未知能力、本地/双身份、服务端拒绝和首次改密。
- [x] 交给 WUM-014 真实联调；不能将模拟接口测试当作后端验证。

## 交付与记录

- 2026-10-21：由 `/root` 认领，按既有前端能力模型实施。无阻塞；所有文档与共享文案由本任务负责人维护。

- 2026-10-21：生产构建通过，导航浏览器测试15/15通过（含5项新增能力场景、中英文、窄屏与键盘）。独立只读评审通过，交付 WUM-014。日志 `/tmp/wum-fix-build.log`、`/tmp/wum-fix-navigation.log`；接口为契约 mock，真实服务待014。首次构建未加载环境中的 protoc 导致工具尝试联网失败，加载既有环境脚本后通过，未改锁文件或构建依赖。
