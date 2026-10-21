# WUI-109：网络列表与创建交接

负责人 `iteration_exec2`，分支 `feat/ecosystem-web-ui`。采用 [公共契约 v1](../contract.md) 和 [WUI-102 前态](../../require/evidence/README.md)。任务只修改 NetworkList、专属测试及本文，不改共享 API、NetworkDetail、ACL、locale 或 package。

## 有证据的修改

首轮专项确认创建请求挂起时 Cancel 仍可用。原流程允许关闭后重新打开，从而清空正在提交的草稿；提交函数也没有忙碌入口守卫。

- `openCreate/createNetwork` 添加 `creating` 守卫，拒绝同一请求期间重新初始化及重复提交。
- 请求期间禁止 Dialog 关闭、Escape 和取消；原表单用 disabled fieldset 暂停编辑，UrlListInput 含非原生按钮的区域另用 inert 防止继续增删。
- 失败仍保留草稿，finally 恢复编辑和取消；成功使用接口返回的 network_id 跳转。

Gateway/Manual/Standalone、secure_mode 默认值、高级初始节点、normalizeVirtualSubnet、搜索/排序、权限及已有 2 秒 PeriodicTask 保持。未增加后台行为或接口，也未把本次忙碌保护写入公共框架。

## 实际验证（2026-10-21）

目录 `/workspace/EasyTier-iteration/easytier-web/frontend`；独占生产构建输出。生产组件和 HTTP 适配器运行于动态 loopback preview，浏览器独立 context；HTTP 全为 fixture，结束关闭浏览器/服务。

| 命令 | 结果 |
| --- | --- |
| 首轮 `node --test tests/web-ui-network-list.test.mjs` | 0/4；三项测试误将 CIDR 规范化理解为掩码归零；另一项确认 pending Cancel 未禁用 |
| `node node_modules/vue-tsc/bin/vue-tsc.js -b` | 通过 |
| `node node_modules/vite/bin/vite.js build` | 通过；已有 caniuse 数据陈旧、大 chunk 警告 |
| 修复后专项（英文） | 4/4 通过 |
| 最终专项（pending 改中文深色） | **4/4 通过，0 失败、0 跳过** |

首轮 CIDR 断言错误经源码核实：原函数只 trim 并在无斜杠时补 `/24`，没有掩码归零。测试改用 `10.123.4.0` 输入验证既有补 `/24` 行为，生产函数未修改，不能将首轮三个错误称为产品缺陷。

最终专项覆盖：

1. 英文浅色 360px，Gateway 创建使用空 peer_urls、默认 secure_mode=true；失败保留长名称，显式重试使用返回 networkId 跳转。
2. 同环境切高级模式，Manual 保留虚构网关 URL，secure_mode=false；失败保留草稿后显式重试。
3. 无网关 Standalone 的空 peer_urls、secure_mode=false；失败重试和跳转。
4. 中文深色 360px，请求挂起时取消禁用、Escape 不关闭、输入禁用；额外 click 事件不产生第二个 POST；失败恢复编辑，取消可关闭。

前三项检查整页无横向溢出。不是完整两语言/主题/三宽度组合，也不宣称覆盖了本次未改动的全部列表排序和后台机制；通用前态与最终集中联调分别归 WUI-102/112。

## 限制与交接

POST 成功由 fixture 返回，仅验证界面跳转和请求体，不证明真实网络创建、后台持久化、配置下发、成员连接或 VPN。目标详情 fixture 不提供真实资源，因此详情加载能力由 WUI-110 另测。WUI-111 继续使用原 networkDetail 路由，未新增事件/props。状态、看板和提交由总控维护。
