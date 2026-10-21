---
id: ECO-111
feature: easytier-ecosystem-integration
summary: ../README.md
plans: [PLAN-ECO-008]
task_type: implementation
status: done
owner: /root/iteration_exec3
depends_on: ["ECO-104", "ECO-105"]
read_scope: ["process/easytier-ecosystem-integration-done/plan/design/contract.md", "process/easytier-ecosystem-integration-done/plan/design/baseline.md", "easytier-web/frontend/src/modules/ecosystemTypes.ts", "easytier-web/src/restful/network.rs", "easytier-web/src/db/entity/devices.rs", "easytier-web/src/db/entity/network_members.rs"]
write_scope: ["easytier-web/frontend/src/modules/api.ts", "easytier-web/frontend/src/modules/ecosystemApi.ts", "easytier-web/frontend/tests/ecosystem-api.test.mjs", "process/easytier-ecosystem-integration-done/plan/design/api/"]
requirements: [ECO-02, ECO-04, ECO-05, ECO-06, ECO-07, ECO-08, ECO-09, ECO-11, ECO-12, ECO-13, ECO-16]
acceptance: [ECA-02, ECA-06, ECA-19, ECA-20, ECA-04, ECA-05, ECA-08, ECA-21, ECA-03, ECA-18, ECA-07, ECA-09, ECA-16, ECA-12, ECA-14, ECA-13, ECA-15]
parallel_group: eco-pages
evidence: ["ECO-EV-001", "ECO-EV-002", "ECO-EV-003", "ECO-EV-005"]
issues: ["ECO-ISSUE-001", "ECO-ISSUE-002"]
updated_at: 2026-10-21
---

# ECO-111：现有接口适配与请求结束

[汇总](../README.md) · [需求与验收](../require/README.md) · [PLAN-ECO-008](../plan/plan-eco-008-api-adapter.md) · [任务看板](README.md)

当前状态与负责人见元数据；执行与验证证据见本任务交接记录。

## 目标与真实依据

把真实存在的接口接入拓扑视图能力，集中转换已有DTO，保证按需查询结束且迟到响应不会串对象。

- [easytier-web/frontend/src/modules/api.ts](https://github.com/CiBeDerORG/EasyTier/blob/1e92296b17fc2e85dcd4521b6012232e11abed95/easytier-web/frontend/src/modules/api.ts) 的 `ApiClient / list_networks / list_network_members`提供既有认证客户端和网络查询；[easytier-web/src/restful/network.rs](https://github.com/CiBeDerORG/EasyTier/blob/1e92296b17fc2e85dcd4521b6012232e11abed95/easytier-web/src/restful/network.rs) 的 `handle_list_machines`按用户归属返回机器数据。

关联根汇总依据：ECO-EV-001、ECO-EV-002、ECO-EV-003、ECO-EV-005；待核实项：ECO-ISSUE-001、ECO-ISSUE-002。文件存在只证明静态边界，未确认的接口/页面能力不得写成已实现或已验证。

## 输入、输出与启动条件

前置：[ECO-104 公共展示组件与语言资源](task-eco-104-shared-components.md)、[ECO-105 独立预览与验证入口](task-eco-105-preview-harness.md)

输入为101核查记录与103固定接入清单/类型；输出为必要的api.ts小范围扩展、拟新增ecosystemApi.ts、契约行为验证和D/api/能力缺口记录。不得创建假定的 /resources、/navloom 等服务端路径。

未取得本任务依赖的契约、事实或环境时，只推进可独立核实部分，记录受影响结论；不为补页面效果编造接口、设备能力或成功状态。

## 写入范围与多 Agent 协作

路径均相对仓库根；`read_scope` 只读，`write_scope` 是本任务独占产物范围。尚不存在的 ecosystem 源文件、专项验证文件及 `plan/design/` 产物均为拟新增，不代表本轮已创建。

- `easytier-web/frontend/src/modules/api.ts`
- `easytier-web/frontend/src/modules/ecosystemApi.ts`
- `easytier-web/frontend/tests/ecosystem-api.test.mjs`
- `process/easytier-ecosystem-integration-done/plan/design/api/`

可与 ECO-106～110 五组页面并行，消费者只依赖103接口契约。

本需求唯一写入 api.ts、ecosystemApi.ts；WUI对api.ts只读。共享类型归103，页面和MainPage不属本任务。 共享文件调度遵循[跨需求共享文件调度](../../README.md#跨需求共享文件调度)。根汇总、需求/计划索引、看板与任务状态由协调者单一维护；执行者交付结果，不覆盖他人状态。

## 实施步骤

1. 只调用已核查方法/路径，保留snake_case契约、统一认证和401/403/业务错误处理；转换仅在适配边界。
2. 数据读取由进入、切换、手动刷新和明确操作触发；取消或忽略过期结果，操作响应后最多按需查询受影响对象一次。
3. 记录无接口/权限/环境的能力；不轮询、自动重试/重放或建立WebSocket/SSE，不接入旧定时器。

## 完成与交接

- [x] 接口行为、错误、未知结果和迟到响应在实际实现阶段有针对性证据；mock只证明相应前端契约。
- [x] 无后端/协议/config-server改动；不改变既有客户端消费者和账号错误语义。

交给 ECO-112；接口契约问题回103，已有后端缺口记录到汇总，不扩大为后端任务。

保护 `config-server` 客户端连接、认证、配置下发与接入命令；不新增心跳、轮询、定时任务、订阅或自动重放。仅修改本文件等文档时不构建、不测试、不运行 CI。未来实际代码验证须隔离工作树、构建产物、服务端口和数据；不能共写 frontend-lib/dist、frontend/dist 或争用固定端口，无法隔离时串行。

## 当前记录

2026-10-21：从对应计划拆出本任务，补充依据、独占范围、依赖和交接；状态为 backlog，owner 为 null，未实施、未验证。

2026-10-21：104/105通过后认领实际API适配与专属验证；真实服务联调留114，禁止虚构后端或静默空数据。

2026-10-21：监工审查通过，类型检查、9/9Axios本地HTTP适配及6/6旧客户端回归通过；info=null缺身份明确报错，真实API联调待114。
