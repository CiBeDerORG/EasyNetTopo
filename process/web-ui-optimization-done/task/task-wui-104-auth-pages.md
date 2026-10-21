---
id: WUI-104
feature: web-ui-optimization
summary: ../README.md
plans: ["PLAN-WUI-003"]
status: done
owner: /root/iteration_exec2
depends_on: ["WUI-103"]
read_scope: ["easytier-web/frontend/src/modules/userManagement.ts", "easytier-web/frontend/src/modules/api.ts", "easytier-web/frontend/src/components/MainPage.vue", "process/web-ui-optimization-done/plan/contract.md"]
write_scope: ["easytier-web/frontend/src/components/Login.vue", "easytier-web/frontend/src/components/ChangePassword.vue", "easytier-web/frontend/tests/web-ui-auth.test.mjs", "process/web-ui-optimization-done/plan/accounts/auth.md"]
requirements: ["WUI-01", "WUI-02", "WUI-03", "WUI-04", "WUI-05"]
acceptance: ["WUA-01", "WUA-02", "WUA-03", "WUA-04"]
parallel_group: WUI-G3
updated_at: 2026-10-21
---

# WUI-104：登录注册与本人改密页面

[汇总](../README.md) · [PLAN-WUI-003](../plan/plan-wui-003-accounts.md) · [需求与验收](../require/README.md) · [任务看板](README.md)

当前状态与负责人见元数据；执行与验证证据见本任务交接记录。

## 代码依据与启动条件

核查基线：`feat/web-user-management@1e92296b17fc2e85dcd4521b6012232e11abed95`；工作树 `/workspace/EasyTier-review-user-management-c3337d08`。文档分支不能替代该实现基线；执行前复核采用提交。根汇总 WUI-EV-001～006 提供相关现状，关联问题：WUI-ISSUE-001。

- [src/components/Login.vue](https://github.com/CiBeDerORG/EasyTier/blob/1e92296b17fc2e85dcd4521b6012232e11abed95/easytier-web/frontend/src/components/Login.vue)：`loadPolicy/onSubmit/onRegister`。
- [src/components/ChangePassword.vue](https://github.com/CiBeDerORG/EasyTier/blob/1e92296b17fc2e85dcd4521b6012232e11abed95/easytier-web/frontend/src/components/ChangePassword.vue)：`canChangePassword/changePassword`。

静态阅读仅证明上述入口存在；具体页面缺陷仍以WUI-102取证为准。未核实的行为/环境写清影响，不能补造问题或结果。

前置任务：[WUI-103](task-wui-103-shared-foundation.md)。收到对应输出、限制和采用版本后才进入本任务执行；前置验收及采用版本见下方交接记录与任务元数据。

## 目标、输入与输出

按证据优化已有登录、注册和本人改密表单反馈与排版，保留已交付身份行为。

交付两个页面及专项证据给WUI-111；没有证实缺陷的区域保留不改，并记录理由。

## 写入范围与多 Agent 边界

- 已有文件 `easytier-web/frontend/src/components/Login.vue`。
- 已有文件 `easytier-web/frontend/src/components/ChangePassword.vue`。
- 拟新增 `easytier-web/frontend/tests/web-ui-auth.test.mjs`。
- 拟新增 `process/web-ui-optimization-done/plan/accounts/auth.md`。

在WUI-103契约交付后可与WUI-105、WUI-106、WUI-107、WUI-108、WUI-109、WUI-110并行，各自只写独占文件。共同API、组件契约和公共文案只读，变更由指定写入者修订并重新交接，不直接依赖其他页面尚未落地实现。

公共主题/CSS/locales/package.json与专项支持由WUI-103唯一写入；MainPage/main.ts及navigation回归由WUI-111负责；跨页面dashboard与真实用户E2E由WUI-112负责。其他任务提交变更请求，不越界改写。跨需求重叠文件按[共享文件调度](../../README.md#跨需求共享文件调度)领取：协调者核对实际采用提交和当前唯一写入者后允许写入；交还时提供提交/文件清单、契约差异、验证与未覆盖范围，接收方核对后再写。两方同时就绪默认WUI先ECO后，只有一方就绪可独立推进，不增加跨需求硬性depends_on。

元数据、看板、根汇总及全局索引由协调者统一更新，不列为并行执行者的公共写入范围。

## 执行与交接

1. 只处理WUI-102已记录且归属本页的问题；校验、提交中、失败、取消和焦点采用公共约定。
2. 保留注册策略、OIDC入口、本地密码能力和必须改密；local_password_unavailable后不重新展示可提交的密码表单。
3. 不写MainPage、用户管理组件、API、locales；相关调整分别交111、105、103。

未来实际代码验证先隔离工作树、frontend-lib/dist与frontend/dist产物、浏览器数据和服务端口（既有入口常用5198），记录运行目录及端口；不能隔离时串行验证。本轮不执行这些命令。

## 完成条件与失败处理

- [x] 原路由、登录/注册结果、改密成功后退出及失败清理符合既有契约；失败不伪装成功。
- [x] 实际实现覆盖中英文、主题、键盘与适用身份；专项mock只证明前端，真实身份链路交112。
- [x] 保留config-server连接、认证、配置下发及接入命令；不新增轮询、心跳、定时任务、订阅、后台扫描或自动重放。
- [x] 记录最终文件、契约版本、输出证据与未覆盖范围；失败回派对应所有者，不扩大write_scope或捏造完成。

2026-10-21：按小计划生成本任务。只做文档路径/编号/依赖与范围核对；本次没有构建、测试或CI。以上实现与运行验证均为未来执行条件。

2026-10-21：103前置通过后认领登录改密保持及针对回归，无证实问题不强行改代码。

2026-10-21：监工审查保持源码与12项mock生产组件认证回归通过，未宣称真实OIDC联调。
