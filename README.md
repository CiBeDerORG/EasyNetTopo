# EasyNetTopo

EasyNetTopo 是一个独立的统一设备与服务管理平台，**不以安装 EasyTier 为启动前提**。

## 当前产品方向

1. 用户先部署 EasyNetTopo，打开 Web 管理后台。
2. **优先建设应用商店与本机应用生命周期管理**：通过应用商店安装和管理 **官方原版 EasyTier**，无需用户手工安装 EasyTier。
3. 上述基础能力完成后，再研究 EasyTier 的配置服务器 / 管理 API 等集成方式，逐步提供网络节点和网络信息管理。
4. 后续按需接入 EasyHostAgent（主机与应用管理）、EasyADB（ADB 设备与自动化），这些能力均为可选扩展。

以上是产品方向和开发顺序，**不是已实现的功能清单**。仓库已重新清空；当前只是重新建立项目说明和知识入口，尚无可运行平台和应用商店代码。

## 研发知识中心

项目背景、已确认的产品取舍和长期上下文统一存放在独立的私有知识仓库 [DevKnowledge](https://github.com/CiBeDerORG/DevKnowledge)：

- [EasyNetTopo 知识入口（INDEX.md）](https://github.com/CiBeDerORG/DevKnowledge/blob/main/02-%E9%A1%B9%E7%9B%AE/EasyNetTopo/INDEX.md)
- [EasyNetTopo 长期背景（MEMORY.md）](https://github.com/CiBeDerORG/DevKnowledge/blob/main/02-%E9%A1%B9%E7%9B%AE/EasyNetTopo/MEMORY.md)
- [跨项目 AI 开发规范](https://github.com/CiBeDerORG/DevKnowledge/blob/main/01-%E5%B7%A5%E7%A8%8B%E8%A7%84%E8%8C%83/AI%E5%BC%80%E5%8F%91%E8%A7%84%E8%8C%83.md)

知识仓库与代码仓库分别维护，不使用 Git Submodule、符号链接或自动复制知识文件；访问私有知识库需要相应的 GitHub 权限。知识库的新变更在 PR 审核并合并后才会进入其 main 分支。

## AI Agent 入口

在本仓库开展研发前阅读 [AGENTS.md](AGENTS.md)，再按需读取 DevKnowledge 项目 INDEX / MEMORY。源码、部署和工程规则以本仓库当前实际文件为准。
