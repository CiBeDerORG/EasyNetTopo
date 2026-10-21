# Web 用户管理：升级、初始化与恢复

适用范围为[需求 UM-01–12](../require/README.md)与[PLAN-WUM 契约](README.md)规定的本地控制台账号管理及审核修复。实施与验收见[任务看板](../task/README.md)。这里提供操作者步骤；本次开发没有执行生产部署、真实账号修改、VPN 或防火墙操作。

本轮状态：**已处理**。WUM-001–011 的实现与隔离验收已完成，功能分支 `feat/web-user-management` 已正常推送。此状态不代表生产部署或下文明确未执行的验证已完成。

## 部署与身份边界

- 新管理页面和 `/api/v1/admin/*` 必须与浏览器同源。生产使用 HTTPS，并配置 `ET_WEB_PUBLIC_ORIGIN=https://console.example.invalid` 为实际受信任外部 origin；示例域名不可直接用于生产。Cookie 为 HttpOnly、SameSite=Lax，HTTPS 配置启用 Secure。
- TLS 终止代理需将同源页面与 API 转发到同一后端，并保留可信 Host。后端不信任客户端 Forwarded 头。不要把后端直接暴露为绕过代理的另一公开入口。
- 未配置该变量时，新管理接口只接受 loopback HTTP Host；非 loopback 管理请求被拒绝。loopback HTTP 用于开发，不能据此认为生产 HTTP 受支持。非法 origin 配置导致启动失败。
- 管理写入要求 JSON 和 `X-Requested-With: EasyTier`；拒绝跨源/null Origin及跨源预检。旧认证与其他接口保留兼容行为，这不是全站 CORS 改造。旧跨源前端需增加同源代理才能使用新管理能力。
- 停用控制台访问、改密和撤销 Web 会话不删除设备或网络，也不承诺断开数据面。关闭自助注册不关闭独立设备/OIDC 自动建号。外部 webhook 身份不等于本地 Web 身份，不能将本地停用解释为删除外部账号。
- OIDC 用 issuer/subject 稳定绑定。同名未绑定账号不会自动继承本地管理员。身份迁移需操作者核实绑定，不能仅按显示名匹配。
- 本人改密仅在服务端明确支持本地密码时开放，不能按账号来源判断。有本地密码的绑定账号仍可改密；OIDC-only 账号直接访问改密地址时提示到外部认证平台操作，其他未知能力提示联系管理员。`config-server` 的客户端连接、设备认证和配置下发与这次页面修复无关，本次源码完全保留。

## 升级已有数据库

1. 确认数据库路径、二进制版本、代理 origin、身份模式及可用的管理员恢复渠道。在维护窗口停止数据库写入者，保存原二进制、配置和数据库的一致性备份。SQLite 有 WAL 时使用受支持的在线备份方式，或在所有连接关闭后备份完整数据库；不要仅复制正在写入的主文件。
2. 在隔离副本上先启动新版本。启动自动执行新增 `m20261021_000011_user_management`；此前实际注册了八份迁移，最后编号为 `m20261001_000010_member_ip_allocation`，编号存在空缺，不代表十份文件。
3. 对比用户 ID、用户名、凭据可用性、组关系、设备与网络归属。新增元数据不推测历史来源或日期：`source=unknown`，未知凭据能力和历史时间为 null。成功本地登录可确认本地凭据能力，但不反推历史来源。
4. 验证管理员菜单及权限、普通用户无管理权限、已有密码登录、默认密码提示、会话撤销、使用中的身份模式与回退副本。账号数据及业务资源必须保持原归属。
5. 按既有上线流程完成受控验证后再开放管理访问。本指南没有执行或授权任何额外生产变更。

用户已选择兼容策略 A：历史默认凭据仍可登录，不因已知默认密码自动停用或强制改密。本人页面持续显示立即改密提示；本人改密并重新登录后提示消失。新创建/重置账号使用随机临时凭据，24 小时有效且首次登录必须改密；这与历史兼容策略不同。

## 新部署的首次初始化

旧发布迁移仍预置 `admin`（admins 组）和 `user`（users 组）。本功能不改写已发布迁移，不自动清理这两个账号，也不将首个公开注册用户提升为管理员。

在服务对外开放前，先用受保护输入显式替换预置管理员凭据。下列路径为示例，密码文件须由操作者安全准备、仅本人可读（例如 0600），内容为 12–128 字符的新密码和可选末尾换行；不要把密码放进 argv、环境变量、终端回显或命令历史。

```sh
./easytier-web --db /path/to/new.sqlite --recover-admin admin \
  < /protected/path/admin-password.txt
```

该命令在 Web/配置服务和日志系统启动前退出。成功只报告完成，不打印密码。失败应停止后续开放步骤并检查固定错误代码；不能以创建了数据库文件判断成功。

随后在受控访问环境中处理普通预置 `user`：由本人登录改密，或由管理员明确停用其控制台访问。不要为了仅改密码执行 `--recover-admin user`，该命令会同时授予管理员角色。执行 `--recover-admin another-name` 只会创建/恢复指定管理员，**不会替换或清理预置 admin/user**。无需另外建立一套默认管理员流程。

## 管理员遗失或停用后的恢复

停止正常服务及其他数据库写入者，先备份，再由拥有数据库文件权限的操作者执行：

```sh
./easytier-web --db /path/to/existing.sqlite --recover-admin existing-name \
  < /protected/path/recovery-password.txt
```

存在账号时保留 ID 与资源，设置新本地凭据、恢复控制台访问、授予管理员组并递增会话版本，旧密码与旧 Web 会话失效。不存在时创建显式本地管理员。该命令不是单纯改密：不得用于未经授权的提权。恢复动作记录 `actor_id=null` 的 recover/bootstrap 审计。确认新密码登录、资源归属与旧会话失效后恢复正常服务。

显式绑定已经核实的 OIDC 身份：

```sh
./easytier-web --db /path/to/existing.sqlite \
  --bind-oidc-user existing-name \
  --oidc-issuer https://idp.example.invalid \
  --oidc-subject fictional-verified-subject
```

示例 issuer/subject 均为虚构值。绑定不授予角色、不解除停用；同一身份已绑定其他用户时拒绝。IdP 登录仍须满足 issuer、签名、state、nonce/PKCE 等验证。不要将外部显示名当作身份凭据。

## 备份恢复与回退限制

备份恢复会回到备份时的凭据、权限、资源与会话状态；升级后新增的停用、版本、审计及身份绑定可能丢失。因此恢复副本必须先保持管理入口隔离，核对身份和访问政策后再开放。不要在旧数据库上期待新版本状态仍有效。

旧二进制不认识 `user_access/auth_version/user_identities` 的访问与绑定限制。移除新元数据后，旧密码哈希仍然存在；旧二进制可启动不表示仍能强制停用或会话撤销。不能将直接降级称为安全回退。应按维护窗口计划选择一致性备份和匹配版本，并重新核实访问控制。本次只在隔离库验证了备份恢复和新增迁移 down 的状态边界，**没有运行旧二进制或生产代理回退**。

## 验证命令与证据

环境需要仓库指定 Rust、protoc/C 工具链、Node/pnpm，以及本机 Chromium。不要使用生产库、生产环境变量或真实身份。以下从仓库根运行；升级测试会启动临时后端进程，使用 loopback HTTP；配置 TCP listener 使用临时端口 0，沿用程序的 wildcard 绑定，无客户端接入，不创建 Core/TUN 或网络实例。

```sh
pnpm --dir easytier-web/frontend install --frozen-lockfile
pnpm --dir easytier-web/frontend test:user-management
cargo test --locked -p easytier-web user_management_upgrade_ -- --include-ignored
cargo test --locked -p easytier-web wum_external_webhook_and_device_heartbeat_preserve_disabled_account
```

统一前端入口先构建前端与 `easytier-web --features embed`，再运行所有用户管理测试。升级测试默认使用 `target/debug/easytier-web`；自定义 Cargo target 目录时以 `WUM_TEST_WEB_BINARY` 指向已构建的本次二进制。两项真实进程测试带有显式 ignore 原因，普通 Cargo/nextest CI 默认跳过，因为归档测试不保证附带独立 Web 二进制；上述 `--include-ignored` 是必须的升级验收入口，已实际运行。显式运行时找不到二进制会明确失败，不静默跳过。浏览器可用 `WEB_TEST_CHROMIUM` 指定已安装路径，默认 `/usr/bin/chromium`。测试会清理临时库；命令日志只报告状态，不应输出凭据。

用户管理浏览器 E2E 已单独修复路径发现：优先使用 `WUM_TEST_WEB_BINARY`；未设置时，在 `easytier-web/frontend` 运行 Cargo metadata 获取实际 target 目录，支持 `CARGO_TARGET_DIR` 及 Cargo 配置。相对覆盖路径以该 frontend 目录为基准，路径不存在或不可执行时明确失败，不回退仓库旧产物。自动发现对应包脚本默认的 host/debug 构建；设置 `CARGO_BUILD_TARGET`、`build.target` 或其他 profile 时应显式指定可在本机运行的本次二进制。Rust 升级测试和旧 `central-e2e` 的路径逻辑未在本次修改。

```sh
CARGO_TARGET_DIR=/path/to/build-output pnpm --dir easytier-web/frontend test:user-management
# 使用已经构建的指定程序直接验证真实用户管理 E2E：
WUM_TEST_WEB_BINARY=/path/to/easytier-web pnpm --dir easytier-web/frontend exec node --test tests/user-management-e2e.test.mjs
```

已执行结果（2026-10-21）：

| 检查 | 实际结果与证据 |
| --- | --- |
| 用户管理统一入口 | 32/32：客户端 mock 6、组件 mock 7、真实后端/SQLite/Chromium/OIDC 9、生产预览导航 mock 10；`/tmp/wum010-runner-final.log` |
| OIDC 异常路径补强 | 1/1；包含错误 state/nonce、token 错误明文及 Rust Debug 字节序列泄漏检查；`/tmp/wum010-oidc-final.log` |
| 外部 webhook 与中心 DeviceAuth | 1/1；真实 loopback HTTP 到用户映射及停用前后资源/会话授权保持；`/tmp/wum010-webhook.log` |
| 存储和账号服务 | 9/9；`/tmp/wum005-tests.log` |
| 认证生命周期 | 5/5；`/tmp/wum005-auth-recheck.log` |
| 管理 HTTP / 权限隔离 | 3/3 与 1/1；`/tmp/wum006-http.log`、`/tmp/wum006-permissions.log` |
| 旧库升级、离线 CLI 与备份 | 3/3（显式 `--include-ignored` 执行两项真实进程/HTTP验收、一项既有迁移回归）；`/tmp/wum011-upgrade.log` |
| 无管理员时公开注册业务 | 1/1；删除全部管理员组关系后调用真实注册 Backend，账号仍为普通用户且无 users.manage；`/tmp/wum004-registration-no-admin.log`（业务层，非 HTTP captcha 联调） |

`/tmp` 日志是本次环境证据，不是随仓库发布的永久工件；可用以上命令重现。联调曾发现 OIDC 错误日志携带外部响应体，以及生产 i18n 裸 @ 导致空弹窗，两项均已修复并重验。没有把未运行的旧代码日志红测试算作证据。

| 验收 | 覆盖与限制 |
| --- | --- |
| UA-01 | 真实管理 HTTP 权限；普通用户真实浏览器无入口；真实数据库权限回归 |
| UA-02 | 真实 HTTP 并发创建/冲突与浏览器创建 |
| UA-03 | 真实注册关闭接口、浏览器入口及管理员仍可创建 |
| UA-04 | 多真实 Cookie 会话停用、恢复及旧会话不复活 |
| UA-05 | 重置后旧凭据/会话失效，真实浏览器临时密码受限改密 |
| UA-06 | 本地签名 OIDC 身份服务完整流程、稳定绑定、同名冲突与停用；历史未知字段实测 |
| UA-07 | 真实数据库事务并发与 HTTP 最后两位管理员竞态 |
| UA-08 | 实际设备认证/会话与数据库资源保留；没有运行真实 VPN 数据面通信 |
| UA-09 | 外部 loopback webhook 到本地映射完整函数链和重复身份保护；不声称删除/停用外部身份 |
| UA-10 | 真实已打开页面撤权后 401 退出；403 能力刷新由生产导航 mock 验证 |
| UA-11 | 真实离线失败保留草稿和失效退出；迟到响应/403 分支由导航 mock 验证 |
| UA-12 | 真实旧库迁移、默认凭据登录/警告/改密、CLI初始化与恢复、OIDC绑定不提权、备份恢复和元数据降级边界；另有无管理员时注册不提权的真实业务层回归；未运行旧二进制 |
| UA-13 | 实际服务与浏览器日志/审计扫描、OIDC 异常脱敏；数据库完全不可写时不能保证失败审计落库 |
| UA-14 | 真实 HTTP 同源/CSRF/Cookie/禁止缓存与 Secure 配置分支；未执行 HTTPS 代理部署 |

本次没有运行全量 Cargo 网络套件、真实 VPN/防火墙修改、生产 IdP、生产账户、生产部署或旧二进制回退。支持范围及上述限制必须随交付保留。

## 审核修复验证（2026-10-21，WUM-012～014）

基线 `0326521d`；在 `feat/web-user-management` 继续交付，需求、计划和任务已同步。页面使用已存在的身份能力和错误码；没有 Rust、数据库、协议、config-server、设备认证、配置下发或计时器改动。独立代码复核通过。

| 检查 | 结果与范围 |
| --- | --- |
| 生产构建及统一用户管理入口 | `pnpm --filter easytier-frontend test:user-management` 通过，含前端类型/构建、Rust embed 构建及 **43/43**：路径6、客户端mock6、组件mock7、真实服务/SQLite/Chromium/OIDC9、生产页面mock15；`/tmp/wum-fix-full.log` |
| UA-15 改密能力 | 新增中英文/窄屏/键盘、未知能力、OIDC来源但本地凭据可用、服务端能力拒绝测试；真实OIDC登录后检查菜单与直接路由，并确认真实后端拒绝本地改密；原首次改密流程继续通过 |
| UA-16 程序路径 | 真实 Cargo metadata 覆盖默认、绝对/相对 target、Cargo配置与显式覆盖；移除先前审核临时软链接后，在 `CARGO_TARGET_DIR=/workspace/.easytier-env/target` 下完整统一入口通过，没有设置 `WUM_TEST_WEB_BINARY` |
| 设备认证兼容回归 | `cargo test --locked -p easytier-web wum_external_webhook_and_device_heartbeat_preserve_disabled_account -- --test-threads=1` **1/1** 通过；`/tmp/wum-fix-device-auth.log`；不代表真实VPN数据面测试 |
| 附加旧 Dashboard 回归 | 未完成：默认 Playwright headless shell 缺失，33项均在启动钩子失败，未执行行为断言；安装下载被CDN返回403 Domain forbidden。记录 `/tmp/wum-fix-dashboard.log`、`/tmp/wum-fix-browser-install.log`，未修改该测试或降低断言 |

用户管理43项使用已安装的系统Chromium，和上表旧Dashboard浏览器依赖限制不同。原有Browserslist过期与构建大包警告仍存在。新改动的Markdown链接和 `git diff --check` 已检查；`/tmp` 为当前环境临时证据，可用记录的命令重现。未重新运行全量Rust/网络测试、生产IdP/HTTPS代理或生产部署；在途改密与撤销会话的并发语义仍为先前单列的加固建议。
