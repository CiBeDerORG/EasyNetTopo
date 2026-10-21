# 前端架构规范

本文适用于 Web 控制台、共享组件库、配置生成器、GUI 的前端和 uptime 前端。
与之配套阅读 [架构总览](README.md)、[命名规范](naming.md)、[后端规范](backend.md)；新增交互还应阅读 [产品设计规范](product-design.md)。
`MUST` 表示必须遵守，`SHOULD` 表示默认遵守且偏离时说明理由，`MAY` 表示按需采用。
下面的“现状”描述代码事实；“规则”约束新增和修改的代码，不要求批量迁移历史实现。

## 1. 已有技术与职责边界

| 路径 | 现状与职责 |
| --- | --- |
| `easytier-web/frontend-lib` | Vue 3、TypeScript、PrimeVue 4、Tailwind 3；共享配置编辑、状态展示、ACL、远程管理和协议类型适配。 |
| `easytier-web/frontend` | Vue 3、TypeScript；Web 控制台、HTTP API 适配、认证和控制台主题；手动声明的 hash 路由。 |
| `easytier-web/config-generator` | Vue 3、TypeScript；通过 `easytier-core` 编译出的 WASM 解析与生成配置，复用共享组件库。 |
| `easytier-gui/src` | Vue 3、TypeScript、Pinia、自动文件路由；桌面/移动壳层、Tauri 适配和平台交互。 |
| `easytier-contrib/easytier-uptime/frontend` | 独立 npm 项目，Vue 3、JavaScript、Element Plus；节点健康与管理界面，不在根 pnpm 工作区内。 |

事实入口：各目录的 `package.json`、根 `pnpm-workspace.yaml`、
`frontend-lib/src/easytier-frontend-lib.ts`、`frontend/src/main.ts` 和 `easytier-gui/src/main.ts`。

**规则：**

- MUST 沿用目标子项目的栈和依赖管理器；不能为单个功能引入另一套 UI 框架、全局状态库或路由框架。
- MUST 将 Tauri Rust、服务权限和原生网络实现留在后端；Vue 负责交互和展示，不能充当权限校验的唯一位置。
- SHOULD 将 Web 与 GUI 都需要的网络配置、展示和纯转换逻辑放入 `frontend-lib`。
- MUST 将账号会话、HTTP 路由、原生窗口、VPN 权限和托盘行为保留在各宿主应用，不反向放入共享组件库。
- MAY 保留 uptime 的独立栈；将其迁入主工作区是单独的架构变更，不能作为普通功能的隐含前置步骤。

## 2. 依赖方向与适配器

```text
Web / GUI 页面 ── 使用 ── 共享组件/配置模型
                       └── RemoteClient 契约 ── HTTP / Tauri invoke 适配器
配置生成器 ── 使用 ── 共享 Config 组件/配置模型
           └── 直接适配 ── Core 配置 WASM
```

该图表示共享组件通过注入的契约访问宿主能力，不能依赖宿主实现。配置生成器的配置解析/生成直接调用 WASM，不经过 `RemoteClient`。

- MUST 在共享组件需要运行期宿主后端能力时使用 `frontend-lib/src/modules/api.ts` 中已有的 `RemoteClient` 契约；不要为配置生成器的直接 WASM 路径强行增加该接口。
- MUST 在 Web 的 `src/modules/api.ts` 封装 HTTP，在 GUI 的 `src/composables/backend.ts` 封装 `invoke`。
- MUST 在新增共享交互时检查 Web 和 GUI 对应适配器；不支持的宿主应给出明确状态，不能静默成功。
- MUST 从共享库公开入口导入；不要新增指向 `node_modules/...`、其他包 `dist/...` 或内部源文件的深层导入。
- SHOULD 让组件依赖小型能力接口，复杂转换用纯函数，避免组件直接耦合多个传输客户端。
- MUST 保持配置生成器调用现有 WASM 能力；不要在 JavaScript 中另写一套 TOML/配置语义解释器。

现有参考：`frontend/src/modules/api.ts` 的 `WebRemoteClient`，
`easytier-gui/src/modules/api.ts` 的 `GUIRemoteClient`。

## 3. 组件、状态与异步生命周期

- SHOULD 为新增 TypeScript Vue 组件使用 Composition API 和 `<script setup lang="ts">`，并遵循所在文件的格式。
- MUST 让页面编排路由和请求，领域组件负责单一任务；不能将连接管理、表单校验和展示复制到多个页面。
- MUST 显式声明 props、事件和可空值；子组件不得绕过既有双向绑定契约偷偷改变父级业务状态。
- SHOULD 用局部 `ref` 保存交互状态、`computed` 表达派生状态，避免以 `watch` 反复同步可直接计算的值。
- MUST 区分“表单草稿”“已保存配置”“运行中实例状态”；保存成功不等于运行成功，连接存在不等于业务可达。
- MUST 在更新 props、网络实例或路由参数后丢弃失效请求结果；可用清理回调、请求取消或序列号防止竞态覆盖。
- MUST 清理组件拥有的定时器、事件监听、订阅、图表实例和轮询；应用级资源须有明确生命周期归属。
- MUST 处理提交中、加载失败和重复点击；提交失败保留草稿，不能将失败当作空列表或零值展示。
- SHOULD 将复杂状态转换提取为 composable 或纯模块，保持初始化、暂停、恢复和释放行为可测试。
- MUST 复用宿主已有的状态方案；GUI 已有 Pinia 不意味着 Web 或共享库必须增加 Pinia。
- MUST 在读取 localStorage 等持久化输入时校验结构并处理损坏数据；新增结构应有迁移或兼容策略。
- MUST 在删除网络、重置配置和切换配置源时说明受影响的本地持久化及运行实例，避免状态分叉。

现有参考：`frontend-lib/src/components/Status.vue` 的实例切换清理，
`easytier-gui/src/composables/config.ts` 的持久化数据校验。

## 4. 类型、协议与生成代码

- MUST 将 `easytier-proto/proto` 视为已有 protobuf 协议类型的来源，按生成脚本更新类型，不能手改生成文件。
- MUST 保留 wire 字段名；当前 `codegen-proto.mjs` 使用 `use_proto_field_name`，因此协议对象中的 `snake_case` 是有效契约。
- MUST 通过 `frontend-lib/src/types/network.ts` 及兼容层进行网络配置规范化、序列化和后端转换。
- MUST 分清 protobuf 类型、传输 DTO 与表单视图模型；表单专用字段不得意外发送给后端。
- MUST 保留 uint64、UUID、枚举、可空字段、缺省值和旧配置的兼容语义，不能以 `Number()` 或强制断言掩盖精度丢失。
- SHOULD 在边界使用 `unknown`、结构校验和类型收窄；不得新增无解释的 `any` 或大范围 `@ts-ignore`。
- MAY 对生成器输出保留生成器本身的类型检查策略；不能由此跳过手写业务代码的类型检查。
- MUST 将新增 API 请求和响应类型放在对应 API/领域模块，避免 Vue 文件各自定义不一致的协议副本。

生成目录包括 `frontend-lib/src/generated/` 和 `config-generator/src/generated/`；
它们是构建产物。GUI 的自动导入、组件和路由类型文件同样应通过配置的工具生成。

## 5. 样式、国际化与可访问性

- MUST 复用已有 PrimeVue/Element Plus 组件及所在项目的主题变量，不能为同一语义重复造按钮、表单或对话框。
- SHOULD 沿用现有 Tailwind 工具类及局部样式；新增全局选择器应说明作用范围，避免覆盖共享库及其他宿主。
- MUST 保持主题由宿主控制；Web 使用自有主题时应沿用 `skipPrimeVue` 等既有扩展点。
- MUST 在浅色、深色、窄屏和长文本下保持主要操作可见；颜色不能是连接状态、错误或风险的唯一表达方式。
- MUST 为新增面向用户的主应用文案使用既有 i18n 机制，并同步 `frontend-lib/src/locales/en.yaml` 与 `cn.yaml` 的键。
- MUST 保持现有 locale 标识 `en`、`cn`，不能直接改为 `zh-CN` 而破坏持久化和消费者。
- SHOULD 在尚未接入共享 i18n 的独立 uptime 中沿用本地实现；国际化迁移应单独设计，不声称它已存在。
- MUST 为输入提供可访问名称和关联错误信息；图标按钮必须有名称，禁用原因不能只放在不可聚焦的 tooltip 中。
- MUST 使用语义化控件支持键盘操作；对话框管理初始焦点、关闭和焦点返回，状态更新应能被辅助技术理解。
- MUST 复用共享库现有 tooltip 方案；不要重新引入其入口注释已明确避开的 PrimeVue Tooltip。

## 6. 安全与敏感配置

- MUST 将浏览器校验视为用户反馈，权限、ACL、CIDR、配置语义仍由服务端或核心验证。
- MUST 保留现有 API 客户端的会话和失败处理；不能绕过统一客户端重复实现鉴权或扩大跨域策略。
- MUST 将设备别名、服务端错误、URL 和导入配置视为不可信输入；默认通过文本绑定渲染，新增 HTML 渲染须有明确净化方案。
- MUST 避免将网络密钥、私钥、凭据和完整配置写入日志、遥测、路由参数、截图或错误提示。
- MUST 对密钥展示、复制、配置导出和持久化作明确设计；新增敏感持久化应说明宿主、安全边界、生命周期和清除方式。
- MUST 对 Tauri 命令与外部链接使用显式能力和协议校验，不能从用户输入拼接执行 shell 命令。
- MUST 保留旧认证流程兼容性；现有客户端摘要算法不能被描述为完整的密码保护方案，也不能由前端自行更换协议。

## 7. 命名与跨宿主兼容

统一要求见 [命名规范](naming.md)，本层额外强调：

- MUST 遵循目标目录现有文件命名；历史的 GUI `mobile_vpn.ts`、`config_source.ts` 不因新增规范被批量重命名。
- SHOULD 使用清晰的领域名，如网络、实例、设备、成员、路由和凭据；不要用含义不明的 `data2`、`temp`、`manager` 隐藏职责。
- MUST 保持公开导出、事件名、localStorage 键、路由名、Tauri 命令和 wire 字段兼容；确需变更时列出消费者和迁移方式。
- MUST 在命名上区分 `save`、`validate`、`run`、`stop` 与 `delete`，避免将修改配置和操作运行状态混为一个动作。

## 8. 验证与完成条件

以下是仓库已有命令入口，不代表本次文档变更已执行或所有云环境都具备运行条件。
在仓库根目录运行主工作区命令；仅修改文档时检查链接、命令和范围即可，不必安装前端依赖。

| 改动范围 | 最小相关验证 |
| --- | --- |
| 共享组件/配置类型 | `pnpm --filter easytier-frontend-lib test`，包含组件测试、配置转换测试和共享库构建。 |
| Web 页面/API/主题 | `pnpm --filter easytier-frontend build`；交互变更再执行 `pnpm --filter easytier-frontend test:browser`。 |
| GUI Vue/适配器 | `pnpm --filter easytier-gui build`；VPN 生命周期变更执行 `pnpm --filter easytier-gui test:mobile-vpn`；前端规范检查用 `pnpm --filter easytier-gui lint`。 |
| 配置生成器/WASM 边界 | `pnpm --filter easytier-config-generator build`，并手动验证导入、编辑、导出及校验失败路径。 |
| uptime | `npm --prefix easytier-contrib/easytier-uptime/frontend ci` 后运行 `npm --prefix easytier-contrib/easytier-uptime/frontend run build`；按变更验证交互。 |

- MUST 先确认工具链和锁文件；主工作区 CI 使用 Node 22、pnpm 10，安装采用 `pnpm install --frozen-lockfile`。
- MUST 注意 GUI `packageManager` 字段目前仍标记 pnpm 9；遇到版本约束冲突应记录原因，不能静默重写清单或锁文件。
- MUST 在共享库改动后验证受影响宿主；构建脚本已包含共享库构建，避免绕过它使用过期 `dist`。
- MUST 在运行浏览器测试前准备 Playwright Chromium 和系统依赖；例如 `pnpm --filter easytier-frontend exec playwright install chromium`。
- MUST 在配置生成器构建前准备 Rust、`wasm32-unknown-unknown`、`wasm-pack` 和 protoc；详情以包脚本和 CI 为准。
- MUST 将 Web `test:e2e` 作为有专门基础设施的集成验证：现有脚本依赖名为 `rust` 的 Docker 容器、共享仓库路径和 `target/debug` 二进制。
- MUST 在使用自定义 `CARGO_TARGET_DIR` 时检查上述 E2E 路径假设；普通 GUI 构建不等于 Tauri 原生构建或设备上的 VPN 验证。
- SHOULD 为新增状态转换、协议映射和修复过的交互缺陷增加行为测试；优先现有 Vitest、Vue Test Utils 或浏览器测试入口。
- MUST 在交付时分别报告实际执行、通过、失败、跳过和未运行的检查，不能把打包成功当作真实网络连接成功。

## 9. AI 修改前后检查

修改前 MUST 确认所属宿主、共享能力边界、协议来源及适用产品流程。
涉及契约时 MUST 同时定位生产者、转换层、消费者与测试。
修改后 MUST 确认生成文件没有手工改动、异步资源可释放、异常状态有反馈、敏感字段未泄露，
并记录受影响宿主和实际验证结果；如暂不能验证原生或集成能力，给出具体缺失条件。
