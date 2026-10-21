---
id: ECO-112
feature: easytier-ecosystem-integration
summary: ../README.md
plans: [PLAN-ECO-009]
task_type: implementation
status: done
owner: /root/iteration_exec3
depends_on: ["ECO-106", "ECO-107", "ECO-108", "ECO-109", "ECO-110", "ECO-111"]
read_scope: ["process/easytier-ecosystem-integration-done/plan/design/contract.md", "easytier-web/frontend/src/modules/ecosystemTypes.ts", "easytier-web/frontend/src/modules/ecosystemApi.ts", "easytier-web/frontend/src/components/ecosystem/topology/", "easytier-web/frontend/src/components/ecosystem/nodes/", "easytier-web/frontend/src/components/ecosystem/resources/", "easytier-web/frontend/src/components/ecosystem/access/", "easytier-web/frontend/src/components/ecosystem/navigation/"]
write_scope: ["easytier-web/frontend/src/components/ecosystem/EcosystemPage.vue", "easytier-web/frontend/tests/ecosystem-composition.test.mjs", "process/easytier-ecosystem-integration-done/plan/design/integration-composition.md"]
requirements: [ECO-01, ECO-07, ECO-15]
acceptance: [ECA-01, ECA-15, ECA-03, ECA-18]
parallel_group: eco-composition
evidence: ["ECO-EV-001", "ECO-EV-002", "ECO-EV-004"]
issues: ["ECO-ISSUE-003"]
updated_at: 2026-10-21
---

# ECO-112：拓扑页面组合与能力注入

[汇总](../README.md) · [需求与验收](../require/README.md) · [PLAN-ECO-009](../plan/plan-eco-009-host-integration.md) · [任务看板](README.md)

当前状态与负责人见元数据；执行与验证证据见本任务交接记录。

## 目标与真实依据

组合已交付独立页面，集中管理当前网络/节点/资源选择和API注入，交给宿主接线。

- [easytier-web/frontend/src/main.ts](https://github.com/CiBeDerORG/EasyTier/blob/1e92296b17fc2e85dcd4521b6012232e11abed95/easytier-web/frontend/src/main.ts) 的 `routes`说明宿主使用Vue路由；拟新增 EcosystemPage.vue 尚不存在，需要单独组合后再接入。
- [easytier-web/frontend/src/components/MainPage.vue](https://github.com/CiBeDerORG/EasyTier/blob/1e92296b17fc2e85dcd4521b6012232e11abed95/easytier-web/frontend/src/components/MainPage.vue) 的 `loadNavigation`已有周期逻辑，组合页面不能把资源加载绑定到它。

关联根汇总依据：ECO-EV-001、ECO-EV-002、ECO-EV-004；待核实项：ECO-ISSUE-003。文件存在只证明静态边界，未确认的接口/页面能力不得写成已实现或已验证。

## 输入、输出与启动条件

前置：[ECO-106 拓扑选择与可访问表达](task-eco-106-topology-view.md)、[ECO-107 节点资料与类型目录](task-eco-107-node-details.md)、[ECO-108 服务设备组与设备详情](task-eco-108-resource-details.md)、[ECO-109 公网映射与导航设置表单](task-eco-109-access-settings.md)、[ECO-110 资源导航入口与回链](task-eco-110-navigation-view.md)、[ECO-111 现有接口适配与请求结束](task-eco-111-api-adapter.md)

输入为106～111的完成产物和相同契约版本；输出为拟新增EcosystemPage.vue、组合行为证据和D/integration-composition.md。组件内部源码仅只读消费，缺陷回原任务。

未取得本任务依赖的契约、事实或环境时，只推进可独立核实部分，记录受影响结论；不为补页面效果编造接口、设备能力或成功状态。

## 写入范围与多 Agent 协作

路径均相对仓库根；`read_scope` 只读，`write_scope` 是本任务独占产物范围。尚不存在的 ecosystem 源文件、专项验证文件及 `plan/design/` 产物均为拟新增，不代表本轮已创建。

- `easytier-web/frontend/src/components/ecosystem/EcosystemPage.vue`
- `easytier-web/frontend/tests/ecosystem-composition.test.mjs`
- `process/easytier-ecosystem-integration-done/plan/design/integration-composition.md`

在六组实现之后串行组合；其他需求不写本文件时可并行推进其独立页面。

本任务不写跨需求共享文件；如发现需要扩围，先交协调者调整所有权，再安排串行交接。 共享文件调度遵循[跨需求共享文件调度](../../README.md#跨需求共享文件调度)。根汇总、需求/计划索引、看板与任务状态由协调者单一维护；执行者交付结果，不覆盖他人状态。

## 实施步骤

1. 连接选择、详情、访问设置与导航事件，将当前对象和请求来源集中表达，避免子页面重复请求。
2. 对象切换时取消或丢弃过期响应，清除不再适用的选择/草稿；公共成功/失败按真实结果呈现。
3. 在独立入口验证组合行为，不提前修改正式路由或MainPage，也不扩大类型/适配层所有权。

## 完成与交接

- [x] 各页面/API已交付，组合后的输入输出、迟到响应、失败恢复和按需请求有证据。
- [x] 交付采用提交和组件契约；不以组件能够渲染声称真实外部操作已通过。

交给 ECO-113；局部缺陷按106～111所有权回派，修复后只复验受影响组合。

保护 `config-server` 客户端连接、认证、配置下发与接入命令；不新增心跳、轮询、定时任务、订阅或自动重放。仅修改本文件等文档时不构建、不测试、不运行 CI。未来实际代码验证须隔离工作树、构建产物、服务端口和数据；不能共写 frontend-lib/dist、frontend/dist 或争用固定端口，无法隔离时串行。

## 当前记录

2026-10-21：从对应计划拆出本任务，补充依据、独占范围、依赖和交接；状态为 backlog，owner 为 null，未实施、未验证。

2026-10-21：106–111全部验收，认领组合页请求/身份/选择生命周期及交互验证，不提前写正式路由。

2026-10-21：监工验收页面组合及代次隔离；类型检查通过，组合3/3、支撑4/4，新增pending成员遇overview失败精准回归1/1通过（非全套重跑）。真实后端联调留ECO114。
