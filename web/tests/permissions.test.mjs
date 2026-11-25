import test from 'node:test'
import assert from 'node:assert/strict'
import { canRecoverFailedUninstall, getAppPermissions, isInstalledStatus, normalizeNativeStatus, recoveredUninstallStatus } from '../src/features/host-store/rules.ts'

const easynettopo = {
  source_id: 'source', app_id: 'easynettopo', name: 'EasyNetTopo',
  revision: 'abc123', summary: '', category: 'infrastructure', manifest_sha256: ''
}
const row = {
  source_id: 'source', app_id: 'easynettopo', managed: true,
  allowed: true, state: 'running'
}

test('EasyNetTopo exposes update only for a managed running installation', () => {
  const result = getAppPermissions(easynettopo, row, 'linux')
  assert.equal(result.showUpdate, true)
  assert.equal(result.canUpdate, true)
  for (const action of ['Install', 'Uninstall', 'ForceUpdate', 'Adopt']) {
    assert.equal(result['show' + action], false, action)
    assert.equal(result['can' + action], false, action)
  }
})

test('EasyNetTopo cannot install, delete, or force update missing/external apps', () => {
  for (const status of [
    undefined,
    { ...row, managed: false, state: 'not_installed' },
    { ...row, managed: false, external: true, force_updatable: true },
    { ...row, state: 'missing' },
    { ...row, state: 'unknown' }
  ]) {
    const result = getAppPermissions(easynettopo, status, 'fnos')
    assert.equal(result.canUpdate, false)
    assert.equal(result.showInstall, false)
    assert.equal(result.showUninstall, false)
    assert.equal(result.showForceUpdate, false)
  }
})

test('EasyNetTopo Mac UI allows only verified managed updates', () => {
  const permissions = getAppPermissions(easynettopo, row, 'macos')
  assert.equal(permissions.showUpdate, true)
  assert.equal(permissions.canUpdate, true)
  assert.equal(permissions.showInstall, false)
  assert.equal(permissions.showUninstall, false)
  assert.equal(permissions.showForceUpdate, false)
  for (const status of [
    undefined,
    { ...row, managed: false, state: 'not_installed' },
    { ...row, managed: false, external: true },
    { ...row, state: 'missing' }
  ]) {
    assert.equal(getAppPermissions(easynettopo, status, 'macos').canUpdate, false)
  }
})

test('Other applications retain normal install and uninstall actions', () => {
  const other = { ...easynettopo, app_id: 'portainer' }
  const installed = getAppPermissions(other, { ...row, app_id: 'portainer' }, 'linux')
  assert.equal(installed.showUpdate, true)
  assert.equal(installed.showUninstall, true)
  const missing = getAppPermissions(other, undefined, 'linux')
  assert.equal(missing.showInstall, true)
})

test('仅展示的目录条目不提供远程操作，已有受管记录仍可卸载', () => {
  const app = { ...easynettopo, app_id: 'maestro', catalog_only: true }
  for (const status of [undefined, { ...row, managed: false, external: true }]) {
    const permissions = getAppPermissions(app, status, 'macos')
    for (const action of ['Install', 'Update', 'Uninstall', 'ForceUpdate', 'Adopt']) {
      assert.equal(permissions['show' + action], false)
      assert.equal(permissions['can' + action], false)
    }
  }
  const retained = getAppPermissions(app, row, 'linux')
  assert.equal(retained.showUninstall, true)
  assert.equal(retained.canUninstall, true)
  assert.equal(retained.showUpdate, false)
  assert.equal(getAppPermissions(app, { ...row, allowed: false }, 'linux').canUninstall, false)
  assert.equal(getAppPermissions({ ...easynettopo, catalog_only: true }, row, 'macos').showUninstall, false)
})

test('卸载失败仅在完整的远端 Docker 清单确认资源不存在时恢复卡片', () => {
  const app = { ...easynettopo, app_id: 'demo' }
  const failed = {
    source_id: 'source', app_id: 'demo', managed: true, allowed: true,
    state: 'missing', action: 'uninstall', result: 'failed', message: '退出码 1'
  }
  const inventory = { agent_id: 'a', observed_at: '', status: 'ok', complete: true, resources: [] }
  assert.equal(canRecoverFailedUninstall(failed, inventory, 'demo'), true)
  const restored = recoveredUninstallStatus(failed)
  assert.equal(restored.state, 'not_installed')
  assert.equal(restored.managed, false)
  assert.equal(restored.result, undefined)
  assert.equal(restored.message, undefined)
  assert.equal(getAppPermissions(app, restored).canInstall, true)
  assert.equal(canRecoverFailedUninstall({ ...failed, state: 'running' }, inventory, 'demo'), false)
  assert.equal(canRecoverFailedUninstall(failed, { ...inventory, complete: false }, 'demo'), false)
  assert.equal(canRecoverFailedUninstall(failed, { ...inventory, resources: [
    { kind: 'compose_project', name: 'demo', resource_id: 'x', state: 'stopped', managed: true }
  ] }, 'demo'), false)
})

test('macOS 原生工具只在 Agent 确认未安装且允许时开放安装', () => {
  const app = { ...easynettopo, app_id: 'maestro', native_install: 'macos_homebrew' }
  const ready = { ...row, app_id: 'maestro', state: 'not_installed', managed: false, mode: 'macos_homebrew' }
  const perms = getAppPermissions(app, ready, 'macos')
  assert.equal(perms.showInstall, true)
  assert.equal(perms.canInstall, true)
  for (const action of ['Update', 'Uninstall', 'ForceUpdate', 'Adopt']) {
    assert.equal(perms['show' + action], false)
    assert.equal(perms['can' + action], false)
  }
  assert.equal(getAppPermissions(app, { ...ready, result: 'running', action: 'install' }, 'macos').canInstall, false)
  assert.equal(getAppPermissions(app, { ...ready, managed: true, result: 'failed' }, 'macos').canInstall, true)
  const unpaired = getAppPermissions(app, ready, 'macos', false)
  assert.equal(unpaired.canInstall, false)
  assert.match(unpaired.blockReason, /配对/)
  assert.equal(getAppPermissions(app, { ...ready, state: 'installed', version: '1.40.0' }, 'macos').showInstall, false)
  const unavailable = getAppPermissions(app, { ...ready, allowed: false, state: 'unknown', message: '未找到 Homebrew' }, 'macos')
  assert.equal(unavailable.canInstall, false)
  assert.equal(unavailable.blockReason, '未找到 Homebrew')
  assert.equal(getAppPermissions({ ...app, revision: '' }, ready, 'macos').canInstall, false)
  assert.match(getAppPermissions(app, ready, 'linux').blockReason, /macOS/)
  assert.equal(getAppPermissions(app, ready, 'linux').canInstall, false)
})

test('旧 Agent 同名 Docker 状态不能充当原生已安装或安装许可', () => {
  const app = { ...easynettopo, app_id: 'maestro', native_install: 'macos_homebrew' }
  const legacy = { ...row, app_id: 'maestro', state: 'running', mode: 'compose' }
  const nativeKeys = new Set(['source/maestro'])
  const status = normalizeNativeStatus(legacy, nativeKeys)
  assert.equal(status.state, 'unknown')
  assert.equal(status.managed, false)
  const perms = getAppPermissions(app, status, 'macos')
  assert.equal(perms.showInstall, true)
  assert.equal(perms.canInstall, false)
  assert.match(perms.blockReason, /更新.*EasyHostAgent/)
  const good = { ...legacy, mode: 'macos_homebrew', state: 'installed', version: '1.40.0' }
  assert.equal(normalizeNativeStatus(good, nativeKeys), good)
  assert.equal(normalizeNativeStatus(row, nativeKeys), row)
})

test('原生安装受理记录不计作已安装，成功的手工安装可计入', () => {
  const pending = { ...row, mode: 'macos_homebrew', state: 'not_installed', managed: true }
  for (const result of ['running', 'failed', 'unknown']) {
    assert.equal(isInstalledStatus({ ...pending, result }), false)
  }
  assert.equal(isInstalledStatus({ ...pending, managed: false, state: 'installed' }), true)
  assert.equal(isInstalledStatus(row), true)
})
