# WUI-108：实例配置宿主交接

负责人 `iteration_exec2`；分支 `feat/ecosystem-web-ui`。采用 [公共契约 v1](../contract.md)、[设备列表交接](list.md) 及 [WUI-102 前态](../../require/evidence/README.md)。真实前态没有设备，以下专项运行实际生产宿主、共享 RemoteManagement 和 WebRemoteClient，但 HTTP 响应均为虚构 fixture，不能证明设备连通。

## 已复现问题与修复边界

两个虚构宿主包含相同实例 UUID。从 A 切换到 B 后，抽屉标题已经是 B，但实例选择器仍显示 A 的网络名。修改前专项等待 B 元数据 10 秒超时。静态依据是共享 RemoteManagement 的 `networkMetaCache` 按实例 UUID 缓存，而 DeviceManagement 原来仅替换 `api`，复用了共享组件实例。

DeviceManagement 给 RemoteManagement 增加 `:key="deviceId"`，让宿主变化销毁旧宿主组件状态并建立新宿主状态。这属于宿主身份的生命周期边界，无需修改共享缓存或 API；同一宿主的实例切换仍复用组件。共享组件既有卸载逻辑负责停止自身周期任务，本次未新增轮询、重试、订阅或后台任务。

保留 `deviceId/instanceId` 深链、`selectedInstanceId` 路由同步、`get_remote_client(deviceId)` 注入，以及新配置从当前设备获取 hostname 的原契约。未改 RemoteManagement、配置模型、config-server、认证、Core 或真实网络设置。

## 实际验证（2026-10-21）

目录 `/workspace/EasyTier-iteration/easytier-web/frontend`，独占 frontend 构建输出；专项 helper 使用动态 loopback preview 端口、系统 Chromium、独立 browser context，结束关闭服务和浏览器。仅使用虚构账号与 `192.0.2.*` 地址。

| 命令 | 实际结果 |
| --- | --- |
| 修改前 `node --test tests/web-ui-instance-host.test.mjs` | 2/3 通过；宿主切换元数据测试超时，0 跳过 |
| 修改后 `node node_modules/vue-tsc/bin/vue-tsc.js -b` | 通过 |
| 修改后 `node node_modules/vite/bin/vite.js build` | 通过；既有 caniuse 数据陈旧、大 chunk 警告 |
| 修改后 `node --test tests/web-ui-instance-host.test.mjs` | **3/3 通过，0 失败、0 跳过** |

三项专项覆盖：

1. 原实例深链打开；选择另一实例后 URL 同步，刷新保持目标实例，浏览器返回恢复前一实例路径，HTTP 请求使用对应宿主。
2. 不刷新页面切换到含相同实例 UUID 的宿主 B；标题、实例元数据与 runtime 错误归属均为 B，不保留 A 的可见错误，且实际请求 B 的 metas。
3. 在 B 创建配置时，仅捕获 B 的配置 PUT，生成 hostname 为 B，路由保持 B 并切到新 UUID，不向 A 读取实例。PUT 由 fixture 接收，不表示后端持久化或设备下发成功。

各场景断言无浏览器 pageerror。测试为 1440px 英文专项，不冒充全语言、全宽度矩阵。首轮失败退出时出现一次 preview `/api/v1/machines` 连接拒绝日志；最终修复后运行无该日志，三个测试均进入页面并完成断言。

## 交接与未验证

交付 `DeviceManagement.vue`、专属 `tests/web-ui-instance-host.test.mjs` 和本文；任务状态、看板、提交由总控维护。WUI-111 保持原设备/实例路由，无新增 props、事件或公共文案。共享组件真实在线实例、配置保存持久化、配置下发、实例启停、VPN、真实节点及公网访问仍未验证，fixture 的不可用 runtime 提示不伪装成功。
