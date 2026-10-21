---
id: PLAN-WUI
feature: web-ui-optimization
status: done
summary: ../README.md
requirement: ../require/README.md
task_board: ../task/README.md
updated_at: 2026-10-21
---

# 现有 Web 交互优化与页面美化计划索引

入口：[需求/计划汇总](../README.md)、[WUI-01～05 与 WUA-01～04](../require/README.md)、[任务入口](../task/README.md)。本计划替代原拓扑工作包 `ECO-012` 的执行安排，与拓扑开发独立；当前8份计划已细化为13项任务；认领与验收以任务明细为事实源，当前汇总见下方实施快照。

## 基础与实施顺序

采用基线、已核查源码和待核实项统一引用根汇总的 [WUI-EV-001～006](../README.md#当前事实与代码依据) 与 [WUI-ISSUE-001～004](../README.md#问题限制与下一步)。启动时确认 `feat/web-user-management@1e92296b17fc2e85dcd4521b6012232e11abed95` 或后续采用提交的差异，不重做用户管理，不等待尚未接入的拓扑、EasyADB、家居或导航后端；未取证的页面问题仍是待核实。

1. WUI-001 核查原任务、页面、接口和请求来源，取得实际问题与前态证据。
2. WUI-002 建立公共视觉/交互、页面输入输出及独立预览/验证约定。
3. WUI-003～006 对应的WUI-104～110按七组不重叠页面并行，实际实现时各自完成构建与针对性行为验证。
4. WUI-007 串行接线宿主、导航和页面；WUI-008 最后单独真实联调、兼容回归和验收。

## 独立计划目录

各文件包含目标、依据、待核实项、前置/输出、写入范围及退出条件。工作包编号保持不变，任务使用独立的WUI-101～113编号；以下 `draft` 表示计划状态，不表示已执行进度。

| 计划 | 工作包 / 职责 | 前置 | 状态 |
| --- | --- | --- | --- |
| [PLAN-WUI-001](plan-wui-001-baseline.md) | WUI-001 基线与体验问题清单 | 已交付用户管理基线 | draft |
| [PLAN-WUI-002](plan-wui-002-shared-foundation.md) | WUI-002 公共视觉/交互、文案及验证基础 | WUI-001 | draft |
| [PLAN-WUI-003](plan-wui-003-accounts.md) | WUI-003 账号与用户页面 | WUI-002 | draft |
| [PLAN-WUI-004](plan-wui-004-dashboard.md) | WUI-004 工作台页面 | WUI-002 | draft |
| [PLAN-WUI-005](plan-wui-005-devices.md) | WUI-005 设备与实例页面 | WUI-002 | draft |
| [PLAN-WUI-006](plan-wui-006-networks.md) | WUI-006 网络页面 | WUI-002 | draft |
| [PLAN-WUI-007](plan-wui-007-integration.md) | WUI-007 宿主壳层与接线 | WUI-003～006 | draft |
| [PLAN-WUI-008](plan-wui-008-acceptance.md) | WUI-008 真实联调与验收 | WUI-007 | draft |

## 计划到任务的追踪

| 小计划 | 已生成任务 |
| --- | --- |
| [PLAN-WUI-001](plan-wui-001-baseline.md) | [WUI-101 源码、路由与请求基线](../task/task-wui-101-source-baseline.md)、[WUI-102 现有页面前态与问题取证](../task/task-wui-102-page-evidence.md) |
| [PLAN-WUI-002](plan-wui-002-shared-foundation.md) | [WUI-103 公共契约、主题与文案基础](../task/task-wui-103-shared-foundation.md) |
| [PLAN-WUI-003](plan-wui-003-accounts.md) | [WUI-104 登录注册与本人改密页面](../task/task-wui-104-auth-pages.md)、[WUI-105 用户管理组件交互与视觉](../task/task-wui-105-user-management.md) |
| [PLAN-WUI-004](plan-wui-004-dashboard.md) | [WUI-106 工作台信息与操作反馈](../task/task-wui-106-dashboard.md) |
| [PLAN-WUI-005](plan-wui-005-devices.md) | [WUI-107 设备列表与设备资料展示](../task/task-wui-107-device-list.md)、[WUI-108 实例配置宿主交互](../task/task-wui-108-instance-host.md) |
| [PLAN-WUI-006](plan-wui-006-networks.md) | [WUI-109 网络列表与创建入口](../task/task-wui-109-network-list.md)、[WUI-110 网络详情与 ACL 交互](../task/task-wui-110-network-detail-acl.md) |
| [PLAN-WUI-007](plan-wui-007-integration.md) | [WUI-111 宿主壳层与页面接线](../task/task-wui-111-host-integration.md) |
| [PLAN-WUI-008](plan-wui-008-acceptance.md) | [WUI-112 真实组合联调与集中回归](../task/task-wui-112-real-integration.md)、[WUI-113 验收归档与交接](../task/task-wui-113-acceptance-handoff.md) |

## 公共边界与唯一写入者

| 公共范围 | 本需求内负责任务 |
| --- | --- |
| Web 公共样式、theme、共享 cn/en 文案、必要 package.json 测试接线与专项测试支持 | WUI-103；页面任务提交需求，不直接抢写 |
| MainPage.vue、main.ts 与页面接线 | WUI-111；页面交付后串行处理，并维护跨页user-management-navigation回归 |
| dashboard.test.mjs、user-management-e2e.test.mjs 的必要集中回归调整 | WUI-112；缺陷回派原模块 |
| 根汇总、需求正文、计划索引、看板和任务元数据 | 协调者；各包设计/证据记录按独立计划范围分配 |

各页面只写独占文件和局部样式；精确范围见对应计划与现有任务明细的 `write_scope`。API 客户端 `src/modules/api.ts`、`userManagement.ts`、主题持久化逻辑、共享网络/配置组件、后端、协议、构建依赖与锁文件默认只读。必要扩围先更新需求与所有权，不由页面执行者自行扩大。

保持原 API、`snake_case`、路由、权限及数据模型，保护 OIDC/local、注册策略、强制改密和临时凭据清除。`config-server` 连接、认证、配置下发及接入命令的协议、地址、端口、参数和复制内容不变。不新增定时器、轮询、自动刷新、心跳、WebSocket/SSE、后台扫描或自动重放，不扩大旧计时器职责，也不借美化重写必要底层机制。

## 与其他需求的协调

本需求与[拓扑集成](../../easytier-ecosystem-integration-done/README.md)不存在待实现工作包的硬前置关系。若同时推进，协调者对 `MainPage.vue`、`main.ts`、主题/样式、locales、`package.json` 和 API 入口建立跨需求唯一写入安排，明确先后提交及接收方，再继续相关写入。独立页面仍可并行，隔离 worktree 不能替代共享语义协调。具体互斥文件和领取/交还顺序统一见[共享文件调度](../../README.md#跨需求共享文件调度)；只有一项就绪可独立执行，两项同时就绪默认WUI先、ECO后。后续构建/浏览器验证还须隔离产物目录与端口，不能隔离时串行。

拓扑只引用本需求交付，不继续认领 `ECO-012` 或重复维护本需求状态；本计划不认领拓扑新页面/新接口。用户管理是已交付基础，不重新开启 WUM 实施任务。

## 验收映射与交付

| 验收 | 模块内证据 | 接线与统一联调 |
| --- | --- | --- |
| WUA-01 原任务与入口 | WUI-003～006 原流程及失败/边界 | WUI-007/008 对照基线复验入口、深链接、前进/返回及角色 |
| WUA-02 交互反馈 | WUI-002 约定，WUI-003～006 各自验证 | WUI-008 用隔离服务/数据核对权限、保存/应用及对象切换 |
| WUA-03 视觉与可访问性 | WUI-002～006 主题、语言、360/768/1440 宽度及键盘 | WUI-008 对比真实前后截图、长文本、对比及焦点 |
| WUA-04 账号与连接兼容 | WUI-003 保留账号边界，WUI-005/007 保留设备入口及接入命令 | WUI-008 用真实用户回归、网络记录及源码差异确认兼容，无新增周期机制 |

实际脚本、环境核实、失败回派与退出条件见 [WUI-008](plan-wui-008-acceptance.md)。交付问题清单、前后证据、真实验证结果与剩余限制；未执行或仍失败的承诺验收不能标为完成。代码按页面/公共样式/接线提交回退；回退前端不撤销用户已经通过现有接口保存的业务配置。

## 当前记录与文档规则

2026-10-21（前次整理）：从原计划拆出8份独立计划文件，当时尚未生成任务明细。

2026-10-21（本次拆分）：8份小计划已继续拆成WUI-101～113共13项任务，全部backlog/null；7项页面任务可按槽位并行，之后111接线、112真实联调、113验收。见[任务看板](../task/README.md)，未进入开发。

本次以及其他仅文档/状态/目录变更不构建、不运行测试或 CI、不安装验证依赖，只检查必要目录、链接和内容一致性，不修改 CI 配置。各计划中的应用验证均属于后续实际代码实施。


## 当前实施快照

已验收 13/13；；其余 0 项 backlog。

总控维护任务与共享锁；监工 `/root/iteration_monitor` 审查后才能完成或进入联调。实际验证见各任务交接，未运行场景不作通过结论。
