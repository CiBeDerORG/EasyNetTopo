---
id: PLAN-WUI-005
feature: web-ui-optimization
work_package: WUI-005
summary: ../README.md
status: done
requirement: ../require/README.md
task_board: ../task/README.md
updated_at: 2026-10-21
---

# WUI-005：设备与实例页面

入口：[汇总](../README.md)、[计划索引](README.md)、[任务入口](../task/README.md)。当前仍是实施前的计划草案；已按下表生成具体任务，全部未认领、未执行。

## 目标与需求

基于现有设备查看、选择、实例配置与保存入口，优化列表/详情、字段分组、长文本、状态表达和对象切换反馈。对应 [WUI-01～05、WUA-01～04](../require/README.md)，保留原设备/实例操作及权限，不把后续拓扑资源当作已有能力。

## 已知基础与待核实项

- [WUI-EV-001/002](../README.md#当前事实与代码依据)已核对设备路由、MainPage 的 `enrollCommand` 与 `copyEnrollCommand`；接入命令由宿主生成，本包不取得该文件所有权。
- [WUI-ISSUE-001/002](../README.md#问题限制与下一步)记录真实页面问题与完整请求来源待核实。对象切换、迟到响应和保存反馈是需核实的场景，不能预先断言存在缺陷。
- 基线为根汇总所列交付提交；本包页面文件范围沿用原计划，实施前读相关调用链并核实采用提交的行为。

## 前置、输入与输出

前置为 [WUI-002](plan-wui-002-shared-foundation.md)，输入为 WUI-001 的设备/实例任务与请求证据、既有配置组件能力及公共交互约定。

输出为页面改动、局部样式、公共文案/样式申请、对象切换与适用失败场景的验证和前后证据。保存、应用、失败及结果未知依据真实响应表达；保留适当非敏感草稿，不重放写操作，不改变配置数据语义。

## 写入范围与并行边界

- `easytier-web/frontend/src/components/DeviceList.vue`、`DeviceDetails.vue`、`DeviceManagement.vue`。
- 分别拟新增 `easytier-web/frontend/tests/web-ui-device-list.test.mjs`、`tests/web-ui-instance-host.test.mjs` 与 `process/web-ui-optimization-done/plan/devices/`。
- 共享 RemoteManagement 和配置组件只消费、不改写；API 客户端、后端、协议与网络内核默认只读。
- 与 WUI-003/004/006 按页面并行。公共主题/locales/样式/测试接线归 WUI-002，MainPage/main.ts 与接入命令排版接线归 WUI-007；跨需求共享文件先由协调者安排所有权。

## 退出条件与后续验证

实际实施时先完成自身构建与针对性验证；设备/实例旧入口、深链接、权限、选择/切换、适用失败恢复及原操作结果保持可用。按 WUA-03 覆盖中英文、主题、360/768/1440 像素宽度、长文本与键盘路径。

`config-server` 协议、地址、端口、参数、复制内容及连接/认证/配置下发不变，不新增定时器、轮询、订阅、后台扫描或重放。将页面契约与证据交 [WUI-007](plan-wui-007-integration.md)，跨页面兼容由 WUI-008 联调。本次只拆文档，不构建、不运行测试或 CI。

## 已拆分任务

| 任务 | 独立职责 | 前置任务 | 状态 / 负责人 |
| --- | --- | --- | --- |
| [WUI-107 设备列表与设备资料展示](../task/task-wui-107-device-list.md) | 设备列表与设备资料展示 | WUI-103 | backlog / null |
| [WUI-108 实例配置宿主交互](../task/task-wui-108-instance-host.md) | 实例配置宿主交互 | WUI-103 | backlog / null |

具体任务的read_scope、write_scope、输入输出、共享文件交接和验收由任务明细维护。计划编号与任务编号不一一对应；跨需求写入遵循[共享文件调度](../../README.md#跨需求共享文件调度)。
