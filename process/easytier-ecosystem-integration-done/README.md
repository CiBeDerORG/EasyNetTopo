---
feature: easytier-ecosystem-integration
status: done
baseline: feat/web-user-management@1e92296b17fc2e85dcd4521b6012232e11abed95
updated_at: 2026-10-21
---

# 网络拓扑与资源集成需求/计划汇总

本文件先汇总范围、现状、进度和问题，后续需求、计划及任务从这里展开。详细内容必须引用已核对代码或明确标为拟新增/待核实，不能把目标模型写成现有功能。

本轮按“需求 → 小计划 → 小任务”继续拆分：保留11个原工作包，安排15项独立任务（ECO-101～115），先核实源码/API与页面证据，再交付契约和公共基础，随后六组独立页面/适配任务并行，最后串行组合、接线、真实联调与验收。本期15项任务已全部验收；缺失外部能力明确未支持，不据此补造接口。

## 目标与基线

目标是在 EasyTier Web 按“网络拓扑 → 网络节点 → 类型 → 服务/设备组 → 设备”组织资源，补充地点、型号、访问及导航入口。本期按已存在的接口能力进行页面设计与接入；缺口记录为未支持/待确认，不自动扩展为资源后端、网关或外部集成开发。deploy 安装方案后置。

2026-10-21 只读核对的实现基线为 `feat/web-user-management@1e92296b17fc2e85dcd4521b6012232e11abed95`，对应实现工作树干净。采用文档快照为 `docs/architecture-and-user-management-plan@670eb3b42800d42333356da0cb15f41d76c92f1e`。没有将用户管理实现合并到文档分支；设计启动时应核实实际采用提交及其后续差异。

## 进度与内容入口

| 内容 | 文档进度 | 实现进度 | 下一步 |
| --- | --- | --- | --- |
| [需求](require/README.md) | 模型、范围和用户约束已形成草案；外部能力仍待核实 | 契约及组件/组合已验收，外部业务能力依契约明确未支持 | 按ECO-103固定接口范围完成宿主接线、真实联调 |
| [PLAN-ECO](plan/README.md) | 11个工作包各有 `plan-eco-编号-名称.md`；另有一份公共说明，不计工作包 | 11计划/15任务已完成；限制见验收记录 | 交付与回退见验收记录 |
| [任务看板](task/README.md) | 已生成15份 task-eco-101～115 明细，覆盖11个计划工作包 | 见当前实施快照 | 已完成；见[验收范围](plan/design/acceptance.md) |

总体为 `done`；页面取证按任务执行，未取得证据前不作通过结论。

## 当前事实与代码依据

以下链接固定到已核查的 `1e92296b`，不是引用当前文档分支的旧源码。

| 编号 | 类型 | 现状/目标 | 代码依据与处理 |
| --- | --- | --- | --- |
| ECO-EV-001 | 事实 | 宿主已有登记信息，但不是手机/家居设备模型 | [devices.rs](https://github.com/CiBeDerORG/EasyTier/blob/1e92296b17fc2e85dcd4521b6012232e11abed95/easytier-web/src/db/entity/devices.rs) 的 `Model` 使用 `user_id`、`machine_id` 并存 hostname、alias、OS 等；此模型未含人工地点和型号字段，不能编造已有保存能力 |
| ECO-EV-002 | 事实 | 中央网络成员记录引用宿主，已有机器查询入口 | [network_members.rs](https://github.com/CiBeDerORG/EasyTier/blob/1e92296b17fc2e85dcd4521b6012232e11abed95/easytier-web/src/db/entity/network_members.rs) 的 `Model.device_id`；[network.rs](https://github.com/CiBeDerORG/EasyTier/blob/1e92296b17fc2e85dcd4521b6012232e11abed95/easytier-web/src/restful/network.rs) 的 `handle_list_machines` 与 `/api/v1/machines`，具体页面字段和权限仍须逐项对应 |
| ECO-EV-003 | 事实 | 已有 TCP/UDP 端口转发配置原语 | [gateway.rs](https://github.com/CiBeDerORG/EasyTier/blob/1e92296b17fc2e85dcd4521b6012232e11abed95/easytier-core/src/config/gateway.rs) 的 `PortForwardConfig` 含绑定地址、目标地址和协议；它本身不证明资源级公网授权、导航发布或撤销活动流能力已具备 |
| ECO-EV-004 | 事实 | 既有 Web 含周期请求，复用需控制副作用 | [MainPage.vue](https://github.com/CiBeDerORG/EasyTier/blob/1e92296b17fc2e85dcd4521b6012232e11abed95/easytier-web/frontend/src/components/MainPage.vue) 的 `setInterval(loadNavigation, 10_000)`；[NetworkDetail.vue](https://github.com/CiBeDerORG/EasyTier/blob/1e92296b17fc2e85dcd4521b6012232e11abed95/easytier-web/frontend/src/components/NetworkDetail.vue) 的 `setInterval(loadNodeDetail, 5_000)`，新增页面不得借用这些定时器扩展资源状态查询 |
| ECO-EV-005 | 拟新增/待核实 | 节点资料扩展、服务/设备组/设备层级、公网资源发布与 Navloom 入口 | 来自用户目标及[需求](require/README.md)，已按ECO-103核对并固定为未支持；不能推定 EasyADB、家居、Navloom 或 deploy 仓库/API 已接入 |

## 问题、缺口与下一步

| 编号 | 问题与来源 | 当前状态 | 影响 | 下一步/解除条件 | 负责人 |
| --- | --- | --- | --- | --- | --- |
| ECO-ISSUE-001 | 地点/型号及业务资源所需字段、写入与权限能力，见上述模型与[计划待决项](plan/README.md#8-当前待决项与记录) | 已核实缺口并冻结契约，人工资料保存未支持 | 不能承诺资料可保存或资源可控制 | 见ECO-103契约与111适配，缺口不自动转后端任务 | 总控与对应任务所有者 |
| ECO-ISSUE-002 | EasyADB、家居、Navloom 的实际仓库/API、端点与权限未核实，见[需求待细化项](require/README.md#13-待细化事项与验证记录) | 本期未支持；不虚构外部API | 外部资源操作与导航发布未实现 | 实际涉及接入前核对来源与契约；未核实能力明确显示未支持 | 总控与对应任务所有者 |
| ECO-ISSUE-003 | 已有组件包含周期状态行为，见上述 MainPage/NetworkDetail 代码 | 已确认的复用约束 | 直接复用可能违反新增页面不增加定时查询的约定 | ECO-102记录旧请求，103明确纯展示/按需边界，113正式接线不得新增轮询、心跳、订阅或定时任务 | 总控与对应任务所有者 |
| ECO-ISSUE-004 | 页面证据、隔离账号/设备、公网入口及验收环境，见[计划待决项](plan/README.md#8-当前待决项与记录) | 已完成真实空态取证；114联调已完成，真实设备/公网未验证 | 对应真实行为不能判为可用或通过 | 沿用102真实页面证据，114补本轮API与空闲请求证据；真实设备缺口保留 | 总控与对应任务所有者 |

状态以可观察代码和记录为限；尚未全面核查的项目不写成“无问题”。保留 `config-server` 客户端连接、认证、配置下发及接入命令；新增页面仅按进入页面、切换对象、手动刷新或明确操作请求按需查询。

## 关联需求

- [需求](require/README.md)
- [计划](plan/README.md)
- [任务看板](task/README.md)

以 EasyTier Web 为入口，按网络拓扑、节点、服务/设备组及设备组织资源，按已有接口能力安排访问与导航。当前已有需求、11份小计划及15项任务文档；页面和正式接线已验收，真实隔离联调已完成，外部业务集成按契约明确未支持。

复用[已完成用户管理](../web-user-management-done/README.md)。[现有 Web 交互优化与页面美化](../web-ui-optimization-done/README.md)已独立管理，不属于本需求工作包，也不是拓扑接入的开发前置。双方涉及共享宿主文件时，按[全局规则](../README.md)串行交接。

## 维护记录

2026-10-21：根据交付基线只读核查设备、成员模型、机器 API、端口转发配置及现有请求入口，补齐汇总和待核实项。未编造外部 API 或后端能力，未认领/实施工作包，未运行构建、测试或 CI。

2026-10-21：将计划拆成索引、公共说明及11个独立工作包文件；任务入口链接具体计划，当时任务明细待依据实际契约拆分，未将文件整理计作实施进度。

2026-10-21：按最新“需求拆分”定义完成15项小任务，实际状态全部backlog，待核实事项落到101/102/103等任务与启动条件；计划/看板/明细同步多Agent批次、单一写入者和跨需求交接，未执行应用检查。


## 组合阶段问题（2026-10-21）

ECO-112监工预审发现：刷新overview时把带旧数据的成员失败状态转为ready，若overview再次失败会误报成员已加载。已由exec3修复并补精准1/1回归，监工已最终验收。真实设备/公网缺口仍按103契约明确未支持或未验证。


## 正式产物验证发现

ECO-113 正式宿主测试发现旧 frontend-lib/dist 未含 ECO-104 新文案，预览直接使用源码语言文件未暴露此差异。已交 exec3 以独占构建更新产物；完整 codegen 因系统缺少 protoc 且下载 DNS EAI_AGAIN 未成功。schema/既有生成源码未修改，类型检查与 Vite 构建另行记录，不声称完整代码生成通过。


## 当前实施快照

已验收 15/15；；其余 0 项 backlog。

总控维护任务与共享锁；监工 `/root/iteration_monitor` 审查后才能完成或进入联调。实际验证见各任务交接，未运行场景不作通过结论。

## 最终交付

本期 ECO-101～115 全部验收；[最终验收](plan/design/acceptance.md)区分已实现、真实/fixture验证、未支持、未验证和后置。完整代码生成因缺protoc与DNS错误未成功；实际类型与Vite构建通过，生成源码未改。真实设备/VPN/公网/部署/旧二进制回退未验证。后续外部资源能力须另行明确需求，本轮不创建后端平台。
