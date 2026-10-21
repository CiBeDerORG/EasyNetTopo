---
id: PLAN-WUI-003
feature: web-ui-optimization
work_package: WUI-003
summary: ../README.md
status: done
requirement: ../require/README.md
task_board: ../task/README.md
updated_at: 2026-10-21
---

# WUI-003：账号与用户页面

入口：[汇总](../README.md)、[计划索引](README.md)、[任务入口](../task/README.md)。当前仍是实施前的计划草案；已按下表生成具体任务，全部未认领、未执行。

## 目标与需求

在原有登录、注册、本人改密和用户管理任务内，根据取证结果优化表单、列表、详情与弹窗的操作反馈及视觉。对应 [WUI-01～05、WUA-01～04](../require/README.md)，不重做已交付用户管理，不增加业务接口。

## 已知基础与待核实项

- [WUI-EV-001/002](../README.md#当前事实与代码依据)已核对 `main.ts` 的账号相关路由和 `ChangePassword.vue` 的 `canChangePassword`，基线为汇总所列 `1e92296b` 完整提交。
- OIDC-only、本地密码能力与强制改密边界必须保留；已有注册策略、权限及临时凭据清除不得削弱。
- 本专项未取得账号页体验证据；具体缺陷、优先级和修改范围由 [WUI-001](plan-wui-001-baseline.md) 处理 [WUI-ISSUE-001](../README.md#问题限制与下一步) 后确定。文件范围沿用原计划，实施前核实采用提交。

## 前置、输入与输出

前置为 [WUI-002 公共约定](plan-wui-002-shared-foundation.md)，输入为基线问题、原账号流程和公共输入输出/样式/文案约定。

输出为账号页的具体问题到改动映射、优化后的页面与局部样式、适用失败/边界验证及前后证据。沿用既有搜索/筛选/分页、确认/取消和适用草稿行为，权限与业务结果以现有接口为准；不虚构成功、不自动重试写操作。

## 写入范围与并行边界

- `easytier-web/frontend/src/components/Login.vue`、`ChangePassword.vue`、`users/`。
- WUI-104拟新增 `easytier-web/frontend/tests/web-ui-auth.test.mjs`；WUI-105维护已有 `tests/user-management-components.test.mjs` 及拟新增 `tests/web-ui-users.test.mjs`。原 `tests/user-management-navigation.test.mjs` 跨MainPage、账号及权限状态，唯一写入者改由WUI-111负责。
- 本包设计记录拟放 `process/web-ui-optimization-done/plan/accounts/`，不维护第二套执行状态。
- 与 WUI-004～006 按页面并行；公共主题、样式、文案和测试脚本由 WUI-002 接收需求并单一修改。MainPage/main.ts 及MainPage内的账号管理HTTP生命周期、身份刷新、迟到响应和临时凭据宿主状态由 WUI-111 接线；104负责Login/ChangePassword内的请求反馈，105负责用户管理组件内交互，集中 E2E 由 WUI-112 负责。
- API 客户端、`userManagement.ts`、后端及协议只读；确有范围变化先由协调者更新需求和所有权。

## 退出条件与后续验证

实际实现时先完成自身构建与账号相关行为验证；覆盖原路由、适用身份、重复提交、失败/取消、焦点及临时凭据清除。中文/英文、明暗主题、360/768/1440 像素宽度和键盘路径的证据按 WUA-03 记录。

保留原登录与改密能力，不改变 `config-server` 连接/认证/配置下发/接入命令，不增加周期请求或后台机制。向 [WUI-007](plan-wui-007-integration.md) 交付页面输入输出、证据和限制；真实组合验收由 WUI-008 负责。未取证或未执行项如实记录，本次纯文档不运行构建、测试或 CI。

## 已拆分任务

| 任务 | 独立职责 | 前置任务 | 状态 / 负责人 |
| --- | --- | --- | --- |
| [WUI-104 登录注册与本人改密页面](../task/task-wui-104-auth-pages.md) | 登录注册与本人改密页面 | WUI-103 | backlog / null |
| [WUI-105 用户管理组件交互与视觉](../task/task-wui-105-user-management.md) | 用户管理组件交互与视觉 | WUI-103 | backlog / null |

具体任务的read_scope、write_scope、输入输出、共享文件交接和验收由任务明细维护。计划编号与任务编号不一一对应；跨需求写入遵循[共享文件调度](../../README.md#跨需求共享文件调度)。
