---
id: PLAN-WUM-010
feature: web-user-management
status: done
summary: ../README.md
plan_index: README.md
requirement: ../require/README.md
task_board: ../task/README.md
baseline: feat/web-user-management@1e92296b17fc2e85dcd4521b6012232e11abed95
updated_at: 2026-10-21
---

# PLAN-WUM-010：改密与 E2E 审核修复计划

本文件拆自已交付的 PLAN-WUM，保留原节号及当时的计划/契约记录；不是新增任务或重新执行授权。计划细项编号与任务编号不一一对应。总体进度、代码基线及已知问题见[需求/计划汇总](../README.md)，执行状态见[任务看板](../task/README.md)。

- 关联任务：WUM-012～014。
- 现状依据：根汇总的 WUM-EV-002、WUM-EV-004；对应实现已在交付提交，不能将历史“待实施/待验证”措辞当作当前缺口。
- 验证依据：[运维验收记录](operations.md)及任务明细；本次只拆文档，未运行构建、测试或 CI。

## 10. 审核修复计划（2026-10-21）

对应需求第13节 UM-11/12，基于 `0326521d` 继续在 `feat/web-user-management` 实施。沿用 `/auth/me` 和改密错误码，不改 Rust、设备链路或数据库。

1. [WUM-012 改密能力与反馈](../task/task-wum-012-password-capability.md)：协调者负责 MainPage、ChangePassword、中英文文案与导航测试；入口及表单均使用明确的本地密码能力，并处理服务器能力拒绝。
2. [WUM-013 E2E 程序路径](../task/task-wum-013-e2e-binary.md)：独立 Agent 负责 E2E 文件的产物路径解析、专用 helper/路径回归和必要脚本，不改前端组件。优先显式二进制，其次 Cargo metadata 的 target 目录，保留错误信息。
3. [WUM-014 修复联调与交付](../task/task-wum-014-review-integration.md)：等待上述两项交付，再执行生产构建、浏览器测试和真实服务 E2E；复核 diff 中没有 config-server 相关修改，记录通过/失败/未测范围后提交并正常推送。

WUM-012 与013写入范围互不重叠，可并行；013的 E2E 文件交给014后，由014追加真实 OIDC 本人改密页面回归。所有需求/计划/任务/看板/运维记录仅由协调者写入。前期审核建立的忽略目录软链接须移除后验证，不能掩盖路径缺陷。失败回派对应所有者，联调通过前不标记完成。

执行结果：WUM-012/013 独立验证后完成 WUM-014 联调，统一入口43/43、设备认证兼容1/1通过。无固定路径软链接，无后端/config-server变更，独立复核通过。旧Dashboard附加回归受浏览器下载403限制，未计入通过；完整证据与限制见[运维记录](operations.md)。本次按授权提交并正常推送用户管理分支，原生态规划工作区保持不动。
