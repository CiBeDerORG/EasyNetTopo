# AI 直接调用 EasyNetTopo GitHook

> 本文面向 AI Agent、自动化编排和希望从 ChatGPT 网页端触发更新的使用者。
> 当前接口实现以本仓库 main 分支 internal/httpapi/task_routes.go 为准。
> Hook 是普通 HTTP API。AI 运行环境本身具备 HTTP POST 能力时，可直接调用，不需要 MCP。

## 1. 先理解调用关系

~~~text
用户用自然语言提出更新要求
  → AI 确认目标主机与应用
    → AI 的 HTTP 工具直接 POST EasyNetTopo Hook
      → EasyNetTopo 校验 Token 和目标
        → 现有 HT/AT 任务调度
          → EasyHostAgent
            → Deploy 现有更新机制
~~~

Hook 不负责首次安装、任意脚本执行或指定镜像版本。请求不传版本号，沿用 Deploy 当前配置的版本。

## 2. 必需配置

### 2.1 EasyNetTopo 服务端

在 EasyNetTopo 实际使用的 <SOFTWARE_ROOT>/deploy/.secrets 中配置：

~~~dotenv
EASYNETTOPO_HOOK_TOKEN=<至少32字符的随机密钥>
~~~

推荐生成：

~~~bash
openssl rand -hex 32
~~~

该命令生成 64 位十六进制密钥。配置修改后必须让 EasyNetTopo 容器重新加载环境变量；只改宿主机文件不会改变已运行容器的环境。

### 2.2 AI 调用端

将同一个 Token 放入 AI Agent 的服务端 Secret / Credentials 管理中：

- 不要放进系统提示词、用户消息、普通配置变量或源码仓库。
- 不要让模型读取或回显 Token。
- 如果 AI 工具平台支持 HTTP Bearer 认证，应由工具运行时自动附加 Authorization Header。
- 如果调用端没有安全凭据存储，不要把生产 Hook Token 硬编码在工具定义中。

### 2.3 网络要求

当前历史部署地址：

~~~text
http://49.235.163.195:7017/api/hooks/deploy/update
~~~

这是 HTTP 示例地址，不应在不可信公网链路上传输生产 Bearer Token。正式接入 AI 云端服务前，优先配置 HTTPS 反向代理或受控安全隧道，并确认 AI 工具运行环境可以访问该域名。

不要把 localhost、内网地址或仅用户电脑可达的地址误当成 ChatGPT 云端可达地址。

## 3. HTTP 请求格式

~~~http
POST /api/hooks/deploy/update
Authorization: Bearer <EASYNETTOPO_HOOK_TOKEN>
Content-Type: application/json
~~~

JSON：

~~~json
{
  "trigger_source": "chatgpt_web",
  "targets": [
    {
      "host_name": "Linux-01",
      "service_name": "easytier"
    },
    {
      "host_name": "Mac mini m6",
      "service_name": "easyhostagent"
    }
  ]
}
~~~

字段说明：

| 字段 | 必填 | 说明 |
| --- | --- | --- |
| trigger_source | 是 | 触发来源标识，建议 ChatGPT 网页端用 chatgpt_web，GitHub Actions 用 github_actions |
| targets | 是 | 1–64 个更新目标 |
| targets[].host_name | 是 | EasyNetTopo 主机管理中登记的主机名称，必须精确匹配 |
| targets[].service_name | 是 | 应用 app.yaml 的唯一 id；也支持唯一应用展示名称 |

额外约束：

- trigger_source 长度 2–40，只允许小写字母开头，后续为小写字母、数字或下划线。
- 请求体最大 32 KiB，不能增加未声明字段。
- 主机必须存在且已配对。
- easyhostagent 是内置保留目标，不需要在 Git 应用目录中登记。
- 普通应用必须能在已导入的应用目录中唯一解析；歧义或不存在时整个请求返回 422，不会部分受理。
- 同一次请求中的重复目标由调度器去重；不同请求之间没有幂等去重保证。

## 4. 最简单的调用示例

### 4.1 curl

~~~bash
curl --fail-with-body --silent --show-error \
  -X POST "https://<EASYNETTOPO_DOMAIN>/api/hooks/deploy/update" \
  -H "Authorization: Bearer $EASYNETTOPO_HOOK_TOKEN" \
  -H "Content-Type: application/json" \
  --data-binary '{
    "trigger_source": "chatgpt_web",
    "targets": [
      {"host_name": "Linux-01", "service_name": "easytier"}
    ]
  }'
~~~

### 4.2 Python

~~~python
import os
import requests

url = "https://<EASYNETTOPO_DOMAIN>/api/hooks/deploy/update"
token = os.environ["EASYNETTOPO_HOOK_TOKEN"]

payload = {
    "trigger_source": "chatgpt_web",
    "targets": [
        {"host_name": "Linux-01", "service_name": "easytier"}
    ],
}

response = requests.post(
    url,
    headers={
        "Authorization": f"Bearer {token}",
        "Content-Type": "application/json",
    },
    json=payload,
    timeout=30,
)
response.raise_for_status()
print(response.json())
~~~

AI Agent 有原生 HTTP 工具时，将以上请求映射为一个工具调用即可；不需要执行 shell，也不需要创建额外服务。

## 5. AI 应如何决定是否调用

推荐工具语义只保留一个写操作：

**trigger_deploy_update**

输入：
- trigger_source：由工具端固定或受控设置，不让模型任意伪造。
- targets：主机名称与应用 ID 的数组。

调用前：
1. 从用户当前请求中识别目标主机和应用。
2. 目标名称必须来自可信主机/应用清单或用户明确提供的信息；不能凭空猜测。
3. 若目标不明确，先询问缺失信息。
4. 批量操作时向用户展示完整主机和应用列表。
5. 只有用户明确要求执行更新后，才发起 POST。

调用后：
- HTTP 202 / accepted=true：只能报告“任务已受理/已排队”。
- HTTP 401：Token 不匹配。
- HTTP 503 hook_not_configured：服务端 Token 未配置或长度不足。
- HTTP 400：JSON、trigger_source 或目标数量等参数无效。
- HTTP 415：Content-Type 不正确。
- HTTP 422 invalid_targets：主机或服务不存在、未配对或不能唯一解析。
- 网络超时或响应丢失：不要直接自动重发；先核实是否已经创建任务，避免跨请求重复触发。

**重要：202 不代表更新成功。** 必须取得 HT/AT 最终 succeeded 状态后才能向用户报告成功。

## 6. 当前 Hook 能否查询任务状态

不能使用 Hook Token 查询任务列表。

- Hook Token 只用于 POST 更新触发。
- 当前 GET /api/v1/tasks 使用 EasyNetTopo 登录 Cookie。
- 不要将管理员 Cookie 交给模型，也不要把 Hook Token 当作管理员 API Token。

因此，当前 AI 调用完成后可以报告受理结果和返回的任务 ID；如果没有其他受控状态查询工具，必须说明最终执行状态需要在 EasyNetTopo 页面确认。

后续若要实现 AI 自动跟踪结果，应增加独立的最小权限只读查询接口或安全代理工具，而不是扩大 Hook Token 的权限。

## 7. ChatGPT 网页端接入选择

### 7.1 普通 ChatGPT 对话

普通 ChatGPT 网页对话不能仅凭一个 API URL 就任意发送外部 POST。需要一个 ChatGPT 能调用的工具/连接器。不能把“模型知道 URL”理解为“模型拥有 HTTP 网络权限”。

### 7.2 GPT Actions

GPT Actions 可以通过 OpenAPI 描述一个 REST 操作，并使用 Bearer API Key 认证。从技术实现看，这是最轻的 ChatGPT API 连接方式。

但截至 2026-11-10，OpenAI 官方说明个人 ChatGPT 账户（Free、Go、Plus、Pro）不能新建或发布 GPT；同时 OpenAI 已公布自定义 GPT 的迁移/退役安排。因此，不建议现在为新集成投入 GPT Actions。已有 GPT 是否可编辑、可用，取决于账户实际权限。

### 7.3 ChatGPT 自定义 MCP / App

这是 ChatGPT 原生工具接入路径，但需要远程服务、认证与连接配置，并非只填写 URL。OpenAI 当前说明：Pro 自定义 MCP 连接仅支持读取/获取，完整写操作面向 Business、Enterprise、Edu。Plus 不应假定可在网页端使用自定义 MCP 写工具。

如果未来账户和工作区权限支持，可以把 MCP 层限制为一个工具，只负责安全地代理本 Hook；不需要把任务系统、数据库或部署执行器重写成 MCP。

### 7.4 当前推荐

- Codex 或其他有 HTTP 工具的 AI Agent：直接调用现有 Hook。
- ChatGPT Plus 普通网页端：目前没有可保证可用的原生自定义 HTTP 写调用方式；不要承诺只配置 URL 就能用。
- 暂不为单个接口建设完整 MCP。
- 保持 EasyNetTopo Hook 为标准 HTTP API，后续 ChatGPT 接入层只做轻量代理，不侵入 HT/AT 调度逻辑。

官方资料：
- GPT Actions：https://help.openai.com/en/articles/9442513-configuring-actions-in-gpts
- GPT 创建与可用性：https://help.openai.com/en/articles/8554397-creating-and-editing-gpts
- ChatGPT 自定义 MCP：https://help.openai.com/en/articles/12584461-connecting-custom-mcp-servers-in-chatgpt

## 8. 实现权威来源

- Hook 通用使用手册：docs/github-actions-deploy-hook.md
- Hook 路由与请求校验：internal/httpapi/task_routes.go
- EasyNetTopo 工程规则：AGENTS.md

本文只记录 AI 调用方式与产品接入边界。实际执行前仍须核实 main 分支当前实现、Token 是否已加载、网络是否可达及用户是否授权。
