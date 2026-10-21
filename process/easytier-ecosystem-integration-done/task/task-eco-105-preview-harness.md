---
id: ECO-105
feature: easytier-ecosystem-integration
summary: ../README.md
plans: [PLAN-ECO-002]
task_type: implementation
status: done
owner: /root/iteration_exec3
depends_on: ["ECO-103"]
read_scope: ["easytier-web/frontend/tests/dashboard.test.mjs", "easytier-web/frontend/tests/user-management-e2e.test.mjs", "easytier-web/frontend/tests/helpers/webTestBinary.mjs", "process/easytier-ecosystem-integration-done/plan/design/contract.md"]
write_scope: ["easytier-web/frontend/package.json", "easytier-web/frontend/tests/ecosystem-support/", "process/easytier-ecosystem-integration-done/plan/design/test-support.md"]
requirements: [ECO-02, ECO-03, ECO-06, ECO-07, ECO-08, ECO-10, ECO-15, ECO-16]
acceptance: [ECA-02, ECA-06, ECA-19, ECA-20, ECA-11, ECA-08, ECA-21, ECA-03, ECA-18, ECA-07, ECA-10]
parallel_group: eco-foundation
evidence: ["ECO-EV-004"]
issues: ["ECO-ISSUE-004"]
updated_at: 2026-10-21
---

# ECO-105：独立预览与验证入口

[汇总](../README.md) · [需求与验收](../require/README.md) · [PLAN-ECO-002](../plan/plan-eco-002-contract.md) · [任务看板](README.md)

当前状态与负责人见元数据；执行与验证证据见本任务交接记录。

## 目标与真实依据

为独立页面建立可发现、可运行的最小预览/验证接线，避免组件直到正式路由接入才首次参与构建。

- [easytier-web/frontend/package.json](https://github.com/CiBeDerORG/EasyTier/blob/1e92296b17fc2e85dcd4521b6012232e11abed95/easytier-web/frontend/package.json) 的 `scripts.test:browser / scripts.test:e2e`目前分别只覆盖指定旧文件；仅创建新测试文件不会自动进入这些命令。
- [easytier-web/frontend/tests/helpers/webTestBinary.mjs](https://github.com/CiBeDerORG/EasyTier/blob/1e92296b17fc2e85dcd4521b6012232e11abed95/easytier-web/frontend/tests/helpers/webTestBinary.mjs) 的 `resolveWebTestBinary`提供已修复的 Cargo 产物路径解析，不能退回固定 target/debug 假设。

关联根汇总依据：ECO-EV-004；待核实项：ECO-ISSUE-004。文件存在只证明静态边界，未确认的接口/页面能力不得写成已实现或已验证。

## 输入、输出与启动条件

前置：[ECO-103 资源视图与按需查询契约](task-eco-103-domain-contract.md)

输入为103契约及采用提交的现有 runner/依赖；输出为 tests/ecosystem-support/ 开发验证入口、必要 scripts 接线和 D/test-support.md 说明，不增加业务后台机制或依赖升级。

未取得本任务依赖的契约、事实或环境时，只推进可独立核实部分，记录受影响结论；不为补页面效果编造接口、设备能力或成功状态。

## 写入范围与多 Agent 协作

路径均相对仓库根；`read_scope` 只读，`write_scope` 是本任务独占产物范围。尚不存在的 ecosystem 源文件、专项验证文件及 `plan/design/` 产物均为拟新增，不代表本轮已创建。

- `easytier-web/frontend/package.json`
- `easytier-web/frontend/tests/ecosystem-support/`
- `process/easytier-ecosystem-integration-done/plan/design/test-support.md`

可与 ECO-104 并行；共享组件、语言文件由104写，本任务不改。

本需求唯一写入 package.json 与 tests/ecosystem-support/；需要新增脚本接线时各任务提交需求到本任务。跨需求 package.json 写入先按全局调度串行交接。 共享文件调度遵循[跨需求共享文件调度](../../README.md#跨需求共享文件调度)。根汇总、需求/计划索引、看板与任务状态由协调者单一维护；执行者交付结果，不覆盖他人状态。

## 实施步骤

1. 沿用已有 Node/Playwright/构建入口，显式登记各专项文件，按模块允许独立运行。
2. 演示/模拟数据限开发验证入口；不能注入生产请求流程或被当作真实API联调结果。
3. 记录运行条件、隔离数据、产物解析与失败说明；必要 package.json 变动不改变既有脚本语义。

## 完成与交接

- [x] 独立页面具备明确编译/预览/验证方式，真实联调与mock结果能区分。
- [x] 测试支持目录与各页面断言文件职责分离；缺环境/未运行不能写通过。

交给 ECO-106～111 及114；业务或页面缺陷回对应所有者，不在支持目录绕过断言。

保护 `config-server` 客户端连接、认证、配置下发与接入命令；不新增心跳、轮询、定时任务、订阅或自动重放。仅修改本文件等文档时不构建、不测试、不运行 CI。未来实际代码验证须隔离工作树、构建产物、服务端口和数据；不能共写 frontend-lib/dist、frontend/dist 或争用固定端口，无法隔离时串行。

## 当前记录

2026-10-21：从对应计划拆出本任务，补充依据、独占范围、依赖和交接；状态为 backlog，owner 为 null，未实施、未验证。

2026-10-21：103验收后认领；共享locale/package当前归WUI103，先实施独占组件/支撑目录，交还锁后才能修改共享文件。

2026-10-21：监工验收类型检查、4/4支持测试及CLI成功；明确开发fixture不等于业务或后端验收，package锁交还。
