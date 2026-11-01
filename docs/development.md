# EasyNetTopo 最小工程开发与验收

> 本文只对应 `feat/platform-foundation` 的基础工程：**Go API + Vue 3/TypeScript/Vite**。SQLite、登录权限、Git/Agent 管理、应用安装尚未实现，不应把健康检查当成完整平台。

## 开发环境

- Go 1.23 或项目 `go.mod` 声明的更新版本
- Node.js 22（满足 Vite 7 所需的 Node 22.12+）
- npm、Linux/macOS/Windows 开发机均可分别运行前后端
- 当前仅使用标准库 Go API，不连接 Docker/SQLite/Agent

## 启动 Go 后端

~~~bash
go run ./cmd/easynettopo
# 默认监听 127.0.0.1:8080
curl http://127.0.0.1:8080/api/v1/health
go test ./...
~~~

若前端尚未构建，访问根页面会返回 503，但 `/api/v1/health` 应仍可用。未知 API 应返回 404，且不得回退至 HTML。

## 启动 Vue 前端

~~~bash
cd web
npm install
npm run dev
# 开发站点采用 Vite，/api 请求转发至本机 Go 8080
npm run typecheck
npm run build
~~~

在仓库根目录运行 Go 服务时，构建后的 `web/dist` 可由同一 Go 进程直接提供静态文件，不需要生产 Node.js 服务。

**注意**：当前为初始化阶段，尚未生成/提交依赖锁文件，`npm install` 可能解析到 semver 允许的新版本；后续稳定发布前必须提交 `package-lock.json` 并将 CI 改成 `npm ci`。当前没有生产发布包或授权管理界面，不能直接公网部署。

## 权限边界

- 默认监听 `127.0.0.1:8080`，不要在身份认证和 CSRF 等保护实现之前改为公网监听。
- `EASYNETTOPO_LISTEN_ADDRESS` 可以显式设置 Go 监听地址；变更为对外监听需要独立安全评审。
- `EASYNETTOPO_WEB_DIR` 可以指定前端已构建的静态目录，默认 `web/dist`。
- 当前唯一业务 API 为只读 `GET /api/v1/health`；没有 Git Token、Agent Token、远程命令或数据库业务路径。

## 测试门槛

- gofmt、go vet、Go race 单测、Linux amd64/arm64 构建。
- Vue TypeScript 检查、Vite 编译。
- HTTP 断言：健康响应、错误方法 405、未知 API 404、前端资源缺失不影响健康接口、SPA 路由回退。

正式增加应用商店、主机接入、SQLite 前须分别通过设计和安全约束验收，不能以本脚手架 PR 直接表示完整产品完成。
