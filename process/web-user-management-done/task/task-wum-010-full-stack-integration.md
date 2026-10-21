---
id: WUM-010
feature: web-user-management
summary: ../README.md
plans: [PLAN-WUM-003]
status: done
owner: /root/wum010
depends_on: ["WUM-006","WUM-009"]
write_scope: ["easytier-web/frontend/tests/user-management-e2e.test.mjs","easytier-web/frontend/package.json","easytier-web/src/client_manager/session/webhook_validation.rs (tests only, /root/wum003)"]
requirements: ["UM-01","UM-02","UM-03","UM-04","UM-05","UM-06","UM-07","UM-08","UM-09","UM-10"]
acceptance: ["UA-01","UA-02","UA-03","UA-04","UA-05","UA-06","UA-07","UA-08","UA-09","UA-10","UA-11","UA-13","UA-14"]
updated_at: "2026-10-21"
---

# WUM-010：真实前后端联调

拆分后对应计划：[PLAN-WUM-003](../plan/plan-wum-003-verification-and-change.md)。本任务保留原编号、状态和历史交付记录；维护入口为[需求/计划汇总](../README.md)。

需求：[Web 用户管理](../require/README.md)。计划：[PLAN-WUM](../plan/README.md)。[返回看板](README.md)。

## 目标与边界

使用真实后端、数据库和浏览器验证跨端账号管理，集中接入用户管理测试执行入口。现有 test:browser/test:e2e 只指定已有文件，新增测试必须实际被运行。

## 输入与输出契约

输入 WUM-006 管理 API、WUM-009 Web 流程、各模块测试记录和 WUM-001 部署契约；准备隔离测试库、管理员/普通用户与多会话。输出可复现联调结果与验收映射，交付 WUM-011。

## 执行步骤

1. 核对前后端交付使用同一契约，准备隔离数据库、至少管理员与普通用户、多浏览器会话，以及适用身份模式的测试配置。
2. 先通过真实 HTTP 检查授权、账号状态、并发与会话规则，再让浏览器完成列表、创建、停用、重置、角色、审计和错误恢复流程。
3. 为分页筛选及独立撤销会话设置专门场景，将新增测试接入实际执行入口，核对运行结果确实包含新增用例。
4. 按 UA 编号整理证据；失败回派对应模块任务，等待修复交接后重验受影响场景，再将通过结果交给升级验收。

## 实施与验收

- [x] 覆盖关联 UA 场景：真实权限、创建与冲突、停用/恢复、改密、角色并发、会话撤销、注册策略、审计及跨站写入策略。
- [x] 独立检查 UM-02 的服务端搜索、角色/状态筛选、稳定分页、空结果和历史未知值；不能仅验证列表读取成功。
- [x] 独立检查 UM-07：保持密码、角色和访问状态不变，主动撤销全部 Web 会话；旧会话失效，重新登录可用。
- [x] 按适用身份模式验证本地/OIDC/外部映射；未执行或不适用需具体说明，不用 mock 结果替代。
- [x] 验证组件反馈与服务器结果一致，设备/网络行为遵守仅停用控制台的范围；不输出真实凭据或哈希。
- [x] 接入可运行的测试命令并运行；单元、mock、HTTP、浏览器结果分别报告。
- [x] 所有关联运行时验收有结论，未解决缺陷不标为通过；旧库升级/恢复整体结论由 WUM-011 完成。

## 交付清单

- 已接入执行入口的真实前后端联调用例及可复现的环境准备、运行步骤。
- 按 UA 场景记录的结果，区分真实服务、mock、失败、未执行与不适用，敏感数据脱敏。
- 缺陷及回派/复验记录、交给 WUM-011 的运行产物与部署限制。

## 阻塞与交接

当前状态：已处理（`done`）。前置依赖、实现、验收及下游交接均已完成，无本轮遗留阻塞。最终验证范围与未验证项目见[运维验收记录](../plan/operations.md)。

以下保留原执行阶段的依赖与写入规则，不表示当前仍待开发：

等待 WUM-006/009。联调失败回派原所有者并记录关联缺陷；需要改业务文件时先由协调者调整写入范围与任务状态。本项不默认拥有全部前后端代码。退出条件为适用验收通过、缺陷闭环、测试入口与证据可复现。

## 执行记录（历史顺序）

以下记录保留各阶段当时的状态；当前结论为已处理，以文件顶部 `status: done` 和最终验收记录为准。

- 2026-10-21：仅完成任务规划，尚未认领或执行；未运行本任务的实现测试。

- 2026-10-21：006真实HTTP验收、009导航mock验收后认领；从真实隔离后端/DB开始浏览器联调，补完整测试OIDC身份服务与各UA证据，禁止使用真实身份或凭据。

- 2026-10-21：真实embed构建通过；OIDC日志复核发现错误Debug携带响应体风险，回派004，待其修复后联调验证。尚未声明真实浏览器/IdP通过。

- 2026-10-21：移交003独占webhook_validation.rs现有tests区，补真实loopback外部webhook到本地用户映射的完整函数链；不启动Core/TUN/RPC消费者。验证停用后控制面会话与资源保持，实际数据面在线通信仍不在已执行验证内。010统一串行执行该Rust测试。

- 2026-10-21：`cargo test --locked -p easytier-web wum_external_webhook_and_device_heartbeat_preserve_disabled_account` 实际1/1通过（/tmp/wum010-webhook.log）；真实外部HTTP验证2次、node-connected1次，中心DeviceAuth验证2次，停用前后会话授权与账号/设备/网络归属保持。未启动数据面。

- 2026-10-21：统一入口`pnpm --dir easytier-web/frontend test:user-management`实际32/32通过，含客户端mock6、组件mock7、真实后端/SQLite/浏览器/OIDC9、production导航mock10；前端及embed构建通过，日志/tmp/wum010-runner-final.log。先前31/32失败为测试路由定位错误，已按真实/#/auth修正后完整重跑。追加OIDC字节泄漏断言与state/nonce失败路径正在单独复验，未进行旧代码red重演。

- 2026-10-21：最后OIDC补强实际1/1通过（/tmp/wum010-oidc-final.log）：明文及Rust Debug字节数组泄漏检查、有效OAuth错误响应、错误state 400/nonce 401。所有适用运行时验收通过，010交付；UA-12交011。UA-11的403能力刷新/迟到响应为mock导航验证，其余核心动作使用真实服务；未执行真实VPN数据面或HTTPS代理部署。
