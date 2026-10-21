---
id: ECO-103
feature: easytier-ecosystem-integration
summary: ../README.md
plans: [PLAN-ECO-002]
task_type: implementation
status: done
owner: /root/iteration_exec1
depends_on: ["ECO-101", "ECO-102"]
read_scope: ["process/easytier-ecosystem-integration-done/plan/design/baseline.md", "process/easytier-ecosystem-integration-done/plan/design/baseline-evidence/", "easytier-web/frontend/src/modules/api.ts", "easytier-web/src/db/entity/devices.rs", "easytier-web/src/db/entity/network_members.rs"]
write_scope: ["process/easytier-ecosystem-integration-done/plan/design/contract.md", "easytier-web/frontend/src/modules/ecosystemTypes.ts"]
requirements: [ECO-02, ECO-03, ECO-06, ECO-07, ECO-08, ECO-10, ECO-15, ECO-16]
acceptance: [ECA-02, ECA-06, ECA-19, ECA-20, ECA-11, ECA-08, ECA-21, ECA-03, ECA-18, ECA-07, ECA-10]
parallel_group: eco-contract
evidence: ["ECO-EV-001", "ECO-EV-002", "ECO-EV-003", "ECO-EV-004", "ECO-EV-005"]
issues: ["ECO-ISSUE-001", "ECO-ISSUE-002", "ECO-ISSUE-003"]
updated_at: 2026-10-21
---

# ECO-103：资源视图与按需查询契约

[汇总](../README.md) · [需求与验收](../require/README.md) · [PLAN-ECO-002](../plan/plan-eco-002-contract.md) · [任务看板](README.md)

当前状态与负责人见元数据；执行与验证证据见本任务交接记录。

## 目标与真实依据

收敛本期可接入能力、资源视图、组件输入输出和请求语义，建立所有消费者共用的一份契约与类型。

- [easytier-web/src/db/entity/network_members.rs](https://github.com/CiBeDerORG/EasyTier/blob/1e92296b17fc2e85dcd4521b6012232e11abed95/easytier-web/src/db/entity/network_members.rs) 的 `Model.device_id`引用宿主；现有成员与下挂业务设备不是同一个身份。
- [easytier-web/frontend/src/modules/api.ts](https://github.com/CiBeDerORG/EasyTier/blob/1e92296b17fc2e85dcd4521b6012232e11abed95/easytier-web/frontend/src/modules/api.ts) 的 `ApiClient`持有统一认证客户端与现有 DTO；新增视图类型不能冒充已存在的服务端字段。

关联根汇总依据：ECO-EV-001、ECO-EV-002、ECO-EV-003、ECO-EV-004、ECO-EV-005；待核实项：ECO-ISSUE-001、ECO-ISSUE-002、ECO-ISSUE-003。文件存在只证明静态边界，未确认的接口/页面能力不得写成已实现或已验证。

## 输入、输出与启动条件

前置：[ECO-101 源码基线与接口能力清单](task-eco-101-source-capabilities.md)、[ECO-102 现有页面与请求行为取证](task-eco-102-page-baseline.md)

输入为 ECO-101/102 的代码、页面证据及缺口；输出为 D/contract.md 与拟新增 ecosystemTypes.ts，明确本期真实接入清单、未支持能力、身份/状态/props/events/错误和请求结束条件。

未取得本任务依赖的契约、事实或环境时，只推进可独立核实部分，记录受影响结论；不为补页面效果编造接口、设备能力或成功状态。

## 写入范围与多 Agent 协作

路径均相对仓库根；`read_scope` 只读，`write_scope` 是本任务独占产物范围。尚不存在的 ecosystem 源文件、专项验证文件及 `plan/design/` 产物均为拟新增，不代表本轮已创建。

- `process/easytier-ecosystem-integration-done/plan/design/contract.md`
- `easytier-web/frontend/src/modules/ecosystemTypes.ts`

本任务在101/102后串行收敛；不与自己的类型消费者抢写公共类型。

唯一写入 D/contract.md 与 ecosystemTypes.ts；后续所有任务只读。文案、package.json 与正式路由不属本任务。 共享文件调度遵循[跨需求共享文件调度](../../README.md#跨需求共享文件调度)。根汇总、需求/计划索引、看板与任务状态由协调者单一维护；执行者交付结果，不覆盖他人状态。

## 实施步骤

1. 将节点、成员、实例、服务、设备组、设备和端点的身份关系标明来源；未知值与无支持能力显式建模。
2. 固定每个实际操作的入口、请求触发、权限和结果表达；公网许可、映射与导航独立；未确认 API 不进入可执行清单。
3. 定义公共类型及页面输入输出，区分纯展示/用户动作事件与 HTTP 请求；变更类型由本任务单一处理并通知消费者。

## 完成与交接

- [x] 下游可根据同一契约独立实现，不需要猜测另一页面或外部项目尚不存在的实现。
- [x] 契约记录 scope/未支持/未验证项，不把失败后降级当成验收通过；按需请求无心跳、轮询、自动重放。

先交 ECO-104/105；类型或契约缺陷回本任务修正，已开始消费者暂停相关写入后串行交接。

保护 `config-server` 客户端连接、认证、配置下发与接入命令；不新增心跳、轮询、定时任务、订阅或自动重放。仅修改本文件等文档时不构建、不测试、不运行 CI。未来实际代码验证须隔离工作树、构建产物、服务端口和数据；不能共写 frontend-lib/dist、frontend/dist 或争用固定端口，无法隔离时串行。

## 当前记录

2026-10-21：从对应计划拆出本任务，补充依据、独占范围、依赖和交接；状态为 backlog，owner 为 null，未实施、未验证。

2026-10-21：101/102经监工验收，exec1认领契约及公共类型实现；未支持/未验证边界不得降格为成功。

2026-10-21：监工审查契约通过，strict tsc --noEmit及Vue compiler-sfc props/events编译通过；未执行页面联调。
