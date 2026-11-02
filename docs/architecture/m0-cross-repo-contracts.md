# M0 — EasyNetTopo × EasyHostAgent × Deploy 协议对齐清单

> 状态：**M0 评审草案，不代表 Agent OpenAPI 的未实现接口已可用，不代表已通过真机验收。**
> 权威现状：
> - EasyHostAgent: https://github.com/CiBeDerORG/EasyHostAgent/blob/main/docs/api/openapi-v1.json
> - Deploy manifest: https://github.com/CiBeDerORG/deploy/blob/main/schema/app-manifest-v1.schema.json
> - 首例: https://github.com/CiBeDerORG/deploy/blob/main/sub2api/app.yaml

## 1. 项目边界

| 项目 | 负责 | 不负责 |
|---|---|---|
| EasyNetTopo | 用户身份、Git 源、目录快照、主机绑定、操作请求、任务索引、前端 | 运行远程脚本、控制 Docker Engine |
| EasyHostAgent | 宿主机身份/TLS、一次性配对、授权、执行包缓存、本地任务、Docker 真相、日志 | 访问 GitHub、保存 Git Token、运行通用代码插件 |
| Deploy | 版本化 app.yaml/Compose/经批准脚本、schema 和验证器 | 管理平台/主机、用户权限、后台服务 |

## 2. 关键标识与协议兼容

| 字段 | 生成/权威来源 | 消费方 | 约束 |
|---|---|---|---|
| controller_id | EasyNetTopo 首次初始化稳定 UUID | Agent 配对及请求头 | 与登录用户、主机 ID 无关 |
| host_id | EasyNetTopo 稳定 UUID | Agent 现有 device_id 字段 | UUID，明确从“EasyTier device”重定义为“控制端 host” |
| agent_id | Agent 本机持久身份 | 控制端 | 与配对 Token、SPKI 绑定 |
| source_id | EasyNetTopo 创建源时签发 | 目录、任务、审计 | **不代表安全可信**；同名 app 用 source_id 区分 |
| app_id | app.yaml.id | 目录/任务 | 同源唯一，必须与路径一致 |
| catalog_revision | 控制端读取的固定 Git commit | 部署包/任务 | 不使用移动的 branch/latest 当安装身份 |
| bundle_sha256 | 实际发送字节的 SHA-256 | Agent | 不可变，不等于 Git commit |
| task_id | 控制端 UUID | Agent 本地任务 | 相同 ID + 相同请求摘要幂等；不同摘要 409 |
| runtime_key | Agent 实际 Docker project/resource ID | 安装归属 | 相同主机实际资源不得被不同来源重复占用 |

现有 Agent PairingRequest 的 device_id 是 UUID 字段；M0 **优先保留字段名进行非破坏性语义修订**，并更新文档和集成测试。不得擅自修改旧历史数据或认为它就是 EasyNetTopo 现有设备模块（当前没有）。

现有 TaskStart **没有 source_id 字段**，也没有明确多 Git 来源语义；Agent 当前的 OpenAPI 是草案且未全部实现。建议新增 source_id 为应用操作字段并同步以下：
- EasyHostAgent OpenAPI Schema + TaskStart/Task/TaskResult 的审计表达；
- 控制端请求与持久化；
- Agent bundle 审核与来源绑定；
- 不兼容旧 Agent 时明确升级顺序或能力版本检查，**不做去掉字段后静默重试**。

## 3. 沿用 Agent v1 的接口（代码实现状态仍需分开验收）

| 方法 | 路径 | 用途 |
|---|---|---|
| GET | /v1/health、/v1/identity | 可达性、身份发现 |
| POST | /v1/pairings | 使用本机一次性码配对 |
| GET/DELETE | /v1/pairings/current | 查看/撤销当前控制端授权 |
| GET | /v1/host | 宿主机能力快照 |
| PATCH | /v1/host/settings | 软件根目录设置（严格限制已有受管资源变更） |
| GET | /v1/docker/inventory | 完整/不完整 Docker 资源清单 |
| GET | /v1/docker/resources/{resource_id}/logs | 按需查受限日志 |
| HEAD/PUT | /v1/bundles/{sha256} | 包检查/限量上传（Agent **待实现**） |
| POST/GET | /v1/tasks | 创建/列举持久化任务（Agent **待实现**） |
| GET | /v1/tasks/{task_id} | 查询任务（Agent **待实现**） |
| GET | /v1/tasks/{task_id}/logs | 增量日志（Agent **待实现**） |
| POST | /v1/tasks/{task_id}/cancel | 安全点取消（Agent **待实现**） |

已实现的 Docker 内部动作函数 != 已经暴露的持久化任务路由。任务 HTTP 202 表示已持久接受，不能代表安装成功。

## 4. Deploy app.yaml 的 M0 约定

- 直接沿用 Deploy JSON Schema v1 + 语义验证，Sub2API 是首个正式模板；无合法 manifest 的其他应用目录不强行导入商店。
- package.files 的 source 从 Git 仓库根目录计算，destination 从目标 SOFTWARE_ROOT 计算；不复制整个仓库。共享 lib/image-sources.sh 映射到 deploy/lib/image-sources.sh。
- 安装 bundle 包含经过验证的 app.yaml、来源 commit 证明、文件内容摘要和白名单资源；环境配置及真实密钥通过单独任务参数传递，绝不打包。
- 压缩格式/元信息文件路径/规范化归档策略，需在 M0 契约测试中固定（包括稳定排序、拒绝符号链接、路径穿越、特殊文件、越权覆盖及 64 MiB 上限），**当前不能冒称已有统一实现**。
- 旧 Deploy 的 `sub2api/deploy/install.sh`、`update.sh` 应继续保持原有 CLI 行为；新增平台适配时不得删除 `deploy.sh env` 的原有能力。
- Sub2API 所需配置文件：`<SOFTWARE_ROOT>/deploy/.env` 和 `.secrets`、共享库 `deploy/lib/image-sources.sh`；Agent 必须原子初始化/修改，锁保护，复用已有值，生成型 secret 不得覆盖。
- PostgreSQL、Redis 容器不对外公开端口，Web 端口默认 7010；健康验证是 sub2api + postgres + redis 均 healthy，不能仅依据 Shell exit 0。
- Sub2API 使用可变的 latest 镜像 tag，Git commit 固定只能保证源文件版本，不能保证镜像内容；需在任务结果记录实际镜像 digest，或后续采取固定 digest 发布。

## 5. 高权限执行信任边界 —— 未解决前禁止开放脚本执行

1. **source_id、official 字段、用户提供的 app.yaml 都不是信任证明**；不能由调用者声明“可信”来提升权限。
2. Agent 是高权限本地进程，脚本、Compose bind mount、privileged、network host、docker.sock、build 等都可能突破宿主隔离。
3. 本期官方 Sub2API 可以使用经过批准的安装脚本，但**需要选定并验证 Agent 独立可判别的执行授权方式**（例如本机受控允许清单 + 校验包摘要/发布签名）；仅 Controller 自称官方来源不足以形成对不可信源的防护。
4. 第三方 Compose 仅接受经过严格校验的受控操作；不能仅因为不叫 Shell 就认定安全。对用户可编辑 Compose 与危险特权能力需单独授权范围和测试。
5. **M0 安全阻塞项**：确定脚本执行授权校验、可信根维护方式、包更新后的权限延续策略及兼容升级顺序；完成前不要把远程脚本安装当成已经可交付的功能。

## 6. 目录、端口与状态冲突

- Linux：Agent 当前默认 /opt/software，Deploy 当前默认 /software；FnOS 双方为 /vol3/1000/software。新增安装流程应检查并使用 Agent 的真实 SOFTWARE_ROOT；如需调整新默认值，保留已安装主机配置，不自动迁移。
- Agent 当前默认 TCP 7014 与 Deploy Webtop 默认 HTTPS 7014 冲突。安装向导先探测冲突并允许指定端口；是否变更 Agent 的新安装默认值应作为单独兼容决策。
- Agent 当前配对 Store 只有一个持久 grant，已经能表达唯一控制端；成功重配即撤销旧 grant。
- Docker `GET /v1/docker/inventory` 中资源 managed 当前为 false 的情况不能当作受管记录已存在；需要任务执行后建立 Agent 侧受管归属，再和实际容器核对。
- inventory.complete=false、离线、容器 stopped 或 Docker 权限不足不代表已经卸载。

## 7. M0 跨仓库交付分解

| ID | 仓库 | 产出 | 验收 |
|---|---|---|---|
| M0-01 | EasyNetTopo | 平台核心规格、稳定身份/来源/实例约束、适配器接口及最小 HTTP API 草案 | 无具体业务模型污染核心；所选技术边界一致 |
| M0-02 | EasyHostAgent | 独立控制端语义、source_id 扩展、任务及 bundle 版本/能力对齐 | Go 契约测试证明重新配对与旧版兼容行为；不虚报实现 |
| M0-03 | Deploy | Sub2API 现有 app.yaml/脚本/文件映射的契约样本 | Schema + 语义校验/Compose 静态测试通过；不补齐其他应用 |
| M0-04 | 三仓库集成 | 固定 commit → 固定 bundle → 安装请求的 JSON/归档样本及负例 | 三端字段一致，拒绝恶意路径、重放冲突、来源伪造 |
| M0-05 | 三仓库安全评审 | 官方脚本信任策略、第三方 Compose 安全边界 | 没有模糊“受信任 source_id 即可执行”的高权限漏洞 |

M0 完成不代表 V1 端到端已可运行。后续 M1 才进入项目初始化、Git 商店与主机绑定联调，M2+ 进入 Agent 任务实现和 Sub2API 真机安装。

## 8. 暂不修改的协议

- 不增加独立 Git 应用商店服务。
- 不扩展允许任意命令的 Agent API。
- 不为 EasyTier 业务管理预设共享 Rust crate、专属数据库表或主机安装前提。
- 不启动定期任务、离线排队或自动回滚。
