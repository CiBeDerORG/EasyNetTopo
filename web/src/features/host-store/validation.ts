import type { ConfigField } from './types'

export function isConfigFieldVisible(field: ConfigField, values: Record<string, string>): boolean {
  const condition = field.visible_when
  if (!condition) return true
  return String(values[condition.field] ?? '') === condition.equals
}

// 应用清单字段由后端校验来源。这里在提交前提供即时、明确的输入反馈。
export function validateAppConfig(
  fields: ConfigField[],
  values: Record<string, string>
): Record<string, string> {
  const errors: Record<string, string> = {}

  for (const field of fields) {
    if (field.custom_input === false || field.editable === false || !isConfigFieldVisible(field, values)) continue
    const raw = String(values[field.key] ?? '')
    const value = raw
    const label = field.label || field.key

    if (!value) {
      if (field.required) errors[field.key] = label + '为必填项'
      continue
    }

    if (field.options?.length && !field.options.some(option => option.value === value)) {
      errors[field.key] = label + '请选择有效选项'
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
      if (!/^[+-]?\d+$/.test(value) || !Number.isSafeInteger(Number(value))) {
        errors[field.key] = label + '必须填写有效整数'
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

// 只发送允许页面输入的字段；隐藏项与只读项由 Agent 按定义解析。
export function appConfigInput(fields: ConfigField[], values: Record<string, string>): Record<string, string> {
  return Object.fromEntries(fields.filter(field =>
    field.custom_input !== false && field.editable !== false && isConfigFieldVisible(field, values)
  ).map(field => [field.key, String(values[field.key] ?? '')]))
}
