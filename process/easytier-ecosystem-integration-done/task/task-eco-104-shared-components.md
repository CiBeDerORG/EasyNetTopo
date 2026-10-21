---
id: ECO-104
feature: easytier-ecosystem-integration
summary: ../README.md
plans: [PLAN-ECO-002]
task_type: implementation
status: done
owner: /root/iteration_exec1
depends_on: ["ECO-103"]
read_scope: ["process/easytier-ecosystem-integration-done/plan/design/contract.md", "easytier-web/frontend/src/modules/ecosystemTypes.ts", "easytier-web/frontend/src/theme.ts"]
write_scope: ["easytier-web/frontend/src/components/ecosystem/shared/", "easytier-web/frontend/tests/ecosystem-shared.test.mjs", "easytier-web/frontend-lib/src/locales/cn.yaml", "easytier-web/frontend-lib/src/locales/en.yaml", "process/easytier-ecosystem-integration-done/plan/design/shared/"]
requirements: [ECO-02, ECO-03, ECO-06, ECO-07, ECO-08, ECO-10, ECO-15, ECO-16]
acceptance: [ECA-02, ECA-06, ECA-19, ECA-20, ECA-11, ECA-08, ECA-21, ECA-03, ECA-18, ECA-07, ECA-10]
parallel_group: eco-foundation
evidence: ["ECO-EV-004"]
issues: ["ECO-ISSUE-003"]
updated_at: 2026-10-21
---

# ECO-104：公共展示组件与语言资源

[汇总](../README.md) · [需求与验收](../require/README.md) · [PLAN-ECO-002](../plan/plan-eco-002-contract.md) · [任务看板](README.md)

当前状态与负责人见元数据；执行与验证证据见本任务交接记录。

## 目标与真实依据

以既有组件、主题和契约提供无主动拉取状态的公共展示组件，并维护拓扑专项语言键。

- [easytier-web/frontend/src/main.ts](https://github.com/CiBeDerORG/EasyTier/blob/1e92296b17fc2e85dcd4521b6012232e11abed95/easytier-web/frontend/src/main.ts) 的 `app.use(PrimeVue)`已接宿主主题；只复用现有机制，不另建全局主题。
- [easytier-web/frontend/src/components/MainPage.vue](https://github.com/CiBeDerORG/EasyTier/blob/1e92296b17fc2e85dcd4521b6012232e11abed95/easytier-web/frontend/src/components/MainPage.vue) 的 `loadNavigation`已有请求副作用，公共组件不能直接移植它来查询新增资源状态。

关联根汇总依据：ECO-EV-004；待核实项：ECO-ISSUE-003。文件存在只证明静态边界，未确认的接口/页面能力不得写成已实现或已验证。

## 输入、输出与启动条件

前置：[ECO-103 资源视图与按需查询契约](task-eco-103-domain-contract.md)

输入为103的类型/状态/事件约定；输出为拟新增 shared/ 组件、状态行为证据与 D/shared/ 文案/组件说明，语言资源仅加入本需求必要键。

未取得本任务依赖的契约、事实或环境时，只推进可独立核实部分，记录受影响结论；不为补页面效果编造接口、设备能力或成功状态。

## 写入范围与多 Agent 协作

路径均相对仓库根；`read_scope` 只读，`write_scope` 是本任务独占产物范围。尚不存在的 ecosystem 源文件、专项验证文件及 `plan/design/` 产物均为拟新增，不代表本轮已创建。

- `easytier-web/frontend/src/components/ecosystem/shared/`
- `easytier-web/frontend/tests/ecosystem-shared.test.mjs`
- `easytier-web/frontend-lib/src/locales/cn.yaml`
- `easytier-web/frontend-lib/src/locales/en.yaml`
- `process/easytier-ecosystem-integration-done/plan/design/shared/`

可与 ECO-105 并行，源码/文案与测试入口文件不重叠。

本需求唯一写入 cn.yaml、en.yaml 与 ecosystem/shared/；两个需求同时进行时，必须按跨需求调度领取文案文件写入权，不能与 WUI 文案任务并行抢写。 共享文件调度遵循[跨需求共享文件调度](../../README.md#跨需求共享文件调度)。根汇总、需求/计划索引、看板与任务状态由协调者单一维护；执行者交付结果，不覆盖他人状态。

## 实施步骤

1. 实现加载/空/失败/未知/无权限/未支持的纯展示边界；用户动作向宿主发事件，不自行轮询。
2. 复用主题和PrimeVue，处理中文/英文、键盘及长文本；公共文案由本任务独占合入。
3. 把页面后续提出的文案/公共组件需求汇集到本任务，追加修改须先暂停受影响消费者并交接。

## 完成与交接

- [x] 公共组件可独立消费103类型，不发隐式网络请求；调用者能表达真实结果与失败恢复。
- [x] 必要的组件行为验证和语言对照在实际实现阶段完成，证据交下游；本轮不执行。

交给 ECO-106～111；脚本入口问题回105，类型问题回103。

保护 `config-server` 客户端连接、认证、配置下发与接入命令；不新增心跳、轮询、定时任务、订阅或自动重放。仅修改本文件等文档时不构建、不测试、不运行 CI。未来实际代码验证须隔离工作树、构建产物、服务端口和数据；不能共写 frontend-lib/dist、frontend/dist 或争用固定端口，无法隔离时串行。

## 当前记录

2026-10-21：从对应计划拆出本任务，补充依据、独占范围、依赖和交接；状态为 backlog，owner 为 null，未实施、未验证。

2026-10-21：103验收后认领；共享locale/package当前归WUI103，先实施独占组件/支撑目录，交还锁后才能修改共享文件。

2026-10-21：监工验收通过，2纯展示组件、配对译文与6/6浏览器检查，真实后端未验证；后续由原所有者复用105公共harness保持断言。
