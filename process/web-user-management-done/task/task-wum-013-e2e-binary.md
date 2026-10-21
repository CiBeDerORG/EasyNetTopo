---
id: WUM-013
feature: web-user-management
summary: ../README.md
plans: [PLAN-WUM-010]
status: done
owner: /root/wum_e2e_fix
depends_on: ["WUM-010"]
write_scope: ["easytier-web/frontend/tests/user-management-e2e.test.mjs", "easytier-web/frontend/tests/helpers/webTestBinary.mjs", "easytier-web/frontend/tests/user-management-binary.test.mjs", "easytier-web/frontend/package.json (if needed)"]
requirements: ["UM-12"]
acceptance: ["UA-16"]
updated_at: "2026-10-21"
---

# WUM-013：E2E 程序路径

拆分后对应计划：[PLAN-WUM-010](../plan/plan-wum-010-review-fixes.md)。本任务保留原编号、状态和历史交付记录；维护入口为[需求/计划汇总](../README.md)。

需求：[UM-12](../require/README.md)。计划：[PLAN-WUM 第10节](../plan/README.md)。[看板](README.md)。

## 目标与契约

优先使用 `WUM_TEST_WEB_BINARY`；默认通过 Cargo 的实际 target 目录解析调试构建。尊重 `CARGO_TARGET_DIR` 和 Cargo 配置，路径无效明确失败，不使用陈旧固定路径兜底。不改构建依赖或 config-server。

## 执行与验收

- [x] 路径解析与启动错误可诊断；显式路径及自定义 target 目录有针对性验证。
- [x] 单项验证后交付 WUM-014，后者移除旧软链接运行真实 E2E。

## 交付与记录

- 2026-10-21：由 `/root/wum_e2e_fix` 认领，与 WUM-012 并行。代码/测试仅在所列范围；文档由协调者维护。E2E 文件完成后交给014追加真实场景。

- 2026-10-21：路径测试6/6通过（真实 Cargo metadata、临时 Cargo 包，不运行虚构程序），覆盖默认目录、绝对/相对 target、Cargo 配置、显式路径、错误不回退。当前仓库正确解析 `/workspace/.easytier-env/target/debug/easytier-web`；E2E文件已交给014。未改 package.json。
