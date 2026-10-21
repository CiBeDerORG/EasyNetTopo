---
id: WUI-103
feature: web-ui-optimization
summary: ../README.md
plans: ["PLAN-WUI-002"]
status: done
owner: /root/iteration_exec2
depends_on: ["WUI-102"]
read_scope: ["process/web-ui-optimization-done/require/baseline.md", "process/web-ui-optimization-done/require/evidence/", "easytier-web/frontend/src/main.ts"]
write_scope: ["easytier-web/frontend/src/console.css", "easytier-web/frontend/src/style.css", "easytier-web/frontend/src/theme.ts", "easytier-web/frontend-lib/src/locales/cn.yaml", "easytier-web/frontend-lib/src/locales/en.yaml", "easytier-web/frontend/package.json", "easytier-web/frontend/tests/web-ui-support/", "process/web-ui-optimization-done/plan/contract.md", "process/web-ui-optimization-done/plan/shared/"]
requirements: ["WUI-01", "WUI-02", "WUI-03", "WUI-04", "WUI-05"]
acceptance: ["WUA-01", "WUA-02", "WUA-03", "WUA-04"]
parallel_group: WUI-G2
updated_at: 2026-10-21
---

# WUI-103：公共契约、主题与文案基础

[汇总](../README.md) · [PLAN-WUI-002](../plan/plan-wui-002-shared-foundation.md) · [需求与验收](../require/README.md) · [任务看板](README.md)

当前状态与负责人见元数据；执行与验证证据见本任务交接记录。

## 代码依据与启动条件

核查基线：`feat/web-user-management@1e92296b17fc2e85dcd4521b6012232e11abed95`；工作树 `/workspace/EasyTier-review-user-management-c3337d08`。文档分支不能替代该实现基线；执行前复核采用提交。根汇总 WUI-EV-001～006 提供相关现状，关联问题：WUI-ISSUE-001、WUI-ISSUE-004。

- [src/theme.ts](https://github.com/CiBeDerORG/EasyTier/blob/1e92296b17fc2e85dcd4521b6012232e11abed95/easytier-web/frontend/src/theme.ts)：`definePreset(Aura, ...)`。
- [src/main.ts](https://github.com/CiBeDerORG/EasyTier/blob/1e92296b17fc2e85dcd4521b6012232e11abed95/easytier-web/frontend/src/main.ts)：`PrimeVue/skipPrimeVue`。
- [package.json](https://github.com/CiBeDerORG/EasyTier/blob/1e92296b17fc2e85dcd4521b6012232e11abed95/easytier-web/frontend/package.json)：`build/test:browser/test:user-management`。

静态阅读仅证明上述入口存在；具体页面缺陷仍以WUI-102取证为准。未核实的行为/环境写清影响，不能补造问题或结果。

前置任务：[WUI-102](task-wui-102-page-evidence.md)。收到对应输出、限制和采用版本后才进入本任务执行；前置验收及采用版本见下方交接记录与任务元数据。

## 目标、输入与输出

固定公共视觉、交互、组件输入输出及验证约定，并提供七组页面可独立消费的基础。

交付 plan/contract.md 与共享基础，WUI-104～110进入并行批次；共享缺陷回派本任务，不能由页面作者抢写。

## 写入范围与多 Agent 边界

- 已有文件 `easytier-web/frontend/src/console.css`。
- 已有文件 `easytier-web/frontend/src/style.css`。
- 已有文件 `easytier-web/frontend/src/theme.ts`。
- 已有文件 `easytier-web/frontend-lib/src/locales/cn.yaml`。
- 已有文件 `easytier-web/frontend-lib/src/locales/en.yaml`。
- 已有文件 `easytier-web/frontend/package.json`。
- 拟新增 `easytier-web/frontend/tests/web-ui-support/`。
- 拟新增 `process/web-ui-optimization-done/plan/contract.md`。
- 拟新增 `process/web-ui-optimization-done/plan/shared/`。

本任务按依赖顺序执行；同一文件始终只有一个写入者。可与其他需求不重叠的任务并行，不能依据独立worktree认定共享文件可同时改写。

公共主题/CSS/locales/package.json与专项支持由WUI-103唯一写入；MainPage/main.ts及navigation回归由WUI-111负责；跨页面dashboard与真实用户E2E由WUI-112负责。其他任务提交变更请求，不越界改写。跨需求重叠文件按[共享文件调度](../../README.md#跨需求共享文件调度)领取：协调者核对实际采用提交和当前唯一写入者后允许写入；交还时提供提交/文件清单、契约差异、验证与未覆盖范围，接收方核对后再写。两方同时就绪默认WUI先ECO后，只有一方就绪可独立推进，不增加跨需求硬性depends_on。

元数据、看板、根汇总及全局索引由协调者统一更新，不列为并行执行者的公共写入范围。

cn/en语言文件与ECO-104互斥，package.json与ECO-105互斥；公共theme/CSS由本任务负责、ECO只读。具体队列和交接以全局调度为准。

## 执行与交接

1. 据已取证问题制定状态、焦点、表单、文案、明暗主题及响应式约定；不另建主题系统。
2. 本需求内独占公共CSS/theme/locales/package.json与专项验证支持；页面只提交文案/样式变更请求。
3. 仅在实际实现确有需要时接入专项验证文件，不升级依赖或改锁文件；后续共享调整继续由本任务原写入者处理并交接版本。

未来实际代码验证先隔离工作树、frontend-lib/dist与frontend/dist产物、浏览器数据和服务端口（既有入口常用5198），记录运行目录及端口；不能隔离时串行验证。本轮不执行这些命令。

## 完成条件与失败处理

- [x] 组件/路由输入输出、公共样式和文案约定可供页面独立工作，契约变更明确消费者与采用提交。
- [x] 实际代码阶段完成相关构建和针对性验证；纯文档修改不运行测试，当前没有任何已通过结果。
- [x] 保留config-server连接、认证、配置下发及接入命令；不新增轮询、心跳、定时任务、订阅、后台扫描或自动重放。
- [x] 记录最终文件、契约版本、输出证据与未覆盖范围；失败回派对应所有者，不扩大write_scope或捏造完成。

2026-10-21：按小计划生成本任务。只做文档路径/编号/依赖与范围核对；本次没有构建、测试或CI。以上实现与运行验证均为未来执行条件。

2026-10-21：102验收通过，exec2认领；独占公共CSS/theme/locales/package及支持目录，共享锁在交付前不交ECO。

2026-10-21：监工审查通过，类型/生产构建通过，12/12 mock HTTP生产组件布局回归通过；初次Drawer定位失败及修正见交付记录。语言/package锁已交还ECO104/105。111语义类接线后由103原所有者更新CSS选择器并复测。
