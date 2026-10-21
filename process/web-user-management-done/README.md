---
feature: web-user-management
status: done
baseline: feat/web-user-management@1e92296b17fc2e85dcd4521b6012232e11abed95
updated_at: 2026-10-21
---

# Web 用户管理需求/计划汇总

本文件是本需求的汇总入口。后续修改先更新这里的范围、进展与问题，再沿下方链接展开具体内容；任务实际状态仍以任务明细为准。

## 目标、基线与核查范围

目标是让受信平台管理员管理控制台账号、访问状态、角色和凭据，并保留既有设备与网络资源。本需求已经交付，后续独立的拓扑、导航或页面美化不计入其完成范围。

2026-10-21 只读核对的实现基线为 `feat/web-user-management@1e92296b17fc2e85dcd4521b6012232e11abed95`，对应实现工作树无未提交改动。本文所在文档分支为 `docs/architecture-and-user-management-plan@f48602e2674173be55871724f201da5982d62951`，仍有规则、文档迁移及规划的未提交改动；不能用该分支源码代替交付基线。此次核查只读取代码和既有记录，未运行应用验证，也未重新核对远端状态。

## 进度与内容入口

| 内容 | 文档进度 | 实现/交付进度及依据 | 下一步 |
| --- | --- | --- | --- |
| [需求](require/README.md) | 已有需求、验收及审核修复记录；早期缺口分析描述原基线 | 实现范围由 WUM-001～014 的交付记录覆盖，早期“缺少”描述不代表当前仍未实现 | 如修改需求，先对照本汇总与采用提交，澄清历史分析与当前事实 |
| [计划](plan/README.md) | 已拆为索引及10份 `plan-wum-编号-名称.md`，含契约与审核修复记录 | 文档细项不是新增任务；原14项任务已交付 | 保留交付证据；新增独立目标另立需求 |
| [任务看板与明细](task/README.md) | 14份 `task-wum-编号-名称.md`，分别链接对应计划细项 | 14/14 `done`；属于已交付基线的历史结果，本次未重跑 | 仅在有明确的同需求修复时更新任务及汇总 |
| [运维与验收记录](plan/operations.md) | 已记录通过项、环境限制及未测范围 | 历史统一入口 43/43、设备认证兼容 1/1；不代表生产部署或全部测试已通过 | 后续实际部署或代码变更按相应范围验证 |

## 当前事实与代码依据

以下代码链接均固定到上述交付提交；“代码存在”与“本次运行验证通过”分别表达。

| 编号 | 类型 | 已核查内容 | 代码依据与对后续工作的影响 |
| --- | --- | --- | --- |
| WUM-EV-001 | 事实 | 管理 API 与账号服务已实现，不再是假定接口 | [admin_users.rs](https://github.com/CiBeDerORG/EasyTier/blob/1e92296b17fc2e85dcd4521b6012232e11abed95/easytier-web/src/restful/admin_users.rs) 的 `/api/v1/admin/users` 等路由；[UserManagementService](https://github.com/CiBeDerORG/EasyTier/blob/1e92296b17fc2e85dcd4521b6012232e11abed95/easytier-web/src/user_management/mod.rs) 的 `create_user`、`set_access`、`set_role`、`reset_password`、`revoke_sessions` |
| WUM-EV-002 | 事实 | 本人改密受本地密码能力约束 | [ChangePassword.vue](https://github.com/CiBeDerORG/EasyTier/blob/1e92296b17fc2e85dcd4521b6012232e11abed95/easytier-web/frontend/src/components/ChangePassword.vue) 的 `canChangePassword` 要求 `localPasswordEnabled === true`，并处理 `local_password_unavailable`；后续页面修改须保留 |
| WUM-EV-003 | 事实 | 用户管理与改密已有路由，设备接入命令仍使用 config-server | [main.ts](https://github.com/CiBeDerORG/EasyTier/blob/1e92296b17fc2e85dcd4521b6012232e11abed95/easytier-web/frontend/src/main.ts) 的 `users`、`changePassword`；[MainPage.vue](https://github.com/CiBeDerORG/EasyTier/blob/1e92296b17fc2e85dcd4521b6012232e11abed95/easytier-web/frontend/src/components/MainPage.vue) 的 `enrollCommand`，不把账号页面工作扩展到客户端连接、认证或配置下发 |
| WUM-EV-004 | 事实 | 用户管理 E2E 的程序路径解析已有修复 | [webTestBinary.mjs](https://github.com/CiBeDerORG/EasyTier/blob/1e92296b17fc2e85dcd4521b6012232e11abed95/easytier-web/frontend/tests/helpers/webTestBinary.mjs) 的 `resolveWebTestBinary` 优先显式路径，否则读取 Cargo target 目录；自动发现限 host/debug，不代表本次已运行 E2E |

## 问题、限制与待确认

| 编号 | 问题与来源 | 当前状态 | 影响 | 下一步/解除条件 | 负责人 |
| --- | --- | --- | --- | --- | --- |
| WUM-ISSUE-001 | 附加旧 Dashboard 回归缺浏览器，历史下载返回 403，见[运维记录](plan/operations.md) | 历史环境限制；本次未复核环境是否恢复 | 该附加回归不能计入通过，不改变已记录的用户管理专项结果 | 未来实际需要该回归时核实浏览器环境后执行并记录 | `null` |
| WUM-ISSUE-002 | 生产 IdP、HTTPS 代理部署、真实 VPN 数据面及旧二进制回退未覆盖，见[运维记录](plan/operations.md) | 明确未验证 | 已完成状态只覆盖本次约定的隔离实现与验收 | 进入对应部署/变更范围时单列验收，不从现有测试推断生产可用 | `null` |
| WUM-ISSUE-003 | 在途改密与撤销会话并发语义，见[运维记录](plan/operations.md) 的审核限制 | 历史加固建议；未纳入本次修复范围 | 不能宣称已承诺取消所有在途写入 | 后续确需加固时先明确行为契约和独立修复范围 | `null` |
| WUM-ISSUE-004 | 原需求第1～3节的“当前缺口”与源码链接混用了历史分析及交付基线，见[需求](require/README.md) | 本次已解决：标明历史范围，历史链接固定到 `e307dadb` | 避免把已完成用户管理误判为仍待实施 | 后续分析使用本汇总的交付基线；历史分析不参与当前进度统计 | `/root` |

以上是已有记录和本次代码核对能确认的范围，不代表重新完成全量缺陷排查。历史已解决的改密能力及 E2E 路径问题不再列为当前阻塞。

## 关联与历史记录

- [需求](require/README.md)
- [计划](plan/README.md)
- [任务看板与14项明细](task/README.md)
- [运维与已执行验收记录](plan/operations.md)

完成状态来自已提交并推送的 `feat/web-user-management@1e92296b`：包含用户管理、OIDC/local 改密能力修复与 E2E 路径修复。本次仅集中已有文档和证据，没有重新运行测试，也没有把业务代码合并到当前文档分支。源码引用使用已交付提交，避免把旧文档工作区源码当作实现基线。

用户管理与[拓扑资源集成](../easytier-ecosystem-integration-done/README.md)、[现有 Web 交互美化](../web-ui-optimization-done/README.md)是独立需求。后两者复用已交付账号/权限能力，不重做账号系统。保留 `config-server` 客户端连接、认证和配置下发行为。

原工作区未提交补充保留为[需求草案](require/history/local-draft.md)、[计划草案](plan/history/local-draft.md)及[契约任务草案](task/history/wum-001-local-draft.md)。它们的待执行/未核验状态仅是当时记录，不参与当前看板统计。

## 维护记录

2026-10-21：补齐汇总、进度、代码依据和已知限制；按 WUM-ISSUE-004 澄清历史分析。仅整理文档并只读核对源码、链接与状态，未运行构建、测试或 CI，未重新验证历史运行结果。

2026-10-21：计划按职责拆为10份小文件，14项任务文件统一增加 `task-` 前缀并关联具体计划；历史契约入口和原任务编号保持，文档拆分不改变功能完成状态。
