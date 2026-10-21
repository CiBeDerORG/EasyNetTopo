# ECO-110：资源导航说明与宿主回链

依据 [契约 v1](../contract.md) 和 [ECO-110 任务](../../../task/task-eco-110-navigation-view.md)。实际导航登记/发布能力未提供，页面实现不能视为导航服务已交付。

## 组件契约

`easytier-web/frontend/src/components/ecosystem/navigation/EcosystemNavigation.vue` 使用既有 `EcosystemNavigationProps`，显示所选宿主名称（alias → hostname → 未知），复用公共能力说明，并始终说明没有已核实访问链接。节点为空时提示选择宿主，不显示返回按钮。

唯一事件 `backToNode` 传出点击时当前节点的 `{userId, machineId}` 副本，父层 ECO-112 负责切换已有内部宿主视图。组件不创建路由、外链、端点、导航记录或写请求；不把能力 supported 等同于已授权或已发布。即使 supported 也没有编造链接。模板自动转义宿主名，无 HTML/URL 拼装。没有查询状态所有权，加载/失败/无权限由组合层处理。

文案 `navigation.host_context` 由 ECO-104 唯一语言写入者成对加入并冻结后测试；无共享文件越权修改。

## 实际验证与复现

在 `easytier-web/frontend` 执行：

```sh
node tests/ecosystem-support/checkComponents.mjs src/components/ecosystem/navigation/EcosystemNavigation.vue
ECO_NAVIGATION_SCREENSHOTS=1 node tests/ecosystem-support/runTests.mjs navigation
```

类型检查 exit 0；专项 **14/14 通过，0 跳过**，真实 Vue 编译和 Chromium fixture 运行。12 组为 cn/en × light/dark × 360/768/1440，验证无翻译缺失、无横向溢出、无 API 请求、无浏览器错误；Tab 聚焦返回按钮，Enter 和 Space 各发一次含正确身份的事件。另验证 null → 未知名称节点 → null 时返回目标正确、无残留按钮；supported 不产生外链。源码断言无请求、计时器、持久化和外部 URL 构造。

截图为开发样例：[英文浅色](navigation-en-light-360.png)、[英文深色](navigation-en-dark-360.png)、[中文浅色](navigation-cn-light-360.png)、[中文深色](navigation-cn-dark-360.png)。采证后补充段落间距并重新执行上述检查与截图生成；中文深色人工核查，说明与按钮可见。浏览器现有 Browserslist 数据陈旧提示未影响运行，未升级依赖。

未执行真实导航服务、外链可达、公网发布、VPN 或生产环境操作；没有注册导航 API，无法验证发布/撤销或端点权限。fixture 事件证明本组件回链契约，不能替代 ECO-112 宿主切换和 ECO-114 真实联调。
