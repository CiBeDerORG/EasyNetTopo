# Docker 开发部署需求

基线与来源见[汇总](../README.md)，实施见[计划](../plan/README.md)和[任务](../task/README.md)。

| 需求 | 目标 | 验收 |
| --- | --- | --- |
| DDP-R01 | 仓库根唯一 deploy.sh；配套文件 deploy/，配置 .env，运行与缓存 data/；父目录和外层仓库目录名可任意选择，保留内部相对结构。 | DDP-A01：从仓库外运行实际入口仍定位正确，未在源码区生成构建产物。 |
| DDP-R02 | 缺 .env 时从 deploy/.env.example 复制；已有配置保留；秘密作为数据解析，不执行 shell。 | DDP-A02：模板首次启动；已有文件不覆盖；含 $、#、=、空格的秘密不变且不出日志。 |
| DDP-R03 | .env 选择 Git 分支、已有 Buildx 构建器、端口、构建模式和已有后端密钥；Compose 构建与启动。 | DDP-A03：按配置使用分支；拒绝破坏本地修改的切换；拉取只快进；接受 BUILDX_BUILDER，兼容调用环境覆盖与空值默认。 |
| DDP-R04 | 首次构建，后续重用缓存；源码 git pull 后前端 HMR、后端增量编译重启。 | DDP-A04：普通前端改动不重编 Rust，Rust 改动触发编译，生成文件不引起循环。 |
| DDP-R05 | 编译失败就是失败，无旧产物回退。 | DDP-A05：编译前停止旧后端，失败后不启动旧二进制；入口非零退出或 watcher 明确 failed/health 失败，修正源码可再次构建。 |
| DDP-R06 | 保留 fork 的 API、账户及网络语义。 | DDP-A06：同源 /api 与源码共享库；不绕过管理来源检查、不引入数据库服务或镜像发布流程。 |

项目默认部署 Web 控制台、Rust API 和配置下发服务；本轮不新增宿主 TUN/Core 节点。Docker 镜像/构建层属于 Docker 引擎存储，项目 data 约束不涉及移动 Docker daemon 的 data-root。

用户手动 git pull 或使用 update 均可；构建/运行失败保留 data 便于修正后再执行，不保存或切回旧版本。
