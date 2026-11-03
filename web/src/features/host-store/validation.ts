import type { ConfigField } from './types'

// 应用清单字段由后端校验来源。这里在提交前提供即时、明确的输入反馈。
export function validateAppConfig(
  fields: ConfigField[],
  values: Record<string, string>
): Record<string, string> {
  const errors: Record<string, string> = {}

  for (const field of fields) {
    if (field.editable === false) continue
    const raw = String(values[field.key] ?? '')
    const value = raw.trim()
    const label = field.label || field.key

    if (!value) {
      if (field.required) errors[field.key] = label + '为必填项'
      continue
    }

    const length = Array.from(raw).length
    if (field.min_length !== undefined && length < field.min_length) {
      errors[field.key] = label + '至少需要 ' + field.min_length + ' 个字符'
      continue
    }
    if (field.max_length !== undefined && length > field.max_length) {
      errors[field.key] = label + '最多允许 ' + field.max_length + ' 个字符'
      continue
    }

    if (field.type === 'port') {
      const port = Number(value)
      if (!/^\d+$/.test(value) || !Number.isSafeInteger(port) || port < 1 || port > 65535) {
        errors[field.key] = label + '必须是 1～65535 的整数端口'
        continue
      }
      if (field.minimum !== undefined && port < field.minimum) {
        errors[field.key] = label + '不能小于 ' + field.minimum
      } else if (field.maximum !== undefined && port > field.maximum) {
        errors[field.key] = label + '不能大于 ' + field.maximum
      }
      continue
    }

    if (field.type === 'number') {
      // 拒绝 NaN、Infinity、十六进制和纯空格等 Number() 可接受的非普通数字格式。
      if (!/^-?(?:\d+(?:\.\d*)?|\.\d+)$/.test(value) || !Number.isFinite(Number(value))) {
        errors[field.key] = label + '必须填写有效数字'
        continue
      }
      const number = Number(value)
      if (field.minimum !== undefined && number < field.minimum) {
        errors[field.key] = label + '不能小于 ' + field.minimum
      } else if (field.maximum !== undefined && number > field.maximum) {
        errors[field.key] = label + '不能大于 ' + field.maximum
      }
      continue
    }

    if (field.type === 'ipv4') {
      const parts = value.split('.')
      if (parts.length !== 4 || parts.some(part =>
        !/^\d{1,3}$/.test(part) || Number(part) > 255
      )) {
        errors[field.key] = label + '必须填写有效的 IPv4 地址'
      }
    }
  }

  return errors
}
