---
id: WUI-111
feature: web-ui-optimization
summary: ../README.md
plans: ["PLAN-WUI-007"]
status: done
owner: /root/iteration_exec2
depends_on: ["WUI-104", "WUI-105", "WUI-106", "WUI-107", "WUI-108", "WUI-109", "WUI-110"]
read_scope: ["process/web-ui-optimization-done/plan/contract.md", "easytier-web/frontend/src/components/Login.vue", "easytier-web/frontend/src/components/ChangePassword.vue", "easytier-web/frontend/src/components/users/", "easytier-web/frontend/src/components/Dashboard.vue", "easytier-web/frontend/src/components/DeviceList.vue", "easytier-web/frontend/src/components/DeviceDetails.vue", "easytier-web/frontend/src/components/DeviceManagement.vue", "easytier-web/frontend/src/components/NetworkList.vue", "easytier-web/frontend/src/components/NetworkDetail.vue", "easytier-web/frontend/src/components/AclPolicyTab.vue", "easytier-web/frontend/src/modules/api.ts"]
write_scope: ["easytier-web/frontend/src/components/MainPage.vue", "easytier-web/frontend/src/main.ts", "easytier-web/frontend/tests/user-management-navigation.test.mjs", "process/web-ui-optimization-done/plan/integration.md"]
requirements: ["WUI-01", "WUI-02", "WUI-03", "WUI-04", "WUI-05"]
acceptance: ["WUA-01", "WUA-02", "WUA-03", "WUA-04"]
parallel_group: WUI-G4
updated_at: 2026-10-21
---

# WUI-111：宿主壳层与页面接线

[汇总](../README.md) · [PLAN-WUI-007](../plan/plan-wui-007-integration.md) · [需求与验收](../require/README.md) · [任务看板](README.md)

当前状态与负责人见元数据；执行与验证证据见本任务交接记录。

## 代码依据与启动条件

核查基线：`feat/web-user-management@1e92296b17fc2e85dcd4521b6012232e11abed95`；工作树 `/workspace/EasyTier-review-user-management-c3337d08`。文档分支不能替代该实现基线；执行前复核采用提交。根汇总 WUI-EV-001～006 提供相关现状，关联问题：WUI-ISSUE-002、WUI-ISSUE-004。

- [src/components/MainPage.vue](https://github.com/CiBeDerORG/EasyTier/blob/1e92296b17fc2e85dcd4521b6012232e11abed95/easytier-web/frontend/src/components/MainPage.vue)：`refreshIdentity/loadUsers/mutate/canManage/enrollCommand/loadNavigation`。
- [src/main.ts](https://github.com/CiBeDerORG/EasyTier/blob/1e92296b17fc2e85dcd4521b6012232e11abed95/easytier-web/frontend/src/main.ts)：`routes`。
- [tests/user-management-navigation.test.mjs](https://github.com/CiBeDerORG/EasyTier/blob/1e92296b17fc2e85dcd4521b6012232e11abed95/easytier-web/frontend/tests/user-management-navigation.test.mjs)：`mock /api/v1/** 浏览器回归`。

静态阅读仅证明上述入口存在；具体页面缺陷仍以WUI-102取证为准。未核实的行为/环境写清影响，不能补造问题或结果。

前置任务：[WUI-104](task-wui-104-auth-pages.md)、[WUI-105](task-wui-105-user-management.md)、[WUI-106](task-wui-106-dashboard.md)、[WUI-107](task-wui-107-device-list.md)、[WUI-108](task-wui-108-instance-host.md)、[WUI-109](task-wui-109-network-list.md)、[WUI-110](task-wui-110-network-detail-acl.md)。收到对应输出、限制和采用版本后才进入本任务执行；前置验收及采用版本见下方交接记录与任务元数据。

## 目标、输入与输出

在所有页面交付后串行接线导航、壳层和页面输入输出，保持账号及接入流程。

交付接线版本、提交/文件清单和未解决限制给WUI-112；页面缺陷回104～110，公共问题回103。

## 写入范围与多 Agent 边界

- 已有文件 `easytier-web/frontend/src/components/MainPage.vue`。
- 已有文件 `easytier-web/frontend/src/main.ts`。
- 已有文件 `easytier-web/frontend/tests/user-management-navigation.test.mjs`。
- 拟新增 `process/web-ui-optimization-done/plan/integration.md`。

本任务按依赖顺序执行；同一文件始终只有一个写入者。可与其他需求不重叠的任务并行，不能依据独立worktree认定共享文件可同时改写。

公共主题/CSS/locales/package.json与专项支持由WUI-103唯一写入；MainPage/main.ts及navigation回归由WUI-111负责；跨页面dashboard与真实用户E2E由WUI-112负责。其他任务提交变更请求，不越界改写。跨需求重叠文件按[共享文件调度](../../README.md#跨需求共享文件调度)领取：协调者核对实际采用提交和当前唯一写入者后允许写入；交还时提供提交/文件清单、契约差异、验证与未覆盖范围，接收方核对后再写。两方同时就绪默认WUI先ECO后，只有一方就绪可独立推进，不增加跨需求硬性depends_on。

元数据、看板、根汇总及全局索引由协调者统一更新，不列为并行执行者的公共写入范围。

MainPage/main.ts与ECO-113互斥。MainPage的loadUsers/selectUser/loadEvents/mutate、身份刷新、迟到响应与临时凭据清除在本任务维护，104/105仅处理组件交互；不能让组件作者并行改宿主状态。

## 执行与交接

1. 接收104～110交付与103契约版本，核对未解决问题并完成路由/菜单/标题/响应式宿主接线。
2. 本需求唯一改MainPage/main.ts及原navigation集中回归；不借接线吞并页面修复或改API。
3. 执行跨需求共享文件交接后再写MainPage/main.ts；保留全部旧路由、能力判断、临时凭据清除及接入命令内容。

未来实际代码验证先隔离工作树、frontend-lib/dist与frontend/dist产物、浏览器数据和服务端口（既有入口常用5198），记录运行目录及端口；不能隔离时串行验证。本轮不执行这些命令。

## 完成条件与失败处理

- [x] 原登录/改密/管理/设备/网络入口、深链接、前进返回和主题切换继续可用。
- [x] 原navigation测试使用mock HTTP，明确只证明生产前端行为；真实后端结果交112，不新增定时刷新。
- [x] 保留config-server连接、认证、配置下发及接入命令；不新增轮询、心跳、定时任务、订阅、后台扫描或自动重放。
- [x] 记录最终文件、契约版本、输出证据与未覆盖范围；失败回派对应所有者，不扩大write_scope或捏造完成。

2026-10-21：按小计划生成本任务。只做文档路径/编号/依赖与范围核对；本次没有构建、测试或CI。以上实现与运行验证均为未来执行条件。

2026-10-21：110验收后领取宿主MainPage/main.ts与构建独占锁，实施移动导航焦点与语义样式交接。

2026-10-21：监工验收宿主焦点与语义样式交接；最终navigation17/17、foundation12/12、类型检查与生产构建通过。未修改认证、权限、凭据时序或原路由。
