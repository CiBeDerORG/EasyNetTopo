import assert from 'node:assert/strict'
import test from 'node:test'
import { validateAppConfig, appConfigInput } from '../src/features/host-store/validation.ts'

function field(key, type, extra = {}) {
  return { key, label: key, type, required: true, ...extra }
}

test('必填项缺失会返回明确的字段错误', () => {
  const errors = validateAppConfig([field('PORTAINER_ADMIN_PASSWORD', 'secret')], {})
  assert.match(errors.PORTAINER_ADMIN_PASSWORD, /必填/)
})

test('端口必须为 1～65535 的整数，并遵守清单的上下限', () => {
  const fields = [field('web', 'port', { minimum: 1000, maximum: 9000 })]
  for (const invalid of ['0', '65536', '-1', '7000.5', 'oops', '999', '9001']) {
    assert.ok(validateAppConfig(fields, { web: invalid }).web, invalid)
  }
  assert.deepEqual(validateAppConfig(fields, { web: '7017' }), {})
})

test('数值范围、NaN、空白和无效数字格式受到校验', () => {
  const fields = [field('workers', 'number', { minimum: 1, maximum: 8 })]
  for (const invalid of ['0', '9', 'NaN', 'Infinity', '0x10', '1e3', '2.5', ' ']) {
    assert.ok(validateAppConfig(fields, { workers: invalid }).workers, invalid)
  }
  assert.deepEqual(validateAppConfig(fields, { workers: '2' }), {})
})

test('密钥长度受应用清单限制，非必填空值合法', () => {
  const fields = [
    field('password', 'secret', { min_length: 12, max_length: 24 }),
    field('optional', 'secret', { required: false })
  ]
  assert.match(validateAppConfig(fields, { password: 'short', optional: '' }).password, /至少/)
  assert.match(validateAppConfig(fields, { password: 'x'.repeat(25) }).password, /最多/)
  assert.deepEqual(validateAppConfig(fields, { password: 'abcdefghijkl', optional: '' }), {})
})

test('IPv4 地址格式和只读字段不会造成误判', () => {
  const fields = [
    field('bind', 'ipv4'),
    field('generated', 'secret', { editable: false })
  ]
  assert.ok(validateAppConfig(fields, { bind: '999.10.1.1' }).bind)
  assert.deepEqual(validateAppConfig(fields, { bind: '0.0.0.0' }), {})
  assert.deepEqual(validateAppConfig([field('generated', 'secret', { editable: false })], {}), {})
})

test('页面禁止输入和只读字段不参与提交，默认允许编辑', async () => {
  const fields = [
    { key: 'HIDDEN', type: 'string', required: true, custom_input: false },
    { key: 'READONLY', type: 'string', required: true, custom_input: true, editable: false },
    { key: 'EDITABLE', type: 'string', required: true, custom_input: true },
  ]
  assert.deepEqual(appConfigInput(fields, { HIDDEN: 'override', READONLY: 'override', EDITABLE: '值' }), { EDITABLE: '值' })
  assert.deepEqual(validateAppConfig(fields, { EDITABLE: '值' }), {})
})

test('单选 options 校验值来源，条件字段只校验和提交当前角色可见项', () => {
  const fields = [
    field('ROLE', 'string', {
      default: 'cloud',
      options: [
        { label: '云端配置中心', value: 'cloud' },
        { label: '普通节点', value: 'intranet' }
      ]
    }),
    field('WEB_API', 'string', {
      required: true,
      visible_when: { field: 'ROLE', equals: 'cloud' }
    }),
    field('CONFIG_SERVER', 'string', {
      required: true,
      visible_when: { field: 'ROLE', equals: 'intranet' }
    })
  ]

  const cloud = { ROLE: 'cloud', WEB_API: 'http://example.test:7007', CONFIG_SERVER: '' }
  assert.deepEqual(validateAppConfig(fields, cloud), {})
  assert.deepEqual(appConfigInput(fields, cloud), {
    ROLE: 'cloud',
    WEB_API: 'http://example.test:7007'
  })

  const intranet = { ROLE: 'intranet', WEB_API: 'stale-value', CONFIG_SERVER: 'udp://example.test:7008/admin' }
  assert.deepEqual(validateAppConfig(fields, intranet), {})
  assert.deepEqual(appConfigInput(fields, intranet), {
    ROLE: 'intranet',
    CONFIG_SERVER: 'udp://example.test:7008/admin'
  })

  assert.ok(validateAppConfig(fields, { ROLE: 'cloud', WEB_API: '' }).WEB_API)
  assert.ok(validateAppConfig(fields, { ROLE: 'intranet', CONFIG_SERVER: '' }).CONFIG_SERVER)
  assert.ok(validateAppConfig(fields, { ROLE: 'unknown', WEB_API: 'http://example.test' }).ROLE)
})
test('number 与 Agent 保持整数契约，密钥空格原样保留', () => {
  assert.ok(validateAppConfig([{ key: 'COUNT', label: '数量', type: 'number', required: true }], { COUNT: '1.5' }).COUNT)
  assert.deepEqual(validateAppConfig([{ key: 'SECRET', type: 'secret', required: true }], { SECRET: ' ' }), {})
})
