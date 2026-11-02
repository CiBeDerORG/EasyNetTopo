# T001 · EasyNetTopo V1.0 通用集成平台（汇总任务）

> 编号整编：2026-11-02；关联 [R001 权威需求](v1-platform-scope.md)。状态按已读取的 `main` 代码与设计文档保守标注，**未执行本轮业务端到端测试**。历史 M0～M4 是研发里程碑/来源，不是当前 T 编号、完成率或独立分支。任务状态以本文件为准，交付证据以真实源码、测试和运行记录为准。

## 汇总状态

**T001：进行中；已完成 0 / 有效叶子 10。** 分母仅包括下面 10 个叶子任务；“待验证”表示已发现关联实现或契约但尚缺本轮完整业务 DoD 证明，**不等于该模块没有代码**。任何叶子项的验证结果变化后必须同步更新本行及关联需求的交付汇总。

| 任务编号与名称 | 父级 | 关联需求 | 前置依赖 | 当前状态 | 已核实来源 / 未验证项 |
| --- | --- | --- | --- | --- | --- |
| T001 · V1 业务闭环（汇总） | — | R001 | — | 进行中 | 0/10；未完成 Sub2API FnOS 真机闭环 |
| T001.01 · 平台 Go/Vue 初始化与登录闭环 | T001 | R001.01 | 无 | 待验证 | `cmd/easynettopo/`、`internal/httpapi/auth.go`、`web/src/App.vue` 已存在；实际生产登录待验证 |
| T001.02 · Git 源手动刷新与快照闭环 | T001 | R001.02 | T001.01 | 待验证 | `internal/catalog/git.go`、`source.go`、`internal/deploysync/cache.go` 已存在；私有源联网联调待核实 |
| T001.03 · 清单 Schema/语义验证与商店展示 | T001 | R001.02 | T001.02 | 待验证 | `internal/catalog/manifest.go`、`semantic_validation.go`、`catalog_routes.go` 已存在；真实源验收待核实 |
| T001.04 · Agent 主机登记与唯一控制端配对 | T001 | R001.03 | T001.01 | 待验证 | `internal/hosts/client.go`、`host_routes.go` 已存在；可信终端指纹与重新配对实测待核实 |
| T001.05 · Docker/Compose 资源快照与错误语义 | T001 | R001.04 | T001.04 | 待验证 | `internal/hosts/` 与跨仓 Agent Docker 发现代码已存在；FnOS 停止/离线场景待验收 |
| T001.06 · 固定提交部署包生成、校验与传输 | T001 | R001.05 | T001.03、T001.04 | 待验证 | `internal/packagebuilder/`、`internal/hosts/client_bundle.go`；跨仓信任闭环待核实 |
| T001.07 · Agent 持久任务、幂等与资源预检 | T001 | R001.06 | T001.04、T001.06 | 待验证 | EasyHostAgent `docs/m4-persistent-task-admission.md`、`docs/m4-resource-preflight-recovery.md` 为现存证据，完整执行待核实 |
| T001.08 · 脚本可信授权与受控执行安全门禁 | T001 | R001.05、R001.06 | T001.06、T001.07 | 阻塞 | 本机批准包/摘要机制与真实执行器的端到端安全授权尚需验证；不可只凭 source_id 执行脚本 |
| T001.09 · Sub2API 安装、健康检查及生命周期 | T001 | R001.06、R001.07 | T001.07、T001.08 | 待办 | 需要实际安装/更新/卸载及三服务健康、凭据数据保留证据 |
| T001.10 · FnOS/Linux 端到端及负例验收 | T001 | R001.01～R001.07 | T001.01～T001.09 | 待办 | AT-01～AT-13，区分自动契约测试和人工真实设备验收 |

## 历史任务映射与边界

- EasyNetTopo 历史 M0 契约 → T001.02～T001.08 的接口基础；M1/M2 主机接入/资源发现 → T001.01、T001.04、T001.05；M3 Git/包/Schema → T001.02、T001.03、T001.06；M4 准备/资源预检 → T001.07、T001.08。**映射不是完成证明**。
- EasyHostAgent 旧 `docs/task-plan.md` 的 T00～T06 以 **EasyTier 为控制端**，应作为历史依据，不能把那些 DONE 直接赋予新的 EasyNetTopo V1 T 编号。
- Deploy 中的 Sub2API manifest/契约测试与 EasyHostAgent bundle/tasks 代码分别是跨仓依赖；本文件只记录 EasyNetTopo 业务交付，不复制外仓的全部 Task 明细。
- P0 首次闭环与 P1 后续增强是需求交付分期，不强制分别创建分支或任务清单；同一需求只复用一个工作分支和一个 PR（使用 Git 且按项目政策执行时）。

## 状态维护规则

叶子任务仅在当前版本有实际 DoD 和相关验收证据时标 `已完成`。任务被取消/拆分时保留旧编号及映射，不重编号；父级状态从有效叶子归集。真实设备验收与代码开发完成分开记录。修改此文件前先读取最新版本，避免 Agent 并行覆盖。
