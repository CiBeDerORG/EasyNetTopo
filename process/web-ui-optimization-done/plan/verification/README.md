# WUI-112：组合联调与真实页面验收证据

2026-10-21；执行者 `iteration_exec1`。前置 [WUI-111 交接](../integration.md) 已获监工批准，按 [验收计划](../plan-wui-008-acceptance.md) 实施。本目录只记录实际执行证据，任务状态由总控维护。

## 采用版本与隔离

- 冻结 WUI 源码：`97112befa47618d2872c14066cbdfc3f75a7edf7`。前端服务仅使用 `/tmp/wui111-frozen-37747637/dist`；目录名含历史父提交，不代表产物版本。完整 SHA-256 文件清单见 [frozen-assets.json](frozen-assets.json)，entry hash 为 `9e89c4b52c28f2a00be1a16cf31d1b8567bafff44f3af7cd1875ea74b8c1e976`。
- 后端复用 `/workspace/EasyTier/target/debug/easytier-web`（fingerprint含default/ embed）。旧工作区 HEAD `0326521d77a7bc69881f48b2e0a29d00be854084` 与本轮 `easytier-web/src`、`Cargo.toml`、`build.rs` 比较无差异；来源还见 [ECO-102 环境记录](../../../easytier-ecosystem-integration-done/plan/design/baseline-evidence/README.md)。**没有重新构建后端，也没有服务该二进制内的旧 UI。**
- 真实套件每次创建独立 `/tmp` SQLite、虚构管理员与浏览器 context；用户管理套件另启动本地虚构 OIDC 提供方。API、前端、OIDC 使用各自动态 loopback 端口。最后矩阵运行的端口见 [environment.json](environment.json)；其他套件是另外的独立临时环境，不能将该文件解释为所有运行共享同一服务。
- 代理 `cors:false`，且 `changeOrigin:false` 保留浏览器Host/Origin，使请求到达原后端安全校验；不放宽生产CORS/CSRF。测试剔除继承的部署/代理/ET/OIDC环境。config-server沿用已授权隔离容器的TCP端口0、无客户端（旧实现通配监听，不声称loopback）；不发布端口、不修改防火墙。
- 使用系统 `/usr/bin/chromium`。测试结束关闭浏览器、preview、API和OIDC并删除临时数据库；运行结束 `ss -ltnp` 只余原环境1040监听。未触碰并行ECO的新源码及活动dist。

## 实际结果与证据等级

| 范围 | 实际执行结果 | 证据等级 |
| --- | --- | --- |
| `dashboard.test.mjs`完整集中回归 | **33/33通过**，105.4秒 | 冻结生产组件 + fictional HTTP fixture；不是实际设备通信 |
| `user-management-e2e.test.mjs` | **9/9通过**，46.2秒 | 真实API、SQLite、浏览器、虚构OIDC；包括权限/会话失效/强改/本地密码能力/CSRF/审计/故障草稿 |
| `web-ui-integration.test.mjs`真实网络、移动交互、空闲请求 | 修正等待选择器的第二轮 **3项通过、矩阵1项失败**；44.6秒 | 真实API/SQLite；网络ACL和设置实际写入后GET核对、缺失网络错误；移动键盘与复制；22秒空闲 |
| 同文件 `--test-name-pattern='real API page matrix'` | **1/1通过**，42.4秒测试时间；覆盖**96/96组合** | 矩阵修正后专项复验；不是声称上轮4/4整套通过 |
| `git diff --check`、专项脚本语法解析 | 通过 | 文档/脚本检查，不代替运行验证 |

四个专项场景分别均已有通过证据；没有为获得整套漂亮数字重复运行已通过场景。此次不重新构建业务前端（已冻结、111构建验证通过），不把旧的通过数算作本轮测试。

### 真实页面与交互

[page-matrix.json](page-matrix.json)：8页面（登录、关闭注册、本人改密、用户管理、工作台、空设备、网络列表、空成员详情）×中英×明暗×360/768/1440共96项，**整页横向溢出0、pageerror 0**。16张代表截图分别为每页英文浅色1440与中文深色360，其他组合有测量记录而非截图。

对比 [WUI-102 前态](../../require/evidence/README.md)：

| 问题/原任务 | 本轮真实证据 |
| --- | --- |
| WUI-OBS-001：Escape后焦点落BODY | 键盘Enter打开、Escape关闭，连续3次焦点返回移动菜单按钮 |
| WUI-OBS-002：长用户名被挤成竖列 | 360中文深色：用户名单元格172.95px；表格806.39px在299px容器内滚动，不扩大整页；见 [用户截图](users-cn-dark-360.png) |
| WUI-OBS-003：长标题挤压导航按钮 | topbar65px、按钮36px，正文仍完整展示长名；见 [网络详情截图](network-detail-cn-dark-360.png) |
| 设备接入命令 | Clipboard逐字等于实际页面命令，结构保持 `easytier-core --config-server tcp://127.0.0.1:0/<fictional-user>`；未执行，端口0不可当作实际接入地址 |
| ACL与设置 | 页面保存默认deny，经真实GET确认；修改网络名经真实GET确认；密钥input为password；不存在网络显示错误，不展示可编辑ACL |

实际交互尺寸及布尔证据见 [interaction-observations.json](interaction-observations.json)。其标题测量使用保存后附加`-edited`的长名称，矩阵截图使用初始长名称；两次独立隔离服务，不混称同一个网络对象。

[空闲请求记录](idle-requests.json)：离开设备页后观察网络列表22秒，`/networks`13次、`/auth/me`2次、`/blocked-devices`2次，全部GET，`/machines`0次。源码原NetworkList为2秒PeriodicTask，原壳层10秒interval；不能将其误写为只有壳层轮询或完全无轮询。本轮没有新增请求来源；不推断其他页面也具有同样频率。

## 首轮失败与修订

- Dashboard原fixture未提供`/auth/me`，现按真实CurrentUser给普通本地用户完整身份，保留页面/权限断言；浏览器启动显式使用系统Chromium，并支持指定冻结dist。没有为通过而改生产权限逻辑。
- 真实E2E初跑7/9：Vite字符串proxy默认改写Host，引起真实浏览器写请求被原CSRF拒绝；Vite默认CORS中间件抢先回复OPTIONS204，与原403断言不符。改为保留Host并关闭测试代理自身CORS后9/9通过，原安全断言未放宽。
- 新专项第一次before失败：虚构用户名不符合既有校验，返回400；换为符合约束的虚构用户名。之后矩阵等待关闭注册页不存在的`#username`、以及大小写不匹配的提示正则失败；改为真实双语注册关闭状态。移动Drawer采用项目已有`.console-mobile-nav`而非假设`role=dialog`；仍以键盘打开和焦点恢复作为断言。没有改变生产组件。

## 复现命令

在`easytier-web/frontend`执行，先由总控提供匹配的冻结dist及经核实后端；不要让本文件命令使用并行变动的dist。

```sh
WEB_TEST_DIST=/tmp/wui111-frozen-37747637/dist node --test tests/dashboard.test.mjs
WEB_TEST_DIST=/tmp/wui111-frozen-37747637/dist WUM_TEST_WEB_BINARY=/workspace/EasyTier/target/debug/easytier-web node --test tests/user-management-e2e.test.mjs
WEB_TEST_DIST=/tmp/wui111-frozen-37747637/dist WUM_TEST_WEB_BINARY=/workspace/EasyTier/target/debug/easytier-web node --test tests/web-ui-integration.test.mjs
WEB_TEST_DIST=/tmp/wui111-frozen-37747637/dist WUM_TEST_WEB_BINARY=/workspace/EasyTier/target/debug/easytier-web node --test --test-name-pattern='real API page matrix' tests/web-ui-integration.test.mjs
```

专项会更新本目录实际截图/JSON；重跑需记录采用版本，不覆盖前态需求证据。证据不保存Cookie、令牌、临时密码、网络密钥或完整配置。

## 限制与交接

- 真设备/实例未连接；非空成员、路由/peer/ACL统计、WireGuard编辑、运行配置等由33项fixture覆盖，不能称为真实VPN数据通信、配置下发或ACL执行验证。
- 真实OIDC来自本地虚构提供方；生产OIDC、HTTPS反代、线上部署、旧二进制回退仍未执行。当前真实注册策略关闭；未跑开放注册验证码流程。
- 系统Chromium窄viewport不是物理移动设备；未执行完整读屏器审计或全组件对比度认证。
- 此次联调没有发现需要回派生产源码的新增缺陷。WUI-113依据此证据验收归档；ECO接线后的正式生态产物另由ECO-114验证，不能把本冻结WUI产物当作生态路由完成证据。
