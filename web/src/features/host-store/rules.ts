import type { AppPermissions, AppRef, AppStatus, CatalogApp } from './types'

export const appKey = (app: AppRef) => app.source_id + '/' + app.app_id
export function getAppPermissions(app: CatalogApp, row?: AppStatus, platform = ''): AppPermissions {
  const managed = !!row?.managed
  const external = !!row?.external
  const showAdopt = false // 原有 Deploy 应用完成身份核验后自动关联
  // 安装任务已受理但容器尚未创建时，不能把中间态显示成“容器缺失/重新安装”。
  const installing = row?.action === 'install' && row.result === 'running'
  const showInstall = installing || !row || (!managed && !external) || row.state === 'missing'
  const showUpdate = (managed || external) && !!app.revision
  const showUninstall = managed || external
  const canInstall = !installing && !!row?.allowed && !external && row.state !== 'unknown' &&
    (!managed || row.state === 'missing' ||
      (row.action === 'install' && ['failed', 'unknown'].includes(row.result || '') &&
        row.state === 'not_installed'))
  const canUpdate = !!row?.allowed && managed && !external && !!app.revision &&
    ['running', 'stopped', 'partial'].includes(row.state)
  const canUninstall = !!row?.allowed && managed && !external && row.state !== 'unknown'
  const canAdopt = false
  const canForceUpdate = (platform === 'linux' || platform === 'fnos') && !!app.revision &&
    !!row?.force_updatable && (managed && !!row.allowed && !external ||
      external && row.blocked_reason === 'legacy_scripts_missing')
  const grantRequired = false

  let blockReason = ''
  if (!row) blockReason = '正在获取目标主机的应用状态。'
  else if (row.blocked_reason === 'grant_required') blockReason = '目标主机 Agent 版本过旧，请先更新 Agent。'
  else if (row.blocked_reason === 'source_conflict') blockReason = '此应用已有其他 Git 来源的安装记录，不能用当前来源覆盖。'
  else if (row.blocked_reason === 'adoption_required') blockReason = '检测到原 Deploy 部署，正在自动核验并关联。关联完成后可直接更新、强制更新或卸载。'
  else if (row.blocked_reason === 'legacy_scripts_missing') blockReason = '已核实原 Deploy 项目归属，但原始脚本缺失或不安全。可确认强制更新，从经过校验的 Git 应用包补齐标准脚本，保留原数据与配置。'
  else if (row.blocked_reason === 'legacy_project_name') blockReason = '已识别旧版 OpenCode 正在运行，但其 Compose 项目名为 deploy。为避免新旧容器冲突，仅关联运行状态；需先从目标主机安全迁移项目名后才能接管。'
  else if (row.blocked_reason === 'legacy_identity_mismatch') blockReason = '该项目的 Compose 工作目录与接管记录不一致，已禁用管理操作。'
  else if (external || row.blocked_reason === 'external_project') blockReason = '同名项目不属于受支持的 Deploy 目录，无法安全接管或卸载。'
  else if (!row.allowed) blockReason = '应用身份或来源校验未通过，禁止执行远程操作。'
  else if (row.state === 'unknown') blockReason = '无法确认 Docker 状态，暂时禁止安装、更新或卸载。'
  else if (row.state === 'missing') blockReason = '已有受管安装记录，但服务容器缺失；可以重新安装。'

  // EasyNetTopo 是控制台本身：应用商店仅允许更新已安装的受管实例。
  // 不展示安装、卸载、强制更新入口，也不自动赋予外部容器更新权限。
  if (app.app_id === 'easynettopo') {
    // macOS 经 Agent 校验部署身份后复用已有 deploy macOS 更新入口。
    const supported = platform === 'linux' || platform === 'fnos' || platform === 'macos'
    return {
      showInstall: false, canInstall: false,
      showUpdate: !!app.revision, canUpdate: supported && canUpdate,
      showUninstall: false, canUninstall: false,
      showAdopt: false, canAdopt: false,
      showForceUpdate: false, canForceUpdate: false,
      grantRequired: false,
      blockReason: !supported && platform
        ? 'EasyNetTopo 当前主机平台不受支持，请通过 deploy 更新。'
        : blockReason || (!managed ? 'EasyNetTopo 只支持更新已有受管安装，不提供安装或卸载。' : '')
    }
  }

  return {
    showInstall, canInstall, showUpdate, canUpdate, showUninstall, canUninstall,
    showAdopt, canAdopt, showForceUpdate: canForceUpdate,
    canForceUpdate, grantRequired, blockReason
  }
}
