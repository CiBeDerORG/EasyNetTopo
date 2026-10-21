# WUI-105：用户表窄屏可读性

负责人：`iteration_exec2`。采用 [WUI 公共契约 v1](../contract.md)；来源为 [WUI-102 页面证据](../../require/evidence/README.md) 的 OBS-001 与 `layout-observations.json`。分支 `feat/ecosystem-web-ui`，实现基线 `1e92296b17fc2e85dcd4521b6012232e11abed95` 加已审查 WUI-103；最终提交由总控记录。

## 问题与修改

真实取证中，360px 中文用户表用户名列仅约 32.72px，19 字符名称行高约 292.72px；页面整体虽未溢出，身份仍难以阅读。本次仅修改 `frontend/src/components/users/UserManagement.vue` 的局部表格呈现：

- 表格最小宽度 56rem，用户名列最小 12rem；保留全部字段、完整用户名与自然换行，表头不逐字拆行。
- 水平溢出留在原 `.table-scroll`，不扩大整页；区域使用已有用户管理标题作为可访问名称、可用键盘聚焦/滚动，并有焦点轮廓。
- 空结果不渲染无内容的可聚焦区域；原加载、空态和失败显示继续由既有状态控制。

未改 props/events、角色/权限判定、查询分页、HTTP、确认、临时凭据生命周期、config-server 命令、轮询或主题。未改公共 CSS、package、locale、锁文件或共享库。

## 实际验证（2026-10-21）

工作目录 `/workspace/EasyTier-iteration/easytier-web/frontend`，仅本 agent 持有构建产物写入权；测试 preview 和组件 Vite 使用动态 loopback 端口，独立浏览器上下文，用后关闭。

| 验证 | 结果 |
| --- | --- |
| `node node_modules/vue-tsc/bin/vue-tsc.js -b` | 通过 |
| `node node_modules/vite/bin/vite.js build` | 通过；已有 caniuse 数据陈旧、大 chunk 警告保留 |
| `node --test tests/web-ui-users.test.mjs tests/user-management-components.test.mjs` | **19/19 通过，0 失败/跳过** |
| `git diff --check`（本任务文件）、文档相对链接 | 通过 |

新 `tests/web-ui-users.test.mjs` 使用实际生产页面，mock 账户 HTTP，覆盖 cn/en × light/dark × 360/768/1440 的 12 组合。采用合法长度 64 字符虚构用户名，验证用户名列至少 160px、行高小于 160px、完整身份不丢失、整页无水平溢出、窄屏仅表格滚动、键盘右箭头可滚动，以及滚动后的详情按钮可操作。每组合无 pageerror。

原 `user-management-components.test.mjs` 未修改，7 项回归实际通过：搜索/分页、未知/旧数据/失败区分、创建失败草稿与重复提交保护及焦点、确认/拒绝、临时凭据遮蔽/清除、不持久化、中文窄屏权限撤销、筛选/加载/空态、角色/重置/会话撤销与审计分页。

生产页面布局测试采用已构建共享库；本任务未改变共享文案/库，ECO 并行新增文案由其任务独立验证。这里的 mock 组件测试不证明真实后端授权、数据库或 OIDC 集成；全链路归 WUI-112。

## 交接

无需新增公共接口或文案，WUI-111 可直接消费既有事件契约；WUI-112 在全链路中保留窄屏阅读/操作检查。真实节点/实例、VPN和公网不在本次验证内。任务状态、看板、提交由总控维护。
