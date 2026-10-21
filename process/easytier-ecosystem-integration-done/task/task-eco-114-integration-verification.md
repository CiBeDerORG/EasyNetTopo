---
id: ECO-114
feature: easytier-ecosystem-integration
summary: ../README.md
plans: [PLAN-ECO-010]
task_type: integration
status: done
owner: /root/iteration_exec2
depends_on: ["ECO-113"]
read_scope: ["process/easytier-ecosystem-integration-done/plan/design/contract.md", "process/easytier-ecosystem-integration-done/plan/design/integration.md", "easytier-web/frontend/src/components/ecosystem/", "easytier-web/frontend/src/modules/ecosystemApi.ts", "easytier-web/frontend/tests/helpers/webTestBinary.mjs", "easytier-web/frontend/package.json"]
write_scope: ["easytier-web/frontend/tests/ecosystem-integration.test.mjs", "process/easytier-ecosystem-integration-done/plan/design/verification/"]
requirements: [ECO-01, ECO-02, ECO-03, ECO-04, ECO-05, ECO-06, ECO-07, ECO-08, ECO-09, ECO-10, ECO-11, ECO-12, ECO-13, ECO-15, ECO-16]
acceptance: [ECA-01, ECA-02, ECA-03, ECA-04, ECA-05, ECA-06, ECA-07, ECA-08, ECA-09, ECA-10, ECA-11, ECA-12, ECA-13, ECA-14, ECA-15, ECA-16, ECA-18, ECA-19, ECA-20, ECA-21]
parallel_group: eco-verification
evidence: ["ECO-EV-001", "ECO-EV-002", "ECO-EV-003", "ECO-EV-004", "ECO-EV-005"]
issues: ["ECO-ISSUE-001", "ECO-ISSUE-002", "ECO-ISSUE-003", "ECO-ISSUE-004"]
updated_at: 2026-10-21
---

# ECO-114：真实接口联调与失败回派

[汇总](../README.md) · [需求与验收](../require/README.md) · [PLAN-ECO-010](../plan/plan-eco-010-integration-verification.md) · [任务看板](README.md)

当前状态与负责人见元数据；执行与验证证据见本任务交接记录。

## 目标与真实依据

在同一实现提交上联调承诺接入的真实接口，记录范围、失败回派与未验证能力。

- [easytier-web/frontend/package.json](https://github.com/CiBeDerORG/EasyTier/blob/1e92296b17fc2e85dcd4521b6012232e11abed95/easytier-web/frontend/package.json) 的 `scripts`区分旧浏览器/E2E与用户管理入口；专项文件须由105接入，不能把未被执行的文件当覆盖。
- [easytier-web/frontend/tests/helpers/webTestBinary.mjs](https://github.com/CiBeDerORG/EasyTier/blob/1e92296b17fc2e85dcd4521b6012232e11abed95/easytier-web/frontend/tests/helpers/webTestBinary.mjs) 的 `resolveWebTestBinary`解析实际后端产物；真实运行不能依赖旧固定路径或模拟状态。

关联根汇总依据：ECO-EV-001、ECO-EV-002、ECO-EV-003、ECO-EV-004、ECO-EV-005；待核实项：ECO-ISSUE-001、ECO-ISSUE-002、ECO-ISSUE-003、ECO-ISSUE-004。文件存在只证明静态边界，未确认的接口/页面能力不得写成已实现或已验证。

## 输入、输出与启动条件

前置：[ECO-113 正式路由与宿主导航接线](task-eco-113-host-routing.md)

输入为103固定能力清单、106～113交付、可运行隔离环境和真实资源；输出为专项联调脚本/记录、逐ECA证据、缺陷与未覆盖范围。环境缺失仅阻塞对应场景，不能据此擅自降级承诺范围。

未取得本任务依赖的契约、事实或环境时，只推进可独立核实部分，记录受影响结论；不为补页面效果编造接口、设备能力或成功状态。

## 写入范围与多 Agent 协作

路径均相对仓库根；`read_scope` 只读，`write_scope` 是本任务独占产物范围。尚不存在的 ecosystem 源文件、专项验证文件及 `plan/design/` 产物均为拟新增，不代表本轮已创建。

- `easytier-web/frontend/tests/ecosystem-integration.test.mjs`
- `process/easytier-ecosystem-integration-done/plan/design/verification/`

需等待正式接线；可与WUI验证并行的前提是工作树、构建产物、端口和隔离数据均独立，否则串行验证。

仅写ecosystem-integration专项文件；dashboard.test.mjs和user-management-e2e.test.mjs集中调整属于WUI-112，本任务只按需要运行既有入口，不抢写。 共享文件调度遵循[跨需求共享文件调度](../../README.md#跨需求共享文件调度)。根汇总、需求/计划索引、看板与任务状态由协调者单一维护；执行者交付结果，不覆盖他人状态。

## 实施步骤

1. 按计划覆盖查询、保存、权限、归属、映射/导航生命周期和失败恢复，外部能力仅在实际来源及隔离资源可用时验证。
2. 对新增页面记录用户触发至请求结束及空闲请求；观察区间覆盖已知相关旧刷新间隔至少两周期，并检查新增更长周期机制。
3. 真实接口与mock/设计覆盖分开；问题回派103契约、106～110页面、111适配、112组合或113宿主，修复后仅复验相关部分。

## 完成与交接

- [x] 已承诺范围逐项记录通过/失败/未验证及证据，config-server兼容与无新增周期机制有对应记录。
- [x] 未支持/未验证/后置能力明确列出；ECA-17 deploy仍后置，不能宣布所有外部设备已接入。

完成后交给 ECO-115；未执行或仍失败的承诺项不能放行。联调任务不并发修改业务模块。

保护 `config-server` 客户端连接、认证、配置下发与接入命令；不新增心跳、轮询、定时任务、订阅或自动重放。仅修改本文件等文档时不构建、不测试、不运行 CI。未来实际代码验证须隔离工作树、构建产物、服务端口和数据；不能共写 frontend-lib/dist、frontend/dist 或争用固定端口，无法隔离时串行。

## 当前记录

2026-10-21：从对应计划拆出本任务，补充依据、独占范围、依赖和交接；状态为 backlog，owner 为 null，未实施、未验证。

2026-10-21：监工准入，使用113最终独立前端产物与隔离真实backend/SQLite；与WUI冻结产物隔离，空闲观察至少两轮旧周期。

2026-10-21：监工验收：完整11专项75/75（3真实+72fixture/工具），追加真实删除404恢复1/1定向；两模式22秒仅旧周期，真实普通/强改/跨用户拒绝通过。未支持API与真设备/VPN/codegen限制保留。
