import assert from 'node:assert/strict'
import test from 'node:test'
import { validateAppConfig } from '../src/features/host-store/validation.ts'

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
  for (const invalid of ['0', '9', 'NaN', 'Infinity', '0x10', '1e3']) {
    assert.ok(validateAppConfig(fields, { workers: invalid }).workers, invalid)
  }
  assert.deepEqual(validateAppConfig(fields, { workers: '2.5' }), {})
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
