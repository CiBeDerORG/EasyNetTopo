# EasyTier 端口转发 API（主分支接口）

通过 EasyNetTopo 控制端访问时，必须使用管理员登录会话；写操作使用现有同源请求标记 `X-ENT-Request: 1`。

## 控制端接口

| 操作 | 方法与路径 |
|---|---|
| 查询 | `GET /api/v1/hosts/{host_id}/adapters/easytier/port-forwards` |
| 新增 | `POST /api/v1/hosts/{host_id}/adapters/easytier/port-forwards` |
| 删除 | `DELETE /api/v1/hosts/{host_id}/adapters/easytier/port-forwards` |

**查询**可使用 `instance_id` 或 `instance_name` 过滤官方 EasyTier 网络实例，两者至多选择其一；不填则查询所有。查询固定 `resource_id=compose:easytier`，由控制端加入 Agent 请求。

**新增和删除**的 JSON 使用以下字段：

```json
{
  "resource_id": "compose:easytier",
  "instance_name": "home",
  "protocol": "tcp",
  "listen": "127.0.0.1:8080",
  "destination": "10.144.0.20:80",
  "allow_public_bind": false
}
```

- `instance_name` 与 `instance_id` 二选一，**写入必须指定单个网络实例**（是 EasyTier 自身网络 ID，不是 Adapter 实例 ID）。
- `protocol`：`tcp` 或 `udp`；`listen` 和 `destination`：IP 字面量与 1–65535 端口，IPv6 使用 `[::1]:8080` 格式。新增必须有 `destination`。
- 删除可以省略 `destination`，依据协议和监听端口删除；也可以明确填写原目标地址。
- 在 `0.0.0.0` 或 `::` 监听意味着对所有网卡开放；新增时须显式设置 `allow_public_bind: true`，防火墙由管理员自行管理。
- 成功修改响应含 `applied: true`，表示官方 CLI/RPC 调用成功；可以再使用 GET 读取当前规则。并不保证管理中心后续配置下发不覆盖该规则。
- Driver 未开启返回 403；参数无效为 400；CLI、网络或 Core 运行错误为 422/502。不向浏览器提供任意命令执行或直接容器操作能力。

## Agent 直连接口

通过既有配对 Bearer 与控制器身份认证：

- `GET /v1/adapters/easytier/port-forwards?resource_id=compose:easytier[&instance_id=...]`
- `POST /v1/adapters/easytier/port-forwards`
- `DELETE /v1/adapters/easytier/port-forwards`

Agent 将规则转换为官方 `easytier-cli -p <RPC> [--instance-name ... | --instance-id ...] port-forward list/add/remove`。使用 Docker API Exec、固定命令白名单、限定官方镜像与受管的 `compose:easytier`，无需魔改 Core、无需新数据库。

**注意：** EasyTier Web 托管实例的配置持久化与规则覆盖行为尚需真实运行环境验收。此变更未添加任何前端页面或控件。
