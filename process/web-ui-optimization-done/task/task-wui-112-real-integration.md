---
id: WUI-112
feature: web-ui-optimization
summary: ../README.md
plans: ["PLAN-WUI-008"]
status: done
owner: /root/iteration_exec1
depends_on: ["WUI-111"]
read_scope: ["process/web-ui-optimization-done/require/evidence/", "process/web-ui-optimization-done/plan/integration.md", "easytier-web/frontend/package.json", "easytier-web/frontend/tests/helpers/webTestBinary.mjs", "easytier-web/frontend/src/"]
write_scope: ["easytier-web/frontend/tests/web-ui-integration.test.mjs", "easytier-web/frontend/tests/dashboard.test.mjs", "easytier-web/frontend/tests/user-management-e2e.test.mjs", "process/web-ui-optimization-done/plan/verification/"]
requirements: ["WUI-01", "WUI-02", "WUI-03", "WUI-04", "WUI-05"]
acceptance: ["WUA-01", "WUA-02", "WUA-03", "WUA-04"]
parallel_group: WUI-G5
updated_at: 2026-10-21
---

# WUI-112：真实组合联调与集中回归

[汇总](../README.md) · [PLAN-WUI-008](../plan/plan-wui-008-acceptance.md) · [需求与验收](../require/README.md) · [任务看板](README.md)

当前状态与负责人见元数据；执行与验证证据见本任务交接记录。

## 代码依据与启动条件

核查基线：`feat/web-user-management@1e92296b17fc2e85dcd4521b6012232e11abed95`；工作树 `/workspace/EasyTier-review-user-management-c3337d08`。文档分支不能替代该实现基线；执行前复核采用提交。根汇总 WUI-EV-001～006 提供相关现状，关联问题：WUI-ISSUE-003、WUI-ISSUE-004。

- [tests/dashboard.test.mjs](https://github.com/CiBeDerORG/EasyTier/blob/1e92296b17fc2e85dcd4521b6012232e11abed95/easytier-web/frontend/tests/dashboard.test.mjs)：`跨页面mock浏览器回归`。
- [tests/user-management-e2e.test.mjs](https://github.com/CiBeDerORG/EasyTier/blob/1e92296b17fc2e85dcd4521b6012232e11abed95/easytier-web/frontend/tests/user-management-e2e.test.mjs)：`真实服务/SQLite/浏览器联调`。
- [tests/helpers/webTestBinary.mjs](https://github.com/CiBeDerORG/EasyTier/blob/1e92296b17fc2e85dcd4521b6012232e11abed95/easytier-web/frontend/tests/helpers/webTestBinary.mjs)：`resolveWebTestBinary`。
- [package.json](https://github.com/CiBeDerORG/EasyTier/blob/1e92296b17fc2e85dcd4521b6012232e11abed95/easytier-web/frontend/package.json)：`test:browser/test:user-management`。

静态阅读仅证明上述入口存在；具体页面缺陷仍以WUI-102取证为准。未核实的行为/环境写清影响，不能补造问题或结果。

前置任务：[WUI-111](task-wui-111-host-integration.md)。收到对应输出、限制和采用版本后才进入本任务执行；前置验收及采用版本见下方交接记录与任务元数据。

## 目标、输入与输出

验证组合后的真实业务路径与交互/视觉，集中管理跨页面回归并区分证据类型。

交付 plan/verification/ 的真实记录与限制给WUI-113；未执行承诺场景时不报完成。

## 写入范围与多 Agent 边界

- 拟新增 `easytier-web/frontend/tests/web-ui-integration.test.mjs`。
- 已有文件 `easytier-web/frontend/tests/dashboard.test.mjs`。
- 已有文件 `easytier-web/frontend/tests/user-management-e2e.test.mjs`。
- 拟新增 `process/web-ui-optimization-done/plan/verification/`。

本任务按依赖顺序执行；同一文件始终只有一个写入者。可与其他需求不重叠的任务并行，不能依据独立worktree认定共享文件可同时改写。

公共主题/CSS/locales/package.json与专项支持由WUI-103唯一写入；MainPage/main.ts及navigation回归由WUI-111负责；跨页面dashboard与真实用户E2E由WUI-112负责。其他任务提交变更请求，不越界改写。跨需求重叠文件按[共享文件调度](../../README.md#跨需求共享文件调度)领取：协调者核对实际采用提交和当前唯一写入者后允许写入；交还时提供提交/文件清单、契约差异、验证与未覆盖范围，接收方核对后再写。两方同时就绪默认WUI先ECO后，只有一方就绪可独立推进，不增加跨需求硬性depends_on。

元数据、看板、根汇总及全局索引由协调者统一更新，不列为并行执行者的公共写入范围。

## 执行与交接

1. 实际代码实施完成后核实浏览器、隔离服务/数据库/身份、采用提交及测试入口，再执行承诺场景。
2. 集中维护dashboard.test.mjs、user-management-e2e.test.mjs及拟新增专项组合验证；脚本/公共支持调整回103。
3. 按WUA01～04对比前后任务、状态、布局、键盘、身份及请求记录；原dashboard回归为mock，不能当真实后端通过。
4. 失败回派103公共、104～110页面、111宿主，修复交付后只复验受影响范围；不降低断言或代改各页面源码。

未来实际代码验证先隔离工作树、frontend-lib/dist与frontend/dist产物、浏览器数据和服务端口（既有入口常用5198），记录运行目录及端口；不能隔离时串行验证。本轮不执行这些命令。

## 完成条件与失败处理

- [x] 真实服务成功/失败/权限/对象切换及账号边界有证据，config-server行为与命令不变，无新增周期机制。
- [x] 通过、失败、未执行、mock/真实分别记录；历史浏览器下载403及旧通过数不替代本轮结果。
- [x] 保留config-server连接、认证、配置下发及接入命令；不新增轮询、心跳、定时任务、订阅、后台扫描或自动重放。
- [x] 记录最终文件、契约版本、输出证据与未覆盖范围；失败回派对应所有者，不扩大write_scope或捏造完成。

2026-10-21：按小计划生成本任务。只做文档路径/编号/依赖与范围核对；本次没有构建、测试或CI。以上实现与运行验证均为未来执行条件。

2026-10-21：监工准入：采用WUI111冻结前端产物与隔离后端/SQLite做集中与真实联调，和ECO113源码改动隔离。

2026-10-21：监工验收：集中fixture33/33、真实SQLite/OIDC9/9；真实专项三项通过后矩阵1/1定向通过，覆盖96组合（0溢出/异常），非单轮4/4；22秒请求及16截图留档。冻结97112bef前端，实际限制见verification。
