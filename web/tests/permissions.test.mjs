import test from 'node:test'
import assert from 'node:assert/strict'
import { canRecoverFailedUninstall, getAppPermissions, recoveredUninstallStatus } from '../src/features/host-store/rules.ts'

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
