# REQ-001：EasyTier 官方镜像与 EasyHostAgent 通用无状态适配器需求规格

> **文档状态：** 需求讨论已确认，技术细节待按选定 EasyTier 版本验证  
> **文档类型：** 产品需求 + 技术需求 + 跨仓库接口设计  
> **归属项目：** EasyNetTopo  
> **涉及仓库：** `CiBeDerORG/EasyNetTopo`、`CiBeDerORG/EasyHostAgent`、`CiBeDerORG/deploy`  
> **编制日期：** 2026-11-08  
> **文档定位：** 本文记录已确认的目标、边界和验收要求；不代表相关代码已经实现，也不自动授权开始编码、部署或合并。

---

## 1. 需求摘要

本需求旨在将 EasyTier 从现有魔改版本切换回官方原版，并在 EasyHostAgent 内建立一套通用、轻量、无状态的 Adapter Framework（适配器框架），由适配器把第三方软件的原生管理接口转换为 EasyNetTopo 可以消费的统一数据格式。

本次首先解决 EasyTier 的两种部署角色：

1. **配置中心（Center）**：部署在腾讯云，运行 EasyNetTopo、EasyHostAgent，以及包含官方 EasyTier Web/Config Server 和 Core 的中心节点镜像。中心节点提供 Web 管理能力、配置下发能力，并运行一个 EasyTier Core 组网节点。
2. **普通节点（Node）**：例如 fnOS，只安装官方 EasyTier Core 镜像，配置其连接腾讯云配置中心即可。不安装 EasyNetTopo，不运行 Web Embed 或 Config Server。

EasyHostAgent 内置通用 Adapter Framework。第一种适配器为 EasyTier Adapter，通过 Docker Engine API 的 Exec 能力在目标 EasyTier 容器内执行官方 `easytier-cli`，由官方 CLI 连接本地 EasyTier RPC，读取节点、Peer、Route 等信息，再转换成 EasyHostAgent 的统一响应，供 EasyNetTopo 使用。

**Adapter 只承担“请求转发、协议适配、数据转换”的中间层职责，不是数据库、插件管理器、应用生命周期管理器或独立服务。**

## 2. 背景与当前问题

### 2.1 现状

- 当前 deploy 中 EasyTier 的镜像指向用户维护的魔改项目。
- 魔改版本曾承担部分数据读取或管理能力，但这使 EasyTier 上游升级与业务功能耦合。
- EasyNetTopo 需要读取 EasyTier 网络信息、节点、Peer、连接和路由等数据。
- EasyHostAgent 已经承担主机和 Docker 资源访问职责，可以通过它在目标主机上访问 Docker Engine。
- EasyHostAgent 目前没有必要为了本需求增加数据库；现有 Agent 的身份、配对、任务等持久化机制与本需求无关。

### 2.2 需要解决的问题

| 编号 | 问题 | 影响 |
|---|---|---|
| P-001 | EasyTier 使用魔改镜像 | 官方升级、兼容性和问题定位受制于私有修改 |
| P-002 | EasyTier 运行数据与业务代码耦合 | EasyNetTopo 为读取数据而依赖 EasyTier 内部实现 |
| P-003 | 配置中心与普通节点部署形态不同 | 不能使用一份无差别部署定义安装所有节点 |
| P-004 | 不同第三方软件的管理接口不同 | EasyNetTopo 不应分别实现 EasyTier、Mihomo 等软件的底层通信细节 |
| P-005 | Adapter 若单独容器化会增加交互复杂度 | Docker Exec、网络命名空间、容器间地址、权限和版本匹配均变复杂 |
| P-006 | Adapter 若单独编译发布 | 每增加一个适配器就可能增加多系统、多架构构建和发布负担 |
| P-007 | 过度设计实例、数据库和生命周期 | 简单的数据转换被扩展成需要持久化和运维的子系统 |

## 3. 建设目标

### 3.1 总体目标

1. EasyTier 回归官方原版，不再通过修改 EasyTier Core 源码实现 EasyNetTopo 的数据采集。
2. 通过 Center / Node 两种部署角色满足管理中心与普通组网节点的不同需求。
3. EasyHostAgent 内置通用 Adapter Framework，按适配器类型分发请求。
4. EasyTier Adapter 使用官方 CLI / RPC 获取数据，不自行实现 EasyTier 私有 RPC 传输协议。
5. Adapter 不持久化数据、不生成适配器实例 ID、不引入数据库。
6. EasyNetTopo 只消费 EasyHostAgent 的标准接口，不直接访问 EasyTier 容器、CLI 或 RPC。
7. 后续可用同一框架接入 Mihomo、Frigate 等软件，而不改变框架核心。

### 3.2 本期成功标准

- 腾讯云可以按 Center Profile 部署 EasyNetTopo、EasyHostAgent 和 EasyTier 配置中心节点。
- fnOS 可以按 Node Profile 安装官方 EasyTier，并成功连接腾讯云配置中心。
- 中心节点镜像基于官方 EasyTier 镜像或官方同版本发布产物，仅增加必要的启动编排，不修改官方 Core 业务代码。
- EasyHostAgent 能够通过 Docker Exec 执行目标容器内的官方 CLI，并读取有效的节点、Peer、Route 数据。
- EasyHostAgent 将结果转换为稳定的统一响应，EasyNetTopo 能消费并展示。
- Adapter 没有独立数据库、持久化实例、独立容器或独立跨平台二进制。

## 4. 部署角色与组件职责

### 4.1 Center：配置中心部署形态

目标部署位置：腾讯云管理中心。

| 组件 | 是否安装 | 职责 |
|---|---|---|
| EasyNetTopo | 是 | 统一管理平台、网络信息展示和业务集成 |
| EasyHostAgent | 是 | 主机/Docker 执行入口，内置 Adapter Framework |
| EasyTier Center 镜像 | 是 | 运行官方 EasyTier Web Embed、Config Server 和 Core |
| EasyTier Web Embed | 是 | 提供 EasyTier 官方 Web 管理能力及配置中心相关服务 |
| EasyTier Core | 是 | 作为一个实际组网节点运行 |
| EasyTier Adapter | 由 EasyHostAgent 内置 | 读取中心 Core 的运行数据并转换 |

说明：

- EasyNetTopo 的 Web 管理平台与 EasyTier 官方 Web Embed 是两个不同产品组件，不得混为一谈。
- Center 同时承担配置管理能力和一个 Core 组网节点的运行职责。
- “配置中心”是部署角色，不意味着所有业务流量必须经过腾讯云转发。配置控制面和实际数据面应保持概念分离。
- 中心节点 Core 是否承担特定网关、转发或出口职责，应由独立网络配置决定，不因其是 Center 而默认启用。

### 4.2 Node：普通节点部署形态

目标部署位置：fnOS，以及未来其他普通主机。

| 组件 | 是否安装 | 职责 |
|---|---|---|
| EasyNetTopo | 否 | 普通节点不承担统一管理平台职责 |
| EasyTier Web Embed | 否 | 不运行 Web 管理及配置中心服务 |
| EasyTier Config Server | 否 | 不运行配置下发服务 |
| 官方 EasyTier Core | 是 | 按中心下发或预置的配置加入网络 |
| EasyHostAgent | 可按主机管理需求单独安装 | 不是普通 EasyTier 组网的前置条件 |
| EasyTier Adapter | 本期不要求 | 普通节点只需连接配置中心 |

普通节点安装的核心目标是：安装官方 EasyTier Core、配置远端配置中心地址、启动并验证连接。不得因为普通节点没有 EasyNetTopo 或 Adapter 就阻止其正常组网。

如果未来需要采集每台普通节点的本地视图，可以在已有 EasyHostAgent 的主机上复用同一个 Adapter；这属于后续扩展，不是本期普通节点的必需安装项。

### 4.3 两种角色的差异

| 能力 | Center | Node |
|---|---|---|
| EasyNetTopo | 必须 | 不安装 |
| EasyTier Web / Config Server | 必须 | 不运行 |
| EasyTier Core | 必须 | 必须 |
| 配置中心地址 | 本机配置服务 | 腾讯云配置中心 |
| EasyTier Adapter | 通过中心 Agent 读取 | 本期不要求 |
| 角色定位 | 管理、配置、展示、参与组网 | 加入网络、运行组网 |

## 5. EasyTier 镜像策略

### 5.1 总体原则

- 以 EasyTier 官方发布的镜像和二进制为基础。
- 不修改 EasyTier Core 的组网、Peer、路由、配置协议等业务源码。
- 不把 EasyNetTopo 的业务代码、数据库或 Adapter 代码塞进 EasyTier 镜像。
- 通过固定版本的官方二进制和最薄的启动封装，满足 Center 多进程运行需求。
- 普通 Node 直接使用官方 EasyTier 镜像，不使用 Center 专用封装镜像。

### 5.2 Center 镜像：官方基础镜像 + 启动封装

目标形态：

```text
EasyTier 官方镜像 / 官方同版本发布产物
  ├── easytier-core
  ├── easytier-cli
  └── easytier-web-embed
          +
Center 启动封装
  ├── center-entrypoint
  ├── 进程监督
  ├── 信号转发与退出处理
  └── 健康检查
          ↓
EasyTier Center 镜像
```

Center 镜像原则上在一个容器中运行官方 Web Embed 与 Core，避免为了 CLI/RPC 查询再引入一个 Adapter 容器，也减少跨容器网络与权限配置。

启动封装只负责：

1. 校验必要配置和二进制存在性。
2. 启动官方 Web Embed。
3. 等待配置中心服务达到就绪条件。
4. 启动官方 Core，并配置其连接本机配置服务。
5. 监控两个核心进程。
6. 任一关键进程异常退出时按既定策略终止容器或触发失败状态，避免容器显示运行但核心服务已失效。
7. 正确处理 SIGTERM / SIGINT，停止子进程并回收退出状态。
8. 提供同时检查 Web/Config 服务与 Core/RPC 的健康检查。

注意：

- `tini` 可以作为 PID 1，帮助信号转发和僵尸进程回收，但它本身不等同于多进程监督器。不能仅用两个后台命令加 `tini` 就认为多进程生命周期已得到管理。
- 可以使用经过验证的监督方案，或实现边界清晰的入口脚本；不引入不必要的复杂进程管理平台。
- 若官方基础镜像未包含所需的 `easytier-cli` 或 `easytier-web-embed`，只能从同一官方版本的发布产物补入，不得使用不同版本的 CLI/Core 混搭。
- 具体二进制名称、参数、配置文件位置、监听端口、健康检查方式，必须以选定版本实际镜像为准。

### 5.3 Node 镜像：直接使用官方镜像

- 使用官方 EasyTier 镜像和固定版本。
- 只启动官方 Core。
- 配置远端 Config Server 地址和所需网络参数。
- 不启动 Web Embed、Config Server 或 Center 专属入口。
- 不增加 Adapter、Web 管理和数据库。
- 不使用 Center 专用镜像。

### 5.4 版本与镜像约束

- Center 和 Node 使用同一条受控的 EasyTier 上游版本线，避免 Core、CLI、Web Embed 版本不一致。
- deploy 中固定 EasyTier 版本，不使用不受控的 `latest` 作为生产版本。
- Center 封装镜像标签应能明确追溯其对应的官方 EasyTier 版本。
- 上游版本升级时，先验证 Core、CLI、Web Embed、配置中心协议和数据格式，再更新 deploy 的版本引用。
- EasyTier 版本与 EasyHostAgent 版本相互独立，不复用 `EASYHOSTAGENT_RELEASE` 作为 EasyTier 版本变量。
- 镜像应提供实际目标平台所需的架构清单；是否支持 amd64、arm64，以官方镜像和封装镜像构建结果为准。

## 6. EasyHostAgent 通用 Adapter Framework

### 6.1 定位

Adapter Framework 是 EasyHostAgent 内部的 Go 代码模块，属于 Agent 进程的一部分。

它只负责：

- 识别适配器类型；
- 将上层请求转换为对应软件可执行的请求；
- 通过合适的 Transport 与目标软件通信；
- 解析原始结果；
- 转换成 EasyHostAgent 统一响应；
- 将错误转换成统一错误结构。

Adapter 不负责：

- 安装或卸载被适配软件；
- 启动、停止、重启或升级被适配软件；
- 创建或管理 Docker 容器；
- 维护适配器实例；
- 持久化业务数据；
- 管理第三方软件自身的配置生命周期。

### 6.2 适配器类型

通过 `adapter_type` 区分适配器种类，例如：

| 类型标识 | 对应软件 | 预期通信方式 |
|---|---|---|
| `easytier` | EasyTier | Docker Exec + 官方 CLI / 本地 RPC |
| `mihomo` | Mihomo | 官方 HTTP Controller API |
| `frigate` | Frigate | 官方 HTTP API |
| 后续类型 | 其他软件 | 按对应软件官方接口选择 |

适配器类型是编译进 EasyHostAgent 的稳定标识，不是数据库记录。

### 6.3 不设计 Adapter Instance

本需求明确不引入 Adapter Instance 概念：

- 不生成 `adapter_id` 或 Adapter Instance UUID。
- 不创建 Adapter 实例表、注册表文件或实例配置目录。
- 不要求用户先创建适配器实例才能查询数据。
- 不将 Docker 容器 ID 转换成新的 Adapter ID。
- 不在 EasyNetTopo 中维护一份重复的 Adapter 实例清单。

当同一主机存在多个同类型容器时，请求通过已有 Docker Inventory 中的容器资源标识（例如 Docker container ID）明确指定目标。该标识属于现有 Docker 资源身份，不是新建的 Adapter Instance ID，也不由 Adapter 持久化。

### 6.4 编译期注册

采用模块化单体和编译期显式注册：

- EasyHostAgent 启动时将内置 Driver 注册到内存 Registry。
- Registry 根据 `adapter_type` 找到对应实现。
- 不使用 Go `.so` 动态插件加载。
- 不下载或执行第三方 Adapter 插件。
- 不为每种 Adapter 单独发布宿主机二进制。
- 新增 Adapter 时新增 Driver 代码并纳入 EasyHostAgent 原有跨平台构建流程。

### 6.5 无状态与不持久化

这是本需求的硬性约束。

| 内容 | 是否保存 |
|---|---|
| Adapter 类型注册 | 仅进程内存 |
| Adapter 配置数据库 | 不保存 |
| Adapter 实例信息 | 不存在 |
| Adapter 采集结果 | 不持久化 |
| Adapter 历史数据 | 不保存 |
| Adapter 专用缓存文件 | 不创建 |
| Adapter 任务历史 | 不创建 |
| 请求执行期间的临时变量 | 允许，随请求结束释放 |

Adapter 不新增 SQLite 表，不新增 JSON 持久化文件，不改变 EasyHostAgent 现有 identity、pairing、approval、Docker task 等存储。

EasyNetTopo 现有数据库继续服务于平台本身；本需求不为 Adapter 增加数据库表、历史快照或持久化配置。

本期不实现后台轮询、定时采集、历史趋势、持久缓存或 stale 快照。每次请求按需读取；读取失败时返回明确错误，不得用空数组伪装成“网络没有节点”，也不得把采集失败直接解释成节点离线。

### 6.6 通用接口建议

接口保持最小化，避免形成万能插件系统。以下是设计示意，最终签名需结合 EasyHostAgent 现有代码和 API 风格确认：

```go
type Adapter interface {
    Type() string
    Capabilities() []string
    Detect(ctx context.Context, target Target) error
    Collect(ctx context.Context, target Target, req CollectRequest) (*Snapshot, error)
}
```

其中：

- `Type`：返回稳定类型标识，例如 `easytier`。
- `Capabilities`：返回该 Driver 支持的只读能力。
- `Detect`：确认目标资源是否为对应软件，以及必要接口是否可用。
- `Collect`：执行按需采集并返回标准化快照。
- `Target`：引用现有主机/Docker 资源，不创建新的 Adapter 实例身份。
- `CollectRequest`：指定本次需要读取的数据范围，不携带任意 Shell 命令。

### 6.7 Transport 抽象

Adapter Type 与 Transport Type 必须分离。

| Transport | 用途 |
|---|---|
| `docker_exec` | 通过 Docker Engine API 在目标容器内执行固定命令 |
| `http` | 调用目标软件的 HTTP API |
| `host_exec` | 必要时调用受控的宿主机程序 |
| `rpc` | 后续确有必要时接入专用 RPC Client |

EasyTier 第一版使用 `docker_exec`。Mihomo 可以使用 `http`。同一个 Adapter 将来可以更换 Transport，而不改变 `adapter_type` 和 EasyNetTopo 上层数据契约。

Transport 只提供受控通信能力，不提供任意命令执行接口。

## 7. EasyTier Adapter 详细要求

### 7.1 第一版获取方式

第一版采用：

**EasyHostAgent → Docker Engine API Exec → 目标 EasyTier 容器内的官方 easytier-cli → EasyTier 本地 RPC → JSON 输出。**

不在宿主机安装 EasyTier CLI，不新增 CLI 容器，不修改官方镜像中的 Core 业务代码。

使用官方 CLI 的价值：

- CLI 与 Core 来自同一官方版本，协议兼容性由上游维护。
- CLI 本身负责处理 EasyTier RPC 传输、Protobuf 消息和 RPC 调用。
- EasyHostAgent 不必实现 EasyTier 自定义 RPC/Tunnel 传输协议。
- EasyHostAgent 只需处理命令参数、退出码、stdout/stderr、JSON 解析和数据归一化。

### 7.2 官方 RPC 的已知限制与使用要求

根据本次对官方源码的核对，需遵守以下约束：

1. 官方管理 RPC 不是标准 gRPC 服务，不能直接使用 `grpc-go`、`grpcurl` 或普通 HTTP 客户端访问。
2. RPC 使用 EasyTier 自身的 RPC/Tunnel 传输与 Protobuf 消息定义。仅生成 Protobuf Go 结构并不足以实现完整客户端。
3. 官方 CLI 已经是可复用的 RPC 客户端，因此第一版不重复实现 RPC Transport。
4. RPC Portal 支持监听地址和来源 IP 白名单配置；应显式绑定回环地址，例如 `127.0.0.1:<RPC_PORT>`，不要使用有歧义的纯数字监听写法。
5. RPC 管理端口不映射到宿主机公网，不开放云安全组，不允许普通节点直接访问中心 Core 的管理 RPC。
6. RPC 白名单不是细粒度授权系统。不能因为 Adapter 只调用查询命令，就把 RPC Server 本身视为严格只读接口。
7. 不把 RPC 端口暴露给 EasyNetTopo 浏览器或公网；由 EasyHostAgent 在本机通过 Docker Exec 调用 CLI。
8. 具体环境变量、白名单默认值、RPC 参数及消息字段可能随版本变化，必须以固定版本的官方源码和运行测试为准。

### 7.3 首期只读采集范围

| 能力 | 目标数据 | 优先来源 |
|---|---|---|
| 本地节点 | Peer ID、虚拟 IP、主机名、版本、监听信息等 | 官方 CLI Node 查询 |
| 远端 Peer | Peer ID、虚拟 IP、主机名、连接状态、连接类型 | 官方 CLI Peer 查询 |
| 连接指标 | 延迟、收发字节/包、丢包等（上游实际提供时） | Peer 连接信息 / Stats |
| 路由 | 目标网段、下一跳、路由成本、路径延迟、代理网段 | 官方 CLI Route 查询 |
| 统计 | Core 运行统计、流量指标 | 官方 CLI Stats 查询 |
| 健康状态 | CLI 是否存在、RPC 是否可达、Core 是否运行 | Detect / Health |

字段是否存在、字段类型和 JSON 层级必须以选定版本实际输出为准。缺失字段应保留为缺失或 null，不得自行编造。

### 7.4 CLI 调用约束

- 只执行 Adapter 内预定义的只读命令，例如节点、Peer、Route、Stats 查询。
- 使用 Docker Exec 的参数数组调用，不通过 `sh -c` 拼接用户输入。
- 指定目标容器已有的 RPC Portal 地址；容器内部回环地址应与 Core 实际监听配置一致。
- 优先解析官方 CLI 的结构化输出；针对需要完整字段的查询，验证 `--verbose` 输出是否比普通 JSON 展示格式包含更完整的数据。
- 不解析人类可读终端表格作为正式 API。
- 固定 EasyTier CLI 与 Core 版本匹配；不允许使用宿主机上另一版本的 CLI。
- 如果所选官方镜像没有包含 CLI，先验证同版本官方发布产物是否可安全补入 Center 镜像；不能悄悄改用不匹配版本。
- 命令、参数、JSON 结构均须建立版本化测试样本，升级 EasyTier 后执行兼容性测试。

命令形式仅作为设计示意，具体参数顺序以锁定版本的 `easytier-cli --help` 和实测为准：

```bash
easytier-cli \
  --rpc-portal 127.0.0.1:<RPC_PORT> \
  --verbose \
  route list
```

根据需要分别调用节点、Peer、Route、Stats 查询。若一个命令已经返回完整节点和 Peer/Route 数据，可在确认字段完整性后减少重复调用。

### 7.5 未来直接 RPC 的可能性

官方 Protobuf 定义中存在更接近完整网络实例快照的管理接口，例如 `WebClientService.CollectNetworkInfo`。该方向可作为未来优化研究，但不属于第一版实现要求。

只有在 CLI 多次进程启动开销、数据完整性或采集性能经测试确实成为瓶颈时，才评估直接 RPC Client。届时必须复用或正确实现官方 RPC 传输、握手、分包、压缩协商、请求响应关联和服务描述机制，不能把它误当成标准 gRPC。

## 8. 数据标准化与 API 契约

### 8.1 统一外层结构

不同 Adapter 的业务数据不同，不强迫 EasyTier、Mihomo、Frigate 使用相同的业务字段。统一的是响应外壳和错误语义。

建议响应结构：

```json
{
  "schema_version": 1,
  "adapter_type": "easytier",
  "status": "ok",
  "collected_at": "2026-11-08T00:00:00Z",
  "data": {
    "node": {},
    "peers": [],
    "routes": [],
    "statistics": {}
  },
  "error": null
}
```

要求：

- 不包含 Adapter Instance ID。
- `adapter_type` 标识本次使用的 Driver。
- `status` 表示本次调用结果，例如 `ok`、`partial`、`error`。
- `collected_at` 表示本次采集时间。
- `data` 使用该 Adapter 的业务模型。
- `error` 使用统一错误结构；成功时为 null。
- 如果有原生 Peer ID、Machine ID 等上游身份字段，应保留其语义，不得将其伪装成 Adapter ID。
- 需要引用目标容器时使用现有 Docker 资源标识；该标识仅用于本次请求和来源追踪，不生成 Adapter 身份。

### 8.2 EasyTier 业务数据

建议第一版至少标准化：

- `node`：当前 Core 节点信息；
- `peers`：当前 Core 可观察到的远端 Peer；
- `connections`：Peer 下的连接明细（上游提供时）；
- `routes`：当前 Core 的路由表；
- `statistics`：可用的运行统计。

不要把 Network、Node、Peer、Connection、Route 混成一个扁平节点数组：

- Network 是网络/配置域；
- Node 是运行中的节点身份；
- Peer 是当前节点观察到的远端对等节点；
- Connection 是节点间具体连接；
- Route 是目标网段及下一跳路径。

Peer 与 Route 是不同关系，不能把 Route 直接当作节点连线。一个节点代理某个网段，也不代表该网段内的每台设备都是 EasyTier Peer。

### 8.3 网络全量性和数据语义

Core RPC 返回的是当前 Core 实例能够观察到的运行视图，不等于配置中心永久设备清单，也不保证包含所有离线成员。

因此：

- 运行状态与配置中心成员/配置清单分开建模。
- 不把 Peer 列表缺失直接解释成设备已删除。
- 不把 Route 表缺失直接解释成网络配置不存在。
- 采集失败与空数据必须区分。
- 如果未来需要完整的管理成员清单，应单独评估官方 Web/Config Server 的管理数据接口，不把它混入 Core 运行状态。

## 9. EasyHostAgent 与 EasyNetTopo 的调用流程

### 9.1 运行时数据链路

```text
EasyNetTopo 前端
    │ 用户查看 EasyTier 网络信息
    ▼
EasyNetTopo 后端
    │ 使用已有配对关系和 Agent HTTPS 通道
    ▼
EasyHostAgent Adapter API
    │ adapter_type = "easytier"
    ▼
Adapter Registry
    │ 找到编译期注册的 EasyTier Driver
    ▼
EasyTier Adapter
    │ 通过共享 Docker Exec Transport
    ▼
Docker Engine API
    │ ExecCreate / ExecStart / ExecInspect
    ▼
EasyTier Center 容器
    │ 执行同版本官方 easytier-cli
    ▼
EasyTier Core 本机 RPC
    │ 返回官方结构化数据
    ▼
EasyTier Adapter
    │ JSON 解析、字段映射、错误转换
    ▼
EasyHostAgent 标准响应
    ▼
EasyNetTopo 后端与前端
```

### 9.2 调用原则

- 浏览器只访问 EasyNetTopo，不直接访问 EasyHostAgent、Docker 或 EasyTier RPC。
- EasyNetTopo 后端通过现有 Agent 配对和认证机制调用 Agent。
- Agent 通过已有 Docker Engine 访问能力定位目标容器并执行受控查询。
- Adapter 只处理当前请求，不负责后台定时采集。
- EasyNetTopo 根据用户页面需要按需请求；如需降低请求频率，应由上层页面/调用策略决定，不在 Adapter 内建立持久缓存。

### 9.3 Agent API 设计要求

Agent 需要提供“列出已编译适配器类型/能力”和“按类型对目标资源执行采集”的接口语义。

候选路径仅用于讨论，最终必须与 EasyHostAgent 当前 OpenAPI 统一：

- `GET /v1/adapters/types`：返回编译进 Agent 的 Adapter Type 与 capabilities。
- `POST /v1/adapters/{type}/collect`：传入已有 Docker 资源标识和采集范围，返回统一 Snapshot。

接口约束：

- 不提供 Adapter 实例 CRUD。
- 不提供 Adapter 数据库配置 API。
- 不提供任意 Shell/任意 Docker Exec API。
- 不允许调用者提交任意命令字符串。
- 采集接口仅暴露已定义的只读操作。
- API 的请求/响应 Schema、错误码和权限要求必须写入 EasyHostAgent OpenAPI，并由 EasyNetTopo 按该契约接入。

## 10. 安全、可靠性和资源限制

### 10.1 权限

- 第一版 EasyTier Adapter 仅提供只读采集能力。
- Adapter 不调用配置修改、凭据管理、ACL 修改、网络实例删除等写入 RPC。
- Agent 只允许执行内置命令白名单。
- Docker Exec 的目标容器必须来自 Agent 可识别的现有 Docker 资源，不能由请求者任意指定宿主机路径或任意容器命令。
- EasyTier RPC 只监听容器内部回环地址，RPC 端口不发布到公网或宿主机外部网络。
- 不把 RPC 白名单当作完整身份认证；通过 Docker Exec 在容器内部调用，避免将管理 RPC 作为远程服务暴露。

### 10.2 命令执行

- 使用参数数组，不拼接 Shell 字符串。
- 设置单次执行超时、最大 stdout/stderr 大小和并发限制。
- 支持请求取消，确保客户端断开或超时后可以停止不必要的 Exec 等待。
- 分开处理退出码、stdout 和 stderr。
- 错误日志不得记录 Token、私钥、配置凭据或其他敏感信息。
- JSON 解析失败必须返回明确错误，不得静默返回空数据。

### 10.3 状态与错误

至少区分：

- 目标容器不存在；
- 目标容器未运行；
- 容器中缺少官方 CLI；
- CLI 与 Core 版本不匹配或版本无法识别；
- RPC Portal 不可达；
- CLI 命令不受当前版本支持；
- 命令执行超时；
- CLI 返回非零退出码；
- JSON 格式错误或字段结构不兼容；
- 部分数据采集成功；
- 完整采集成功。

不得将上述错误转换成“节点数为 0”或“所有节点离线”。

## 11. 明确不做的内容

以下内容不属于本需求，不得在实施时擅自加入：

1. 不继续维护或扩展 EasyTier 魔改 Core。
2. 不把 Adapter 做成独立 Docker 容器。
3. 不把 Adapter 做成独立宿主机守护进程。
4. 不为每个 Adapter 构建独立的 Linux/macOS/fnOS 二进制。
5. 不引入 Go `.so` 动态插件或远程插件下载执行。
6. 不引入 Adapter 数据库、Adapter Instance ID、实例注册表、配置目录或历史记录。
7. 不实现 Adapter 的安装、更新、卸载、启动、停止、重启等应用生命周期功能；这些仍由 deploy 和现有 Agent Docker 能力负责。
8. 不让 Adapter 直接操作 EasyNetTopo 数据库。
9. 不让 Adapter 直接管理 Docker 容器；它只通过共享 Transport 发起受控查询。
10. 第一版不自行实现 EasyTier RPC/Tunnel 协议，不直接依赖 EasyTier Rust 内部库，不把 EasyTier RPC 当作标准 gRPC。
11. 不通过读取 EasyTier 内部数据库、私有文件或修改源码获取运行数据。
12. 不实现后台定时采集、持久缓存、历史趋势、离线队列或自动回滚。
13. 不把普通 Node 强制升级为配置中心，不要求普通 Node 安装 EasyNetTopo 或 Web Embed。
14. 不把 Core 的 Peer/Route 运行视图冒充配置中心完整成员清单。
15. 不因本需求调整 EasyHostAgent 现有身份、配对、Docker 任务或其他无关功能。

## 12. 跨仓库职责与交付边界

| 仓库 | 需要承担的工作 | 不承担的工作 |
|---|---|---|
| EasyNetTopo | 定义平台侧业务需求；通过 Agent API 请求数据；展示网络、节点、Peer、Route；维护本需求文档 | 直接执行 Docker Exec；直接连接 EasyTier RPC；维护 Adapter 数据库 |
| EasyHostAgent | 建设通用 Adapter Framework；实现 EasyTier Driver；复用 Docker Exec；标准化响应和错误；扩展并维护自身 OpenAPI | 安装 EasyTier；修改 EasyTier Core；持久化 Adapter 实例和采集数据 |
| deploy | 定义 Center / Node 两种安装配置；维护官方版本；构建或引用 Center 薄封装镜像；Node 使用官方镜像；维护配置和生命周期 | 解析网络数据；实现 Adapter；承担 EasyNetTopo 业务展示 |

## 13. 基本实施流程

### 阶段 0：上游版本和镜像验证

1. 选择 EasyTier 官方版本并固定版本号。
2. 检查官方镜像内实际包含的 Core、CLI、Web Embed 二进制。
3. 核对各二进制版本一致性、启动参数和配置目录。
4. 在测试容器中验证 Web Embed、Config Server、Core 能否按预期运行。
5. 验证 Core 的 RPC Portal 可绑定回环地址，CLI 能从同一容器内连接。
6. 验证节点加入配置中心所需的 URL、协议、端口和认证配置。
7. 形成可重复执行的验证记录。

### 阶段 1：deploy 的 Center / Node Profile

1. Center Profile：部署 EasyNetTopo、EasyHostAgent、EasyTier Center 镜像。
2. Center 镜像基于官方版本，增加多进程启动与健康检查封装。
3. Node Profile：直接部署官方 EasyTier Core 镜像。
4. Node Profile 只配置远端 Config Server，不安装 Web Embed/Config Server。
5. 两种 Profile 的升级、卸载和数据保留行为分别验证。
6. 不改变现有 Agent 的安装与升级机制。

### 阶段 2：EasyHostAgent 通用 Adapter Framework

1. 建立最小统一接口。
2. 建立编译期 Registry。
3. 建立共享 Docker Exec / HTTP Transport 能力。
4. 定义统一 Snapshot 和错误结构。
5. 不创建数据库、文件存储、实例 ID 或后台调度器。
6. 将适配器类型和能力通过 Agent API 暴露。

### 阶段 3：EasyTier Adapter

1. 从现有 Docker Inventory 中定位 EasyTier Center 容器。
2. 检查容器运行状态、CLI 存在性和版本。
3. 通过 Docker Exec 调用官方 CLI。
4. 读取 Node、Peer、Route、Stats 等只读数据。
5. 解析结构化输出并转换为统一 Snapshot。
6. 完成超时、退出码、JSON 解析和版本兼容错误处理。

### 阶段 4：EasyNetTopo 接入

1. 通过已配对 Agent 查询适配器类型与能力。
2. 调用 EasyTier Adapter 采集接口。
3. 将 Node、Peer、Connection、Route 分层展示。
4. 展示数据采集时间和采集错误状态。
5. 不将暂时采集失败展示为全网离线或空网络。

### 阶段 5：通用性验证

以 Mihomo 作为第二种适配器验证框架：

- Mihomo Adapter 使用其官方 HTTP Controller API。
- 不复用 EasyTier 的业务模型。
- 复用 Adapter Framework、Registry、统一响应外壳和错误处理。
- 验证新增 Driver 不需要修改框架核心、不需要新增数据库和实例管理。

## 14. 验收标准

### 14.1 部署验收

- [ ] Center Profile 能安装并启动 EasyNetTopo。
- [ ] Center Profile 能启动官方 Web Embed/Config Server。
- [ ] Center Profile 能启动官方 EasyTier Core。
- [ ] Center 镜像中的关键进程均有健康检查。
- [ ] 任一关键进程异常退出时，容器不会长期显示为正常运行。
- [ ] Node Profile 只运行官方 EasyTier Core。
- [ ] Node Profile 不运行 Web Embed 或 Config Server。
- [ ] Node 能连接 Center Config Server 并成功加入网络。
- [ ] Center / Node 均使用固定且可追溯的上游版本。
- [ ] RPC 管理端口未暴露到公网。

### 14.2 Adapter Framework 验收

- [ ] EasyHostAgent 启动后能列出编译期注册的适配器类型。
- [ ] 通过 `adapter_type` 能找到对应 Driver。
- [ ] Adapter 不生成 ID，不存在实例 CRUD。
- [ ] Adapter 不新增数据库表、JSON 持久化文件或缓存目录。
- [ ] Adapter 请求结束后不保留采集结果。
- [ ] EasyTier 和 Mihomo Driver 可共用框架而不互相依赖。
- [ ] 不需要为每种 Adapter 单独构建宿主机二进制。

### 14.3 EasyTier 采集验收

- [ ] Agent 能从现有 Docker Inventory 定位 Center 容器。
- [ ] Agent 能通过 Docker Exec 执行官方 CLI。
- [ ] CLI 与 Core 版本匹配。
- [ ] 能读取本地 Node 信息。
- [ ] 能读取 Peer 信息和可用连接指标。
- [ ] 能读取 Route 信息。
- [ ] 能读取官方提供的运行统计。
- [ ] 输出使用结构化 JSON，不依赖终端表格文本。
- [ ] CLI 缺失、容器停止、RPC 不可达、命令超时和 JSON 不兼容均有明确错误。
- [ ] 采集失败不会被转换成空节点列表或全部节点离线。

### 14.4 安全验收

- [ ] Adapter 只能调用预定义只读命令。
- [ ] 不存在任意 Shell 命令执行入口。
- [ ] Docker Exec 使用参数数组。
- [ ] RPC 仅监听容器内部回环地址。
- [ ] RPC 端口不发布到公网或普通节点。
- [ ] 不输出敏感凭据或私钥。
- [ ] 命令执行有超时、输出大小限制和取消处理。

### 14.5 EasyNetTopo 接入验收

- [ ] EasyNetTopo 只通过 EasyHostAgent API 获取 EasyTier 数据。
- [ ] 页面能区分 Node、Peer、Connection 和 Route。
- [ ] 页面能展示采集时间。
- [ ] Agent 或 RPC 不可用时显示采集错误，不误判为网络中没有节点。
- [ ] 不需要为 Adapter 增加 EasyNetTopo 数据库表。

## 15. 待验证事项与技术决策门槛

以下事项需要在编码前或对应阶段验证，不能仅凭设计文档认定已经完成：

| 编号 | 待验证事项 | 验证要求 | 未验证前的处理 |
|---|---|---|---|
| V-001 | 官方镜像是否同时包含 Core、CLI、Web Embed | 检查固定版本镜像文件和启动帮助 | 不假设二进制一定存在 |
| V-002 | Web Embed 与 Core 是否能稳定放在同一容器 | 双进程启动、信号退出、故障注入 | 不发布 Center 镜像 |
| V-003 | CLI 命令和 JSON 结构 | 对固定版本执行真实查询并保存测试样本 | 不把示例命令视为稳定 API |
| V-004 | CLI 的 `--verbose` 输出完整度 | 对比 Node、Peer、Route 字段 | 不依赖不完整展示格式 |
| V-005 | RPC Portal 的监听和白名单语义 | 检查固定版本文档、源码及容器实测 | 不开放 RPC 端口 |
| V-006 | Config Server 的地址、协议、端口及路径 | Center 与 Node 实际互联测试 | 不锁定未经验证的参数 |
| V-007 | Core/CLI/Web Embed 版本兼容性 | 固定版本组合测试 | 不混用版本 |
| V-008 | Center / Node 镜像架构清单 | 检查 amd64/arm64 构建与拉取 | 不宣称未验证架构支持 |
| V-009 | Agent 在 Linux、fnOS、macOS 下的 Docker Exec 行为 | 分别验证 Docker Engine / Desktop 连接方式 | 不宣称所有平台天然一致 |
| V-010 | Agent API 与 EasyNetTopo 接口契约 | 更新 OpenAPI 并完成跨仓库契约测试 | 不提前锁定未实现的 URL/字段 |

### 15.1 版本调研记录

本需求讨论时核对过的官方版本标签中，v2.6.4 被观察为稳定发布线，v2.7.0 被观察为预发布/不稳定版本。该记录只反映本次调研时的上游状态，不构成永久版本承诺。实施前必须重新检查官方 Releases、Docker Tags、镜像架构和版本说明。

### 15.2 官方资料入口

- EasyTier 官方仓库：https://github.com/EasyTier/EasyTier
- EasyTier v2.6.4 源码标签（本次核对基线）：https://github.com/EasyTier/EasyTier/tree/v2.6.4
- 官方 CLI 实现：https://github.com/EasyTier/EasyTier/blob/v2.6.4/easytier/src/easytier-cli.rs
- 官方管理 Protobuf 定义：https://github.com/EasyTier/EasyTier/blob/v2.6.4/easytier/src/proto/api_manage.proto
- 官方 Docker 镜像：https://hub.docker.com/r/easytier/easytier

## 16. 设计决策汇总

以下内容作为本需求的已确认方向：

1. EasyTier 回归官方原版，不继续通过魔改 Core 获取业务数据。
2. EasyTier 部署区分 Center 与 Node。
3. EasyNetTopo 与 EasyHostAgent 随 Center 部署；普通 Node 只需要安装官方 EasyTier Core 并连接 Center。
4. Center 使用官方 EasyTier 程序加最薄启动封装；Node 直接使用官方镜像。
5. EasyHostAgent 内置通用 Adapter Framework。
6. Adapter 按 `adapter_type` 区分，不建立 Adapter Instance，不生成 Adapter ID。
7. Adapter 是无状态中间层，不存库、不落文件、不维护历史、不做定时采集。
8. EasyTier 第一版通过 Docker Exec 调用容器内官方 CLI，由 CLI 连接本地 RPC。
9. EasyHostAgent 不直接实现 EasyTier 自定义 RPC/Tunnel 协议。
10. Adapter 第一版只提供只读采集能力。
11. EasyNetTopo 通过 EasyHostAgent 标准 API 获取数据，不直接访问 Docker 或 EasyTier RPC。
12. Mihomo 等后续适配器复用同一框架，但各自使用对应官方 API/CLI 和业务数据模型。

**最终原则：官方软件负责自身业务运行；deploy 负责部署形态和版本；EasyHostAgent 负责主机执行与通用适配；Adapter 负责协议转换；EasyNetTopo 负责业务整合与展示。**
