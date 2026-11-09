# Git 应用源与应用商店

> 当前分支支持应用目录、固定版本候选包以及配对主机上的安装、更新、卸载。安装权限和 Agent 能力独立验证，目录存在不等于可执行授权。

## 通用配置与图标 v2

维护者直接编辑来源仓库的 `<应用>/app.yaml`、`config.yaml`、`secrets.yaml`、`assets/icon.svg`，不提供定义管理页面。安装页只渲染预定义表单：`custom_input:false` 隐藏，开启输入且 `editable:false` 时显示只读；默认可编辑。隐藏、只读和未声明字段在 HTTP 请求和 Agent 两端拒绝覆盖。

- `GET /api/v1/catalog/apps/{source}/{app}/config` 返回固定 Commit 的归一化字段和 `revision`，不读取主机真实密钥。
- 应用列表增加 `icon_url`，指向需要管理员会话的图标接口，URL 包含来源、应用和 Commit；图标通过固定 Git 对象读取并限制格式、大小与 SVG 内容，浏览器使用图片元素展示，失败时回退统一文字占位。
- v2 配置定义和生成模板必须进入原有逐文件校验的安装包；脚本与表单版本一致。ZIP 格式仍为 v1。
- 传包前要求目标 `/v1/identity` 的 `app_manifest_versions` 包含 2；旧 Agent 不会静默接收新配置契约。v1 来源仍按原规则读取。
- 新字段和新应用无需修改前端或 Agent 的应用名分支。字段类型和默认值以仓库契约为准，`number` 使用整数语义。

验证：`DEPLOY_CONTRACT_ROOT=<Deploy checkout> go test ./internal/catalog ./internal/hosts`；前端执行 `npm --prefix web test`、`npm --prefix web run build`。本地 Git 固定快照测试覆盖未提交修改不能改变安装包，真实设备安装须单独验收。

### 清单驱动的单选与条件字段

v2 的 `config.yaml` / `secrets.yaml` 字段定义支持两项通用展示元数据，EasyNetTopo 不按应用 ID 或字段键编写专用分支：

```yaml
fields:
  - key: APP_ROLE
    label: 运行角色
    type: string
    required: true
    default: cloud
    custom_input: true
    editable: true
    description: 选择当前运行角色
    options:
      - { label: 云端配置中心, value: cloud }
      - { label: 普通节点, value: intranet }

  - key: APP_WEB_URL
    label: Web API 地址
    type: string
    required: true
    default: ""
    custom_input: true
    editable: true
    description: 仅云端角色需要
    visible_when:
      field: APP_ROLE
      equals: cloud

  - key: APP_CONFIG_SERVER
    label: 远端配置中心
    type: string
    required: true
    default: ""
    custom_input: true
    editable: true
    description: 仅普通节点需要
    visible_when:
      field: APP_ROLE
      equals: intranet
```

- `options`：字符串字段的单选项；`label` 仅用于展示，`value` 是提交给安装脚本的值。默认值必须属于已声明选项。
- `visible_when`：只有指定单选字段当前值匹配 `equals` 时才显示该字段。条件依赖必须引用已声明、无条件显示的字符串单选字段及其合法选项值。
- 页面会为所有可输入字段初始化默认值，但仅验证、展示并提交当前可见字段；切换选项后隐藏字段不会随安装请求提交。
- 服务端会再次按同一清单规则拒绝隐藏、只读、未声明字段及非法选项值，不能依赖浏览器校验提供安全边界。
- 普通字段不声明 `options` / `visible_when` 时维持原有文本、数字、布尔和密钥表单行为。
## 1. 实际入口

管理员登录后通过 **Git 应用源** 输入来源名称、Git 地址和分支。添加操作仅登记 SQLite 元数据；点击“手动同步”时才由 Go 后端启动 Git CLI，不自动后台同步。

API：

- `GET /api/v1/catalog/sources`：列出已登记来源；
- `POST /api/v1/catalog/sources`：新增来源，JSON `{"name":"Deploy","url":"git@github.com:CiBeDerORG/deploy.git","branch":"main"}`；
- `POST /api/v1/catalog/sources/{source_id}/refresh`：手动刷新指定来源并固定本次 Git Commit；
- `GET /api/v1/catalog/apps`：列出成功快照中的应用及图标地址；安装能力由服务配置、主机状态及权限决定。

上述接口全部要求管理员会话；写接口要求同源专用请求头及 Origin 检查。浏览器不保存 Git 令牌。

## 2. 私有 Deploy 的访问方式

当前只支持控制端运行环境**预先配置好的 Git SSH 身份**：

```text
git@github.com:CiBeDerORG/deploy.git
```

- 在运行 EasyNetTopo 的同一系统用户环境准备已获该仓库**读取授权**的 GitHub SSH 身份（优先只读 Deploy Key 或受约束的机器身份）。
- SSH `known_hosts` 应通过可信渠道核验并预先配置。代码使用 `BatchMode=yes`、`StrictHostKeyChecking=yes`，不会自动信任未知主机指纹，也不会提示在 Web 输入私钥。
- 默认仅允许 `github.com`。可由运行环境管理员配置 `EASYNETTOPO_GIT_ALLOWED_HOSTS` 精确允许域名或 `host:port`；例如 `github.com,gitea.example.org:3000,192.168.10.25:3000`。端口不自动通配，内网 IP 必须明确列入白名单，特殊/回环/链路本地地址始终拒绝。
- `https://` 及 `git@host:owner/repo.git` 维持原有支持。`http://`（明文 Git HTTP）**必须额外配置** `EASYNETTOPO_GIT_ALLOW_HTTP=true` 且显式设置 `EASYNETTOPO_GIT_ALLOWED_HOSTS`；两者任一缺失均拒绝。适合完全受控的自建 Git HTTP 服务，不建议跨不可信网络使用。
- 示例：`http://gitea.example.org:3000/team/app.git` 或 `http://192.168.10.25:3000/team/app.git`，路径仍要求 `owner/repo[.git]`，尚不支持多级子组路径。
- 所有 URL 均拒绝内嵌账号/密码、查询参数、片段、编码路径；Git HTTP/HTTPS 强制关闭远端重定向和凭据助手，防止跳转到未批准目标或自动发送现有凭据。请勿将私有 Git 仓库认证信息嵌入明文 HTTP URL。
- 不支持把 `https://user:token@host/repo`、`http://user:password@host/repo`、`file://`、`ssh://` 等未授权 Git 协议提交给平台。暂未设计带密钥引用的 HTTPS 私有仓库授权管理；HTTP 也不提供自动认证。
- **云端 GitHub 连接器与部署机器 SSH 授权是两套身份。** ChatGPT 能读取私有仓库，不意味着 NAS 上的 EasyNetTopo 自动拥有对应 Git 凭据；本仓库不托管私钥。

Git 同步失败仅记录安全的通用错误，并**保留上次成功的 SQLite 商店快照**。当前刷新为用户主动发起的限时请求，不支持自动定时刷新或队列。

## 3. 固定版本和数据约束

- 控制端 Git 仓库缓存位于数据库所在目录的 `git-cache/<source_id>.git`，采用裸仓库，只读取 Git 对象和树，不 checkout 仓库代码或运行仓库脚本。
- 查询完成后使用固定 Git Commit 保存此次只读快照，保留对应 Git Ref；不通过移动的 `main` 代表安装版本。
- 应用唯一身份是 `source_id + app_id`。目录切换由 SQLite 单事务完成，展示的来源版本及应用记录必须一致。
- 快照清单只包含展示字段、来源 Commit 和 Manifest SHA-256，不含真实用户部署密钥。
- 目录预览检查结构关键项、文件存在和安全路径；安装前另行执行完整的 Deploy Schema、配置定义和跨文件语义校验。通过校验不代表发布方身份认证。

## 4. 当前验收与限制

- 可验证已登录操作、同源写请求、非法仓库 URL 拒绝、来源重复、静态 Manifest 约束、失败快照不覆盖旧数据。
- CI 没有注入任何 GitHub SSH 私钥，因此不会从 CI 真实拉取私有 Deploy。实际私有源联调需在具有只读授权的运行主机进行。
- 安装、更新和卸载由配对 Agent 执行受控脚本；本地契约测试不代替目标主机的 Docker 生命周期验收。
- 尚未实现来源删除、更改凭据、远程 HTTPS Token Vault；后续按实际用户需求单独评审。
