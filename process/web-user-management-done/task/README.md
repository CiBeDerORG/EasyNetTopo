# Web 用户管理任务看板

更新日期：2026-10-21。协调者维护此摘要；状态、负责人、依赖及验证证据以任务明细 YAML 与执行记录为准，见 [协作规则](../../README.md)。当前为手工维护的 Markdown 看板，无自动同步服务。

## 当前需求

| 主题 | 需求分析 | 实施计划 | 当前阶段 |
| --- | --- | --- | --- |
| Web 用户管理 | [需求与验收](../require/README.md) | [PLAN-WUM](../plan/README.md) | 14/14 已完成：含审核修复与真实联调；交付 feat/web-user-management |

## 状态概览

| backlog | ready | in_progress | blocked | in_review | done | cancelled |
| --- | --- | --- | --- | --- | --- | --- |
| 0 | 0 | 0 | 0 | 0 | 14 | 0 |

原11项任务均已处理（`done`），累计11个子agent按依赖执行。D-01按用户选择A实现并验证：旧账号保留登录、明确改密提示。测试类型、复现入口与未测范围见[运维验收记录](../plan/operations.md)，不包含生产部署。

## 任务明细

主题入口：[需求目录](../README.md)。每项任务已单独记录执行步骤、验收与交付清单；本看板保留统一状态摘要。

| 任务与明细 | 状态 | 当前负责人 | 直接前置任务 |
| --- | --- | --- | --- |
| [WUM-001 范围与接口契约](task-wum-001-contract.md) | `done` | `/root/wum001` | 无 |
| [WUM-002 按用户查询权限](task-wum-002-permissions.md) | `done` | `/root/wum002` | 无 |
| [WUM-003 账号存储与数据库迁移](task-wum-003-storage.md) | `done` | `/root/wum003` | WUM-001 |
| [WUM-004 认证与会话生命周期](task-wum-004-auth-lifecycle.md) | `done` | `/root/wum004` | WUM-002、WUM-003 |
| [WUM-005 账号管理业务与恢复](task-wum-005-account-service.md) | `done` | `/root/wum005` | WUM-003 |
| [WUM-006 管理 API 与后端接线](task-wum-006-admin-api.md) | `done` | `/root/wum006` | WUM-004、WUM-005 |
| [WUM-007 Web HTTP 客户端与错误语义](task-wum-007-web-client.md) | `done` | `/root/wum007` | WUM-001 |
| [WUM-008 用户管理组件与文案](task-wum-008-user-interface.md) | `done` | `/root/wum008` | WUM-001 |
| [WUM-009 前端导航与身份流程接线](task-wum-009-web-integration.md) | `done` | `/root/wum009` | WUM-007、WUM-008 |
| [WUM-010 真实前后端联调](task-wum-010-full-stack-integration.md) | `done` | `/root/wum010` | WUM-006、WUM-009 |
| [WUM-011 旧库升级与交付验收](task-wum-011-upgrade-acceptance.md) | `done` | `/root/wum011` | WUM-010 |
| [WUM-012 改密能力与反馈](task-wum-012-password-capability.md) | `done` | `/root` | WUM-009 |
| [WUM-013 E2E 程序路径](task-wum-013-e2e-binary.md) | `done` | `/root/wum_e2e_fix` | WUM-010 |
| [WUM-014 修复联调与交付](task-wum-014-review-integration.md) | `done` | `/root` | WUM-012、WUM-013 |

WUM-012/013 已按独立文件并行完成，WUM-014 真实联调通过并随本次修复提交推送。用户指定的 config-server 客户端连接、认证、配置下发逻辑不改动。

## 并行安排与联调（已执行）

- 起始可并行：WUM-001 范围/契约与 WUM-002 权限查询修复。
- 契约交付后可并行：WUM-003 存储、WUM-007 HTTP 客户端、WUM-008 独立组件。
- 存储交付且权限修复完成后：WUM-004 认证与 WUM-005 业务服务可并行；前端继续推进。
- WUM-006 后端接线与 WUM-009 前端接线可分别进行；两者完成后进入 WUM-010 真实联调。
- WUM-011 承接旧库升级、恢复与交付验收。依赖图和文件单一写入者详见 [计划](../plan/README.md)。

认领时由协调者同步明细和看板，填写实际 agent/负责人；不要把建议职责当成已分配身份。新增任务使用 [任务模板](../../templates/task.md)，保留编号和历史证据。
