# 设备列表迁移与职责边界

## 目标

在 EasyNetTopo 中新增独立设备资产列表。设备是被管理的物理设备/终端记录，不等同于 EasyTier 网络节点，也不等同于运行 EasyHostAgent 的主机。

## 明确不纳入本期

- ADB 配对、连接、设备控制、投屏、触控和群控。
- 自动化中心、工作流和定时任务。
- 多线路 IP 管理、临时 IP、线路选择、自动切换和故障回退。
- 重复实现网络节点列表、节点身份、节点地址或网络路由管理。

## 本期设备能力

- 设备列表、搜索、分组、未分组、分组折叠。
- 设备新增、编辑、删除；批量选择和批量删除。
- 设备名称、类型、厂商、型号、单一设备管理 IP、MAC、备注。
- 可选关联一个现有 EasyTier 网络节点。
- 可选标记设备为 OpenWrt 路由器。
- OpenWrt RPCD 账号配置归属具体设备；只有 is_openwrt=true 的设备可读写此配置。

## 网络节点关联

设备不拥有或复制 EasyTier 节点数据。只保存稳定关联身份：

- network_key
- peer_id

关联允许为空；同一个节点身份最多关联一条设备记录。节点离线、入口主机删除或节点暂时不可见时，设备关联记录仍保留。

network_key 与现有网络节点属性使用同一身份范围规则：
- 有网络名时使用现有 network:<name>。
- 无网络名的入口实例使用 entry:<host-id>:instance:<instance-id>。

节点别名、Peer ID、虚拟 IP、连接和路由仍以网络节点模块为准；设备表不复制这些字段。

## 设备分组

- 分组独立于网络节点。
- 删除分组时，设备保留并转入未分组。
- 分组重命名不影响设备 ID。
- 不在分组上保存 OpenWrt RPCD 凭据或发现服务配置。

## OpenWrt 专属配置

- OpenWrt 是设备记录上的布尔标识，且设备类型必须为 router。
- OpenWrt 设备必须填写 RFC1918 IPv4 管理地址。
- RPCD 使用设备唯一管理 IP、协议和端口，固定访问 /ubus；不提供多线路 IP。
- RPCD 用户名和密钥保存在 device_openwrt_configs 子表中。
- 密钥使用 AES-GCM 加密，密钥文件为数据目录下 .device-openwrt-sealing-key，权限必须为 0600；密钥损坏或权限不安全时拒绝启动，不自动重置。
- API 永不返回明文密钥。修改 RPCD 地址参数或用户名时，必须重新输入密钥；相同参数下留空表示保留原密钥。
- 取消 OpenWrt 标识或删除设备时，级联清除该设备的 RPCD 配置。

## OpenWrt 自动发现的待确认边界

旧 EasyADB 的 OpenWrt 扫描器实际通过 Avahi 查询 _adb-tls-connect._tcp，发现的是 Android 无线调试服务，不是通用局域网设备扫描。

当前设备管理实现只建立设备归属的 OpenWrt RPCD 配置和安全存储，不执行扫描。不得因此添加 ADB 连接、配对、控制、投屏或自动化功能。是否保留仅用于发现/展示的 Avahi 扫描，需要用户明确确认后再接入；扫描结果不得自动创建或覆盖设备记录，除非另行确认规则。

## API

- GET/POST /api/v1/device-groups
- PATCH/DELETE /api/v1/device-groups/{id}
- GET/POST /api/v1/devices
- PATCH/DELETE /api/v1/devices/{id}
- GET/PUT/DELETE /api/v1/devices/{id}/openwrt

所有接口要求管理员会话；写接口要求同源请求标记。OpenWrt 密钥只在服务端解封供后续 RPCD 客户端使用。

## 页面

设备列表通过工作台侧栏独立入口 /devices 进入。页面继续复用全局认证、主题和工作台外壳，但不会加载主机商店的应用目录、Docker 清单和执行任务轮询。网络节点数据仅用于设备关联选择，不改变网络节点页面的既有管理流程。
