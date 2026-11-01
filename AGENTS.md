# EasyNetTopo — AI Agent 工作入口

本仓库是 **EasyNetTopo 项目源码与工程规则** 的权威来源；长期项目背景在独立知识中心 [CiBeDerORG/DevKnowledge](https://github.com/CiBeDerORG/DevKnowledge) 中维护。

## 开始任务前

1. 先核对本仓库的当前分支、实际文件与用户本次要求，不依据历史聊天或已清空仓库之前的状态臆测。
2. 当任务涉及产品定位、历史决策或需求时，按需读取：
   - [EasyNetTopo 项目索引](https://github.com/CiBeDerORG/DevKnowledge/blob/main/02-%E9%A1%B9%E7%9B%AE/EasyNetTopo/INDEX.md)
   - [EasyNetTopo 长期背景](https://github.com/CiBeDerORG/DevKnowledge/blob/main/02-%E9%A1%B9%E7%9B%AE/EasyNetTopo/MEMORY.md)
   - [知识中心共同约定](https://github.com/CiBeDerORG/DevKnowledge/blob/main/01-%E5%B7%A5%E7%A8%8B%E8%A7%84%E8%8C%83/AI%E5%BC%80%E5%8F%91%E8%A7%84%E8%8C%83.md)
3. DevKnowledge 为私有仓库，只有获得授权并能访问 GitHub 的工具才可读取；无法读取时报告限制，不猜测内容，也不要求降低访问权限。
4. 只加载与本次任务相关的知识，不递归读取整个 Vault；项目源码、接口、部署、验证与执行规则始终以本仓库当前分支及真实 CI 为准。

## 已确定的产品开发顺序

- **先建设独立 Web 平台和应用商店**，首个目标是让初次安装 EasyNetTopo 的用户在页面中安装并管理**官方原版 EasyTier**。
- **后研究 EasyTier 集成**，再实现网络节点、设备及服务信息管理。
- EasyHostAgent、EasyADB 为后续可选扩展，不是平台启动前置依赖。
- 未开发或未验收的功能不得写成已上线或通过测试。

## 知识沉淀与变更

- 已确认的稳定产品取舍、长期背景按需写入 DevKnowledge 的 EasyNetTopo 项目区域；正式工程设计、代码和规范仍在本仓库维护，知识库引用链接，不复制第二份规则。
- 遵循知识中心的知识录入规范：提炼、查重、核实、区分 `draft` / `accepted` / `superseded`，审查差异。没有明确授权时不要擅自跨仓库推送。
- **无自动跨仓库同步**；ChatGPT 网页端、Codex、OpenCode 等工具都需显式按需读取知识入口，不保证自动共享对话记忆。
- 不提交密码、Token、私钥或敏感日志；不引入与当前任务无关的脚本、服务或文档。
