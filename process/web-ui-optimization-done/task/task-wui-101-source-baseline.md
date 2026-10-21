---
id: WUI-101
feature: web-ui-optimization
summary: ../README.md
plans: ["PLAN-WUI-001"]
status: done
owner: /root/iteration_exec3
depends_on: []
read_scope: ["easytier-web/frontend/src/main.ts", "easytier-web/frontend/src/components/MainPage.vue", "easytier-web/frontend/src/components/DeviceList.vue", "easytier-web/frontend/src/components/NetworkDetail.vue", "easytier-web/frontend/package.json"]
write_scope: ["process/web-ui-optimization-done/require/baseline.md"]
requirements: ["WUI-01", "WUI-02", "WUI-03", "WUI-04", "WUI-05"]
acceptance: ["WUA-01", "WUA-02", "WUA-03", "WUA-04"]
parallel_group: WUI-G0
updated_at: 2026-10-21
---

# WUI-101：源码、路由与请求基线

[汇总](../README.md) · [PLAN-WUI-001](../plan/plan-wui-001-baseline.md) · [需求与验收](../require/README.md) · [任务看板](README.md)

当前已完成静态基线核查并经监工验收；应用运行不属于本任务通过结论。

## 代码依据与启动条件

核查基线：`feat/web-user-management@1e92296b17fc2e85dcd4521b6012232e11abed95`；工作树 `/workspace/EasyTier-iteration`。文档分支不能替代该实现基线；执行前复核采用提交。根汇总 WUI-EV-001～006 提供相关现状，关联问题：WUI-ISSUE-002、WUI-ISSUE-003。

- [src/main.ts](https://github.com/CiBeDerORG/EasyTier/blob/1e92296b17fc2e85dcd4521b6012232e11abed95/easytier-web/frontend/src/main.ts)：`routes`。
- [src/components/MainPage.vue](https://github.com/CiBeDerORG/EasyTier/blob/1e92296b17fc2e85dcd4521b6012232e11abed95/easytier-web/frontend/src/components/MainPage.vue)：`loadNavigation/enrollCommand`。
- [src/components/DeviceList.vue](https://github.com/CiBeDerORG/EasyTier/blob/1e92296b17fc2e85dcd4521b6012232e11abed95/easytier-web/frontend/src/components/DeviceList.vue)：`loadDevices/PeriodicTask`。
- [src/components/NetworkDetail.vue](https://github.com/CiBeDerORG/EasyTier/blob/1e92296b17fc2e85dcd4521b6012232e11abed95/easytier-web/frontend/src/components/NetworkDetail.vue)：`loadAll/loadNodeDetail`。
- [package.json](https://github.com/CiBeDerORG/EasyTier/blob/1e92296b17fc2e85dcd4521b6012232e11abed95/easytier-web/frontend/package.json)：`scripts`。

静态阅读仅证明上述入口存在；具体页面缺陷仍以WUI-102取证为准。未核实的行为/环境写清影响，不能补造问题或结果。

无前置执行任务；采用已核实的1e92296b基线执行。

## 目标、输入与输出

建立后续页面调整采用的真实提交、入口、调用与请求来源清单，不设计假定缺陷。

交付 require/baseline.md，供WUI-102取证、WUI-103定契约；发现跨分支漂移先回写根汇总。

## 写入范围与多 Agent 边界

- 拟新增 `process/web-ui-optimization-done/require/baseline.md`。

本任务按依赖顺序执行；同一文件始终只有一个写入者。可与其他需求不重叠的任务并行，不能依据独立worktree认定共享文件可同时改写。

公共主题/CSS/locales/package.json与专项支持由WUI-103唯一写入；MainPage/main.ts及navigation回归由WUI-111负责；跨页面dashboard与真实用户E2E由WUI-112负责。其他任务提交变更请求，不越界改写。跨需求重叠文件按[共享文件调度](../../README.md#跨需求共享文件调度)领取：协调者核对实际采用提交和当前唯一写入者后允许写入；交还时提供提交/文件清单、契约差异、验证与未覆盖范围，接收方核对后再写。两方同时就绪默认WUI先ECO后，只有一方就绪可独立推进，不增加跨需求硬性depends_on。

元数据、看板、根汇总及全局索引由协调者统一更新，不列为并行执行者的公共写入范围。

## 执行与交接

1. 核对采用分支/提交及相对交付基线的差异，区分已提交源码与未提交变化。
2. 逐页列路由、组件、输入输出、API调用、现有定时/手动触发与清理路径；记录配置接入命令出现于MainPage和DeviceList。
3. 读取现有脚本与回归覆盖，区分mock浏览器和真实服务入口，列明确未核实项。

## 完成条件与失败处理

- [x] 清单逐项有文件、符号、提交和核查范围；搜索未命中不表述为全项目不存在。
- [x] 保留账号边界、现有周期请求基线、接口字段和config-server命令生成语义；没有执行运行验收。
- [x] 保留config-server连接、认证、配置下发及接入命令；不新增轮询、心跳、定时任务、订阅、后台扫描或自动重放。
- [x] 记录最终文件、契约版本、输出证据与未覆盖范围；失败回派对应所有者，不扩大write_scope或捏造完成。

2026-10-21：按小计划生成本任务。只做文档路径/编号/依赖与范围核对；本次没有构建、测试或CI。以上实现与运行验证均为未来执行条件。

2026-10-21：用户授权启动，由 `/root/iteration_exec3` 认领，基线1e92296b；正在核查，尚未交付或验收。状态以本次认领及frontmatter为准。

2026-10-21：交付基线产物并经 `/root/iteration_monitor` 审查通过，源码入口及文档链接已核对，未运行应用测试。
