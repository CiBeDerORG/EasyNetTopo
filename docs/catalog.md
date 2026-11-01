# M1-03 — Git 应用源与只读商店

> 当前功能仅支持**只读应用目录预览**。尚未实现安装包发行、安全信任批准、EasyHostAgent 部署任务，不能因为商店有条目就启用安装。

## 1. 实际入口

管理员登录后通过 **Git 应用源** 输入来源名称、Git 地址和分支。添加操作仅登记 SQLite 元数据；点击“手动同步”时才由 Go 后端启动 Git CLI，不自动后台同步。

API：

- `GET /api/v1/catalog/sources`：列出已登记来源；
- `POST /api/v1/catalog/sources`：新增来源，JSON `{"name":"Deploy","url":"git@github.com:CiBeDerORG/deploy.git","branch":"main"}`；
- `POST /api/v1/catalog/sources/{source_id}/refresh`：手动刷新指定来源并固定本次 Git Commit；
- `GET /api/v1/catalog/apps`：列出成功快照中的应用，只读响应 `install_enabled=false`。

上述接口全部要求管理员会话；写接口要求同源专用请求头及 Origin 检查。浏览器不保存 Git 令牌。

## 2. 私有 Deploy 的访问方式

当前只支持控制端运行环境**预先配置好的 Git SSH 身份**：

```text
git@github.com:CiBeDerORG/deploy.git
```

- 在运行 EasyNetTopo 的同一系统用户环境准备已获该仓库**读取授权**的 GitHub SSH 身份（优先只读 Deploy Key 或受约束的机器身份）。
- SSH `known_hosts` 应通过可信渠道核验并预先配置。代码使用 `BatchMode=yes`、`StrictHostKeyChecking=yes`，不会自动信任未知主机指纹，也不会提示在 Web 输入私钥。
- 默认 Git 域名白名单为 `github.com`。要启用其他明确可信的代码托管域名，可由运行环境管理员设置 `EASYNETTOPO_GIT_ALLOWED_HOSTS=github.com,gitlab.com`。该配置不接受浏览器自行更改。
- 不支持把 `https://user:token@host/repo`、`file://`、`ssh://`、任意自定义 Git 协议提交给平台。暂未设计带密钥引用的 HTTPS 私有仓库授权管理。
- **云端 GitHub 连接器与部署机器 SSH 授权是两套身份。** ChatGPT 能读取私有仓库，不意味着 NAS 上的 EasyNetTopo 自动拥有对应 Git 凭据；本仓库不托管私钥。

Git 同步失败仅记录安全的通用错误，并**保留上次成功的 SQLite 商店快照**。当前刷新为用户主动发起的限时请求，不支持自动定时刷新或队列。

## 3. 固定版本和数据约束

- 控制端 Git 仓库缓存位于数据库所在目录的 `git-cache/<source_id>.git`，采用裸仓库，只读取 Git 对象和树，不 checkout 仓库代码或运行仓库脚本。
- 查询完成后使用固定 Git Commit 保存此次只读快照，保留对应 Git Ref；不通过移动的 `main` 代表安装版本。
- 应用唯一身份是 `source_id + app_id`。目录切换由 SQLite 单事务完成，展示的来源版本及应用记录必须一致。
- 快照清单只包含展示字段、来源 Commit 和 Manifest SHA-256，不含真实用户部署密钥。
- 通过初步 Manifest 校验后展示的应用**并不等同完整通过 Deploy Schema/语义校验**；本阶段的检查仅覆盖结构关键项、文件存在和安全路径，不支持发行包或信任判断。

## 4. 当前验收与限制

- 可验证已登录操作、同源写请求、非法仓库 URL 拒绝、来源重复、静态 Manifest 约束、失败快照不覆盖旧数据。
- CI 没有注入任何 GitHub SSH 私钥，因此不会从 CI 真实拉取私有 Deploy。实际私有源联调需在具有只读授权的运行主机进行。
- 本阶段没有 Agent，不能执行 Docker、Shell 或完成 Sub2API 远程安装。
- 尚未实现来源删除、更改凭据、远程 HTTPS Token Vault；后续按实际用户需求单独评审。
