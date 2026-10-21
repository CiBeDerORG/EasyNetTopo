# ECO-112：生态页面组合与查询所有权

依据 [契约 v1](contract.md)。`EcosystemPage.vue` 只接受已有 `api: ApiClient` 和宿主当前 `currentUserId: number`；113 将从当前身份提供这两个属性，本任务未修改 MainPage 或路由。

## 所有权与生命周期

一个 immediate watch 处理首次进入以及 api/身份变化，创建真实 reader，执行一次 overview。无 onMounted 重复读取、轮询、可见性/重连订阅、自动重试或第二身份存储。子组件保持纯 props/events。

overview 与 members 各有代次：刷新使旧成员代次失效；网络切换清空不同对象成员快照并发起新查询；身份变化清空全部查询、选择、资源类型和错误；卸载失效两层结果。底层请求没有取消，旧 reader 已发出的链式读取仍可能结束，但不能写入新页面状态，不宣称已取消网络请求。

overview 在途禁止重复刷新；网络选择控件在其加载时禁用。手动刷新成功后最多读取一次仍存在的当前网络成员；webhook 不读取中央网络。成员/overview 错误保留对应旧快照，监工发现并修复了刷新曾将成员 error 改为 ready 的问题。刷新使在途成员结果失效时保留 loading/previous，若 overview 失败则结束成员 loading 为安全 error，不悬挂也不假报成功。

选宿主仅定位已存在的同用户 UUID，未知身份提示已有“宿主详情不可用”，不造节点。导航 backToNode 定位详情 section 并聚焦；只通过当前列表匹配，不创建外链。资源类型变化仅影响已选宿主的说明组件。

## 实际验证

从 `easytier-web/frontend` 执行：

```sh
node tests/ecosystem-support/checkComponents.mjs src/components/ecosystem/EcosystemPage.vue tests/ecosystem-support/compositionFixture.ts
node tests/ecosystem-support/runTests.mjs composition support
```

类型检查 exit 0。组合 3/3、支撑 4/4，共 7/7 通过，0 跳过；最终加载结束处理修订后重跑组合 3/3。真实 Vue + 真实 reader + 已验收子组件，用可控 DTO transport，属于开发 fixture，不是真后端：

- 一次初始化、刷新禁重复、两个网络乱序完成、旧成员不污染新网络。
- 当前宿主回链焦点，成员失败保留 previous 后 overview 也失败，错误不消失；手动恢复成功最多一次成员请求。
- 身份切换和卸载时旧 overview 结果不能回写；webhook 当前视图没有中央网络选择。
- 清空网络选择丢弃成员迟到结果，身份变化立即清除已选宿主/详情；online/visibilitychange 事件不增请求。
- 无意外 API 请求和页面异常。共享 harness 的 4 项旧回归继续通过。

105 支撑最小扩展为 `compositionFixture.ts`（仅测试导入，记录调用并可控完成 DTO Promise），harness 增加 composition 键与测试 unmount hook。不向生产组件增加 fixture 属性、不修改 dist/package。

首次执行因目录参数错误未创建专项文件，runner 明确报未交付且未运行测试；纠正后第一轮用例因选定网络没有注册成员而找不到宿主按钮，修正 fixture 为真实 DTO 形状的已登记成员后通过。未将这些失败记录为成功。

剩余：宿主路由/强制改密/身份加载接线由113完成，真实后端与已有宿主周期观察由114完成；本任务不证明真实节点通信、资源发布或公网可达。

## 监工要求的精确补证

另增 `pending members exit loading after overview failure and late response cannot replace previous data`，从已有成功成员快照开始，发起下一次成员查询但保持其 pending；此时再次刷新 overview 并拒绝该 overview。实际断言两个查询都退出 `aria-busy=true`/Loading，两个错误说明保留且旧成员仍可见，两个手动刷新按钮可用；随后完成已失效成员请求，其新内容不能替换旧快照或清除错误；最后手动恢复成功，清除错误与 loading，总调用数符合预期且无隐式请求。

定向命令（frontend 目录）：

```sh
node --test --test-name-pattern='pending members exit' tests/ecosystem-composition.test.mjs
```

实际 **1/1 通过，0 失败、0 跳过**（约4.2秒）。该次仅运行新增精确分支，不将历史3项或共享4项算作本次重跑；未改生产代码。测试依旧使用可控 DTO transport，不是真实后端。
