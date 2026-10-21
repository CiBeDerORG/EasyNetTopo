# ECO-104 公共展示与文案交接

采用 [ECO-103 v1契约](../contract.md)，公共props/events只来自ecosystemTypes.ts。组件仅在Web内消费PrimeVue与现有主题变量，不影响frontend-lib的其他宿主，不发HTTP、不持久化状态、不注册定时器。

- `EcosystemQueryNotice.vue`：idle/loading/ready/error与最近成功查询时间。loading禁用刷新；refresh由父层去重及执行。error使用安全错误种类映射，不接收原始响应内容；存在旧时间时标记最后成功快照，绝不显示失败为空列表。空列表由已成功加载的页面明确表达，此组件不猜数据数量。
- `EcosystemCapabilityNotice.vue`：只展示unsupported及明确原因；supported不输出“已授权/已连接”的成功断言。未知能力由上层查询未完成表达，不伪装为unsupported。
- 状态使用文字和ARIA status/alert，不只靠颜色。按钮为PrimeVue Button，保留键盘行为；长文本换行、容器可伸缩；颜色来自当前主题。

## 文案所有权

`web.ecosystem` 为本轮统一前缀；cn/en成对维护。WUI-103完成后总控已正式交还locale锁；本任务仅新增web.ecosystem块，保留既有文案。已加入：通用标题、刷新、未知、网络/宿主/成员关系说明；query四态与旧快照时间；全部EcosystemErrorKind；全部EcosystemUnsupportedReason；connection状态、resource kind、资料字段、访问设置与回链。页面任务缺文案必须交回104，不各自写locale。

## 验证状态

2026-10-21 实际执行：

- `node node_modules/vue-tsc/bin/vue-tsc.js --noEmit --incremental false -p tsconfig.app.json`：退出0，完整Web类型检查通过，无dist或tsbuildinfo输出。
- `node --test tests/ecosystem-shared.test.mjs`：最终 **6/6通过**，0失败/跳过，约10秒。实际加载Vue组件、真实cn/en语言资源、PrimeVue主题和系统Chromium；数据为组件状态fixture，不是后端。
- 覆盖键盘刷新事件、loading禁止重复操作、失败保留上次时间、权限/缺失对象/超时/非法响应区别、不渲染额外原始诊断、六类未支持原因、supported不伪造成功；检查测试期间无API请求和pageerror。
- CN/EN × light/dark × 360/768/1440共12组布局检查，无页面横向溢出；cn/en全部生态叶子键一致且非空。此矩阵仅证明这两个公共组件，不替代全部页面验收或长时间网络观察。
- 预览使用独立临时Vite缓存、回环随机端口及strictPort，测试结束关闭浏览器/服务并删除缓存，不写共享构建产物。初次使用port:0发现Vite回退默认端口，已修正后完整复跑6/6；更早一次在文案尚未落盘时误启动测试，已中止（退出130），不计为验证通过。
- Browserslist提示缓存数据较旧；未升级依赖或锁文件。未执行完整生产构建、真实API、VPN或公网验证。

拟用词JSON草案已删除，语言事实源仅cn/en.yaml；页面任务新增文案需求由104串行补充，不能复制另一份生产目录。

## ECO-105 接线后测试复用

在105监工验收后，`ecosystem-shared.test.mjs` 改为复用 `createEcosystemPreview/openEcosystemPreview`，删除本套重复的Vite、端口、缓存与页面生命周期实现。使用queryNotice/capabilityNotice两个独立preview，原六项验证保留；刷新后由测试模拟父层置loading，再验证不能重复发出事件。实际语言文件通过既有YAML插件的解析依赖读取，不复制语言事实源。

迁移后执行 `node --test tests/ecosystem-shared.test.mjs`：**6/6通过**，退出0，约10.9秒。仍为组件fixture验证，不代表真实后端；随后交给106及后续消费者。

监工指出迁移后的布局循环只覆盖QueryNotice，已补CapabilityNotice相同12组合；两组件共24次布局加载，六项测试结构与旧断言保留。恢复后实际复跑 **6/6通过**，退出0，约18.2秒；此结果取代前述迁移时覆盖不足的布局结论。
