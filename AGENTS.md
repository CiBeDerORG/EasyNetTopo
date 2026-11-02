# EasyNetTopo — AI Agent 开发入口

> **本文件是该项目实际工程规则入口，不是要求加载整个知识库。** 以本仓库**当前任务所选分支的真实源码与配置**为准；[DevKnowledge](https://github.com/CiBeDerORG/DevKnowledge) 只提供跨项目通用规范及项目背景。**读取规范 ≠ 自动授权开发、建 Issue、初始化、提交、合并或部署。**

## 1. 任务启动：先判定，再按需读取

1. 核实用户当前目标、仓库/工作分支、实际文件、任务授权及已有项目文档；不要把历史聊天、本地旧盘点或其他仓库的能力当作本仓库事实。
2. 仅需要**项目定位/稳定产品决定**时，按需读取 [EasyNetTopo INDEX](https://github.com/CiBeDerORG/DevKnowledge/blob/main/02-%E9%A1%B9%E7%9B%AE/EasyNetTopo/INDEX.md)；**目前没有 EasyNetTopo 的 MEMORY.md**，不得猜测或自动创建。
3. **需求讨论/技术选型/任务拆分**才进入 DevKnowledge 的「需求管理」相应阶段；**授权编码/评审/CI**才进入「开发流程」对应阶段。DevKnowledge 的 [知识路由与准入](https://github.com/CiBeDerORG/DevKnowledge/blob/main/00-%E5%B7%A5%E4%BD%9C%E5%8F%B0/%E7%9F%A5%E8%AF%86%E8%B7%AF%E7%94%B1%E4%B8%8E%E5%87%86%E5%85%A5.md) 及相应规范现已进入正式 `main`；按需从已合并版本读取。未合并的其他评审分支只有用户明确指定时才能作为候选参考，不能当正式规则。**不能因为文档引用就遍历全部规范**。
4. 仅当前任务实际涉及的代码类型才读取通用 + 前端/后端规则；框架专项须**实际依赖/版本 + 真实运行入口代码 + Task 修改范围**同时匹配。没有技术栈或证据不足时，只执行通用原则，**禁止自行确定框架、安装技术栈、初始化空工程**。
5. Agent **产品运行**规范须真实存在 LLM Agent 的运行与工具调用代码；Codex/OpenCode 等仅作为编码助手、或项目讨论了 AI，**都不得触发 Agent 产品/MCP/RAG/A2A 专项**。微服务、模块化单体也不能根据 Docker、目录或项目名推断。
6. **私有 DevKnowledge 访问准入**：本项目的知识库链接**仅供定位，不能匿名访问**。
   - ChatGPT 网页版等云端工具：须使用**当前会话已连接且有 `CiBeDerORG/DevKnowledge` 读取权限的 GitHub 连接器**；没有连接器/授权时不能把 URL 当作可抓取网页。
   - 本地 Codex、OpenCode、NAS、服务器：仅在**实际运行环境**已有 SSH Key/ssh-agent 和 GitHub 读取权限时，才可使用 `git@github.com:CiBeDerORG/DevKnowledge.git`。本地已有 clone 可在授权和网络可用时 `git fetch origin main`，按已确认分支/Commit 定向读取文件；**不把陈旧 checkout 当最新规范**。
   - **身份隔离**：ChatGPT 云端不能自动继承用户本机或 NAS SSH 密钥；SSH 连接测试成功也不意味着已获本私有仓库授权。访问失败时报告原因，不猜测内容、不要求将仓库公开、不索取私钥/Token、不关闭主机校验、不擅自调整权限。
   - 成功授权只允许**按当前 Task 的严格准入读取必要文件**，不意味着自动加载整个 DevKnowledge；修改知识库还需明确写入授权。

## 2. 当前可验证的工程基线

- 以 **2026-11-01 已核实的 GitHub `main`** 为起点：本仓库只有 `README.md`、`AGENTS.md`，**没有可运行前端/后端、真实依赖/锁文件、测试与 CI**。每次开发需重新核实目标分支，不能将本快照当成永远不变。
- **已确认目标技术选型，尚未初始化代码**：后端 Go，前端 Vue 3 + TypeScript + Vite，数据存储 SQLite；模块化单体 + 内置能力适配器。具体 Go 版本、SQLite 驱动、第三方依赖及 CI 需在授权初始化时核实并锁定，不把目标设计当已实现工程。
- **正式项目资料**优先存本仓库现有文档结构；首次有实际规格/架构设计时再按需采用 `docs/requirements/`、`docs/architecture/`、`docs/adr/`。**不预建空目录、占位文件、无消费者的公共封装或任务 Markdown**。GitHub Issues/PR/Actions 才是实际任务、变更与技术验证的动态权威来源（存在时）。
- 框架专项须在初始化真实依赖与入口后，按实际修改范围重新判定；本轮已确认 Vue 3/Go 技术方向，但仅有方案文档时仍不得伪称框架已落地或加载不存在的专项。

## 3. 产品目标与实施界限

1. **独立的通用集成管理平台**：平台不依赖 EasyTier、Sub2API 或其他具体业务系统，统一管理应用源、连接、主机/资源、权限及任务，业务专属 API 按需由内置适配器接入。
2. **首个验收闭环**：EasyNetTopo 使用私有 Deploy Git 应用源的 Sub2API 模板，通过独立 EasyHostAgent 完成 IP 直连、TLS 一次性配对、远程安装、真实健康状态、日志及保留数据的管理；不要求补齐 Deploy 其他应用元数据。
3. **单控制中心**：EasyNetTopo V1.0 是 Agent 唯一有效控制端；成功重新配对替换旧授权。Agent 是独立可选安装组件，平台无 Agent 时仍可启动。
4. **安装与业务集成解耦**：未来 EasyTier 可以作为独立应用安装，再通过已授权 API 接入专属组网管理能力；此集成不作为 V1.0 Sub2API 首例依赖，也不要求 EasyNetTopo 自身进行组网。
5. 正式规格见 [V1.0 需求](docs/requirements/v1-platform-scope.md)、[架构设计](docs/architecture/v1-modular-integration.md)、[M0 跨仓库协议](docs/architecture/m0-cross-repo-contracts.md)（在本次评审分支内）；**需求与选型确认不等于已初始化/开发/验收授权**，任务状态以 GitHub Issue/PR/CI 为准。

## 4. 编码与提交的常规约束

- **统一简体中文**：需求分析、架构讨论、文档、界面文案、测试说明、Issue/PR、评审及交付报告用中文；新写/实质修改的解释性代码注释用中文。代码标识符、API/协议、依赖、许可证、第三方内容按技术原貌保留。
- **提交信息使用中文**：Commit 标题和正文以及 PR/Issue 说明采用中文，可保留工具要求的 `feat:` / `fix:` 等前缀；不要为中文化批量改写历史、上游代码和无关文件。
- 简单任务最少改动；业务 Task 以**可独立验证的完整业务能力**为单位，不机械按前后端分裂。修改遵守项目未来实际技术栈的格式、类型、Lint、测试规则。
- 需要部署、账号操作、合并和其他高风险外部行为时须有明确授权；真实 GitHub CI 与用户设备验收分别报告，不能互相冒充。
- 知识沉淀只限已确认且长期有价值的结论，按需在 DevKnowledge 建链接/摘要；**不存在自动跨仓库任务状态同步**，未经授权不得跨仓库写入。

**原则：该仓库定义项目事实和执行规则；DevKnowledge 负责按需提供通用方法。严格准入，缺证据就不套框架。**
