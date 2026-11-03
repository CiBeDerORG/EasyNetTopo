import type { AppPermissions, AppRef, AppStatus, CatalogApp } from './types'

export const appKey = (app: AppRef) => app.source_id + '/' + app.app_id
export const isDeployManagedApp = (app: AppRef) => app.app_id === 'sub2api' || app.app_id === 'portainer'

export function getAppPermissions(app: CatalogApp, row?: AppStatus): AppPermissions {
  const managed = !!row?.managed
  const external = !!row?.external
  const showAdopt = isDeployManagedApp(app) && external
  const showInstall = !row || (!managed && !external) || row.state === 'missing'
  const showUpdate = (managed || external) && !!app.revision
  const showUninstall = managed || external
  const canInstall = !!row?.allowed && !external && row.state !== 'unknown' &&
    (!managed || row.state === 'missing' ||
      (row.action === 'install' && ['failed', 'unknown'].includes(row.result || '') &&
        row.state === 'not_installed'))
  const canUpdate = !!row?.allowed && managed && !external && !!app.revision &&
    ['running', 'stopped', 'partial'].includes(row.state)
  const canUninstall = !!row?.allowed && managed && !external && row.state !== 'unknown'
  const canAdopt = !!app.revision && !!row?.adoptable
  const canForceUpdate = !!app.revision && isDeployManagedApp(app) &&
    !!row?.force_updatable && row.blocked_reason === 'legacy_scripts_missing'
  const grantRequired = !!row && (row.blocked_reason === 'grant_required' ||
    (!row.allowed && !external && !row.blocked_reason))

  let blockReason = ''
  if (!row) blockReason = '正在获取目标主机的应用状态。'
  else if (row.blocked_reason === 'grant_required') blockReason = '未授权操作此应用。请在目标主机授予对应的 Agent 应用权限。'
  else if (row.blocked_reason === 'source_conflict') blockReason = '此应用已有其他 Git 来源的安装记录，不能用当前来源覆盖。'
  else if (row.blocked_reason === 'adoption_required') blockReason = '检测到原 Deploy 脚本安装的外部项目。请先确认接管，之后即可通过原 Deploy 脚本更新或卸载。'
  else if (row.blocked_reason === 'legacy_scripts_missing') blockReason = '已核实原 Deploy 项目归属，但原始脚本缺失或不安全。可确认强制更新，从经过校验的 Git 应用包补齐标准脚本，保留原数据与配置。'
  else if (row.blocked_reason === 'legacy_identity_mismatch') blockReason = '该项目的 Compose 工作目录与接管记录不一致，已禁用管理操作。'
  else if (external || row.blocked_reason === 'external_project') blockReason = '同名项目不属于受支持的 Deploy 目录，无法安全接管或卸载。'
  else if (!row.allowed) blockReason = '当前控制端未获得此应用的 Agent 授权，请先在目标主机授权。'
  else if (row.state === 'unknown') blockReason = '无法确认 Docker 状态，暂时禁止安装、更新或卸载。'
  else if (row.state === 'missing') blockReason = '已有受管安装记录，但服务容器缺失；可以重新安装。'

  return {
    showInstall, canInstall, showUpdate, canUpdate, showUninstall, canUninstall,
    showAdopt, canAdopt, showForceUpdate: showAdopt && row?.blocked_reason === 'legacy_scripts_missing',
    canForceUpdate, grantRequired, blockReason
  }
}
