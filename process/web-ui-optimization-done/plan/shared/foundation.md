# WUI-103 公共基础交付

对应 [公共契约 v1](../contract.md)、[WUI-103](../../task/task-wui-103-shared-foundation.md)。负责人 `iteration_exec2`；工作区 `/workspace/EasyTier-iteration`，分支 `feat/ecosystem-web-ui`。源基线 `1e92296b17fc2e85dcd4521b6012232e11abed95`，最终提交由总控登记。

## 改动与所有权

- `frontend/src/console.css`：根据 WUI-102 长标题导致菜单宽 9.41px 的证据，明确面包屑可收缩、操作区/按钮不可收缩，标题省略但保留完整 DOM。无新业务状态、接口、依赖或周期请求。
- `frontend/tests/web-ui-support/webUiBrowser.mjs`：生产 preview 和浏览器有单一生命周期所有者，动态 loopback 端口；每项测试独立 context，HTTP fixture 由测试定义。启动浏览器失败也关闭服务。
- `frontend/tests/web-ui-support/foundation.test.mjs`：真实生产组件与样式，mock HTTP；覆盖 cn/en、light/dark、360/768/1440 长网络名称，验证标题全文、topbar 高度、按钮宽度、整页溢出、移动菜单可打开/关闭及 pageerror。
- `frontend/package.json`：仅新增 `test:web-ui:foundation` 构建后执行上述专项测试；不变更依赖或锁文件。
- `plan/contract.md`：固定已有页面 props/events、视觉/状态/焦点/错误与兼容契约、共享文件所有权、WUI-111 语义类交接。
- `theme.ts`、`style.css`、cn/en locale 未修改。已有符合预期的表单/错误反馈不重写。

## 实际验证

| 命令/检查 | 结果与限制 |
| --- | --- |
| `node scripts/codegen-proto.mjs`（frontend-lib，PATH 使用已有 protoc） | 采证准备阶段通过，生成源码无差异 |
| `node node_modules/vue-tsc/bin/vue-tsc.js -b`、`node node_modules/vite/bin/vite.js build`（frontend-lib） | 采证准备阶段通过；本次 CSS 修改未改变共享库 |
| `node node_modules/vue-tsc/bin/vue-tsc.js -b`、`node node_modules/vite/bin/vite.js build`（frontend） | WUI-103 修改后通过；构建有既有 caniuse 数据陈旧和大 chunk 警告，无依赖升级 |
| `node --test tests/web-ui-support/foundation.test.mjs` 第一次 | 8/12 通过，4 项窄屏用例因测试错误使用 Drawer `role=dialog` 超时；核实 PrimeVue 实际 role 为 complementary，修正为现有 `.console-mobile-nav` 定位，未弱化布局断言 |
| 同命令第二次 | 12/12 通过，0 失败/跳过；四种语言/主题的窄屏按钮均至少 28px，topbar 不高于 80px，完整标题保留，菜单可打开/关闭 |

pnpm 入口在此环境触发自动依赖检查并尝试写不存在的 `/home/agent/.local/share/pnpm`，因此实际按清单逐项执行生成、类型检查、构建工具。新 npm 脚本已接线，但不把未成功运行的 pnpm 包装命令写成通过。

浏览器回归只使用新工作区生产 dist，preview 端口动态分配，不改写 dist，不占其他 agent 的 Vite/缓存。fixture 只用于布局与组件动作，不证明数据库、OIDC、VPN、ACL运行或真实拓扑。ECO-102/WUI-102 真实 API 采证已另行记录并关闭服务、清理临时数据库。

## 后续交接

- 当前结构选择器是保留 MainPage 独占写入范围的过渡；WUI-111 增加 `console-breadcrumb`、`console-page-title`、`console-topbar-actions` 及标题可视查看方式后，回派 WUI-103 串行替换选择器和跑同一针对性验证。不能把这一阶段说成 OBS-003 所有可访问性交接已闭合。
- WUI-111 另修移动菜单焦点返回；当前测试只检查打开/关闭，不宣称焦点缺陷已解决。
- WUI-105 修用户表窄列；不在公共 CSS 写页面特定补丁。
- 监工验收后由总控交还 locale/package 锁，ECO-104/105 再接写。后续页面公共文案/样式请求继续经唯一所有者串行处理。

2026-10-21，WUI-111 串行交接：已由同一公共资源负责人将 topbar 的 DOM 顺序选择器替换为 `console-breadcrumb`、`console-page-title`、`console-topbar-actions`。宿主补完整 title 提示及关闭焦点策略；foundation 12 项矩阵全部通过，360px 组合额外断言焦点恢复。见 [宿主接线证据](../integration.md)。
