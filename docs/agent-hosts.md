# M2-01：EasyHostAgent 主机直连与安全配对

> 范围：主机登记、IP:port + HTTPS、可信终端核验身份与一次性配对、主机及 Docker 只读查询。**没有远程安装/脚本执行能力**。

## 真实操作步骤

1. EasyHostAgent 在受管主机上作为独立进程启动。主机必须有一个直接可达的 TCP 地址，通常为 `IP:7014`。默认端口可能与 Deploy 的 Webtop 7014 冲突，实际应检查后配置，不自动改变已有端口。
2. EasyNetTopo 管理员登录，输入主机名称和目标 `IP:port` 创建主机记录；此步骤**不连接 Agent、不建立信任**。
3. 在目标主机**本机可信终端**查看 Agent 的固定 `agent_id` 和 TLS SPKI SHA-256 公钥指纹，使用 Agent 的本地配对命令生成一次性配对码。不能只根据 HTTP `/v1/identity` 自报内容判断是否可信。
4. 回到 EasyNetTopo 页面，输入核对过的 Agent UUID、64 位小写十六进制指纹、32 位小写十六进制配对码。浏览器将其送往本机控制端；**不会直接访问 Agent**。
5. Go 后端仅连接输入的 IP:port，经 TLS 握手检验服务证书公钥指纹及 `EasyHostAgent {agent_id}` 身份后，检查 `/v1/identity` JSON 的版本、UUID 和指纹；随后通过 `POST /v1/pairings` 交换控制端安装 UUID、主机 UUID、一次性码。
6. 配对返回的 Access Token 仅留在 Go 后端，使用 OS 私有目录下的 32 字节本机密钥通过 AES-GCM 密封保存于 SQLite；接口只返回主机非敏感记录。再次配对成功后 Agent 旧控制端的授权立即失效。
7. 管理员可以按需查询 `GET /v1/host` 与 `GET /v1/docker/inventory`。请求携带 Bearer、`X-EHA-Controller-ID`、`X-EHA-Agent-ID`，并持续执行证书指纹检查。请求失败或 `complete=false` **不得推断 Docker 容器被删除**。

## HTTP 入口

- `GET /api/v1/hosts`：当前主机列表；
- `POST /api/v1/hosts`：`{"name":"fnOS","endpoint":"192.168.1.20:7014"}`；
- `POST /api/v1/hosts/{host_id}/pair`：`{"agent_id":"...","spki_sha256":"...","pairing_code":"..."}`；
- `GET /api/v1/hosts/{host_id}/snapshot`：当前真实主机资源；
- `GET /api/v1/hosts/{host_id}/docker`：当前 Docker/Compose 资源快照。

全部接口要求管理员会话；POST 要求同源请求头及 Origin 校验。TLS 指纹绑定主机，不允许通过重新填写另一个 Agent UUID 或不同证书指纹默默替换。查询均为按需，无心跳、离线排队或自动资源操作。

## 凭据安全与部署注意

- Agent Token 不返回浏览器，不写 Git，也不记录原始日志。
- 加密密钥默认存 `<SQLite 所在目录>/.agent-sealing-key`，文件权限 0600。**密钥与数据库放在同一主机只是降低意外泄露风险，不等于 HSM/独立密钥托管**：有整个数据目录读取权的攻击者可能同时取得密钥和数据库。备份和主机文件系统权限必须统一保护。
- 初始化该文件失败、权限异常或密钥损坏会拒绝启动相关主机能力，不会静默重置凭据。
- 控制端目前仅允许本机回环 HTTP；远程浏览器访问须由管理员独立部署受保护的 HTTPS 反向代理，不能直接把后端改成公网监听。
- 如果 Agent 已完成配对但控制端本地持久化失败，须从 Agent 本机重新开放配对窗口；不能自动假定之前的 Token 仍有效。
- 当前主机终端配对码 TTL/失败尝试限制来自 EasyHostAgent，控制端不自行放宽。
- 不允许 URL 中携带 DNS 名称、SSH URL、HTTP 明文、链路本地地址。可使用普通 IPv4 和括号形式的 IPv6。

## 验收与未完成事项

自动测试覆盖：非法地址、错误指纹、身份不匹配、合法 TLS 配对、后续强制带 Token 查询、管理员身份保护、Cookie 不回显 Token、Docker incomplete=false 的保守处理。

CI 中的模拟 Agent 不能代替真实 Linux/FnOS 主机，真实配对、Agent 自签身份与现有 HTTP Handler 的互操作仍需在已授权环境下验证。

**未完成**：应用包上传、Agent 持久化任务与脚本执行信任策略、Sub2API 安装、Docker 资源写操作。当前商店展示仅预览，也不能把普通外部 Docker 容器自动视为 EasyNetTopo 受管资源。
