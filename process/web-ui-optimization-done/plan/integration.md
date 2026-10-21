# WUI-111：宿主壳层接线交接

负责人 `iteration_exec2`，分支 `feat/ecosystem-web-ui`。总控确认 WUI-104～110 全部经监工验收后交付 MainPage/main.ts 独占锁；采用 [公共契约 v1](contract.md)。本次同时按 WUI-103 原作者授权串行完成公共 CSS 语义选择器交接。

## 修改与所有权

WUI-102 的真实页面前态记录移动 Drawer 关闭后焦点落到 BODY、长标题挤压控件。WUI-103 已修尺寸，本次完成宿主行为：

- 菜单打开时记录所在路径。Drawer `after-hide` 后等待 Vue 更新，主动关闭（Escape/关闭）回到仍连接且可见的菜单按钮；如果路径变化则聚焦新页面 `main-content`，不再将导航后的焦点拉回旧入口。
- 身份变成强制改密时移除不可用的导航按钮及 Drawer，并将正在导航的焦点回退到当前内容。普通关闭也检查按钮连接及可见性，不向隐藏/已卸载目标聚焦。
- 为 breadcrumb、page title、actions 添加明确语义类；公共 CSS 原依赖 `first-child/last-child` 的规则改成这些类。标题保留完整 DOM 和 `title` 属性，详情页正文完整标题继续保留，截断只影响 topbar 呈现。

未改 `main.ts`：现有账号、设备/实例、网络深链及网络详情按 networkId 重建的路由契约已满足本轮输入。未改权限判断、OIDC/本地密码能力、临时凭据清除、迟到响应序号、config-server 接入命令或后台计时器。没有新增轮询、自动重试或订阅。

## 实际验证（2026-10-21）

工作目录 `/workspace/EasyTier-iteration/easytier-web/frontend`。本任务独占生产 dist；浏览器测试分别使用动态 loopback preview 端口、独立 context、系统 Chromium，用后关闭。测试运行真实生产组件，HTTP 为虚构 fixture，不代表真实账号/网络操作。

| 命令 | 实际结果 |
| --- | --- |
| 首次 `vue-tsc -b` | 失败：PrimeVue Button 声明未暴露 `$el`；模板 ref 改用 Vue `ComponentPublicInstance`，未关闭检查 |
| `node node_modules/vue-tsc/bin/vue-tsc.js -b` | 通过 |
| `node node_modules/vite/bin/vite.js build` | 通过；已有 caniuse 数据陈旧、大 chunk 警告 |
| `node --test tests/user-management-navigation.test.mjs tests/web-ui-support/foundation.test.mjs` | 28/28 通过（原 15 项导航、移动焦点 1 项、标题矩阵 12 项） |
| `node --test --test-name-pattern='forced-password identity' tests/user-management-navigation.test.mjs` | 新增强改迟到身份专项 1/1 通过 |
| 最终 `node --test tests/user-management-navigation.test.mjs` | **17/17 通过，0 失败、0 跳过** |
| `git diff --check` | 通过 |

导航专项覆盖连续三次 Escape 返回菜单按钮、选择 Dashboard 聚焦主内容、Drawer 打开时路径外部变化后聚焦主内容，以及后台 403 引起强改身份刷新时移除按钮并安全回退。既有导航验证覆盖权限收回、401、迟到管理请求、凭据清除、注册策略、旧默认密码非阻断提醒及 OIDC/本地密码能力。

foundation 的 12 项覆盖 cn/en × light/dark × 360/768/1440：完整标题 DOM 与 title 属性、topbar 高度≤80px、可见按钮宽≥28px、无整页横向溢出；360px 各组合另验证关闭后焦点回菜单按钮。真实长标题与移动问题的闭环限于这些实际场景，不宣称所有交互均已验证。

## 下一步与限制

交付 MainPage、原导航测试、本文，以及 WUI-103 串行更新的 console.css/foundation 测试。WUI-112 负责最终集成与真实隔离后端证据；ECO-113 须等待总控正式释放宿主锁后接线，并保留新语义类和焦点策略。线上部署、真实 VPN、HTTPS 代理、真实配置下发与旧二进制回退仍未验证。任务状态、看板与提交由总控维护。
