---
id: PLAN-WUI-006
feature: web-ui-optimization
work_package: WUI-006
summary: ../README.md
status: done
requirement: ../require/README.md
task_board: ../task/README.md
updated_at: 2026-10-21
---

# WUI-006：网络页面

入口：[汇总](../README.md)、[计划索引](README.md)、[任务入口](../task/README.md)。当前仍是实施前的计划草案；已按下表生成具体任务，全部未认领、未执行。

## 目标与需求

在既有网络列表、详情、创建/编辑及已支持成员、ACL、凭据和配置操作内，优化页签、表单、列表、操作区及反馈。对应 [WUI-01～05、WUA-01～04](../require/README.md)，不增加拓扑业务视图、不改变后端规则或权限契约。

## 已知基础与待核实项

- [WUI-EV-001/004](../README.md#当前事实与代码依据)核对现有网络路由及 `NetworkDetail.vue` 的 5 秒节点详情刷新，不能将“本专项不新增周期请求”误写成当前完全没有周期请求。
- [WUI-EV-005、WUI-ISSUE-001/002](../README.md#问题限制与下一步)中的体验问题与完整请求来源待补齐；具体交互改动依据 WUI-001 证据确定。
- 基线为根汇总所列交付提交。原计划页面范围启动时按实际调用链复核，不能借优化推断新的 ACL、凭据或成员能力已存在。

## 前置、输入与输出

前置为 [WUI-002 公共基础](plan-wui-002-shared-foundation.md)。输入为网络任务/请求基线、已有 API 与配置组件语义、公共视觉和输入输出约定。

输出为网络页问题到改动映射、页面及局部样式、公共文案/样式请求、适用操作与失败/边界验证、前后证据。保存/应用状态按真实接口结果表达，保留兼容字段及已有复杂配置入口；失败不伪装为保存成功。

## 写入范围与并行边界

- `easytier-web/frontend/src/components/NetworkList.vue`、`NetworkDetail.vue`、`AclPolicyTab.vue`。
- 分别拟新增 `easytier-web/frontend/tests/web-ui-network-list.test.mjs`、`tests/web-ui-network-detail.test.mjs` 与 `process/web-ui-optimization-done/plan/networks/`。
- 不改后端规则、ACL 或凭据契约；共享网络/配置组件、API 客户端、主题持久化、后端、协议、依赖和锁文件默认只读。
- 与 WUI-003～005 按文件并行；公共主题、样式、文案和测试脚本归 WUI-002，壳层与路由归 WUI-007，元数据由协调者维护。

## 退出条件与后续验证

实际代码实施时先完成自身构建及网络页面行为验证，保留原入口、深链接、权限和业务结果；覆盖校验、重复提交、失败/取消、对象切换及适用草稿行为。中英文、明暗主题、360/768/1440 像素宽度和键盘路径按 WUA-03 记录。

保持既有必要底层机制，不扩大旧计时器职责，不新增轮询、自动刷新、心跳、WebSocket/SSE、定时任务或自动重放；不改变 `config-server`。交付 [WUI-007](plan-wui-007-integration.md) 页面契约与证据，跨页面联调由 WUI-008 负责。本次仅拆分文档，不运行构建、测试或 CI。

## 已拆分任务

| 任务 | 独立职责 | 前置任务 | 状态 / 负责人 |
| --- | --- | --- | --- |
| [WUI-109 网络列表与创建入口](../task/task-wui-109-network-list.md) | 网络列表与创建入口 | WUI-103 | backlog / null |
| [WUI-110 网络详情与 ACL 交互](../task/task-wui-110-network-detail-acl.md) | 网络详情与 ACL 交互 | WUI-103 | backlog / null |

具体任务的read_scope、write_scope、输入输出、共享文件交接和验收由任务明细维护。计划编号与任务编号不一一对应；跨需求写入遵循[共享文件调度](../../README.md#跨需求共享文件调度)。
