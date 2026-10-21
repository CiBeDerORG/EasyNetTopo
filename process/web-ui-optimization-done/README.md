---
feature: web-ui-optimization
status: done
baseline: feat/web-user-management@1e92296b17fc2e85dcd4521b6012232e11abed95
updated_at: 2026-10-21
---

# 现有 Web 交互优化与页面美化需求/计划汇总

本需求独立管理既有 Web 页面的交互与视觉优化，基于已完成的用户管理功能开展，与拓扑集成分别管理依赖。WUI-101～113已全部验收；真实与fixture证据、未验证范围见最终验收。

本文件作为唯一的需求/计划汇总，先记录现状、进度与问题，再据此展开需求、计划和具体任务。页面问题必须来自实际证据，不能只凭组件名称推断布局缺陷或虚构用户反馈。

当前任务进度见下方实施快照与任务明细。

## 基线与核查范围

2026-10-21 只读核对的实现基线为 `feat/web-user-management@1e92296b17fc2e85dcd4521b6012232e11abed95`，实现工作树无未提交改动。采用文档快照为 `docs/architecture-and-user-management-plan@670eb3b42800d42333356da0cb15f41d76c92f1e`。没有把实现合并到文档分支；当前核对只确认源码入口和约束，不能当作运行截图、体验审计或测试结果。

## 进度与内容入口

| 内容 | 文档进度 | 实现进度 | 下一步 |
| --- | --- | --- | --- |
| [require/README.md](require/README.md) | 已整理 WUI-01～05、WUA-01～04 及兼容边界 | 已完成真实前态取证及WUI-103～111优化验收 | 已完成前后对照；见[验收记录](plan/acceptance.md) |
| [PLAN-WUI](plan/README.md) | 8个工作包各有 `plan-wui-编号-名称.md`，索引列明依赖与文件所有权 | 各计划状态按关联任务更新；实现见看板 | 已联调并归档 |
| [task/README.md](task/README.md) | 已生成13个小任务、依赖图和共享文件安排 | 见当前实施快照 | 冻结111产物联调已完成，ECO最终宿主另行验收 |

总体为 `done`。文档形成与页面实现、运行验收分别记录。

## 当前事实与代码依据

以下源码链接固定到已核查的交付提交；需要进一步体验证据的判断明确保留为待核实。

| 编号 | 类型 | 已核查内容 | 代码依据与设计约束 |
| --- | --- | --- | --- |
| WUI-EV-001 | 事实 | 现有登录、改密、用户、Dashboard、设备及网络路由已存在 | [main.ts](https://github.com/CiBeDerORG/EasyTier/blob/1e92296b17fc2e85dcd4521b6012232e11abed95/easytier-web/frontend/src/main.ts) 的 `routes` 包含 `/auth`、`/h/users`、`/h/password`、`/h/deviceList`、`/h/networks` 等；优化保留原入口和路由语义 |
| WUI-EV-002 | 事实 | 本人改密区分本地密码能力，接入命令有现有生成逻辑 | [ChangePassword.vue](https://github.com/CiBeDerORG/EasyTier/blob/1e92296b17fc2e85dcd4521b6012232e11abed95/easytier-web/frontend/src/components/ChangePassword.vue) 的 `canChangePassword`；[MainPage.vue](https://github.com/CiBeDerORG/EasyTier/blob/1e92296b17fc2e85dcd4521b6012232e11abed95/easytier-web/frontend/src/components/MainPage.vue) 的 `enrollCommand` 和 `copyEnrollCommand`，须保留 OIDC/local 边界及命令内容 |
| WUI-EV-003 | 事实 | Web 已有 PrimeVue 主题，无需假定从零建设 | [theme.ts](https://github.com/CiBeDerORG/EasyTier/blob/1e92296b17fc2e85dcd4521b6012232e11abed95/easytier-web/frontend/src/theme.ts) 使用 `definePreset(Aura, ...)` 并定义明暗主题语义；视觉规范应在既有机制上调整 |
| WUI-EV-004 | 事实 | 原页面含周期查询，需记录基线以防增加 | [MainPage.vue](https://github.com/CiBeDerORG/EasyTier/blob/1e92296b17fc2e85dcd4521b6012232e11abed95/easytier-web/frontend/src/components/MainPage.vue) 的 10 秒导航刷新、[NetworkDetail.vue](https://github.com/CiBeDerORG/EasyTier/blob/1e92296b17fc2e85dcd4521b6012232e11abed95/easytier-web/frontend/src/components/NetworkDetail.vue) 的 5 秒节点详情刷新；本专项不增加轮询、心跳、订阅或定时任务，也不借美化重写底层机制 |
| WUI-EV-005 | 待核实 | 哪些页面存在反馈不清、焦点、窄屏或视觉层级问题及其优先级 | 102真实前态及112真实后态证据已完成，三项观察缺陷已验收关闭 |
| WUI-EV-006 | 事实 | 各页面和回归存在独立与集中所有权边界 | 本轮静态读取交付提交中的 [Dashboard.vue](https://github.com/CiBeDerORG/EasyTier/blob/1e92296b17fc2e85dcd4521b6012232e11abed95/easytier-web/frontend/src/components/Dashboard.vue) 的 `loadSummary/PeriodicTask(2000)`、[DeviceList.vue](https://github.com/CiBeDerORG/EasyTier/blob/1e92296b17fc2e85dcd4521b6012232e11abed95/easytier-web/frontend/src/components/DeviceList.vue) 的 `enrollCommand/PeriodicTask(1000)`、[NetworkList.vue](https://github.com/CiBeDerORG/EasyTier/blob/1e92296b17fc2e85dcd4521b6012232e11abed95/easytier-web/frontend/src/components/NetworkList.vue) 的 `loadNetworks/PeriodicTask(2000)`；NetworkDetail另有2秒主刷新，ACL按networkId的watch加载。原 `tests/dashboard.test.mjs` 是跨页面mock回归，归WUI-112；`user-management-navigation.test.mjs`跨宿主/身份，归WUI-111，二者不能冒充真实后端验证 |

## 问题、限制与下一步

| 编号 | 问题与来源 | 当前状态 | 影响 | 下一步/解除条件 | 负责人 |
| --- | --- | --- | --- | --- | --- |
| WUI-ISSUE-001 | 缺少本专项页面前态、实际体验问题与优先级，见[需求当前记录](require/README.md#当前记录与待明确内容) | 已由101/102核实 | 基线真实问题已分配并修复 | 见require/evidence与各页面交接，112作最终对照 | 总控与对应任务所有者 |
| WUI-ISSUE-002 | 原页面定时请求和对象切换行为，见上述代码 | 101已固定来源，112补最终运行观察 | 复用或接线可能意外增加请求/改变既有行为 | 保留101/103/111已验收边界，112检查空闲请求 | 总控与对应任务所有者 |
| WUI-ISSUE-003 | 浏览器与隔离测试环境可用性，及用户管理交付的旧 Dashboard 浏览器限制，见[历史记录](../web-user-management-done/plan/operations.md) | 系统Chromium与隔离后端已核实，112完整集中及真实回归已通过 | 后续真实联调须先确认适用运行入口，不能沿用历史通过数当新结果 | 实际代码阶段按采用提交核对环境和原有回归，再执行WUI-112，证据交WUI-113验收 | 总控与对应任务所有者 |
| WUI-ISSUE-004 | 与拓扑可能同时写 MainPage、main.ts、主题、文案或脚本，见[跨需求协调](plan/README.md#与其他需求的协调) | 共享文件已按原所有者串行实施并交接 | 共享文件并行写会冲突，独立 worktree 不能消除语义冲突 | WUI-103公共、WUI-111宿主、WUI-112集中回归按[共享文件调度](../README.md#跨需求共享文件调度)领取/交还，独立页面按任务并行 | 总控与对应任务所有者 |

这里只汇总已知问题和未核实项，不宣称已完成全量体验检查。保护 `config-server` 客户端连接、认证、配置下发及接入命令，不以美化为由删除现有功能。

关联：[已交付用户管理](../web-user-management-done/README.md)、[独立拓扑集成需求](../easytier-ecosystem-integration-done/README.md)、[全局协作规则与需求索引](../README.md)。

## 拆分来源

原拓扑文档中的 `ECO-17`、`ECA-22～25` 和工作包 `ECO-012` 已迁入本需求，不再作为拓扑计划的实施责任或前置条件。对应关系见[需求编号迁移表](require/README.md#编号迁移)。旧编号保留迁移记录，不复用，也不表示这些功能已经实施。

## 文档变更规则

仅修改本需求的分析、计划、任务、状态、排版或链接时，不运行构建、单元/集成/E2E 测试、CI，也不安装测试依赖；只核对必要的路径、链接和状态一致性。实际页面或其他代码修改另按真实变更范围验证。本次目录整理不修改 CI 配置。

## 维护记录

2026-10-21（前次整理）：按交付基线只读核查路由、改密、接入命令、主题和部分请求入口，补齐进度与问题汇总。当时计划仍是待细化工作包，尚未生成任务，也未取得本专项页面缺陷证据；未运行构建、测试或 CI。

2026-10-21（前次整理）：按职责拆为8份计划明细，当时仍未生成或认领任务。

2026-10-21（本次拆分）：结合交付源码和现有计划生成13项任务，补齐read_scope/write_scope、依赖与并行批次、公共文件单写入者及接线/联调/验收。新增静态依据WUI-EV-006；未发现或宣称未经取证的视觉缺陷。全部backlog/null，没有运行构建、测试或CI。


## 本轮运行发现（2026-10-21）

- WUI-107：窄屏卡片偏好重复显示、别名重复提交已修复并验收；见任务交接。
- WUI-108：跨宿主同实例UUID保留旧元数据，已由宿主deviceId生命周期修复并验收。
- WUI-109：创建pending可关闭/重开改草稿，已修复并验收；不改变原CIDR语义。
- WUI-110：真实组件HTTP fixture复现ACL读取503仍显示默认放行/无规则、保存pending可编辑规则；已修复并通过监工验收（专项6/6、既有15/15）。必须区分未知策略与成功读取，不能提交假默认覆盖。
- WUI-112已处理：显式系统Chromium并补真实形状auth/me fixture，完整集中33/33及真实9/9通过，未改生产契约。


## 联调进行记录

WUI-112 集中回归33/33通过（WUI-111冻结dist、HTTP fixture），真实E2E首轮7/9：测试代理默认changeOrigin/CORS改变请求安全语义，已修正测试代理并重跑9/9通过，不修改生产安全策略。真实矩阵初始虚构用户名超过既有长度限制被400拒绝，修正合法测试输入后重跑，最终96项矩阵通过；首轮失败独立保留。最终证据以112交接为准。


## 当前实施快照

已验收 13/13；；其余 0 项 backlog。

总控维护任务与共享锁；监工 `/root/iteration_monitor` 审查后才能完成或进入联调。实际验证见各任务交接，未运行场景不作通过结论。

## 最终交付

本期 WUI-101～113 全部验收；[最终验收](plan/acceptance.md)及[真实联调记录](plan/verification/README.md)保留初次失败与定向复验、16张截图及96项矩阵。真实设备/VPN、生产OIDC、HTTPS代理、部署与旧二进制回退未验证。
