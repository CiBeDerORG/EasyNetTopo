---
id: ECO-113
feature: easytier-ecosystem-integration
summary: ../README.md
plans: [PLAN-ECO-009]
task_type: implementation
status: done
owner: /root/iteration_exec3
depends_on: ["ECO-112"]
read_scope: ["easytier-web/frontend/src/components/ecosystem/EcosystemPage.vue", "easytier-web/frontend/src/modules/api.ts", "easytier-web/frontend/src/components/ChangePassword.vue", "process/easytier-ecosystem-integration-done/plan/design/integration-composition.md"]
write_scope: ["easytier-web/frontend/src/main.ts", "easytier-web/frontend/src/components/MainPage.vue", "easytier-web/frontend/tests/ecosystem-host.test.mjs", "process/easytier-ecosystem-integration-done/plan/design/integration.md"]
requirements: [ECO-01, ECO-07, ECO-15]
acceptance: [ECA-01, ECA-15, ECA-03, ECA-18]
parallel_group: eco-host-integration
evidence: ["ECO-EV-004"]
issues: ["ECO-ISSUE-003"]
updated_at: 2026-10-21
---

# ECO-113：正式路由与宿主导航接线

[汇总](../README.md) · [需求与验收](../require/README.md) · [PLAN-ECO-009](../plan/plan-eco-009-host-integration.md) · [任务看板](README.md)

当前状态与负责人见元数据；执行与验证证据见本任务交接记录。

## 目标与真实依据

将已组合页面接入正式路由和导航，保留已有账号、主题、网络配置及设备接入行为。

- [easytier-web/frontend/src/main.ts](https://github.com/CiBeDerORG/EasyTier/blob/1e92296b17fc2e85dcd4521b6012232e11abed95/easytier-web/frontend/src/main.ts) 的 `routes`已有auth、users、password、deviceList和networks及别名；旧路由须保持。
- [easytier-web/frontend/src/components/MainPage.vue](https://github.com/CiBeDerORG/EasyTier/blob/1e92296b17fc2e85dcd4521b6012232e11abed95/easytier-web/frontend/src/components/MainPage.vue) 的 `enrollCommand / copyEnrollCommand / loadNavigation`分别承担config-server接入命令、复制与导航加载；不得借拓扑接线改变其既有语义。

关联根汇总依据：ECO-EV-004；待核实项：ECO-ISSUE-003。文件存在只证明静态边界，未确认的接口/页面能力不得写成已实现或已验证。

## 输入、输出与启动条件

前置：[ECO-112 拓扑页面组合与能力注入](task-eco-112-page-composition.md)

输入为112可用组合页、既有宿主行为基线及共享文件交接记录；输出为正式菜单/路由接线、宿主兼容证据和D/integration.md。

未取得本任务依赖的契约、事实或环境时，只推进可独立核实部分，记录受影响结论；不为补页面效果编造接口、设备能力或成功状态。

## 写入范围与多 Agent 协作

路径均相对仓库根；`read_scope` 只读，`write_scope` 是本任务独占产物范围。尚不存在的 ecosystem 源文件、专项验证文件及 `plan/design/` 产物均为拟新增，不代表本轮已创建。

- `easytier-web/frontend/src/main.ts`
- `easytier-web/frontend/src/components/MainPage.vue`
- `easytier-web/frontend/tests/ecosystem-host.test.mjs`
- `process/easytier-ecosystem-integration-done/plan/design/integration.md`

不与任何MainPage/main.ts写入者并行；与其他需求无共享写范围的页面任务可并行。

本需求唯一写入 MainPage.vue 和 main.ts；与 WUI-111 互斥。双方同时就绪默认 WUI先、ECO后，只有一项就绪可独立执行，不增加跨需求硬depends_on；保留用户管理已交付状态处理与凭据清除。 共享文件调度遵循[跨需求共享文件调度](../../README.md#跨需求共享文件调度)。根汇总、需求/计划索引、看板与任务状态由协调者单一维护；执行者交付结果，不覆盖他人状态。

## 实施步骤

1. 只增加本期已确定入口和注入，保留原用户身份、角色能力刷新、迟到响应处理、强制改密与临时凭据清除。
2. 保护config-server协议/地址/端口/参数/复制内容，以及客户端连接、认证、配置下发；不扩大旧导航计时器职责。
3. 领取共享文件写入权后修改，记录采用提交/变更段及交回对象；缺陷回对应页面或契约任务，不并发代改。

## 完成与交接

- [x] 原路由/深链接、用户入口与权限、命令复制、主题行为继续可用，新增资源查询没有周期副作用。
- [x] MainPage/main.ts跨需求交接完成，正式接线结果可供114在同一提交真实联调。

交给 ECO-114；宿主回归由本任务修正，公共类型回103，页面/适配回各任务。

保护 `config-server` 客户端连接、认证、配置下发与接入命令；不新增心跳、轮询、定时任务、订阅或自动重放。仅修改本文件等文档时不构建、不测试、不运行 CI。未来实际代码验证须隔离工作树、构建产物、服务端口和数据；不能共写 frontend-lib/dist、frontend/dist 或争用固定端口，无法隔离时串行。

## 当前记录

2026-10-21：从对应计划拆出本任务，补充依据、独占范围、依赖和交接；状态为 backlog，owner 为 null，未实施、未验证。

2026-10-21：ECO112和WUI111已验收，领取MainPage/main.ts及构建锁，保留WUI111语义与焦点改动。

2026-10-21：监工验收正式宿主接线：生产preview22/22（生态5+导航17），中英文bundle已验证；先lib后frontend类型与Vite通过。完整codegen缺protoc且DNS错误未通过，schema与生成源未改。
