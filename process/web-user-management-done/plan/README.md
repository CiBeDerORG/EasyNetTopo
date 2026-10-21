---
id: PLAN-WUM
feature: web-user-management
status: done
summary: ../README.md
requirement: ../require/README.md
task_board: ../task/README.md
baseline: feat/web-user-management@1e92296b17fc2e85dcd4521b6012232e11abed95
updated_at: 2026-10-21
---

# Web 用户管理计划索引

先读取[需求/计划汇总](../README.md)中的目标、代码依据、进度与问题，再按下表进入具体计划。需求依据为[用户管理需求](../require/README.md)，执行状态以[14项任务明细及看板](../task/README.md)为准。

原11项实施/隔离验收及3项审核修复已交付，计划保持 `done`。本次将原文按职责拆为10个小计划文件；它们是文档细项，不是新增10项开发任务，历史任务仍为14项。源码和已知限制以根汇总的 `1e92296b` 交付基线为准，当前文档分支不作为实现基线。

## 具体计划目录

| 计划编号 | 内容 | 原文范围 | 执行状态来源 |
| --- | --- | --- | --- |
| [PLAN-WUM-001](plan-wum-001-scope.md) | 目标、需求追踪与契约准备 | 原第1、2节 | WUM-001～011 的已交付记录 |
| [PLAN-WUM-002](plan-wum-002-parallel-delivery.md) | 依赖、并行交付与文件所有权 | 原第3、4节 | WUM-001～011 的已交付记录 |
| [PLAN-WUM-003](plan-wum-003-verification-and-change.md) | 验证门槛与变更规则 | 原第5、6节 | WUM-001、WUM-006、WUM-010、WUM-011 的已交付记录 |
| [PLAN-WUM-004](plan-wum-004-account-contract.md) | 账号、角色、会话与凭据契约 | 原第8.1节 | WUM-001、WUM-003～006 的已交付记录 |
| [PLAN-WUM-005](plan-wum-005-http-contract.md) | HTTP、DTO、分页与错误契约 | 原第8.2节 | WUM-006～009 的已交付记录 |
| [PLAN-WUM-006](plan-wum-006-storage-contract.md) | 存储、事务与服务契约 | 原第8.3节 | WUM-003～006 的已交付记录 |
| [PLAN-WUM-007](plan-wum-007-ui-contract.md) | 用户管理组件与客户端契约 | 原第8.4节 | WUM-007～009 的已交付记录 |
| [PLAN-WUM-008](plan-wum-008-deployment-compatibility.md) | 部署、恢复与兼容契约 | 原第8.5、8.7节 | WUM-004～007、WUM-009～011 的已交付记录 |
| [PLAN-WUM-009](plan-wum-009-contract-history.md) | 契约形成、评审与首期交付历史 | 原第7、8节导言、8.6及9节 | WUM-001～011 的已交付记录 |
| [PLAN-WUM-010](plan-wum-010-review-fixes.md) | 改密与 E2E 审核修复计划 | 原第10节 | WUM-012～014 的已交付记录 |

## 顺序、进度与问题入口

原顺序为契约与权限基础 → 独立存储/客户端/组件 → 认证与业务 → 前后端接线 → 真实联调 → 升级交付；详细依赖图、并行窗口与共享文件交接见[并行交付计划](plan-wum-002-parallel-delivery.md)。审核修复 WUM-012/013 独立完成后，再进入 WUM-014 联调。

- 文档进展：具体计划已按职责归档；本索引只导航和汇集，不维护第二套任务状态。
- 实现进度：14/14 已交付，为历史结果；本次未重新执行实现或验收。
- 问题与限制：[根汇总的问题记录](../README.md#问题限制与待确认)，包括旧 Dashboard 浏览器限制、生产/真实数据面未验证和在途改密加固建议。
- 下一步：按真实新增目标建立独立需求或明确同需求修复范围；先核实采用提交，再修改相关小计划与任务。

## 8. WUM 契约 v1.1（2026-10-21）

此节保留原“第8节”入口与锚点，具体契约已经拆分：

- [8.1 角色、状态与凭据](plan-wum-004-account-contract.md)
- [8.2 HTTP 与 DTO](plan-wum-005-http-contract.md)
- [8.3 存储与服务](plan-wum-006-storage-contract.md)
- [8.4 UI 边界](plan-wum-007-ui-contract.md)
- [8.5 部署、恢复及8.7兼容澄清](plan-wum-008-deployment-compatibility.md)
- [8.6 契约评审及形成历史](plan-wum-009-contract-history.md)

## 10. 审核修复计划（2026-10-21）

此节保留原“第10节”入口。UM-11/12 对应 WUM-012～014 的独立写入范围、交接顺序和执行结果见[审核修复计划](plan-wum-010-review-fixes.md)。

## 维护记录

2026-10-21：依据用户要求将计划拆为 `plan-编号-名称.md`；保留需求追踪、依赖、文件所有权、完整契约、决策与历史交付证据。仅检查文档内容与链接，不运行构建、测试或 CI。实际验收结果仍见[运维记录](operations.md)，不将本次整理当作新的验证通过。
