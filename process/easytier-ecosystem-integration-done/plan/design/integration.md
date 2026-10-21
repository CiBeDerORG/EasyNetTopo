# ECO-113：正式宿主接线

在已验收 `97112bef` 的 WUI-111/ECO-112 基础接线，按 [ECO 契约](contract.md) 和 [WUI 宿主交接](../../../web-ui-optimization-done/plan/integration.md)。协调者授予 MainPage/main.ts/frontend dist 独占锁，后追加 frontend-lib dist 构建锁；未写 WUI-112 的 `/tmp/wui111-frozen-37747637/dist`。

## 最小修改

- `main.ts` 导入已验收 EcosystemPage，新增 `/h/ecosystem`（name=ecosystem），既有路径和别名未改。
- `MainPage.vue` 工作区导航增加现有中英文 `web.ecosystem.title` 标签。普通用户可访问自己的生态视图，不依赖 users.manage；原管理菜单权限不变。
- RouterView 提供已验证 `currentUser.id` 为 currentUserId，保留 api 注入及网络详情按 networkId 重建。现有 currentUser/强制改密分支保证没有身份时或强改时不挂载生态组件；身份变化由112清空状态。
- 未改 WUI 的语义类/title、after-hide 焦点、强改焦点回退、OIDC/本地密码判定、临时凭据清理、config-server 命令与旧导航计时器。生态读取不接入旧 loadNavigation 周期。

## 实际验证

正式 production preview（动态 loopback 端口、独立 context）运行真实构建应用，HTTP 为虚构 fixture，不是真实账号/后端：

```sh
# frontend-lib：保留既有生成源码，仅消费新 locale
node node_modules/vue-tsc/bin/vue-tsc.js -b
node node_modules/vite/bin/vite.js build
# 然后 frontend
node node_modules/vue-tsc/bin/vue-tsc.js -b
node node_modules/vite/bin/vite.js build
node --test tests/ecosystem-host.test.mjs tests/user-management-navigation.test.mjs
```

两包类型检查与 Vite 构建通过；浏览器 **22/22 通过，0 失败、0 跳过**（5项新生态宿主、17项既有导航）：

- 普通账号深链、active导航与title、无管理入口；首次仅一次机器列表读取。
- webhook移动导航仍可进入生态，不请求中央网络；切换页面后焦点落主内容，无整页横向溢出。
- 强制改密深链不挂载生态，不请求机器/console-info/networks。
- 正式中英文文案与未支持提示已打包，无 web.ecosystem 翻译键泄漏。
- config-server 命令的协议、地址、端口、用户名及复制字符串精确一致；主题切换继续持久化。
- 既有17项覆盖角色回收、401、迟到管理响应、临时凭据清除、OIDC/本地密码能力、注册与强改、移动焦点行为。

`git diff --check` 通过，`git diff --name-only -- easytier-web/frontend-lib/src/generated` 为空。旧计时器空闲观察与真实后端留114，未将fixture验证当作生产/VPN验证。

## 构建问题与边界

首次仅构建 frontend，发现 frontend-lib/dist 陈旧、不含 ECO104 文案，5项中的最初2项因此找不到文本；另一项强改 fixture 同时标注 OIDC 且无本地密码，已修正为本地临时账号。旧导航17项当时通过，未隐瞒失败。总控授权后按先lib后frontend更新产物，最终22项通过。

尝试标准 `node scripts/codegen-proto.mjs` 失败：系统无 protoc，wrapper 获取版本时 github.com DNS `EAI_AGAIN`。未重试下载/安装，未更改依赖/lock/schema/生成源码；总控确认本次schema未改，可用现有生成源码运行类型与Vite构建。因此**不声称完整 pnpm build/codegen 通过**。已有 Browserslist 数据陈旧和大 chunk 提示仍在。

最终本地产物：

- frontend `assets/index-No5yjWNX.js` SHA256 `ed035866825a24eb84fe4c0aed58e22f9838281756bbebaaa5e177cc757473c0`
- frontend-lib `dist/easytier-frontend-lib.js` SHA256 `1504f56488af36580fcb16275f2834fe4ca04c54f3256a81c6c4f3cb86a0463f`

交给114开展真实隔离API联调。无生产部署、真实账户变更、VPN/公网操作，提交由协调者统一执行。
