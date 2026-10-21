# WUI-110 网络详情与 ACL 交接

采用 [公共契约](../contract.md)；核查分支 `feat/ecosystem-web-ui`，实现期间 HEAD `82592542941712062688517bcec415ddfc4e9076`。写入仅 NetworkDetail.vue、AclPolicyTab.vue、专属 browser 测试及本文；不改 API、共享配置模型、locale、MainPage 或集中测试。

## 实际问题与改动

| 原任务 / 复现 | 修正行为 | 证据 |
| --- | --- | --- |
| ACL GET 503 后同时显示默认放行、无规则，Add Rule 可点击 | 读取成功前只展示加载/错误；显式 Retry，错误不解释为默认策略 | 先红：默认放行文本 count=1；修复后专项通过 |
| ACL PUT pending 仍能新增/改默认动作，可能覆盖未保存草稿 | 保存入口防重、编辑/删/拖放入口保护与控件禁用；失败保留策略草稿，成功采用真实响应 | 先红：Add Rule disabled=false；断言一次 PUT 和失败后可再次提交 |
| 初始网络读取 pending 时离开页面，迟到响应继续发 members 请求 | 捕获 networkId 与请求代次；卸载失效，不发旧对象后续请求，不启动已卸载页面计时器 | 先红：离开后 members 请求1次；修复后0次。旧 PeriodicTask.stop 本身清空 task，不能把此证据写成已证实持续孤儿轮询 |
| 设置页网络密钥默认明文，pending 保存仍可改草稿 | 复用 PrimeVue Password 默认遮蔽、保留显式查看；fieldset 禁用与 saveSettings 入口防重；失败保留非敏感草稿 | 先红：type=text、disabled=false；修复后 password、一次 PATCH、草稿保留 |

ACL 读取/保存代次保护 success/error/finally；networkId 切换清旧策略及规则编辑器，卸载后的保存不向下一页面发成功提示。规则顺序、enabled=false、空规则语义、snake_case payload 不变。异常 ACL 不输出原始 Axios 请求/策略。NetworkDetail keyed 路由仍由 MainPage 所有，未另造全局状态。

## 验证

- `node node_modules/vue-tsc/bin/vue-tsc.js -b` 与 `node node_modules/vite/bin/vite.js build`：通过；dist 唯一写入锁经总控临时转交。共享库未修改，沿用已验收构建。Vite 仍提示既有 >500 kB chunk 警告，不因本任务重构构建。
- `node --test tests/web-ui-network-detail.test.mjs`：6/6 通过（19.2秒）。实际生产 Vue 组件、系统 Chromium、动态 loopback preview、独立 browser context；所有 HTTP 都是明确虚构数据。
- 专项含 ACL错误/显式重试、保存防重/失败草稿、初次读取卸载、设置密钥与草稿、CN/EN × light/dark ×360/768/1440 共12组合、键盘 Enter 页签、无整页溢出/未翻译键、ACL成功保留规则顺序及 false、旧保存卸载不弹新toast。
- 既有 `dashboard.test.mjs` 网络/成员/配置/凭据子集：初跑因缺 Playwright bundled Chromium 在 before hook 失败，未执行业务断言；通过 `/tmp/wui110-system-chromium.mjs` 只指定 `/usr/bin/chromium` 重跑15/15通过（54.7秒），覆盖初始节点协议/CIDR、PublicServer/Gateway、轮询保留草稿、ACL随机ID、成员迟到配置/增删改/自动IP/WireGuard、凭据导出、节点详情及logger。集中测试源码和断言不修改。

## 边界

保留原详情2秒 PeriodicTask、节点详情5秒 timer、ACL按对象加载；不新增自动重试、订阅、轮询或写入。真实 config-server/设备配置下发、VPN 数据通信不在这些 HTTP fixture 证据内；本任务不能据此宣布真实设备应用成功。无后台/协议/凭据生成语义改动。初次真实取证已有空成员基线；本轮非空成员、ACL及配置语义通过明确 fixture 验证，不伪装真实设备。
