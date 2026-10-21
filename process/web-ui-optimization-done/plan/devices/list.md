# WUI-107：设备列表与资料展示

负责人 `iteration_exec2`；分支 `feat/ecosystem-web-ui`。采用 [公共契约 v1](../contract.md)、[WUI-102 前态](../../require/evidence/README.md)。真实前态没有连接设备，本任务以明确的虚构 HTTP 数据补充实际生产组件验证，不宣称真实设备联通。

## 已复现问题与最小修复

| 复现 | 原行为 | 修改 |
| --- | --- | --- |
| localStorage 保留 `deviceList.viewMode=card`，360px、中文深色、列表有设备 | 桌面卡片网格与移动列表同时可见 | DeviceList 局部 `max-width:767px` 隐藏卡片网格；保留原移动紧凑列表，桌面偏好不丢失 |
| 别名请求挂起时连续 Enter | 专项红测实际捕获 **2 个 PUT，期望 1 个** | `saveAlias` 增加 `aliasSaving` 入口守卫；保存期间禁用输入/确认/取消及关闭。失败后保留草稿并重新允许明确提交 |

只修改 `frontend/src/components/DeviceList.vue`；`DeviceDetails.vue` 未发现本次有证据需要修改的缺陷，保持纯展示。未修改 props、路由、machine_id、实例选择、别名 API、权限、共享 RemoteManagement 或 Core。没有新增定时器、轮询、重试队列、后台订阅。

## 实际验证（2026-10-21）

工作目录 `/workspace/EasyTier-iteration/easytier-web/frontend`，本 agent 独占构建输出；生产 preview 动态 loopback 端口、独立 browser context，用后关闭。虚构宿主使用文档地址 `192.0.2.*`，未建立客户端连接。

| 命令 | 结果 |
| --- | --- |
| 修改前 `node --test tests/web-ui-device-list.test.mjs` | 2/4 通过；卡片重复失败，别名重复返回两个错误 toast 使初始定位不唯一 |
| 修改前同文件 `--test-name-pattern='alias save'` | 0/1；将 toast 定位明确为首项后，写入计数明确断言 `2 !== 1`，证实业务重复提交 |
| 修改后 `node node_modules/vue-tsc/bin/vue-tsc.js -b` | 通过 |
| 修改后 `node node_modules/vite/bin/vite.js build` | 通过；保留既有 caniuse 数据陈旧和大 chunk 警告 |
| 修改后 `node --test tests/web-ui-device-list.test.mjs` | **4/4 通过，0 失败/跳过** |
| 本任务 `git diff --check` 与文档相对链接 | 通过 |

四项专项使用真实生产组件、mock HTTP 和可观察状态：

1. 桌面英文浅色：按地址筛选、展开长主机名资料、选择设备，保留 `/h/deviceList/device/<machine_id>` 和匹配抽屉标题。
2. 360px 中文深色：已保存卡片偏好不重复移动行，展开长字段资料，整页无横向溢出。
3. 别名失败保留草稿；挂起时再次 Enter 不增加 PUT；显式重试成功更新列表，目标仍是原稳定 machine_id。
4. 768px 英文浅色：空态命令和复制内容严格为 `easytier-core --config-server tcp://127.0.0.1:11010/fixture-admin`；查询失败不是空结果，手动重试恢复空态。剪贴板由测试替身捕获，不操作真实系统凭据。

上述是本次受影响场景，不是完整三宽度/两语言/两主题笛卡尔积复测；WUI-102 已有通用前态，WUI-112 再做整合回归。DeviceList 的命令仍按 `window.location.hostname` 生成，MainPage 的 `getApiBase` host 推导保持另一所有者的既有语义，未自行统一。

## 交接

WUI-108 继续消费原 `deviceList` 与路由契约；WUI-111 无需新事件或公共文案。集中 `tests/dashboard.test.mjs` 仍归 WUI-112，本任务没有修改。真实设备状态、实例启动、配置下发、VPN或公网能力未验证；fixture 验证不能替代这些结果。状态、看板与提交由总控维护。
