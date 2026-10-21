# EasyNetTopo 魔改开发部署

仅用于 `CiBeDerORG/EasyNetTopo`。一个仓库根入口 `deploy.sh` 完成配置、分支选择、开发环境构建和 Docker Compose 部署。默认运行 Web 控制台、Rust Web API 与配置下发服务；前端使用 Vite HMR，后端使用 Cargo 增量编译并重启。

## 目录

总目录和外层仓库目录名都可以自行选择，以下以总目录 `/vol3/1000/software/modding/`、仓库目录 `EasyNetTopo/` 为例。已经克隆为 `easytier/`、`EasyNetTopo-proxy/` 或其他名字时，使用实际路径即可，无须重命名或重新克隆。

| 路径（相对总目录） | 用途 |
| --- | --- |
| `EasyNetTopo/deploy.sh` | 唯一人工操作入口；从任何当前目录调用都按脚本自身路径定位。 |
| `EasyNetTopo/deploy/` | Compose、Dockerfile、模板、内部运行器、前端开发配置、文档和验证。 |
| `EasyNetTopo/.env` | 分支、端口、构建参数与密钥；首次由模板创建，不提交 Git。 |
| `EasyNetTopo/data/workspaces/backend/` | 后端编译工作区。 |
| `EasyNetTopo/data/workspaces/frontend/` | 前端工作区，含 node_modules、Proto 生成代码和其他构建输出。 |
| `EasyNetTopo/data/cache/` | Cargo registry/git、target、pnpm store、Vite 和工具缓存。 |
| `EasyNetTopo/data/web/et.db` | 应用 SQLite 数据库；旁边可能有 SQLite 的日志文件。 |
| `EasyNetTopo/data/run/` | 状态和部署锁；`status` 区分构建中、运行、失败、停止。 |
| `EasyNetTopo/data/tmp/`、`EasyNetTopo/data/config/`、`EasyNetTopo/data/state/`、`EasyNetTopo/data/local/`、`EasyNetTopo/data/logs/` | 工具临时文件、配置、状态与应用文件日志。 |

源码只读挂载，运行器将源代码同步到 `data/workspaces/` 后编译。原有前端/共享库构建逻辑无须为了缓存路径而修改；输出自然都在 `data/` 内。`.git`、`.env`、已有缓存不复制进编译工作区。

Docker 自身的镜像层、容器元数据与 Docker 收集的控制台日志仍由 Docker 的 data-root 管理；本脚本不修改 Docker daemon 配置。上表的项目缓存、工作区和应用持久数据均在仓库的 `data/`。

### 主目录约束

- 不校验外层目录名称，也没有固定 `/vol3/` 路径；入口根据脚本位置定位，Git 使用 `git -C`，Compose 使用显式 `--project-directory`。
- `up` 和 `update` 要求 `deploy.sh` 实际位于 Git 仓库根目录，并要求 `origin` 指向 `CiBeDerORG/EasyNetTopo`；HTTPS 和 SSH 地址均支持。
- 保留仓库内部的 `deploy/`、`easytier-web/`、`easytier-core/` 等相对结构及包名；内部 Cargo/pnpm 工作区路径不会因为外层仓库改名而变化。
- `.env` 必须是仓库中的普通文件，`data/` 不能是符号链接；仓库路径不能包含回车或换行。
- 从外部目录调用时使用实际脚本路径；不支持把入口单独移出仓库，或通过仓库外的单文件符号链接定位配套文件。

移动正在运行的部署时，先执行 `./deploy.sh stop`，再整体移动仓库及其 `data/`，最后从新位置重新执行 `deploy.sh`，让容器使用新的宿主挂载路径。

## 首次使用

宿主需要 Linux amd64 或 arm64、Bash 4+、Git、Docker Engine、Docker Compose v2 和 `flock`（通常由 util-linux 提供）。不要求宿主安装 Rust、Node、pnpm 或 Python。

```bash
mkdir -p /vol3/1000/software/modding
cd /vol3/1000/software/modding
git clone --branch main https://github.com/CiBeDerORG/EasyNetTopo.git EasyNetTopo

# 可先生成配置再编辑；直接运行无参数入口也会自动创建配置。
./EasyNetTopo/deploy.sh init
${EDITOR:-vi} EasyNetTopo/.env
./EasyNetTopo/deploy.sh
```

如果已有仓库，使用已经合入这些部署文件的分支，然后执行 `./deploy.sh`。在父目录手动拉取可用 `git -C EasyNetTopo pull --ff-only`，其中 `EasyNetTopo` 替换为实际目录名；Git 操作发生在项目仓库中，总目录本身不需要是 Git 仓库。

新仓库初始版本的入口仍校验旧仓库地址，并且缺少执行权限；已经克隆该版本时，在项目目录先手动 `git pull --ff-only origin main` 取得修复，再执行入口。修复同时恢复 `deploy.sh` 的可执行模式。旧入口的 `update` 会被 origin 检查拒绝，不能用它完成这次首次修复更新。

首次会构建包含 Rust 1.95、Node 22、pnpm 10 与 protoc 35.1 的开发环境镜像，并安装项目依赖。之后复用该镜像、Cargo 和 pnpm 缓存。`CARGO_BUILD_PROFILE=dev` 适合频繁修改；`release` 使用项目原有优化配置，编译成本更高。

容器与宿主使用相同 UID/GID，因此正常情况下 `data/` 不会因容器而变成其他用户所有。如果原先用 root 部署过，请先让当前部署用户具备该目录的读写权限。

默认 Web 访问地址为 `http://localhost:11210`。在远端 NAS 上开发时，可以使用 SSH 端口转发：

```bash
ssh -L 11210:127.0.0.1:11210 your-user@your-nas
```

然后在自己电脑访问 `http://localhost:11210`。若通过域名访问，在 `.env` 调整监听地址并配置已有的 HTTPS 反向代理，见下文。

## `.env` 配置

完整模板见 [.env.example](.env.example)。缺少 `.env` 时复制模板，已有文件不会被覆盖。

```dotenv
# 留空使用当前 Git 分支；新仓库默认分支为 main，也可显式设置 main。
GIT_BRANCH=
COMPOSE_PROJECT_NAME=easynettopo-modding
BUILDX_BUILDER=
BUILDKIT_PROGRESS=auto

CARGO_BUILD_PROFILE=dev
CARGO_BUILD_JOBS=2
EASYTIER_WATCH=true

WEB_BIND_ADDRESS=127.0.0.1
WEB_PORT=11210
API_BIND_ADDRESS=127.0.0.1
ET_API_SERVER_PORT=11211
CONFIG_BIND_ADDRESS=0.0.0.0
ET_CONFIG_SERVER_PORT=22020
ET_CONFIG_SERVER_PROTOCOL=udp

ET_WEB_PUBLIC_ORIGIN=
ET_WEBHOOK_URL=
ET_WEBHOOK_SECRET=
ET_INTERNAL_AUTH_TOKEN=
```

值按字面读取，不执行 shell，不展开 `$VAR`、`${VAR}` 或 `$(...)`。支持整值单引号/双引号；注释放单独一行。含空格、`#`、`$`、`=` 的秘密可以写为 `ET_INTERNAL_AUTH_TOKEN='fictional-$value#=example'`。不支持跨行值或 shell 的 `export` 语法；已有配置项名称写错会明确报错。

`ET_WEBHOOK_SECRET` 与 `ET_INTERNAL_AUTH_TOKEN` 采用项目已有含义，空值表示不启用对应能力。模板不设置通用默认密钥，也不新增管理员密码变量。可选空字符串在启动后端前移除，避免现有 CLI 将空 URL 误认为已配置值。

内部容器固定使用 `/data`；部署总目录由你克隆仓库的位置决定，不需要在 `.env` 重复配置绝对路径。Web、API、配置服务端口都由 `.env` 决定；`ws` 配置服务使用 TCP 端口。

新模板的 Compose 项目名默认为 `easynettopo-modding`。已有 `.env` 中的项目名保持不变；若此前依赖旧默认值且未填写此项，可显式设置 `COMPOSE_PROJECT_NAME=easytier-modding` 继续管理原有容器。不同部署并存时应使用不同项目名和宿主端口。沿用旧 `.env` 时也请检查 `GIT_BRANCH`，使用新仓库中实际存在的分支。

### 复用已有 BuildKit

在仓库 `.env` 中填写 Buildx 构建器名称，例如：

```dotenv
BUILDX_BUILDER=easyadb-proxy
BUILDKIT_PROGRESS=plain
```

`easyadb-proxy` 是构建器名称，对应容器可以叫 `buildx_buildkit_easyadb-proxy0`。使用 `sudo docker buildx ls` 查看以 root 部署时可用的名称；填写后照常执行 `sudo bash deploy.sh`，启动日志会显示选用的构建器。已有配置文件不会被模板覆盖。

也可仅对本次命令指定：

```bash
sudo env BUILDX_BUILDER=easyadb-proxy BUILDKIT_PROGRESS=plain bash deploy.sh
```

选择顺序为：命令环境中的非空 `BUILDX_BUILDER` → `.env` 的值 → Docker 当前选择。`update` 或切换分支重新执行入口时继续使用相同规则。填写 `easyadb-proxy` 时，根入口调用 `deploy/build-mihomo.sh` 复用已有构建器内的 Mihomo 代理：宿主 Buildx 获取 Docker Hub 认证令牌走可达的 `proxy-net` 网桥地址，Dockerfile 的 apt/npm/protoc 下载走构建器原有代理地址。构建产物通过 `--load` 导入本机，随后继续使用原有 Docker Compose 启动服务；不会创建、重配或删除已有 BuildKit，也不会修改 Docker daemon。其他构建器仍使用原 Compose 构建路径。

`BUILDKIT_PROGRESS` 同样支持命令环境中的非空值覆盖 `.env`，留空默认 `auto`；选择 `plain` 可逐行显示构建步骤，便于复制日志。可选值为 `auto`、`plain`、`tty`、`quiet`、`none`、`rawjson`，含义见 [Docker 构建日志配置](https://docs.docker.com/build/building/variables/#buildkit_progress)。它只控制构建日志，不影响代理或应用配置。

`easyadb-proxy` 模式从现有 `buildx_buildkit_easyadb-proxy0` 容器读取代理设置，不把凭据写入项目仓库或日志。前置要求是已有 BuildKit 容器已连接 `proxy-net`，且运行中的 `mihomo` 也连接此网络；若已有代理凭据发生变化，先通过 EasyADB 原部署入口维护共享构建器。**该模式仅覆盖镜像认证和 Dockerfile RUN 下载；运行容器内 Cargo/pnpm 的下载代理仍需单独验证。**若仍提示 `.env` 不支持 `BUILDX_BUILDER` 或 `BUILDKIT_PROGRESS`，先手动 `git pull --ff-only origin main` 更新入口，再执行部署；旧版入口的 `update` 同样会在读取配置时被拒绝。

### 远程域名与 HMR

项目现有管理 API 的来源校验要求 **HTTPS 或 loopback HTTP**，仅改成 NAS 的 HTTP IP 地址不能绕过该校验。域名示例：

```dotenv
WEB_BIND_ADDRESS=0.0.0.0
ET_WEB_PUBLIC_ORIGIN=https://easytier.example.invalid
VITE_ALLOWED_HOSTS=easytier.example.invalid
VITE_HMR_HOST=easytier.example.invalid
VITE_HMR_PORT=443
VITE_HMR_PROTOCOL=wss
```

将 HTTPS 和 WebSocket 转发至配置的 Web 端口，并保留浏览器 Host。浏览器始终请求同源 `/api/`，由 Vite 转发到 backend；不会拿到 Docker 内部的服务名或后端共享密钥。这里复用已有反向代理，不增加第二套登录或权限逻辑。

## 日常命令

| 命令（在总目录执行） | 行为 |
| --- | --- |
| `./EasyNetTopo/deploy.sh` | 等同 `up`：选择 `.env` 分支，停止旧服务，构建环境并启动，等待后端/前端就绪。 |
| `./EasyNetTopo/deploy.sh update` | 停服务，按所选分支执行 `git pull --ff-only`，重新读取更新后的部署入口并部署。 |
| `./EasyNetTopo/deploy.sh stop` | 停止并删除项目容器/网络，保留 `data/`。 |
| `./EasyNetTopo/deploy.sh status` | 显示容器和编译状态；未就绪时返回非零退出码。 |
| `./EasyNetTopo/deploy.sh logs` | 跟随全部日志；可追加 `backend` 或 `frontend`。 |

`up` 不自动拉取远端，适合未提交的本地魔改。`update` 与切换分支要求工作区干净；脚本不会替你 reset、stash 或解决冲突。分支必须包含本部署功能，否则切换后会明确失败。要改分支，在 `.env` 设置 `GIT_BRANCH` 后执行入口即可。

默认开启源码监听，平时保存文件或手动 `git pull` 即可：

- 前端 Vue/TS/CSS 和共享组件源码变化：同步后由 Vite HMR 更新。
- Rust、相关构建清单、宏、Proto 或嵌入资源变化：先停后端，再增量编译并启动。
- 前端依赖清单或 Proto 变化：先停前端，按锁文件安装/生成后重新启动。
- `.env`、Dockerfile、Compose 或运行器自身变化：执行 `./deploy.sh` 重新应用。

手动 `git pull` 的文件更新不是跨服务原子事务；监听器会防抖并在编译期间检查源码是否又变更。需要一次明确停止、拉取、重新部署时使用 `update`。

## 失败语义

编译失败就是失败：旧后端在编译前停止，失败后不重新启动旧二进制，不恢复旧镜像、不切回旧提交、不回滚数据库。`data/cache/target` 中可能仍有之前的编译文件，它们只是缓存，失败路径不会启动它们。

`up` 发现构建/运行状态 failed 时返回非零。监听开启时，运行器容器可保持运行以观察后续文件修改，但健康状态失败、应用进程已停止；修正源码后可再次增量编译。`EASYTIER_WATCH=false` 时失败直接退出。Docker 不自动反复重启失败容器。

Vite 的普通页面编译错误由其错误覆盖层和日志反馈；开发服务器与 Rust 编译是独立生命周期。后端运行失败不会被前端进程仍在运行掩盖，`status` 检查两者。

若第一次构建超过 `START_TIMEOUT`，入口返回非零，容器内编译可能仍在进行；查看 `status`/`logs`，按需要增大超时或执行 `stop`。修改端口/布尔值等配置时如写错，先修正 `.env` 再使用入口。

后端当前使用进程生成的浏览器会话签名密钥，重启后可能需要重新登录；SQLite 业务数据仍在 `data/web/`。部署脚本保留原有会话与认证行为。

## 验证

实际执行范围与尚未执行项见 [verification.md](verification.md)。脚本/运行器的替身进程测试与真实 Docker/EasyTier 启动严格分开记录。
