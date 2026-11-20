# 临时主机反向接入与预安装（一期）

> 实现分支：`feat/reverse-temporary-host-20261113`。实际部署前以 CI 和真实 Agent 联调为准。

## 目标和边界

内网 EasyHostAgent 仅需主动访问公网 EasyNetTopo，使用 HTTPS 短轮询完成发现、接入、预安装、应用包下载及结果反馈。**不会**建立 WebSocket、MQTT、gRPC、VPN、SSH 隧道，也不允许通用 Shell 下发。正式主机的已有直连配对与市场操作不变。

## 连接与发现

1. EasyNetTopo 配置反向接入高熵密钥，并提供有效 HTTPS 域名。
2. 内网 Agent 设置同一密钥、控制端 HTTPS 根地址（不含 /api 路径）。
3. 管理端“主机发现”开启后，发现窗口持续最多 30 分钟；Agent 持其自身 P-256 私钥为请求签名，报到带证书、Agent UUID、操作系统、架构和版本。
4. 服务端要求签名/请求体摘要/时间窗/随机 nonce 校验。首次报到只产生候选，不产生执行权限；管理员核对目标机可信终端显示的 Agent UUID 和 SPKI 指纹后，点击“加入临时主机”。
5. 临时主机进入已有主机列表；暂不允许直连、更新、卸载、远程任意命令，仅可打开原应用市场配置“预安装”。
6. 相同 Agent ID 的重复报到更新上次通信时间，不复制记录，不续期。

**安全警告：共享发现密钥不是管理员密码、不是普通网页登录凭据。** 只分发给可信 Agent，及时轮换。确认候选时须独立核对 Agent 身份；不得仅信任候选接口自报公钥。

## 不可续期的固定 TTL

`accepted_at` 为管理员确认时的服务端 UTC 时间；`expires_at=accepted_at+1800s`。Agent 心跳、重启、重新报到、任务领取都不能延长它。到期或人工剔除后，立即禁止新任务，取消待领取任务并移出列表，保留任务审计；已领取任务允许受限的应用包下载和最终结果回报，不重跑未知执行。

列表卡片展示独立的“临时有效期”和“预计下次领取倒计时”（根据最近实际报到和轮询计划估算，不承诺网络实时到达）。轮询默认 10 秒，错误指数退避至 30 秒，间隔带抖动；持续中断 5 分钟后优先重新报到，正常每 10 分钟重新报到。

## 预安装流程

原应用市场使用原有动态安装表单验证配置，保存固定 Git Revision、Bundle SHA256 和参数的加密快照，生成 `reverse_discovery` 触发来源的任务。Agent 下次主动领取后，下载指定任务 Bundle，核验 SHA256，复用现有 `market.Service.Start` 安装执行器，再回报结果。安装任务 ID 在本机私有目录持久化；重复领取/ACK/结果回传幂等，网络结果未知时禁止盲目重装。

控制端旧有直连 Worker 忽略反向任务。完成/取消任务后删除 Bundle 二进制和加密参数，保留轻量任务索引与审计。

## 配置（Linux / fnOS / macOS）

控制端真实 `<SOFTWARE_ROOT>/deploy/.secrets`：
```dotenv
EASYNETTOPO_REVERSE_BOOTSTRAP_TOKEN=<随机生成的64位十六进制密钥>
```
同一控制端的 `deploy/.env` 在 **HTTPS 终止于受信任代理、后端内部仍为 HTTP** 时设置：
```dotenv
EASYNETTOPO_REVERSE_BEHIND_TLS_PROXY=true
```
必须阻断公众直接访问明文后端 7017，仅暴露受信任 HTTPS 代理的域名与 443；若控制端自身直接运行 TLS 则该开关不需要启用。不要使用 HTTP URL。

Agent 真实 `deploy/.env`：
```dotenv
EASYHOSTAGENT_CONTROLLER_URL=https://control.example.com
```
Agent 真实 `deploy/.secrets`：
```dotenv
EASYHOSTAGENT_REVERSE_BOOTSTRAP_TOKEN=<与控制端相同的密钥>
```
Linux / fnOS 的 deploy 安装器写入 root 私有 `/etc/easyhostagent/agent.env`；macOS 安装器从当前用户持有且权限 0600 的 `deploy/.secrets` 复制密钥到 Agent 私有数据目录，并只将密钥**文件路径**加入 LaunchAgent plist。没有配置这两个值时，Agent 继续原有直连模式。

建议用 `openssl rand -hex 32` 生成密钥，切勿提交真实密钥到 GitHub、CI、日志或聊天。

## 主要 API

管理员（登录+同源请求标记）：`GET/PUT /api/v1/reverse/discovery`，`POST /api/v1/reverse/discovery/{agent_id}/{accept|reject}`，`POST /api/v1/hosts/{host_id}/preinstall`，`DELETE /api/v1/hosts/{host_id}/temporary`。

Agent（每次签名+共享凭据+HTTPS）：`POST /api/v1/reverse/report`、`POST /api/v1/reverse/claim`、`POST /api/v1/reverse/tasks/{task_id}/{ack|result}`、`GET /api/v1/reverse/tasks/{task_id}/bundle`。

## 验收重点

- 发现关时新 Agent 不可入库；发现开时仅候选，不可领取应用。
- 未通过私钥签名、密钥不符、重放、伪造 Agent UUID、证书指纹改变均拒绝。
- 接受后恰好 30 分钟，不允许任何形式续期；过期/剔除从主机列表消失。
- 仅预安装；原直连管理 API 不能被临时主机绕过调用。
- 安装表单与正式市场一致；固定包摘要下载、内容验证、参数仅在服务端加密保存。
- 重复领取/ACK/结果反馈、网络断开和两端重启不会自动重复安装。
- Linux/fnOS/macOS 的部署安装流程保持旧配置兼容。
