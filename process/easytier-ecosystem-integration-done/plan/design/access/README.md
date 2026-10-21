# ECO-109：独立访问能力说明

实现依据为 [ECO-103 契约](../contract.md) 与 [ECO-109 任务](../../../task/task-eco-109-access-settings.md)。验证时功能分支 HEAD 为 `c9a0498b2c453ac54f5038c87f48b4286f4c0170`，本组件尚未提交。任务状态由协调者维护。

## 交付与组合层输入

`easytier-web/frontend/src/components/ecosystem/access/EcosystemAccessSettings.vue` 接收已有 `EcosystemAccessSettingsProps`，分别显示 `publicAccess`、`portMappings`、`navigation`；无事件、请求、持久化或计时器。每项使用自己的能力值与原因，复用 ECO-104 的能力说明。supported 仅显示能力可用，不声称已授权、服务启用或发布成功。语言新增 `access.supported_note` 由 ECO-104 唯一写入者更新，未改变其他文案。

ECO-112 可直接传入 overview 的对应三项能力。实际适配层当前三项均 unsupported；契约未定义资源授权/发布/端口映射/导航保存 API，本页没有无效表单、开关、外部链接或模拟成功状态。正常、空数据、加载、失败、无权限由组合层查询状态负责，本组件只表达输入能力，不能将请求失败自动变成未支持。未知能力尚无该契约枚举；不得自行制造 supported。

## 实际验证

在 `easytier-web/frontend` 运行：

```sh
node tests/ecosystem-support/checkComponents.mjs src/components/ecosystem/access/EcosystemAccessSettings.vue
ECO_ACCESS_SCREENSHOTS=1 node tests/ecosystem-support/runTests.mjs access
```

- 类型检查通过（exit 0）；真实 Vue 编译、Chromium 运行的专项测试 **14/14 通过，0 跳过**。
- 12 组为 cn/en × light/dark × 360/768/1440；均显示三个独立说明、无缺失翻译、无水平溢出、无浏览器错误、无 API 请求。
- Tab/Enter/Escape 不产生事件或写入，页面无交互控件，未引入键盘陷阱；此处并非完整宿主菜单焦点验收。
- 三项逐个切换 supported，其他两项文字保持原值，没有出现保存操作；源码检查未出现请求、timer、持久化或写事件。
- 四张 360px 截图由上述命令可重复生成：[英文浅色](access-en-light-360.png)、[英文深色](access-en-dark-360.png)、[中文浅色](access-cn-light-360.png)、[中文深色](access-cn-dark-360.png)。人工检查中文深色截图布局、换行与三项说明正常。顶部明确标识开发样例；不是生产数据或公网可达证据。
- `git diff --check` 通过。现有 Browserslist 数据陈旧提示未影响测试，未更新依赖。

首次类型命令误从仓库根调用 frontend 相对路径，报 MODULE_NOT_FOUND；随后从 frontend 正确目录运行成功。未将失败或零测试记作通过。

## 保留限制

未运行真实公网开放、端口转发、导航服务、生产账号、VPN 或完整构建；没有这些接口的业务操作仍未实现。此交付只完成已冻结契约中的能力说明，不能关闭相关后端需求。真实 API/宿主联调留给 ECO-114，未来能力扩展需先修订契约。
