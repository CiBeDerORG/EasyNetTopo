---
id: PLAN-WUI-002
feature: web-ui-optimization
work_package: WUI-002
summary: ../README.md
status: done
requirement: ../require/README.md
task_board: ../task/README.md
updated_at: 2026-10-21
---

# WUI-002：公共视觉与交互基础

入口：[汇总](../README.md)、[计划索引](README.md)、[任务入口](../task/README.md)。当前仍是实施前的计划草案；已按下表生成具体任务，全部未认领、未执行。

## 目标与需求

根据基线问题统一既有 Web 的视觉、交互和页面输入输出约定，为四类页面计划下的七个独立页面任务提供公共基础。对应 [WUI-01～05、WUA-01～04](../require/README.md)，重点为反馈、主题、可访问性及兼容边界；不另建主题系统或新业务能力。

## 已知基础与待核实项

- 基线及固定源码链接见[汇总 WUI-EV-001～004](../README.md#当前事实与代码依据)。`theme.ts` 已使用 `definePreset(Aura, ...)` 及明暗主题；原接口、路由和部分周期请求已有实现。
- 具体公共调整取决于 [WUI-001](plan-wui-001-baseline.md) 的真实证据；[WUI-ISSUE-001/002/004](../README.md#问题限制与下一步)中的问题清单、请求来源和跨需求所有权不能假定已解决。
- 新专项测试支持目录尚未建立；采用提交的脚本、浏览器和可独立预览/测试入口须在实际实施时核实。

## 前置、输入与输出

前置为 WUI-001 的基线和问题清单。输入包含原组件/主题机制、页面任务、接口与请求边界。

输出为公共视觉与交互约定、各页面输入输出、文案/样式申请方式、独立预览与针对性验证入口。约定覆盖校验、忙碌/禁用、防重复提交、失败草稿、确认/取消、焦点、迟到响应、空/加载/失败/无权限/结果未知；按真实业务响应区分已受理、已保存与已应用。样式沿用 Vue、PrimeVue 和已有主题，覆盖中英文、明暗主题及响应式。

## 写入范围与所有权

- 本需求内唯一写入 `easytier-web/frontend/src/console.css`、`src/style.css`、`src/theme.ts`，以及 `easytier-web/frontend-lib/src/locales/cn.yaml`、`en.yaml`。
- 必要的 `easytier-web/frontend/package.json` 测试接线由本包统一处理；拟新增 `easytier-web/frontend/tests/web-ui-support/`。不升级依赖、不修改锁文件，不把创建测试文件等同于已接入 runner。
- 契约记录拟放 `process/web-ui-optimization-done/plan/contract.md` 和 `plan/shared/`；总体需求、计划索引和任务元数据仍由协调者维护。
- 页面包只提交公共文案/样式需求及本地样式，不直接抢写上述文件；本包按契约回传公共修改。跨需求共享范围按[协调规则](README.md#与其他需求的协调)安排唯一写入者和交接顺序。
- 下游为 [WUI-003](plan-wui-003-accounts.md)、[WUI-004](plan-wui-004-dashboard.md)、[WUI-005](plan-wui-005-devices.md)、[WUI-006](plan-wui-006-networks.md)，按不同页面并行。

## 退出条件与后续验证

公共约定及输入输出明确、页面具备可独立使用的入口，必要测试支持实际接入现有 runner；代码实施时完成自身构建和针对性验证，再交付页面包。保持原 API、`snake_case`、鉴权与权限、主题持久化及数据模型；不增加定时器、轮询、自动刷新、心跳、WebSocket/SSE、后台扫描或自动重放。

`config-server` 的连接、认证、配置下发及接入命令内容保持不变。契约问题由本包修订后交接消费者，不扩大只读模块范围；未解决项及影响回写汇总。本次仅拆分文档，不运行构建、测试或 CI。

## 已拆分任务

| 任务 | 独立职责 | 前置任务 | 状态 / 负责人 |
| --- | --- | --- | --- |
| [WUI-103 公共契约、主题与文案基础](../task/task-wui-103-shared-foundation.md) | 公共契约、主题与文案基础 | WUI-102 | backlog / null |

具体任务的read_scope、write_scope、输入输出、共享文件交接和验收由任务明细维护。计划编号与任务编号不一一对应；跨需求写入遵循[共享文件调度](../../README.md#跨需求共享文件调度)。
