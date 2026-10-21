---
id: WUM-006
feature: web-user-management
summary: ../README.md
plans: [PLAN-WUM-005, PLAN-WUM-006, PLAN-WUM-008]
status: done
owner: /root/wum006
depends_on: ["WUM-004","WUM-005"]
write_scope: ["easytier-web/src/restful/admin_users.rs","easytier-web/src/restful/admin_users/","easytier-web/src/restful/mod.rs"]
requirements: ["UM-01","UM-02","UM-03","UM-04","UM-05","UM-06","UM-07","UM-09"]
acceptance: ["UA-01","UA-02","UA-04","UA-05","UA-06","UA-07","UA-10","UA-13","UA-14"]
updated_at: "2026-10-21"
---

# WUM-006：管理 API 与后端接线

拆分后对应计划：[PLAN-WUM-005](../plan/plan-wum-005-http-contract.md)；[PLAN-WUM-006](../plan/plan-wum-006-storage-contract.md)；[PLAN-WUM-008](../plan/plan-wum-008-deployment-compatibility.md)。本任务保留原编号、状态和历史交付记录；维护入口为[需求/计划汇总](../README.md)。

需求：[Web 用户管理](../require/README.md)。计划：[PLAN-WUM](../plan/README.md)。[返回看板](README.md)。

## 目标与边界

将已完成的认证策略和账号业务接入受保护的 HTTP 管理路由；这是后端内部集成任务，不承担所有前端或部署验收。admin_users 相关路径为建议新增。

## 输入与输出契约

输入为 WUM-004 认证/会话与 WUM-005 服务。输出与 WUM-001 契约一致的真实 HTTP 行为、错误包络及能力检查，供 WUM-007/009 已完成的前端接线在 WUM-010 中联调。

## 执行步骤

1. 接收认证与服务交付，逐一核对管理端点、权限要求、安全 DTO 和错误映射；契约差异先交协调者处理。
2. 在本项拥有的 handler 和 restful/mod.rs 中注册模块、连接路由/服务及认证策略，集中落实管理授权与部署要求的 Cookie/同源/CSRF 保护。
3. 建立真实 HTTP 与隔离数据库测试路径，准备匿名、普通、管理员及受限会话，执行各管理操作及授权/冲突边界验证。
4. 对照契约复核实际响应、审计和敏感信息处理，将实现缺陷回派原所有者，形成前后端联调输入。

## 实施与验收

- [x] 每个管理路由服务端校验身份和 users.manage；匿名 401、普通用户 403；受限改密会话不能调用管理动作。
- [x] 使用安全 DTO，完成分页/参数/冲突等错误映射；无密码哈希泄漏，不扩大内部共享 token 权限。
- [x] 按部署契约实现 Cookie/同源/CSRF 策略；本项独占 restful/mod.rs 的路由、注入和必要 Session/CORS 接线。
- [x] 用真实 HTTP + 数据库验证创建、状态、重置、角色、撤销、审计和必要并发场景；记录故障与边界结果。

## 交付清单

- 已接入真实路由的管理 API 与安全 DTO/错误映射，说明消费的认证和服务版本。
- 真实 HTTP/数据库验证记录及脱敏请求示例，覆盖不同身份和相关失败路径。
- 给 WUM-010 的服务启动/测试准备说明、部署策略和剩余联调项；不包含真实凭据。

## 阻塞与交接

当前状态：已处理（`done`）。前置依赖、实现、验收及下游交接均已完成，无本轮遗留阻塞。最终验证范围与未验证项目见[运维验收记录](../plan/operations.md)。

以下保留原执行阶段的依赖与写入规则，不表示当前仍待开发：

等待 WUM-004/005 交付。可与 WUM-009 并行。输入缺陷回派原任务；不在其仍开发时修改认证或服务文件。退出条件为管理 API 的针对性集成验证通过且契约一致，交付 WUM-010。

## 执行记录（历史顺序）

以下记录保留各阶段当时的状态；当前结论为已处理，以文件顶部 `status: done` 和最终验收记录为准。

- 2026-10-21：仅完成任务规划，尚未认领或执行；未运行本任务的实现测试。

- 2026-10-21：004/005验收后认领；浏览器受限中间件须位于认证层内，管理路由同源保护必须覆盖OPTIONS并防止外层permissive CORS提前处理；内部token路径保持独立。

- 2026-10-21：v1.2接线澄清见计划8.7：新增管理路径独立严格同源保护含OPTIONS，旧auth跨源兼容保持；ET_WEB_PUBLIC_ORIGIN明确部署origin并控制Secure，未配仅loopback管理。实际验证待本项交付。环境短暂断连通知后协调者只读核实仓库可用/无未提交业务改动/无残留构建，未重复启动测试。

- 2026-10-21：管理路由已接生产共用account_routes/browser_session_routes，真实权限/受限会话中间件、参数安全错误、仅安全DTO及no-store；管理同源检查在legacy CORS外层含OPTIONS，内部token仍独立。ET_WEB_PUBLIC_ORIGIN明确HTTPS/Secure与loopback开发边界。
- 验证：`cargo test --locked -p easytier-web admin_http` 3/3通过（真实127.0.0.1动态端口HTTP+SQLite，所有管理方法身份矩阵/限制/参数/创建冲突/生命周期/独立撤销/筛选分页/审计/并发/Origin预检/旧auth跨源兼容/Secure配置）；原权限回归1/1再次通过。首轮测试fixture编译错误已修正。日志 `/tmp/wum006-http.log`、`/tmp/wum006-permissions.log`。真实浏览器与完整IdP交010。

- 审计范围明确：路由层匿名/未授权/受限/CSRF拒绝不执行业务且不写账号审计；服务层目标/自身/最后管理员等业务拒绝由005单独事务记denied。只读请求不记审计；不承诺全量HTTP安全事件日志。
