---
id: WUI-102
feature: web-ui-optimization
summary: ../README.md
plans: ["PLAN-WUI-001"]
status: done
owner: /root/iteration_exec3
depends_on: ["WUI-101"]
read_scope: ["process/web-ui-optimization-done/require/baseline.md", "process/web-ui-optimization-done/require/README.md", "easytier-web/frontend/src/components/"]
write_scope: ["process/web-ui-optimization-done/require/evidence/"]
requirements: ["WUI-01", "WUI-02", "WUI-03", "WUI-04", "WUI-05"]
acceptance: ["WUA-01", "WUA-02", "WUA-03", "WUA-04"]
parallel_group: WUI-G1
updated_at: 2026-10-21
---

# WUI-102：现有页面前态与问题取证

[汇总](../README.md) · [PLAN-WUI-001](../plan/plan-wui-001-baseline.md) · [需求与验收](../require/README.md) · [任务看板](README.md)

当前状态与负责人见元数据；执行与验证证据见本任务交接记录。

## 代码依据与启动条件

核查基线：`feat/web-user-management@1e92296b17fc2e85dcd4521b6012232e11abed95`；工作树 `/workspace/EasyTier-review-user-management-c3337d08`。文档分支不能替代该实现基线；执行前复核采用提交。根汇总 WUI-EV-001～006 提供相关现状，关联问题：WUI-ISSUE-001、WUI-ISSUE-003。

- [src/main.ts](https://github.com/CiBeDerORG/EasyTier/blob/1e92296b17fc2e85dcd4521b6012232e11abed95/easytier-web/frontend/src/main.ts)：`routes`。
- [src/components/Login.vue](https://github.com/CiBeDerORG/EasyTier/blob/1e92296b17fc2e85dcd4521b6012232e11abed95/easytier-web/frontend/src/components/Login.vue)：`onSubmit/onRegister`。
- [src/components/ChangePassword.vue](https://github.com/CiBeDerORG/EasyTier/blob/1e92296b17fc2e85dcd4521b6012232e11abed95/easytier-web/frontend/src/components/ChangePassword.vue)：`canChangePassword`。

静态阅读仅证明上述入口存在；具体页面缺陷仍以WUI-102取证为准。未核实的行为/环境写清影响，不能补造问题或结果。

前置任务：[WUI-101](task-wui-101-source-baseline.md)。收到对应输出、限制和采用版本后才进入本任务执行；前置验收及采用版本见下方交接记录与任务元数据。

## 目标、输入与输出

通过实际页面证据确认需要改进的流程和优先级，不把需求中的优化目标冒充当前缺陷。

交付 require/evidence/ 的证据与问题清单，交WUI-103收敛契约；未核实范围继续限制相应实施。

## 写入范围与多 Agent 边界

- 拟新增 `process/web-ui-optimization-done/require/evidence/`。

本任务按依赖顺序执行；同一文件始终只有一个写入者。可与其他需求不重叠的任务并行，不能依据独立worktree认定共享文件可同时改写。

公共主题/CSS/locales/package.json与专项支持由WUI-103唯一写入；MainPage/main.ts及navigation回归由WUI-111负责；跨页面dashboard与真实用户E2E由WUI-112负责。其他任务提交变更请求，不越界改写。跨需求重叠文件按[共享文件调度](../../README.md#跨需求共享文件调度)领取：协调者核对实际采用提交和当前唯一写入者后允许写入；交还时提供提交/文件清单、契约差异、验证与未覆盖范围，接收方核对后再写。两方同时就绪默认WUI先ECO后，只有一方就绪可独立推进，不增加跨需求硬性depends_on。

元数据、看板、根汇总及全局索引由协调者统一更新，不列为并行执行者的公共写入范围。

## 执行与交接

1. 在后续实际执行本取证任务时确认可访问的隔离运行版本、数据和身份；此轮文档拆分不启动应用。
2. 按WUI-101入口记录用户动作、实际结果、截图/流程、语言主题宽度、键盘与失败场景，敏感值脱敏。
3. 逐条关联源码入口，区分发现的问题、未复现、未覆盖与建议；未取证的页面不得预填缺陷。

## 完成条件与失败处理

- [x] 每个待改问题有页面/动作/结果/证据/影响及优先级依据。
- [x] 缺少运行环境时明确阻塞的页面证据与解除条件，不运行构建/测试来完成本次文档编辑，也不伪造截图。
- [x] 保留config-server连接、认证、配置下发及接入命令；不新增轮询、心跳、定时任务、订阅、后台扫描或自动重放。
- [x] 记录最终文件、契约版本、输出证据与未覆盖范围；失败回派对应所有者，不扩大write_scope或捏造完成。

2026-10-21：按小计划生成本任务。只做文档路径/编号/依赖与范围核对；本次没有构建、测试或CI。以上实现与运行验证均为未来执行条件。

2026-10-21：WUI-101经监工验收后由 `/root/iteration_exec3` 认领取证，复用ECO-102建立的隔离服务但独立浏览器上下文；尚未交付。

2026-10-21：监工验收WUI-EVIDENCE-1通过。96项真实API前态、18截图与故障注入分别记录，三项实证问题交103/105/111；未覆盖真实设备等范围保留。
