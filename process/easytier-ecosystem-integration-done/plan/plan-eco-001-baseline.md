---
id: PLAN-ECO-001
feature: easytier-ecosystem-integration
kind: work-package
work_package: ECO-001
summary: ../README.md
status: done
owner: null
requirement: ../require/README.md
task_board: ../task/README.md
depends_on: []
requirements: [ECO-07, ECO-15]
acceptance: [ECA-03, ECA-18]
tasks: [ECO-101, ECO-102]
updated_at: 2026-10-21
---

# PLAN-ECO-001：基线与复用清单

[需求/计划汇总](../README.md) · [计划索引](README.md) · [需求与验收](../require/README.md) · [任务入口](../task/README.md)

本文件描述工作包计划，执行状态以关联任务与本文件元数据为准；公共范围见 [PLAN-ECO-000](plan-eco-000-overview.md)。路径别名 `D`、`F`、`L` 的定义也见该公共说明，所有拟新增文件仍未实现。

## 对应小任务

原工作包编号描述计划职责；下列 ECO-101 起编号是实际任务，不与 ECO-001～011 或已迁出 ECO-012 混用。执行状态读取各任务，不以计划成文替代完成。

| 任务 | 前置任务 | 并行组 |
| --- | --- | --- |
| [ECO-101 源码基线与接口能力清单](../task/task-eco-101-source-capabilities.md) | 无 | `eco-discovery` |
| [ECO-102 现有页面与请求行为取证](../task/task-eco-102-page-baseline.md) | 无 | `eco-discovery` |

共享文件与验证产物依[跨需求调度](../../README.md#跨需求共享文件调度)交接；详细独占范围、输入输出及失败回派见任务。

## 现状依据与未决项

依据 [根汇总](../README.md) 的 ECO-EV-001～005；关联 ECO-ISSUE-001～004。核查基线为 `feat/web-user-management@1e92296b17fc2e85dcd4521b6012232e11abed95`。先核对已知交付基线及后续差异；源码核查只证明已读取范围，不替代页面运行证据。

## 依赖与拟定写入范围

| 工作包 | 前置 | 独占写入范围与职责 | 交付给 |
| --- | --- | --- | --- |
| ECO-001 基线与复用清单 | 无 | 拟新增 `D/baseline.md` 与 `D/baseline-evidence/`；只读核查实现分支、页面/API/脚本，不改业务代码 | ECO-002 |

## 基线清单与退出条件

| 项目 | 已知事实与处理 |
| --- | --- |
| 用户指定基础 | 已核验并推送 `feat/web-user-management` / `1e92296b17fc2e85dcd4521b6012232e11abed95`，包含用户管理及其改密/E2E审核修复；实际设计基于此成果 |
| 当前文档工作区 | `docs/architecture-and-user-management-plan`，HEAD `f48602e2`；不能当作已核验的用户管理实现分支 |
| 本次核对 | 只读核对上述实现快照的路由与组件；未启动本专项页面审计、制作效果图或改动业务代码 |
| 执行前补齐 | 由 ECO-001 复核启动时的分支/SHA与后续差异，记录页面证据、已有认证/权限/接口、构建入口及测试环境 |
| 基线记录的用途 | 已知实现可支持范围规划；具体缺陷和改版方案仍须实际页面证据，不以源码清单替代视觉审计 |

用户管理按已交付分支集中为独立 done 需求，本地旧草案另存历史；不重做其功能。实施保护已有工作区改动，使用独立 checkout/worktree 也不能绕过跨需求共享文件的串行交接。

记录源码路径/符号/提交、页面入口、实际请求与权限、已有脚本和环境限制。页面前态及问题需运行证据；纯文档整理只读核查，不安装依赖或运行应用。达到 M1 后交给 ECO-002 固定实际接入清单。

## 本次记录

2026-10-21：从原 PLAN-ECO 分离本工作包文档，补充根汇总依据引用；只检查文档目录、链接与状态，未运行构建、测试或 CI；当时未生成本包任务明细。

2026-10-21：根据本计划继续拆出上列任务，明确代码依据、输入输出、写入范围与多 Agent 交接；仍未认领或实施。
