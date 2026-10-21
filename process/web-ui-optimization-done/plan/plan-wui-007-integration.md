---
id: PLAN-WUI-007
feature: web-ui-optimization
work_package: WUI-007
summary: ../README.md
status: done
requirement: ../require/README.md
task_board: ../task/README.md
updated_at: 2026-10-21
---

# WUI-007：宿主壳层与接线

入口：[汇总](../README.md)、[计划索引](README.md)、[任务入口](../task/README.md)。当前仍是实施前的计划草案；已按下表生成具体任务，全部未认领、未执行。

## 目标与需求

在四组页面交付后统一宿主壳层、导航、标题、内容区及页面输入输出，保持原入口和已有业务能力。对应 [WUI-01～05、WUA-01～04](../require/README.md)；本包负责接线，最终真实组合验收另由 WUI-008 执行。

## 已知基础与待核实项

- [WUI-EV-001/002/004](../README.md#当前事实与代码依据)已核对 `main.ts` 的现有路由、MainPage 的接入命令生成/复制和 10 秒导航刷新。静态核对不等于页面前进/返回或网络请求已运行验证。
- [WUI-ISSUE-002/004](../README.md#问题限制与下一步)中的完整请求基线和共享文件写入安排必须在受影响接线前明确；独立 worktree 不能消除语义冲突。
- 采用提交及变化范围以根汇总和 WUI-001 交付为准，具体壳层问题不能仅凭页面名称推断。

## 前置、输入与输出

前置为 [WUI-003 账号](plan-wui-003-accounts.md)、[WUI-004 工作台](plan-wui-004-dashboard.md)、[WUI-005 设备](plan-wui-005-devices.md)、[WUI-006 网络](plan-wui-006-networks.md) 的模块交付，以及 WUI-002 的公共契约。

输入包含各页面输入输出、单项构建/行为证据、公共变更和剩余限制。输出为完成接线的宿主、路由/导航及页面组合说明、针对性验证与交给 WUI-008 的可运行应用。保留原路由/别名/深链接、前进/返回、主题切换、接入命令和能力判断。

## 写入范围与公共所有权

- WUI-111在本需求唯一写入 `easytier-web/frontend/src/components/MainPage.vue`、`easytier-web/frontend/src/main.ts` 和 `tests/user-management-navigation.test.mjs`；页面包不并行改写。MainPage内的账号管理HTTP调用、身份刷新、迟到响应及临时凭据宿主状态由111负责；Login/ChangePassword内的请求反馈由104负责，用户管理组件交互由105负责。
- 接线设计记录拟放 `process/web-ui-optimization-done/plan/integration.md`；总体计划、汇总和任务元数据由协调者维护。
- 公共样式/主题/locales/测试脚本问题回 WUI-002，页面缺陷回原页面包；`src/modules/api.ts`、`userManagement.ts`、主题持久化逻辑、共享组件、后端和协议默认只读。
- 与拓扑同时推进时，执行[跨需求协调](README.md#与其他需求的协调)，明确 MainPage/main.ts 等共同文件的唯一写入者、先后提交和接收方；不认领拓扑新页面或新接口。

## 退出条件与后续验证

实际实施时完成接线所需构建和针对性验证，确认旧入口、适用角色及页面契约可交付，未解决问题及影响进入汇总。接入命令协议、地址、端口、参数和复制内容不变；`config-server` 连接、设备认证与配置下发不变。

保留必要既有请求机制，不增加周期请求、心跳、订阅、定时任务、扫描或自动重放。交付 [WUI-008](plan-wui-008-acceptance.md) 进行真实联调，不能用本包接线完成代替整体验收。本次只拆分文档，不构建、不运行测试或 CI。

## 已拆分任务

| 任务 | 独立职责 | 前置任务 | 状态 / 负责人 |
| --- | --- | --- | --- |
| [WUI-111 宿主壳层与页面接线](../task/task-wui-111-host-integration.md) | 宿主壳层与页面接线 | WUI-104、WUI-105、WUI-106、WUI-107、WUI-108、WUI-109、WUI-110 | backlog / null |

具体任务的read_scope、write_scope、输入输出、共享文件交接和验收由任务明细维护。计划编号与任务编号不一一对应；跨需求写入遵循[共享文件调度](../../README.md#跨需求共享文件调度)。
