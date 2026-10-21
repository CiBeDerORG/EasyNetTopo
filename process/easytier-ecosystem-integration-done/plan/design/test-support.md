# ECO-105：独立预览与验证入口

交接版本 `ECO-TEST-SUPPORT-1`，依赖 [ECO-103 v1 契约](contract.md)。实现位于 `easytier-web/frontend/tests/ecosystem-support/`，与生产源码和各页面断言文件分开。沿用 Node test、Vue、Vite、Playwright 和 vue-tsc，没有新增依赖、替换框架或修改锁文件。

## 职责与数据边界

- `fixtures.ts`：只含明确虚构的视图模型，使用公共类型并对七组组件 props 做 `satisfies` 检查。包含 central、webhook、empty、unknown、loading、error、idle；未支持能力始终明确存在，不构造业务资源/公网端点/凭据或设备健康结论。
- `browserHarness.mjs`：建立回环地址的独立 Vite 服务和临时 cache；挂载指定生态组件，注入 fixture props 与 cn/en、浅/深主题。各浏览器 context 独立，API 请求记录 method/path 并 abort，不保存请求体。关闭服务即清理 cache；异常路径同样清理。
- `HarnessProbe.vue` 与 `harness.test.mjs`：测试支持入口自身能真实编译/挂载 Vue，读写 fixture 和捕获事件；它们不是业务组件，也不替代任务106～111的行为测试。
- `checkComponents.mjs`：临时 tsconfig 继承既有严格配置，定向运行 vue-tsc --noEmit，包含 fixtures.ts；不写 tsbuildinfo、frontend/dist 或 frontend-lib/dist。
- `runTests.mjs`：显式登记专项测试文件；用户指定未交付文件或未知suite时失败，不能以零测试结果宣称通过。每项测试业务断言由其对应任务写入。
- `preview.mjs`：手动预览入口，页面显式标注“Development sample — not live network data”。收到 SIGINT/SIGTERM 后关闭服务。不注册任何生产路由或向生产客户端注入fixture。

Vite的开发资源与HMR属于测试开发服务；生产生态组件仍不得新增轮询、订阅或后台请求。测试fixture不能作为真实Web/API/节点/公网验收证据。

## 运行方式

以下命令在 `easytier-web/frontend` 执行。使用现有依赖与可用 Chromium；浏览器路径取 `WEB_TEST_CHROMIUM`，未设置时沿用项目的 `/usr/bin/chromium`。不自动安装依赖或构建共享库，需总控先确认既有共享产物与采用源码一致；本轮复用由WUI/ECO环境所有者准备的产物，构建时遵循输出锁。

```sh
# 支持入口自测；默认仅support，不冒称所有生态业务通过
node tests/ecosystem-support/runTests.mjs support

# 定向类型检查，包括真实Vue导入props/emits及fixture契约
node tests/ecosystem-support/checkComponents.mjs

# 下游组件创建后，明确选取其类型检查与业务测试
node tests/ecosystem-support/checkComponents.mjs src/components/ecosystem/topology/EcosystemTopology.vue
node tests/ecosystem-support/runTests.mjs topology

# 手动预览；输出动态端口的URL，附scenario/lang/theme查询参数
node tests/ecosystem-support/preview.mjs src/components/ecosystem/topology/EcosystemTopology.vue topology
```

WUI-103交还package.json锁后已新增 `test:ecosystem`、`check:ecosystem`、`preview:ecosystem`，保持全部原脚本（包括 `test:web-ui:foundation`）。可用 `npm run check:ecosystem`、`npm run test:ecosystem -- support`、`npm run preview:ecosystem` 调用同一入口，不安装依赖。项目仍沿用pnpm锁文件，没有更换包管理或生成npm锁文件。

| suite | 显式文件 | 所有者 |
| --- | --- | --- |
| support | tests/ecosystem-support/harness.test.mjs | ECO-105 |
| shared | tests/ecosystem-shared.test.mjs | ECO-104 |
| topology | tests/ecosystem-topology.test.mjs | ECO-106 |
| nodes | tests/ecosystem-nodes.test.mjs | ECO-107 |
| resources | tests/ecosystem-resources.test.mjs | ECO-108 |
| access | tests/ecosystem-access.test.mjs | ECO-109 |
| navigation | tests/ecosystem-navigation.test.mjs | ECO-110 |
| api | tests/ecosystem-api.test.mjs | ECO-111 |
| composition | tests/ecosystem-composition.test.mjs | ECO-112 |
| host | tests/ecosystem-host.test.mjs | ECO-113 |
| integration | tests/ecosystem-integration.test.mjs | ECO-114 |

可以传多个suite；`all`要求表中全部文件交付，不自动忽略尚未完成的任务。真实integration前必须由监工检查前置与隔离环境；注册入口并不等于现在已具备联调条件。

## 页面任务接入

各页面测试导入 `createEcosystemPreview/openEcosystemPreview`：

```js
const preview = await createEcosystemPreview({
    componentPath: 'src/components/ecosystem/nodes/EcosystemNodeDetails.vue',
    fixtureKey: 'nodeDetails',
    eventNames: ['selectResourceKind'],
});
const view = await openEcosystemPreview(browser, preview, {
    scenario: 'unknown', language: 'cn', theme: 'dark', width: 360,
});
try {
    // 断言实际页面文本和事件；由页面任务提供具体业务判断。
    assert.deepEqual(view.requests, []);
    assert.deepEqual(view.errors, []);
} finally {
    await view.close();
    await preview.close();
}
```

fixtureKey为 topology、nodeDetails、resourceDetails、access、navigation、queryNotice、capabilityNotice。浏览器中 `window.ecosystemFixture.setProps(next)` 用于显式状态切换；`events` 记录 `{ name,args }`，`clearEvents()` 清观察。测试必须检查真实文本/事件/请求数量，不能仅断言自己的mock变量。

API适配器ECO-111应另写transport级测试，不通过fixture假装请求真实服务。ECO-114复用已有 `tests/helpers/webTestBinary.mjs:resolveWebTestBinary` 定位真实Web二进制，保留WUM_TEST_WEB_BINARY/CARGO_TARGET_DIR语义；本任务没有创建第二套二进制解析或后端启动框架。

## 实际验证及限制

- 定向 `checkComponents.mjs`：退出0，真实Vue probe与公共fixture类型检查通过。
- `runTests.mjs support`：3项通过，覆盖7状态真实Chromium挂载、事件/响应式更新、非法入口拒绝。后续业务组件尚未由这些结果验收。
- 尚未交付时显式运行 `runTests.mjs topology`：按预期退出1，说明对应文件未交付且未运行测试；不是topology测试失败或通过。
- 浏览器显示现有Browserslist数据过期提示；没有借此升级依赖。没有运行完整生产构建、后端、网络或公网验收。
- 首次写入辅助脚本时命令工作目录错误，文件未创建且Node报MODULE_NOT_FOUND；修正工作目录后完成上述实际运行。没有将该失败隐藏为成功记录。
- package.json已获总控正式交锁，追加入口后以npm run验证；系统pnpm命令会自动触发依赖状态检查/install，因 `/home/agent/.local/share/pnpm` 不存在报ENOENT，未执行目标脚本。没有重试安装、升级包或改锁文件，直接Node/npm脚本入口已实际运行。
- 总控指出Vite的port:0可能回退默认端口，已改为先通过net.listen(0)取得回环空闲端口，再Vite strictPort:true；释放到监听间如发生争用会显式失败，不悄悄换端口。新增双预览端口隔离断言。
- 手动preview CLI已实际返回HTTP200，SIGTERM后退出0；定向类型与支持测试最终结果见交接记录。根看板/任务状态由总控维护。

2026-10-21最终交接：`npm run check:ecosystem` 退出0；`npm run test:ecosystem -- support` **4/4通过、0跳过**（在增加明确端口隔离后重跑）。package三个入口已落地，保留WUI基础脚本和既有用户管理命令。停止写入package，交总控/监工审查。下游仍需自己的断言文件和真实联调证据。
