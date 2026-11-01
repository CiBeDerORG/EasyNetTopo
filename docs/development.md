# EasyNetTopo 本地开发与运行

> 当前分支：M1 管理员登录与 SQLite 基础版（非生产发布）。不依赖 EasyTier、Deploy 或 EasyHostAgent 即可运行。Git 源管理、主机控制、任务和完整权限体系仍未实现。

## 技术环境

- Go 1.23+（以后按 go.mod 的具体声明和 CI 选定工具链）
- Node.js 22.12+、npm
- SQLite 使用纯 Go 驱动；不要求目标 Linux/FnOS 预装 SQLite 工具或 CGO

## 首次初始化

### 运行后端

~~~bash
go run ./cmd/easynettopo
# 默认仅监听 127.0.0.1:8080
~~~

首次运行自动在系统用户配置目录的 `easynettopo/app.db` 建立 SQLite 文件和稳定 `controller_id`。数据目录限制 0700，数据库文件限制 0600。不要删除或重新生成此文件，否则控制端身份及登录数据将丢失。

可以显式设置 `EASYNETTOPO_DATABASE_PATH` 为绝对路径，例如 `/var/lib/easynettopo/app.db`。运行进程用户需要能够创建该目录，用户及目录必须受信任。

### 创建管理员（只在本机终端执行一次）

~~~bash
go run ./cmd/easynettopo admin init
~~~

终端输入两遍 admin 密码，长度 12–72 字节。密码不会通过 HTTP 提交，也不应通过命令行参数或环境变量传入。重复初始化会失败，不能覆盖现有账号。

如果在容器中运行，可对同一挂载数据卷使用交互式 `docker exec -it <容器名> /path/to/easynettopo admin init`。必须使用与服务相同的数据库路径。

### 开发前端

~~~bash
cd web
npm ci
npm run dev
npm run build
~~~

在仓库根目录启动 Go 服务时，构建后 `web/dist` 会由该进程直接提供；未构建时 HTTP 健康接口仍可使用：

~~~bash
curl http://127.0.0.1:8080/api/v1/health
go test ./...
~~~

前端首次初始化页仅提示在可信终端创建 admin。登录后可查看受保护的 `controller_id`，目前未提供其他写操作。

## 安全限制

- **仅允许环回地址监听**，包括通过 `EASYNETTOPO_LISTEN_ADDRESS` 配置的地址；拒绝 `0.0.0.0`、公网或局域网 IP。需要远程访问时先部署完成身份隔离、HTTPS 反向代理及受信任的网络访问策略；当前版本不宣称公网生产安全。
- 密码采用 bcrypt 哈希，Cookie HttpOnly、SameSite=Strict；会话存入 SQLite 的是随机 Token 的 SHA-256 摘要，有效期 12 小时。
- 变更类 API 需要 `X-ENT-Request: 1` 请求头，校验请求 Origin；浏览器只调用同源 Go API，不允许跨站读取或管理。
- 当前仅支持单一管理员（admin），不支持角色、密码找回、审计界面、多租户与用户管理；这些功能必须另行评审后实现。
- 密钥库和 Agent/私有 Git 凭据保护尚未完成，**不得以本管理员认证为由直接开启远程 Shell、Git Token 存储或 Agent 高权限执行功能**。
- 用户主动离线删除数据库不属于自动“恢复”场景。备份必须同时保护文件和密钥，SQLite WAL 模式需要使用一致性备份手段，不要只在线复制主 DB 文件。

## CI 与验收

- Go 格式、vet、Race 单元测试、Linux amd64/arm64 无 CGO 编译。
- 前端 `npm ci`、`vue-tsc`、Vite 编译。
- HTTP 未授权、错误密码、登录、注销、请求头拒绝，以及 SQLite 重启后的控制端 UUID 和会话保持。
- Git/Agent 集成不在本阶段，请参考独立的 M0 跨仓库协议 PR。