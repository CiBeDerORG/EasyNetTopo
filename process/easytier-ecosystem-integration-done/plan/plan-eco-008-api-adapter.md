---
id: PLAN-ECO-008
feature: easytier-ecosystem-integration
kind: work-package
work_package: ECO-008
summary: ../README.md
status: done
owner: null
requirement: ../require/README.md
task_board: ../task/README.md
depends_on: [PLAN-ECO-002]
requirements: [ECO-02, ECO-04, ECO-05, ECO-06, ECO-07, ECO-08, ECO-09, ECO-11, ECO-12, ECO-13, ECO-16]
acceptance: [ECA-02, ECA-06, ECA-19, ECA-20, ECA-04, ECA-05, ECA-08, ECA-21, ECA-03, ECA-18, ECA-07, ECA-09, ECA-16, ECA-12, ECA-14, ECA-13, ECA-15]
tasks: [ECO-111]
updated_at: 2026-10-21
---

# PLAN-ECO-008：现有接口适配

[需求/计划汇总](../README.md) · [计划索引](README.md) · [需求与验收](../require/README.md) · [任务入口](../task/README.md)

本文件描述工作包计划，执行状态以关联任务与本文件元数据为准；公共范围见 [PLAN-ECO-000](plan-eco-000-overview.md)。路径别名 `D`、`F`、`L` 的定义也见该公共说明，所有拟新增文件仍未实现。

## 对应小任务

原工作包编号描述计划职责；下列 ECO-101 起编号是实际任务，不与 ECO-001～011 或已迁出 ECO-012 混用。执行状态读取各任务，不以计划成文替代完成。

| 任务 | 前置任务 | 并行组 |
| --- | --- | --- |
| [ECO-111 现有接口适配与请求结束](../task/task-eco-111-api-adapter.md) | ECO-104、ECO-105 | `eco-pages` |

共享文件与验证产物依[跨需求调度](../../README.md#跨需求共享文件调度)交接；详细独占范围、输入输出及失败回派见任务。

## 现状依据与未决项

依据 [根汇总](../README.md) 的 ECO-EV-001、002、003、005；关联 ECO-ISSUE-001、002。核查基线为 `feat/web-user-management@1e92296b17fc2e85dcd4521b6012232e11abed95`。handle_list_machines 与宿主/成员模型提供已知入口；每个实际操作仍须核对方法、DTO、授权与错误，不编造资源接口。

## 依赖与拟定写入范围

| 工作包 | 前置 | 独占写入范围与职责 | 交付给 |
| --- | --- | --- | --- |
| ECO-008 现有接口适配 | ECO-002 | `F/src/modules/api.ts`；拟新增 `F/src/modules/ecosystemApi.ts`、`F/tests/ecosystem-api.test.mjs`、`D/api/`；复用认证与普通 HTTP 请求，记录缺口 | ECO-009 |

## 本包内容

只接入 ECO-002 固定的实际能力清单，复用认证与普通 HTTP 请求；保持现有 snake_case DTO，必要转换集中在适配边界。按页进入、切换对象、手动刷新、明确操作触发查询；取消或忽略迟到响应，不自动重试/重放，不挂到既有定时器。缺接口、权限或环境逐项记录，不模拟执行成功。

完整查询/状态/权限契约由 [ECO-002](plan-eco-002-contract.md) 维护；需求与验收 ID 见本文件元数据及[公共追踪表](plan-eco-000-overview.md#4-需求覆盖与能力边界)。各能力不足时保留缺口，不能扩大为后端或外部项目开发。

## 交付条件

按已固定契约提供本包产物和独立验证证据，再交给 [ECO-009](plan-eco-009-host-integration.md)；不直接依赖其他尚未完成的独立页面。源码实施时先通过自身类型/构建及必要状态验证，不能等到宿主接线才首次加载。

## 本次记录

2026-10-21：从原 PLAN-ECO 分离本工作包文档，补充根汇总依据引用；只检查文档目录、链接与状态，未运行构建、测试或 CI；当时未生成本包任务明细。

2026-10-21：根据本计划继续拆出上列任务，明确代码依据、输入输出、写入范围与多 Agent 交接；仍未认领或实施。
