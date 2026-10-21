---
id: PLAN-ECO-009
feature: easytier-ecosystem-integration
kind: work-package
work_package: ECO-009
summary: ../README.md
status: done
owner: null
requirement: ../require/README.md
task_board: ../task/README.md
depends_on: [PLAN-ECO-003, PLAN-ECO-004, PLAN-ECO-005, PLAN-ECO-006, PLAN-ECO-007, PLAN-ECO-008]
requirements: [ECO-01, ECO-07, ECO-15]
acceptance: [ECA-01, ECA-15, ECA-03, ECA-18]
tasks: [ECO-112, ECO-113]
updated_at: 2026-10-21
---

# PLAN-ECO-009：页面与宿主接线

[需求/计划汇总](../README.md) · [计划索引](README.md) · [需求与验收](../require/README.md) · [任务入口](../task/README.md)

本文件描述工作包计划，执行状态以关联任务与本文件元数据为准；公共范围见 [PLAN-ECO-000](plan-eco-000-overview.md)。路径别名 `D`、`F`、`L` 的定义也见该公共说明，所有拟新增文件仍未实现。

## 对应小任务

原工作包编号描述计划职责；下列 ECO-101 起编号是实际任务，不与 ECO-001～011 或已迁出 ECO-012 混用。执行状态读取各任务，不以计划成文替代完成。

| 任务 | 前置任务 | 并行组 |
| --- | --- | --- |
| [ECO-112 拓扑页面组合与能力注入](../task/task-eco-112-page-composition.md) | ECO-106、ECO-107、ECO-108、ECO-109、ECO-110、ECO-111 | `eco-composition` |
| [ECO-113 正式路由与宿主导航接线](../task/task-eco-113-host-routing.md) | ECO-112 | `eco-host-integration` |

共享文件与验证产物依[跨需求调度](../../README.md#跨需求共享文件调度)交接；详细独占范围、输入输出及失败回派见任务。

## 现状依据与未决项

依据 [根汇总](../README.md) 的 ECO-EV-004；关联 ECO-ISSUE-003。核查基线为 `feat/web-user-management@1e92296b17fc2e85dcd4521b6012232e11abed95`。MainPage 已有导航定时刷新和 config-server 接入命令；接线不得扩大周期请求或修改设备接入行为。

## 依赖与拟定写入范围

| 工作包 | 前置 | 独占写入范围与职责 | 交付给 |
| --- | --- | --- | --- |
| ECO-009 页面与宿主接线 | ECO-003、ECO-004、ECO-005、ECO-006、ECO-007、ECO-008 | `F/src/main.ts`、`F/src/components/MainPage.vue`；拟新增 `F/src/components/ecosystem/EcosystemPage.vue`、`F/tests/ecosystem-composition.test.mjs`、`F/tests/ecosystem-host.test.mjs`、`D/integration-composition.md`、`D/integration.md`；统一路由、菜单、页面组合与 API 注入 | ECO-010 |

## 本包内容

整合已交付独立页面、API 注入、正式路由、菜单及宿主布局。保留旧登录、用户管理、原网络配置、主题行为和 config-server 接入命令。依赖 ECO-003～008 全部交付，不等待独立 WUI；如与 WUI 写同一文件，由协调者串行交接。

完整查询/状态/权限契约由 [ECO-002](plan-eco-002-contract.md) 维护；需求与验收 ID 见本文件元数据及[公共追踪表](plan-eco-000-overview.md#4-需求覆盖与能力边界)。各能力不足时保留缺口，不能扩大为后端或外部项目开发。

## 交付条件

统一接线完成后将采用提交、页面/适配层产物、能力清单与已知问题交给 [ECO-010](plan-eco-010-integration-verification.md)。保留原路由兼容，核对新增页面没有继承周期状态查询。

## 本次记录

2026-10-21：从原 PLAN-ECO 分离本工作包文档，补充根汇总依据引用；只检查文档目录、链接与状态，未运行构建、测试或 CI；当时未生成本包任务明细。

2026-10-21：根据本计划继续拆出上列任务，明确代码依据、输入输出、写入范围与多 Agent 交接；仍未认领或实施。
