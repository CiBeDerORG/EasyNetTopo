---
id: ECO-107
feature: easytier-ecosystem-integration
summary: ../README.md
plans: [PLAN-ECO-004]
task_type: implementation
status: done
owner: /root/iteration_exec1
depends_on: ["ECO-104", "ECO-105"]
read_scope: ["process/easytier-ecosystem-integration-done/plan/design/contract.md", "easytier-web/frontend/src/modules/ecosystemTypes.ts", "easytier-web/frontend/src/components/ecosystem/shared/", "easytier-web/frontend/tests/ecosystem-support/", "process/easytier-ecosystem-integration-done/plan/design/baseline.md"]
write_scope: ["easytier-web/frontend/src/components/ecosystem/nodes/", "easytier-web/frontend/tests/ecosystem-nodes.test.mjs", "process/easytier-ecosystem-integration-done/plan/design/nodes/"]
requirements: [ECO-03, ECO-09, ECO-10]
acceptance: [ECA-11, ECA-09, ECA-16, ECA-10]
parallel_group: eco-pages
evidence: ["ECO-EV-001", "ECO-EV-002", "ECO-EV-005"]
issues: ["ECO-ISSUE-001"]
updated_at: 2026-10-21
---

# ECO-107：节点资料与类型目录

[汇总](../README.md) · [需求与验收](../require/README.md) · [PLAN-ECO-004](../plan/plan-eco-004-node-details.md) · [任务看板](README.md)

当前状态与负责人见元数据；执行与验证证据见本任务交接记录。

## 目标与真实依据

展示宿主资料、所选网络上下文与资源类型入口；有真实写入能力的字段才提供可完成的保存流程。

- [easytier-web/src/db/entity/devices.rs](https://github.com/CiBeDerORG/EasyTier/blob/1e92296b17fc2e85dcd4521b6012232e11abed95/easytier-web/src/db/entity/devices.rs) 的 `Model`已有hostname/alias/device_os等，未含人工地点/型号；[easytier-web/src/restful/network.rs](https://github.com/CiBeDerORG/EasyTier/blob/1e92296b17fc2e85dcd4521b6012232e11abed95/easytier-web/src/restful/network.rs) 的 `handle_update_machine_alias`只证明别名写入能力。

关联根汇总依据：ECO-EV-001、ECO-EV-002、ECO-EV-005；待核实项：ECO-ISSUE-001。文件存在只证明静态边界，未确认的接口/页面能力不得写成已实现或已验证。

## 输入、输出与启动条件

前置：[ECO-104 公共展示组件与语言资源](task-eco-104-shared-components.md)、[ECO-105 独立预览与验证入口](task-eco-105-preview-harness.md)

输入为 ECO-103 契约、ECO-104 公共组件/文案、ECO-105 独立验证接线；输出为本目录页面、局部样式、必要行为验证和设计说明。与其他页面通过已约定的 props/events、插槽和注入能力交接；生产数据由后续适配/组合层提供，开发样例不得冒充真实接口。

未取得本任务依赖的契约、事实或环境时，只推进可独立核实部分，记录受影响结论；不为补页面效果编造接口、设备能力或成功状态。

## 写入范围与多 Agent 协作

路径均相对仓库根；`read_scope` 只读，`write_scope` 是本任务独占产物范围。尚不存在的 ecosystem 源文件、专项验证文件及 `plan/design/` 产物均为拟新增，不代表本轮已创建。

- `easytier-web/frontend/src/components/ecosystem/nodes/`
- `easytier-web/frontend/tests/ecosystem-nodes.test.mjs`
- `process/easytier-ecosystem-integration-done/plan/design/nodes/`

可与 ECO-106～111 中除自身外的其他任务并行；write_scope 目录与专项测试互不重叠。

本任务不写跨需求共享文件；如发现需要扩围，先交协调者调整所有权，再安排串行交接。 共享文件调度遵循[跨需求共享文件调度](../../README.md#跨需求共享文件调度)。根汇总、需求/计划索引、看板与任务状态由协调者单一维护；执行者交付结果，不覆盖他人状态。

## 实施步骤

1. 人工资料与接口资料分开呈现，未知硬件不从hostname推断，GeoIP不能覆盖人工地点。
2. 根据契约处理待接入、无字段、无保存接口、无权限和保存失败；界面表单不能自行补数据库能力。
3. 通过类型选择事件交给组合页面；不直接导入另一个并行任务尚未完成的资源详情组件。

## 完成与交接

- [x] 覆盖本计划关联验收中的正常、空、失败、未知、无权限和未支持状态；真实能力范围服从已冻结接入清单。
- [x] 实际实现时自身完成类型/构建和针对性行为验证，中文/英文、主题、窄屏及键盘可用；未执行项记录原因。

向 ECO-112 交付组件输入输出、采用契约版本、文件清单与证据。契约缺陷回103，文案/公共组件回104，验证入口回105，接口语义回111；本任务不跨目录代改。

保护 `config-server` 客户端连接、认证、配置下发与接入命令；不新增心跳、轮询、定时任务、订阅或自动重放。仅修改本文件等文档时不构建、不测试、不运行 CI。未来实际代码验证须隔离工作树、构建产物、服务端口和数据；不能共写 frontend-lib/dist、frontend/dist 或争用固定端口，无法隔离时串行。

## 当前记录

2026-10-21：从对应计划拆出本任务，补充依据、独占范围、依赖和交接；状态为 backlog，owner 为 null，未实施、未验证。

2026-10-21：认领节点资料/资源类型入口，未支持人工字段不假保存，按103身份与能力契约。

2026-10-21：监工验收只读节点资料/资源导航通过；类型检查5/5浏览器含12布局，未知/人工字段/GeoIP分离，不代表真实设备读取。
