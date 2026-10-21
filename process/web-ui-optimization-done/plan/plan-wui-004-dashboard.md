---
id: PLAN-WUI-004
feature: web-ui-optimization
work_package: WUI-004
summary: ../README.md
status: done
requirement: ../require/README.md
task_board: ../task/README.md
updated_at: 2026-10-21
---

# WUI-004：工作台页面

入口：[汇总](../README.md)、[计划索引](README.md)、[任务入口](../task/README.md)。当前仍是实施前的计划草案；已按下表生成具体任务，全部未认领、未执行。

## 目标与需求

在现有 Dashboard 任务与数据语义内，依据实际证据优化信息层级、卡片、留白及加载/空/失败反馈。对应 [WUI-01～05、WUA-01～04](../require/README.md)，不新增业务视图或借美化删除原入口。

## 已知基础与待核实项

- [WUI-EV-001/003](../README.md#当前事实与代码依据)确认现有 Dashboard 路由及公共主题机制；页面具体布局缺陷尚未取证。
- [WUI-ISSUE-001/003](../README.md#问题限制与下一步)记录实际体验与浏览器环境待核实，历史 Dashboard 浏览器限制不等于当前已复现或已解除。
- 采用提交与范围见根汇总；本包读取 Dashboard 的实际数据和调用边界后细化，不能由文件名推断已有功能或故障。

## 前置、输入与输出

前置为 [WUI-002](plan-wui-002-shared-foundation.md) 的公共约定，输入包含 WUI-001 任务/问题/请求基线及 Dashboard 相关实际证据。

输出为页面问题与改动映射、Dashboard 局部页面修改、公共文案/样式需求、独立验证及前后证据。刷新反馈服从既有请求触发方式，不能新增自动刷新或将数据读取失败显示成设备离线。

## 写入范围与并行边界

- `easytier-web/frontend/src/components/Dashboard.vue`。
- 拟新增 `easytier-web/frontend/tests/web-ui-dashboard.test.mjs` 与 `process/web-ui-optimization-done/plan/dashboard/`。
- 不写 MainPage/main.ts，不覆盖其他页面的集中回归文件；已有 `tests/dashboard.test.mjs` 的本专项必要调整统一归 WUI-008。
- 与 WUI-003/005/006 并行，公共样式/主题/locales/测试接线由 WUI-002 单一负责；根汇总和索引由协调者维护。
- API 客户端、共享网络/配置组件、后端、依赖和锁文件默认只读；需要变更边界时先更新需求与所有权。

## 退出条件与后续验证

实际代码实施时完成自身构建与针对性行为验证，保留原入口、数据结果与权限；覆盖适用失败/边界、中英文、明暗主题、360/768/1440 像素宽度和键盘操作。截图须可对应实际操作与采用提交，不以静态效果代替交互验收。

不改变 `config-server` 或原接入命令，不新增轮询、心跳、订阅、定时任务或其他后台机制。交付 [WUI-007](plan-wui-007-integration.md) 页面输入输出与证据，环境或历史问题由 WUI-008 区分记录。本次仅拆分文档，不构建、不运行测试或 CI。

## 已拆分任务

| 任务 | 独立职责 | 前置任务 | 状态 / 负责人 |
| --- | --- | --- | --- |
| [WUI-106 工作台信息与操作反馈](../task/task-wui-106-dashboard.md) | 工作台信息与操作反馈 | WUI-103 | backlog / null |

具体任务的read_scope、write_scope、输入输出、共享文件交接和验收由任务明细维护。计划编号与任务编号不一一对应；跨需求写入遵循[共享文件调度](../../README.md#跨需求共享文件调度)。
