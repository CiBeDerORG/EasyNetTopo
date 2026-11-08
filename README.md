# EasyNetTopo

**EasyNetTopo 是独立的通用集成管理平台**，不以任何具体业务系统为运行前提。它提供统一的 Git 应用源、主机连接、受管资源、应用生命周期和任务管理；各业务系统独立安装并通过适配器按需集成。

> **现状核对（2026-11-02，main）**：已有 Go `cmd/internal`、Vue `web/`、应用源/主机/Bundle 相关源码与 GitHub Actions 工作流；这仅证明代码和自动化入口存在，**不是 Sub2API 实际安装或 FnOS 联调已通过**。以下仍以已确认的产品方向为准；执行时必须检查目标分支最新源码与测试。

## 当前代码基线

- Go 后端、Vue 3/TypeScript 前端与 SQLite 工程已存在。
- 同步 Git 源后，商店按 `<应用ID>/app.yaml` 展示 Docker Compose 应用。选择已配对主机后，使用 Agent 批量状态识别未安装、受管已安装和外部同名项目。未安装显示安装入口；受管已安装显示更新、卸载入口；外部项目默认只读；仅 Sub2API、Portainer 的原有 Deploy 部署在按应用授权、Docker Compose 路径和脚本验证通过后，可由用户明确确认接管。安装支持清单参数，卸载需明确确认。
- 安装、更新和卸载均由控制端向已配对的 EasyHostAgent 提交受控操作，Agent 仅按固定 Git 版本的应用包中声明的 `deploy/install.sh`、`deploy/update.sh`、`deploy/uninstall.sh` 执行；控制端不在容器内运行 Docker 安装命令。页面使用 Toast 反馈受理、执行成功、失败或状态未知；只接受任务不等于完成，需轮询 Agent 实际结果。更新保留既有密钥和配置，卸载默认保留持久化数据。
- 目标主机需在本地授权页面上显示的控制端 ID 与来源 ID。Agent 使用安装时保存的包执行卸载，源条目移除后仍可在该主机看到安装记录。
- 主机管理支持编辑名称和 Agent 连接地址，也支持删除控制端本地主机记录。编辑操作仅更新 `name/endpoint`，不读取或修改 Agent UUID、公钥指纹、密封 Token 和原配对时间；需要重新配对时，点击主机「编辑」并展开默认折叠的配对设置，单独使用一次性码完成。
- 删除必须有明确确认，仅删除控制端这条主机登记及其本地凭据，不卸载 Agent、不操作远端 Docker 应用，也不会自动撤销 Agent 端授权。若要撤销 Agent 端授权，需另外在 Agent 端执行受控操作。
- **历史 R001/T001 需求及任务规划已撤销**。此处不再将旧版 Sub2API 专项闭环、M4 任务预检或更新流程当作已确认的第一期开发范围。新需求及任务须重新确认后建立。

## 技术参考

- [Go 模块化单体与集成适配器设计（历史设计草案）](docs/architecture/v1-modular-integration.md)
- [EasyNetTopo / EasyHostAgent / Deploy 的 M0 协议清单](docs/architecture/m0-cross-repo-contracts.md)

上述设计与协议文件供核对已有工程边界；不能代替重新确认的需求，也不能证明未实现功能已完成。

## 工程和知识管理

实际业务代码和技术设计属于本仓库。旧版需求/任务文件已删除；后续重新确认的需求和任务在本仓库独立建档，Git/PR/CI 仅提供真实发生的开发证据，不替代业务验收。通用 AI 协作规范与长期项目背景位于独立私有仓库 [DevKnowledge](https://github.com/CiBeDerORG/DevKnowledge)，按当前任务严格准入读取，不能假定网页端或本地 SSH 自动具有权限。

- [项目开发入口 AGENTS.md](AGENTS.md)
- [EasyNetTopo 知识入口（私有仓库）](https://github.com/CiBeDerORG/DevKnowledge/blob/main/02-%E9%A1%B9%E7%9B%AE/EasyNetTopo/INDEX.md)

本仓库不使用知识仓库 Submodule、符号链接或全量复制。未实际批准的代码初始化与部署不能以设计文档替代授权；不创建无用占位文件。

## GitHub Actions 构建成功 Hook

EasyNetTopo 的多架构镜像发布工作流在 `main` 分支构建并推送成功后，可自动调用部署 Hook：

- Hook URL：`http://49.235.163.195:7017/api/hooks/deploy/update`
- Actions Secret：`EASYNETTOPO_HOOK_TOKEN`，值必须与 EasyNetTopo 服务端 `/software/deploy/.secrets` 中的同名配置一致（至少 32 个字符）。
- Actions Variable：`EASYNETTOPO_HOOK_TARGETS_JSON`，填写 JSON 数组，例如：
  `[{"host_name":"Mac-mini","service_name":"easynettopo"},{"host_name":"Linux-server","service_name":"portainer"}]`。
- 未配置目标数组时，工作流会提示并跳过 Hook；配置目标后，只有 `main` 镜像发布成功才会提交更新请求。

Hook 只负责受理任务（HTTP 202），不代表远端更新已完成。任务状态由 EasyNetTopo 前端查看。

**安全提醒：** 当前地址按部署方要求使用 HTTP。Bearer Token 会以明文经过网络，不应在不可信公网链路上使用；正式长期使用建议为该地址配置 HTTPS 反向代理，再将工作流 URL 改为 HTTPS。
