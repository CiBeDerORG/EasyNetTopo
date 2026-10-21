# WUI-106：工作台保留与局部回归

负责人 `iteration_exec2`；分支 `feat/ecosystem-web-ui`，恢复核对 HEAD `50e9edb4d8814028a4c35b46e290bf826b47ede2`。采用 [公共契约 v1](../contract.md) 与 [WUI-102 页面前态](../../require/evidence/README.md)。

## 实施结论

WUI-102 已有 Dashboard 中英文、主题和三宽度的真实页面矩阵，未记录需本页修复的确定缺陷；本任务又验证当前计数、部分失败、重试与外部 Console 模式，结果符合原契约。因此 **Dashboard.vue 保留不改**，不为任务编号重构已满足要求的页面。

新增独占 `frontend/tests/web-ui-dashboard.test.mjs`，使用 WUI-103 的生产 preview/browser 支持和完整虚构身份 HTTP fixture。测试只证明真实组件对给定响应的处理，fixture 中的“设备数/在线数/实例数”不是实际 VPN/设备数据。

## 实际验证（2026-10-21）

工作目录 `/workspace/EasyTier-iteration/easytier-web/frontend`。

| 检查 | 结果与边界 |
| --- | --- |
| `node --test --test-name-pattern='overview uses\|external Console' tests/dashboard.test.mjs` | **0/2 通过**；before hook 因默认 Playwright 缓存 Chromium 不存在失败，没有进入业务页面，不算页面功能失败或通过 |
| `node --test tests/web-ui-dashboard.test.mjs` | **3/3 通过**，0 失败/跳过；系统 Chromium、动态 loopback 端口、独立 context，结束关闭 |
| 本任务 `git diff --check` 与文档链接检查 | 通过 |

三个专项场景：

1. 初始虚构计数为设备 2/在线 1/网络 1/实例 1；设备/网络深链存在。网络请求失败时保留最后已知网络数 1，成功的设备源更新为 1；明确失败提示，手动刷新后网络 0 与错误消失。360px 页面无整体横向溢出。
2. 首次 summary/machines 失败显示 `—`，不显示假零或“No devices yet”；已成功网络数据仍展示。
3. 外部 Console 仅展示设备相关三项统计，无中央网络请求/链接；请求挂起时重复刷新未增加并发 summary 请求。每项均无 pageerror。

此次未修改应用源码，不重复构建；使用 WUI-105 已类型检查/生产构建的未变 Dashboard 与基础产物。不能把已有构建或 WUI-102 全矩阵写成本任务重新运行。未变的已有 2 秒 PeriodicTask、卸载清理与 centralEnabled 条件保留，未新增请求源、轮询或重试机制。

## 集中测试回派 WUI-112

集中 `tests/dashboard.test.mjs` 归 WUI-112，未修改。已证实运行阻塞是 `chromium.launch({ headless: true })` 没有使用系统 Chromium/`WEB_TEST_CHROMIUM`，而本环境没有默认下载的浏览器。另静态核实其 fixture 未提供当前 `/auth/me` 身份响应；该点本次未进入浏览器复现，不能写为已运行失败原因。WUI-112 应在其范围修复测试环境及身份 fixture 后重跑集中用例；本专项通过不替代整个集中文件通过。

## 交接与限制

WUI-111 保持原 `api/centralEnabled` 输入和设备/网络路由；无需新的公共样式、文案或事件。WUI-112 负责集中真实链路与回归。没有真实设备/实例、ACL/VPN运行、数据库、OIDC或公网验证；没有修改 config-server、真实账户或网络设置。状态、看板及提交由总控维护。
