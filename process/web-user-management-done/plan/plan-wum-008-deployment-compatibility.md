---
id: PLAN-WUM-008
feature: web-user-management
status: done
summary: ../README.md
plan_index: README.md
requirement: ../require/README.md
task_board: ../task/README.md
baseline: feat/web-user-management@1e92296b17fc2e85dcd4521b6012232e11abed95
updated_at: 2026-10-21
---

# PLAN-WUM-008：部署、恢复与兼容契约

本文件拆自已交付的 PLAN-WUM，保留原节号及当时的计划/契约记录；不是新增任务或重新执行授权。计划细项编号与任务编号不一一对应。总体进度、代码基线及已知问题见[需求/计划汇总](../README.md)，执行状态见[任务看板](../task/README.md)。

- 关联任务：WUM-004～007、WUM-009～011。
- 现状依据：根汇总的 WUM-EV-001、WUM-EV-003；对应实现已在交付提交，不能将历史“待实施/待验证”措辞当作当前缺口。
- 验证依据：[运维验收记录](operations.md)及任务明细；本次只拆文档，未运行构建、测试或 CI。

### 8.5 部署、初始化、恢复与兼容

首期支持同源浏览器部署，生产经 HTTPS；跨源带 Cookie 管理不支持。开发仅 loopback HTTP。WUM-006 对管理写入要求 `Content-Type: application/json` 和 `X-Requested-With: EasyTier`；浏览器 Origin 存在时必须与请求受信任外部 origin 完全一致，拒绝 null/跨源；未提供 Origin 的非浏览器客户端仍须自定义头。CORS 不允许任意来源携带此头，拒绝跨源预检。代理必须保留可信 Host；不信任客户端可伪造 forwarded header。会话 HttpOnly、SameSite=Lax；HTTPS 部署 Secure=true（配置由006独占接入），loopback 测试可 false。既有认证写入及身份/策略接口保持旧跨源客户端兼容，不强制新增头或全局收紧CORS；新管理路径含读取与OPTIONS独立严格同源，部署细节见本文件的“8.7 v1.2 部署接线兼容澄清”。旧 GET logout 保留兼容且不承载其他操作。UA-14 必须验证实际策略而非仅配置存在。

初始化不新增可预测密码、不提升首个注册用户。**D-01 已解除：用户明确选择 A，旧账号保留登录兼容，不因历史默认密码自动停用，并明确提示立即修改默认密码。** 因此既有管理员关系、访问状态及凭据保留，不因用户名或已知固定哈希自动停用、强制改密、提升角色或改变会话权限。

v1.1 新增仅本人可见的 `CurrentUser.default_password_warning:boolean`：WUM-004 在 GET `/auth/me` 将当前账号密码哈希与原迁移已公开固定哈希精确比较，匹配时为 true，否则 false；不按用户名猜测，不返回哈希/默认密码，不放入用户列表、详情或审计。WUM-007 同步 DTO；WUM-008 提供中英文 `user_management.default_password_warning`（“当前账号仍使用历史默认密码，请立即修改密码。” / “This account still uses a historical default password. Change it immediately.”）及改密动作文案；WUM-009 在本人工作区持续显示可访问警告与本人改密入口，不能仅弹一次 toast。改密成功后的重新登录/身份刷新使标志归 false。历史默认凭据仍可登录，提示不阻断任何原有权限；新增临时凭据的强制改密规则不变。WUM-011 以隔离旧库验证默认凭据登录+警告、已改密码无警告及原资源保留。该决定不授权生产账号或凭据操作。

WUM-005 提供显式离线 `--recover-admin <username>` 入口：复用 DB 路径，停止正常服务启动，仅在受控终端从 stdin 读取新密码（不放 argv/env/日志），按旧 MD5+password_auth 存储；存在账号则保留 ID/资源，设为 local 凭据可用（source 历史 unknown 不改）、启用访问、授予 admins、增加版本；新用户名可创建本地管理员。操作员掌握数据库访问权限才可调用，记录 actor_id=null 的 recover/bootstrap 审计。CLI 不输出密码。另提供离线 `--bind-oidc-user <username> --oidc-issuer <issuer> --oidc-subject <subject>`，只建立核实后的绑定，不授予角色、不解除停用；冲突拒绝。恢复/绑定不通过公开 HTTP 暴露。

迁移保留所有旧用户 ID/哈希/关系/资源；历史日期 null。恢复入口已由 WUM-011 用隔离旧库验证；UA-12 已按 D-01 用户选择验证兼容登录和默认密码警告。降级旧二进制无法执行新停用/版本/身份边界，不允许以“可启动”代表安全回退；恢复备份前须在外部维护窗口隔离控制台访问，本文不授权操作真实 VPN/防火墙或生产。

### 8.7 v1.2 部署接线兼容澄清（WUM-006）

现有 `api_host`/前后端分端口部署已使用跨源Cookie调用。为不改变无关旧接口，既有auth与业务CORS保持原兼容行为；v1.1第8.5中既有auth强制同源的描述由本节替代。新管理路径（含GET、写入、OPTIONS）在全局permissive CORS外层独立严格同源；跨源管理不支持，需同源反代。管理写入继续强制JSON和自定义头。

`ET_WEB_PUBLIC_ORIGIN` 配置受信外部origin：HTTPS生产显式配置，Cookie自动Secure；开发仅loopback HTTP。未配置时仅接受loopback Host的HTTP管理，非loopback管理返回明确拒绝，不改变既有其他业务。非法origin配置启动失败，不信任Forwarded。006验证配置/同源/跨源预检行为，011记录运维部署步骤。数据库完全不可写时无法落failure审计，操作仍失败，不把无审计错误冒充成功。
