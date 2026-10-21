---
id: PLAN-ECO-010
feature: easytier-ecosystem-integration
kind: work-package
work_package: ECO-010
summary: ../README.md
status: done
owner: null
requirement: ../require/README.md
task_board: ../task/README.md
depends_on: [PLAN-ECO-009]
requirements: [ECO-01, ECO-02, ECO-03, ECO-04, ECO-05, ECO-06, ECO-07, ECO-08, ECO-09, ECO-10, ECO-11, ECO-12, ECO-13, ECO-15, ECO-16]
acceptance: [ECA-01, ECA-02, ECA-03, ECA-04, ECA-05, ECA-06, ECA-07, ECA-08, ECA-09, ECA-10, ECA-11, ECA-12, ECA-13, ECA-14, ECA-15, ECA-16, ECA-18, ECA-19, ECA-20, ECA-21]
tasks: [ECO-114]
updated_at: 2026-10-21
---

# PLAN-ECO-010：真实接口联调

[需求/计划汇总](../README.md) · [计划索引](README.md) · [需求与验收](../require/README.md) · [任务入口](../task/README.md)

本文件描述工作包计划，执行状态以关联任务与本文件元数据为准；公共范围见 [PLAN-ECO-000](plan-eco-000-overview.md)。路径别名 `D`、`F`、`L` 的定义也见该公共说明，所有拟新增文件仍未实现。

## 对应小任务

原工作包编号描述计划职责；下列 ECO-101 起编号是实际任务，不与 ECO-001～011 或已迁出 ECO-012 混用。执行状态读取各任务，不以计划成文替代完成。

| 任务 | 前置任务 | 并行组 |
| --- | --- | --- |
| [ECO-114 真实接口联调与失败回派](../task/task-eco-114-integration-verification.md) | ECO-113 | `eco-verification` |

共享文件与验证产物依[跨需求调度](../../README.md#跨需求共享文件调度)交接；详细独占范围、输入输出及失败回派见任务。

## 现状依据与未决项

依据 [根汇总](../README.md) 的 ECO-EV-001～005；关联 ECO-ISSUE-001～004。核查基线为 `feat/web-user-management@1e92296b17fc2e85dcd4521b6012232e11abed95`。源码依据与本专项实际运行结果分开；接口、页面证据和隔离环境未核实前不能宣布联调通过。

## 依赖与拟定写入范围

| 工作包 | 前置 | 独占写入范围与职责 | 交付给 |
| --- | --- | --- | --- |
| ECO-010 真实接口联调 | ECO-009 | 拟新增 `F/tests/ecosystem-integration.test.mjs`、`D/verification/`；已有接口的真实验证与失败回派，不擅改页面和业务模块 | ECO-011 |

## 7. 联调、验收与完成条件

### ECO-010 的输入与操作范围

输入为核对过的分支/SHA、ECO-002 能力清单、各页面及适配层交付、可运行应用、隔离测试账号与实际可用资源。缺少某个外部环境只阻塞对应验证；不得伪造设备状态，也不得直接用真实公网开放动作补齐演示。

ECO-009 完成接线后，由 ECO-010 使用真实接口核查查询/保存/授权/访问；缺陷回派对应所有者，契约问题回 ECO-002，适配问题回 ECO-008，路由问题回 ECO-009。修复完成后重验受影响场景，不由联调 Agent 并发修改各任务文件。

| 验收范围 | 所需证据与处理 |
| --- | --- |
| ECA-18 基线与复用 | 实现分支/SHA、已有页面和接口引用；确认原用户管理入口与权限行为保留，未重新加入不采纳功能 |
| ECA-02、ECA-06、ECA-10、ECA-12、ECA-20 页面状态 | 设计覆盖正常/空值/失败/未知/无权限/未支持及多网络选择；使用演示数据时只记录设计验证，真实状态仍需接口证据 |
| ECA-19、ECA-21 按需查询 | 代码与配置检查加浏览器网络记录：用户触发后请求有限结束；空闲不新增周期请求或订阅；已受理后不自动轮询、重试或重放 |
| ECA-01、ECA-03、ECA-04、ECA-05 访问、授权与设备操作 | 真实浏览器、现有服务端和相应设备环境；隐藏按钮、mock 响应或后端可达不能代替验证 |
| ECA-07、ECA-08 连续性与操作记录 | 不改动本地中枢自动化；有实际环境/记录接口时验证；缺少环境记未验证，仅有导航跳转不声称设备动作已审计 |
| ECA-09、ECA-11、ECA-16 资料持久化与归属 | 真实接口的保存再查询、归属错误与待接入状态；只有表单时记设计覆盖及后端缺口 |
| ECA-13、ECA-14、ECA-15 映射与导航生命周期 | 存在实际能力时用隔离资源核验冲突、失败保留、撤销结果、导航/公网独立；无网关或发布 API 时相关能力仍为未支持/未验证 |
| ECA-17 deploy | 后置，不作为本期完成条件，也不建隐含安装任务 |

除需求验收外，检查中文/英文、浅色/深色、桌面/窄屏、键盘与焦点、长名称、URL 和表单校验。旧登录、用户管理、原网络页面和已有链接做针对性回归。

ECA-19 的观察区间在实际基线清单中明确：覆盖已核查相关刷新间隔的至少两个周期，并检查无更长周期的新增任务。历史页面已有请求要记录来源，不能因外层布局既有请求误判新增轮询已通过或已失败。核心判断是新增页面及其复用接线没有引入资源状态周期机制；代码检查和运行证据共同支持结论。

### 验证入口

以下命令来自当前历史基线的 [Web 清单](https://github.com/CiBeDerORG/EasyTier/blob/1e92296b17fc2e85dcd4521b6012232e11abed95/easytier-web/frontend/package.json)与[共享库清单](https://github.com/CiBeDerORG/EasyTier/blob/1e92296b17fc2e85dcd4521b6012232e11abed95/easytier-web/frontend-lib/package.json)，ECO-001 在实现分支重新核对后使用；仅修改文档不运行这些应用检查；以下用于未来实际代码实施。

| 检查 | 当前入口与限制 |
| --- | --- |
| Web 类型与构建 | `pnpm --filter easytier-frontend build` |
| 浏览器交互 | `pnpm --filter easytier-frontend test:browser`；当前只运行 `tests/dashboard.test.mjs`，ECO-002 须接入新增测试，不能只创建未运行的文件 |
| 真实端到端 | `pnpm --filter easytier-frontend test:e2e`；当前只运行 `tests/central-e2e.test.mjs`，并依赖 Rust/容器、共享路径及二进制位置；按实际条件接入 ECO-010 验证 |
| 已交付用户管理回归 | 实现快照已有 `pnpm --filter easytier-frontend test:user-management`，覆盖改密能力与真实 OIDC 等流程；复用其自定义 Cargo 目录解析，不回退审核前的固定路径假设 |
| 共享库 | 实际修改共享组件/逻辑时执行 `pnpm --filter easytier-frontend-lib test` 及相关消费端检查；只复用组件不据此全量重测所有平台 |
| 纯文档修改 | 只做必要目录、链接、编号、状态和内容检查；不运行构建、测试或 CI |

ECO-001 先记录现有测试与浏览器依赖是否可用，区分基线失败、环境缺失与新回归；ECO-010 使用同一基线比较。没有执行的检查不能标为通过，不能为完成美化删除旧回归断言。

达到 M4 后将逐项真实验证结果、失败回派及剩余限制交给 [ECO-011](plan-eco-011-acceptance.md)。未执行/缺环境的项不能填通过；文档内的命令属于未来代码实施阶段。

## 本次记录

2026-10-21：从原 PLAN-ECO 分离本工作包文档，补充根汇总依据引用；只检查文档目录、链接与状态，未运行构建、测试或 CI；当时未生成本包任务明细。

2026-10-21：根据本计划继续拆出上列任务，明确代码依据、输入输出、写入范围与多 Agent 交接；仍未认领或实施。
