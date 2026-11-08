# 应用关联与自动更新：通用 Hook 使用手册

> 适用：EasyNetTopo 控制端、EasyHostAgent、deploy，以及 GitHub Actions 或其他能发送 HTTP 请求的 CI/CD 系统。  
> 当前实现依据：EasyNetTopo `main` 的 `POST /api/hooks/deploy/update`、现有异步任务调度器和 deploy 原有更新入口。  
> **注意：Hook 用于更新已安装且可管理的应用，不负责首次安装、任意脚本执行或直接下发镜像版本。**

## 1. 整体机制与“应用关联”

```text
应用代码仓库构建并成功发布镜像/程序
  └─ CI/CD 读取该仓库的 Token + 目标列表
       └─ POST EasyNetTopo /api/hooks/deploy/update
            ├─ 按 host_name 找到已登记、已配对的主机
            ├─ 按 service_name 找到已导入 Git 应用源的应用
            ├─ 创建主机任务（HT）和应用更新任务（AT）；立即返回 202
            └─ EasyNetTopo 调度已有 EasyHostAgent 应用更新接口
                 └─ Agent 校验受管实例与应用清单，调用对应 deploy 更新机制
                      └─ 更新成功或失败后，结果写回任务状态
```

**关联只需要两个字段：**

| 字段 | 写什么 | 从哪里找 |
| --- | --- | --- |
| `host_name` | 要更新的目标主机名称；大小写及空格必须完全一致 | EasyNetTopo 的主机管理列表 |
| `service_name` | 应用 ID（优先）或唯一的应用展示名称 | EasyNetTopo 应用市场对应 Git 应用源；推荐使用 `<应用目录>/app.yaml` 的 `id` |

这不是“GitHub 仓库名自动等于容器名”：`service_name` 必须在 **EasyNetTopo 已导入的应用目录**中唯一解析到一个应用。使用 `app.yaml` 的 `id` 最稳定；如果不同来源具有相同 `app_id`，当前 Hook **没有** `source_id` 消歧参数，会返回 422，需先解决目录中的重名问题。

**不需要**在请求中填写主机 IP、Agent 端口、实例 ID、Docker 容器 ID、镜像 URL、版本号或脚本。更新版本由 deploy 现有版本配置及各应用原生更新流程决定（A 方案）。

## 2. 前提检查

1. EasyNetTopo 控制端已部署并可从 CI/CD 运行环境访问；`POST /api/hooks/deploy/update` 是它的 HTTP API，不是 Agent 的接口。
2. 目标主机已在 EasyNetTopo 中登记并成功配对 EasyHostAgent，且目标主机当前能被控制端访问。
3. 相应 Git 应用源已在 EasyNetTopo 中添加并刷新，应用目录存在有效的 `app.yaml`，且目标服务名称能够唯一匹配。
4. 目标应用已安装、符合 Agent 的受管身份校验，具有兼容当前操作系统/架构的清单及更新能力；Hook **不会绕过** Agent 的安全/权限检查。
5. 目标应用的镜像/程序已由 CI/CD 成功发布，并且 deploy 当前配置的更新版本确实指向预期的新发布物。Hook 不根据本次 CI 提交号自行改写 deploy 版本配置。

**macOS 特别说明：** macOS 使用普通登录用户的 Docker Desktop 和 `bash deploy.sh ... update`，不使用 `sudo bash deploy.sh`。EasyNetTopo 本身在 macOS 上的自动更新依赖新版本 EasyHostAgent 对现有双 Compose 文件及部署目录的受控核验，并沿用 macOS deploy 更新入口；仅改应用清单中的 `platforms` 不等于完成平台适配。其他应用是否能更新仍以其自身清单与 Agent 检查为准。

## 3. 在 EasyNetTopo 服务端配置 Hook Token（一次）

Hook 通过 `EASYNETTOPO_HOOK_TOKEN` 验证调用者。自行生成一个高强度随机字符串，**至少 32 个字符**；推荐：

```bash
openssl rand -hex 32
```

此命令产生 64 位十六进制字符串。将结果**只**保存到 EasyNetTopo 所在主机的：

```text
<SOFTWARE_ROOT>/deploy/.secrets
```

添加或修改原有配置行（不要重复同名键）：

```dotenv
EASYNETTOPO_HOOK_TOKEN=<你自己生成的64位随机值>
```

常见路径与运行方式：

| 系统 | 配置文件示例 | 部署方式 |
| --- | --- | --- |
| macOS | `/opt/software/deploy/.secrets` | 当前登录用户、`0600`，**不用 sudo** |
| Linux（默认安装根目录） | `/software/deploy/.secrets` | 原部署要求 `root:root`、`0600`，按既有 sudo 部署流程 |
| fnOS | `<fnOS 实际软件根目录>/deploy/.secrets` | 按 fnOS 原有软件根目录及 sudo 部署流程 |
| 自定义根目录 | `<SOFTWARE_ROOT>/deploy/.secrets` | 使用你实际配置的根目录，不能直接照抄别人的路径 |

修改后**必须重建或更新 EasyNetTopo 容器**，使环境变量实际注入；只编辑文件不会改变正在运行的容器。

macOS（以 `/opt/software` 为例）：

```bash
cd /opt/software/deploy
bash deploy.sh env
bash deploy.sh easynettopo update
```

Linux/fnOS（从实际 deploy 仓库目录执行，按原安装参数及 sudo 流程）：

```bash
sudo bash deploy.sh env
sudo bash deploy.sh easynettopo update
```

Linux 如果原安装使用了自定义 `SOFTWARE_ROOT`，必须按该部署的既有方式显式传入同一根目录；**不要因为文档示例擅自迁移目录或修改权限**。

检查**长度**，不要把 Token 本文复制到工单或 CI 日志：

```bash
docker exec easynettopo sh -c 'echo "容器内 Token 长度: ${#EASYNETTOPO_HOOK_TOKEN}"'
```

预期值至少为 32；使用 `openssl rand -hex 32` 时应为 64。若容器内长度为 0，先检查实际使用的 `.secrets` 路径、配置是否加载、容器是否已重新创建。

## 4. 在**发起构建的 GitHub 仓库**中关联目标应用

设置的位置是**发送 Hook 的那一个仓库**，而不一定是 EasyNetTopo 仓库。假设应用 A 的代码和构建工作流位于 GitHub 仓库 A，则在仓库 A 设置 Token 与目标列表；应用 B 的构建在仓库 B，则在仓库 B 独立设置。

进入该仓库：

```text
Settings → Secrets and variables → Actions
```

### 4.1 Secrets（敏感信息）

选择 **Secrets → New repository secret**：

- Name：`EASYNETTOPO_HOOK_TOKEN`
- Secret：与 EasyNetTopo 容器对应的 `<SOFTWARE_ROOT>/deploy/.secrets` 中**完全相同**的 Token

密钥不能放在 Variables 或 Git 仓库文件里，也不要把它打印到 Actions 日志。

### 4.2 Variables（目标关联配置）

选择 **Variables → New repository variable**：

- Name：`EASYNETTOPO_HOOK_TARGETS_JSON`
- Value：下面任一示例的 **JSON 数组本身**，不要在外面再包一个 `targets` 对象。

**示例 A：一个仓库构建成功，更新一台主机上的一个应用**

```json
[
  {
    "host_name": "Mac mini m6",
    "service_name": "easynettopo"
  }
]
```

**示例 B：相同应用发布后，更新多台主机**

```json
[
  {"host_name": "Mac mini m6", "service_name": "easynettopo"},
  {"host_name": "Linux-01", "service_name": "easynettopo"}
]
```

**示例 C：同一次发布涉及多台主机和不同应用**

```json
[
  {"host_name": "Linux-01", "service_name": "app-a"},
  {"host_name": "Linux-01", "service_name": "app-b"},
  {"host_name": "Mac mini m6", "service_name": "app-a"}
]
```

以上主机名和应用 ID 都是示例，需替换成控制端真实存在的对象；没有安装的应用不能因为列在 JSON 中就自动安装。

关联关系的实际效果：

| 触发方 | `EASYNETTOPO_HOOK_TARGETS_JSON` 指向 | 结果 |
| --- | --- | --- |
| 应用 A 的 GitHub 仓库 | 主机 1 的 `app-a` | A 发布成功后只更新这一目标 |
| 应用 A 的 GitHub 仓库 | 主机 1、主机 2 的 `app-a` | A 发布成功后同时提交两台主机的更新 |
| 应用 B 的 GitHub 仓库 | 主机 2 的 `app-b` | B 发布成功后独立触发 B 的更新 |
| 一次构建同时发布 A、B | 多个主机及 `app-a`、`app-b` | 一次 Hook 批量提交更新任务 |

一个 Hook 最少 1 个、最多 64 个目标；相同主机与应用在**同一次请求**里重复出现只生成一次应用任务。**不同 Hook 请求之间没有幂等去重保证**，不要把失败重试当作“绝不会重复更新”。

### 4.3 Hook URL

本项目当前 EasyNetTopo `main` 工作流直接使用：

```text
http://49.235.163.195:7017/api/hooks/deploy/update
```

这是当前项目专用地址，**不是**通用默认地址；迁移到其他控制端时要改为其真实可达入口。该地址当前为 HTTP，Bearer Token 将明文经过网络；公网或长期使用请优先配置 HTTPS 入口与网络访问限制。

为其他仓库复制通用工作流时，推荐额外增加一个普通 Variable：

```text
EASYNETTOPO_HOOK_URL=https://你的受保护域名/api/hooks/deploy/update
```

当前 EasyNetTopo 自带工作流并**未**从这个 URL Variable 读取地址；它继续使用文件里固定的 `HOOK_URL`。只有采用下节通用模板的仓库才需配置这个新 Variable。

## 5. GitHub Actions 构建成功后发送 Hook

### 5.1 EasyNetTopo 自身

`EasyNetTopo/.github/workflows/container-image.yml` 已带有 `Trigger EasyNetTopo deploy Hook` 步骤：

- 仅 `main` 分支且镜像构建**成功发布后**才尝试请求；
- 如果 `EASYNETTOPO_HOOK_TARGETS_JSON` 未填写，则**跳过** Hook；
- 未配置 Token、JSON 不合法或 HTTP 请求失败时，该步骤失败；
- 有效请求返回 HTTP 202，表示任务**已受理**，不是应用已更新完成。

默认无需再复制一遍 Hook 步骤，避免同一个构建重复发送。

### 5.2 其他应用的 GitHub 仓库（通用模板）

在它自己的 Actions workflow 中，把以下步骤放到**真正完成镜像/程序发布的 job 之后**。如果在同一个 job 中编译和发布，直接放在发布步骤后；若是不同 job，使用 `needs` 依赖发布 job。

```yaml
      # 上面的步骤已经完成镜像/程序发布
      - name: Notify EasyNetTopo to update applications
        if: success() && github.ref == 'refs/heads/main'
        shell: bash
        env:
          HOOK_URL: ${{ vars.EASYNETTOPO_HOOK_URL }}
          HOOK_TOKEN: ${{ secrets.EASYNETTOPO_HOOK_TOKEN }}
          HOOK_TARGETS_JSON: ${{ vars.EASYNETTOPO_HOOK_TARGETS_JSON }}
        run: |
          set -euo pipefail
          if [[ -z "${HOOK_TARGETS_JSON:-}" ]]; then
            echo "::notice::No Hook targets configured; skip."
            exit 0
          fi
          [[ -n "${HOOK_URL:-}" ]] || { echo "::error::Missing Hook URL"; exit 1; }
          [[ -n "${HOOK_TOKEN:-}" ]] || { echo "::error::Missing Hook token"; exit 1; }
          jq -e 'type == "array" and length >= 1 and length <= 64' <<< "$HOOK_TARGETS_JSON" >/dev/null
          payload="$(jq -cn --argjson targets "$HOOK_TARGETS_JSON" \
            '{trigger_source:"github_actions",targets:$targets}')"
          curl --fail-with-body --silent --show-error --max-time 30 \
            -X POST "$HOOK_URL" \
            -H "Authorization: Bearer $HOOK_TOKEN" \
            -H "Content-Type: application/json" \
            --data-binary "$payload"
```

这个模板面向安装了 `jq`、较新 `curl` 的 Linux runner（GitHub 提供的 Ubuntu runner 通常符合）；其他 runner 按其工具环境调整。这里刻意**不设置自动重发**，因为服务端接受请求但响应丢失时，重试可能产生另一批更新任务。建议在 Actions 查看 HTTP 返回和 EasyNetTopo 任务状态后再判断是否需要人工重发。

`github.ref == 'refs/heads/main'` 只是模板的发布分支示例；若应用项目使用不同的正式发布分支，应按该项目自己的发布约定调整，避免测试分支触发生产更新。

## 6. 非 GitHub Actions 如何接入

只要能够向 EasyNetTopo 发送 HTTPS POST，GitLab CI、Jenkins、其他构建平台或内部服务也可调用同一个接口。将 Token 保存在各平台的安全凭据管理中，将目标关联数组作为配置管理。

请求协议：

```http
POST /api/hooks/deploy/update
Authorization: Bearer <EASYNETTOPO_HOOK_TOKEN>
Content-Type: application/json
```

```json
{
  "trigger_source": "ci_pipeline",
  "targets": [
    {"host_name": "Linux-01", "service_name": "app-a"},
    {"host_name": "Linux-02", "service_name": "app-a"}
  ]
}
```

`trigger_source` 是顶层必填字段：使用小写字母、数字、下划线；以小写字母开头，总长度 2–40 字符（如 `github_actions`、`gitlab_ci`、`jenkins`）。它会写入任务状态并显示为触发来源。GitHub 的现成模板固定填写 `github_actions`，不需要把该字段写进 `EASYNETTOPO_HOOK_TARGETS_JSON`。

请求 JSON 只允许已声明的字段，大小不超过 32 KiB；目标数组 1–64 项。目标有不存在的主机、重复主机名称、未配对主机、未知/重复应用名称时，整个请求返回 422，**不会部分受理**。

## 7. 成功判断、任务状态与故障排查

### 7.1 接口返回什么

服务端成功接收时返回 **HTTP 202**：

```json
{
  "accepted": true,
  "tasks": [
    {
      "id": "HT-<生成的任务标识>",
      "type": "host",
      "state": "queued",
      "trigger_source": "github_actions"
    },
    {
      "id": "AT-<生成的任务标识>",
      "type": "application",
      "state": "queued",
      "trigger_source": "github_actions"
    }
  ]
}
```

上面省略了响应的其他字段，仅用于说明任务类型与状态；真实响应的 `id` 由服务端生成。

HT 是主机级任务，AT 是应用级任务。同一台主机的应用更新串行调度，不同主机可以并发；更新失败不会把 HTTP 202 改成失败响应，因为 HTTP 请求早已结束。

在 EasyNetTopo 页面查看主机/应用任务状态，或管理员登录后查询 `GET /api/v1/tasks`（该接口需要**登录会话**，不能只拿 Hook Bearer Token 调用）：

- `queued`：排队中
- `preparing`：准备并传输已验证的应用包
- `running`：已下发更新，等待 Agent 结果
- `succeeded`：实际更新完成
- `failed`：更新失败，检查任务详情和 Agent/部署环境
- `interrupted`：执行状态不能安全确认，需人工核实后按页面提示处理

**只有 AT/HT 最终成功，才算对应目标更新成功。** GitHub Workflow 绿色仅证明镜像发布及 Hook HTTP 步骤已执行成功；若目标列表为空，甚至可能仅仅是跳过了 Hook。

### 7.2 常见问题

| 现象 / HTTP 响应 | 原因与处理 |
| --- | --- |
| Actions 里提示 `EASYNETTOPO_HOOK_TARGETS_JSON is empty` | 对应**构建仓库**未配置 Variable；本次跳过，不会创建任务 |
| `503 hook_not_configured` | EasyNetTopo **运行中的容器**没读到有效 Token。检查正确的 `.secrets` 路径、值长度和容器是否更新；不要只检查磁盘文件 |
| `401 unauthorized` | 构建仓库 Secret 与服务端 Token 不一致；也检查 `Bearer ` 前缀 |
| `415` | `Content-Type` 应为 `application/json` |
| `400` | 请求字段、`trigger_source` 格式、目标数量或 JSON 不合法 |
| `422 invalid_targets` | `host_name` 与实际主机名不一致、主机未配对、`service_name` 未注册或在应用源中重名 |
| CI 步骤成功但没有新任务 | 检查是否因空目标列表跳过；或是否查看了错误的控制端 |
| 任务显示“Agent 拒绝应用更新请求” | Hook 已创建任务，但 Agent 未接受更新。检查目标应用是否为受管安装、来源是否一致、平台/架构是否兼容、是否具备更新脚本；该界面消息当前可能未显示 Agent 的具体业务拒绝原因 |
| macOS 上 EasyNetTopo 更新失败 | 检查 deploy 和 EasyHostAgent 是否已更新到支持 macOS 受控更新的版本，以及其 Docker Compose 工作目录、双 Compose 文件及配置是否符合原部署身份 |
| 任务长时间 `running` 或 `interrupted` | 先核实目标主机是否仍在更新，禁止盲目重复下发；必要时在 EasyNetTopo 页面人工解除阻塞 |

### 7.3 安全与运维边界

- 为公网接入使用 **HTTPS**，并限制可信来源网络；明文 HTTP 会暴露 Bearer Token。
- Token 只存服务端受保护的 `.secrets` 和 CI 平台 Secrets；不写入代码、JSON Variables、构建日志或任务记录。
- Hook Token 授权的是“向控制端请求更新”，必须按可触发生产更新的高权限凭据保护。
- Hook 不能携带任意命令、脚本、镜像版本或执行配置；Agent 仍使用既有身份、配对和应用包校验。
- 不要对已经返回 202 的请求自动重复提交；同一次请求会去重，但跨请求不会自动合并。
- 目前没有 Hook 专属数据库、独立部署执行器或 GitHub → Agent 直连通道。任务使用 EasyNetTopo 现有 SQLite 与调度器，Agent 使用原有任务与操作记录。

## 8. 关联配置快速验收清单

1. 在 EasyNetTopo 主机页面核对名称、Agent 配对状态；在应用目录核对 `app.yaml` 的唯一 `id`。
2. 将随机 Token 放入控制端实际 `<SOFTWARE_ROOT>/deploy/.secrets`，更新控制端容器，并确认**容器内** Token 长度至少 32。
3. 在**构建仓库**的 Actions Secrets 创建 `EASYNETTOPO_HOOK_TOKEN`，在 Variables 创建 `EASYNETTOPO_HOOK_TARGETS_JSON`。
4. 对其他应用仓库，添加“发布成功后发送 Hook”的 workflow 步骤并配置 `EASYNETTOPO_HOOK_URL`；EasyNetTopo 自身的现成 workflow 不需要重复添加。
5. 发布新版本后确认 GitHub 构建完成、Hook 收到 HTTP 202；再到 EasyNetTopo 页面确认目标 HT/AT 最终为 `succeeded`。
6. 仅当真实主机和目标应用完成闭环，才视为自动更新配置验收通过。

相关实现：[EasyNetTopo Hook 路由](../internal/httpapi/task_routes.go)、[多架构镜像工作流](../.github/workflows/container-image.yml)、[异步任务需求/状态约定](requirements/REQ-002-async-task-dispatch.md)。
