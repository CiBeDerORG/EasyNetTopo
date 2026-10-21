---
id: ECO-101
feature: easytier-ecosystem-integration
summary: ../README.md
plans: [PLAN-ECO-001]
task_type: analysis
status: done
owner: /root/iteration_exec1
depends_on: []
read_scope: ["easytier-web/frontend/src/modules/api.ts", "easytier-web/src/restful/network.rs", "easytier-web/src/db/entity/devices.rs", "easytier-web/src/db/entity/network_members.rs", "easytier-core/src/config/gateway.rs", "easytier-web/frontend/package.json"]
write_scope: ["process/easytier-ecosystem-integration-done/plan/design/baseline.md"]
requirements: [ECO-07, ECO-15]
acceptance: [ECA-03, ECA-18]
parallel_group: eco-discovery
evidence: ["ECO-EV-001", "ECO-EV-002", "ECO-EV-003", "ECO-EV-005"]
issues: ["ECO-ISSUE-001", "ECO-ISSUE-002"]
updated_at: 2026-10-21
---

# ECO-101：源码基线与接口能力清单

[汇总](../README.md) · [需求与验收](../require/README.md) · [PLAN-ECO-001](../plan/plan-eco-001-baseline.md) · [任务看板](README.md)

当前已完成静态基线核查并经监工验收；应用运行不属于本任务通过结论。

## 目标与真实依据

形成可核对的实现提交、现有接口与缺口清单，供后续契约引用；不设计不存在的外部 API。

- [easytier-web/src/db/entity/devices.rs](https://github.com/CiBeDerORG/EasyTier/blob/1e92296b17fc2e85dcd4521b6012232e11abed95/easytier-web/src/db/entity/devices.rs) 的 `Model`已有 user_id/machine_id、hostname、alias 和 device_os，未含人工地点、厂商/型号字段。
- [easytier-web/src/restful/network.rs](https://github.com/CiBeDerORG/EasyTier/blob/1e92296b17fc2e85dcd4521b6012232e11abed95/easytier-web/src/restful/network.rs) 的 `handle_list_machines / handle_update_machine_alias`提供已鉴权的机器查询与别名写入；这不能证明其他资料已有保存接口。
- [easytier-core/src/config/gateway.rs](https://github.com/CiBeDerORG/EasyTier/blob/1e92296b17fc2e85dcd4521b6012232e11abed95/easytier-core/src/config/gateway.rs) 的 `PortForwardConfig`只有端口转发配置原语，不能直接当成资源公网发布服务。

关联根汇总依据：ECO-EV-001、ECO-EV-002、ECO-EV-003、ECO-EV-005；待核实项：ECO-ISSUE-001、ECO-ISSUE-002。文件存在只证明静态边界，未确认的接口/页面能力不得写成已实现或已验证。

## 输入、输出与启动条件

前置：无任务前置；执行前核对采用提交与适用环境。

输入为用户已确定范围、交付提交及可读取源码；输出为 D/baseline.md，列能力、实际方法/路径/DTO/授权/错误、来源提交、可用性证据与缺口。未提供外部项目来源时写待核实及影响。

未取得本任务依赖的契约、事实或环境时，只推进可独立核实部分，记录受影响结论；不为补页面效果编造接口、设备能力或成功状态。

## 写入范围与多 Agent 协作

路径均相对仓库根；`read_scope` 只读，`write_scope` 是本任务独占产物范围。尚不存在的 ecosystem 源文件、专项验证文件及 `plan/design/` 产物均为拟新增，不代表本轮已创建。

- `process/easytier-ecosystem-integration-done/plan/design/baseline.md`

可与 ECO-102 及 WUI 独立基线核查并行，分别写不同证据文件。

本任务不写跨需求共享文件；如发现需要扩围，先交协调者调整所有权，再安排串行交接。 共享文件调度遵循[跨需求共享文件调度](../../README.md#跨需求共享文件调度)。根汇总、需求/计划索引、看板与任务状态由协调者单一维护；执行者交付结果，不覆盖他人状态。

## 实施步骤

1. 只读核对启动时提交与已记录交付基线的差异，不把文档分支缺失认作功能缺失。
2. 追踪页面客户端到处理器及模型的必要调用链；逐项区分接口存在、语义不足、未发现及运行未验证。
3. 核对脚本入口、浏览器/隔离服务前置，仅记录，不为写清单运行构建或测试。

## 完成与交接

- [x] 每个拟接入操作有真实来源或明确缺口；不存在凭命名推断出的 API。
- [x] 记录宿主/业务设备、GeoIP/人工地点、配置原语/公网发布的区别，并列出仍需 ECO-103 决定的范围。

交给 ECO-103；发现运行问题交 ECO-102 核实，不能把本任务的静态核查标成运行验证通过。

保护 `config-server` 客户端连接、认证、配置下发与接入命令；不新增心跳、轮询、定时任务、订阅或自动重放。仅修改本文件等文档时不构建、不测试、不运行 CI。未来实际代码验证须隔离工作树、构建产物、服务端口和数据；不能共写 frontend-lib/dist、frontend/dist 或争用固定端口，无法隔离时串行。

## 当前记录

2026-10-21：从对应计划拆出本任务，补充依据、独占范围、依赖和交接；状态为 backlog，owner 为 null，未实施、未验证。

2026-10-21：用户授权启动，由 `/root/iteration_exec1` 认领，基线1e92296b；正在核查，尚未交付或验收。状态以本次认领及frontmatter为准。

2026-10-21：交付基线产物并经 `/root/iteration_monitor` 审查通过，源码入口及文档链接已核对，未运行应用测试。
