# EasyNetTopo

EasyNetTopo 是一个独立的统一设备与服务管理平台，**不以安装 EasyTier 为启动前提**。

## 当前产品方向

1. 用户先部署 EasyNetTopo，打开 Web 管理后台。
2. **优先建设应用商店与本机应用生命周期管理**：通过应用商店安装和管理 **官方原版 EasyTier**，无需用户手工安装 EasyTier。
3. 上述基础能力完成后，再研究 EasyTier 的配置服务器 / 管理 API 等集成方式，逐步提供网络节点和网络信息管理。
4. 后续按需接入 EasyHostAgent（主机与应用管理）、EasyADB（ADB 设备与自动化），这些能力均为可选扩展。

以上是产品方向和开发顺序，**不是已实现的功能清单**。**截至 2026-11-01 查询 GitHub `main`**，仅有 `README.md` 和 `AGENTS.md`，尚无实际应用代码、依赖配置、构建/测试及 CI。当前没有已确认的前后端语言或框架，不能从产品规划推断开发技术栈；以后执行任务时以所选分支最新内容为准。

## 研发知识中心

项目背景、已确认的产品取舍和长期上下文统一存放在独立的私有知识仓库 [DevKnowledge](https://github.com/CiBeDerORG/DevKnowledge)：

- [EasyNetTopo 知识入口（INDEX.md）](https://github.com/CiBeDerORG/DevKnowledge/blob/main/02-%E9%A1%B9%E7%9B%AE/EasyNetTopo/INDEX.md)
- [跨项目 AI 开发共同约定](https://github.com/CiBeDerORG/DevKnowledge/blob/main/01-%E5%B7%A5%E7%A8%8B%E8%A7%84%E8%8C%83/AI%E5%BC%80%E5%8F%91%E8%A7%84%E8%8C%83.md)（以正式合并后的版本为准）

知识仓库与代码仓库分别维护，不使用 Git Submodule、符号链接或自动复制知识文件；访问私有知识库需要相应的 GitHub 权限。知识库的新变更在 PR 审核并合并后才会进入其 main 分支。

## 项目初始化与任务管理

新项目初始化时先根据真实业务、部署环境和资源完成技术选型，再由用户授权建立**最小可运行闭环**；只按当前采用的技术加载相应编码规则，不因为计划做 Web 就默认 React、Vue、某一后端或微服务。已确认的正式需求/技术设计存本仓库实际需要的 `docs/` 位置，动态工作项使用 GitHub Issues/PR/CI，不为尚不存在的任务创建空文档。

当前知识库中**不存在** EasyNetTopo 的 `MEMORY.md`；不要猜测其内容或自动建立空记忆。DevKnowledge 尚未合并的规范 PR 只作评审参考，不当作 `main` 的已生效规范。

## AI Agent 入口

在本仓库开展研发前阅读 [AGENTS.md](AGENTS.md)，仅在需要项目长期背景时读取 DevKnowledge 的实际 [INDEX](https://github.com/CiBeDerORG/DevKnowledge/blob/main/02-%E9%A1%B9%E7%9B%AE/EasyNetTopo/INDEX.md)。不递归读取所有规则；源码、部署和执行规则以本仓库所选分支真实文件为准。
