<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import type { CatalogApp, ConfigField } from '../types'

const props = defineProps<{
  modelValue: boolean
  app: CatalogApp | null
  revision: string
  fields: ConfigField[]
  saving: boolean
}>()
const emit = defineEmits<{
  'update:modelValue': [value: boolean]
  submit: [payload: { revision: string; config: Record<string, string> }]
}>()

const values = ref<Record<string, string>>({})
const editableFields = computed(() => props.fields.filter(field => field.editable !== false))

function resetValues() {
  values.value = Object.fromEntries(editableFields.value.map(field => [
    field.key,
    String(field.default ?? '')
  ]))
}
watch(() => props.modelValue, open => { if (open) resetValues() })
watch(() => props.fields, () => { if (props.modelValue) resetValues() }, { deep: true })

function submit() {
  const missing = editableFields.value.find(field =>
    field.required && !String(values.value[field.key] ?? '').trim()
  )
  if (missing) return
  emit('submit', { revision: props.revision, config: { ...values.value } })
}
</script>

<template>
  <el-dialog :model-value="modelValue" :title="'安装配置 · ' + (app?.name || '')"
    width="min(600px, calc(100vw - 28px))" append-to-body :close-on-click-modal="false"
    @update:model-value="emit('update:modelValue', $event)">
    <p class="config-intro">以下参数由应用清单定义。密钥只会提交给 EasyNetTopo 服务端，并通过已配对 Agent 发送给目标主机。</p>
    <el-form v-if="editableFields.length" label-position="top">
      <el-form-item v-for="field in editableFields" :key="field.key" :label="field.label" :required="field.required">
        <el-select v-if="field.type === 'boolean'" v-model="values[field.key]" class="full-width">
          <el-option v-if="!field.required" label="未设置" value="" />
          <el-option label="是" value="true" />
          <el-option label="否" value="false" />
        </el-select>
        <el-input
          v-else
          v-model="values[field.key]"
          :type="field.type === 'secret' ? 'password' : field.type === 'number' || field.type === 'port' ? 'number' : 'text'"
          :show-password="field.type === 'secret'"
          :autocomplete="field.type === 'secret' ? 'new-password' : 'off'"
          :placeholder="field.required ? '请填写' + field.label : '选填'"
        />
        <div v-if="field.description || field.minimum !== undefined || field.maximum !== undefined" class="field-help">
          {{ field.description }}
          <span v-if="field.minimum !== undefined || field.maximum !== undefined">
            {{ field.minimum !== undefined ? '最小值 ' + field.minimum : '' }}
            {{ field.maximum !== undefined ? ' 最大值 ' + field.maximum : '' }}
          </span>
        </div>
      </el-form-item>
    </el-form>
    <el-empty v-else description="该应用无需额外配置" :image-size="70" />
    <template #footer>
      <el-button @click="emit('update:modelValue', false)">取消</el-button>
      <el-button type="primary" :loading="saving" @click="submit">确认安装</el-button>
    </template>
  </el-dialog>
</template>

<style scoped>
.config-intro { margin: 0 0 18px; color: var(--ent-muted); font-size: 11px; line-height: 1.7; }
.full-width { width: 100%; }
.field-help { margin-top: 5px; color: var(--ent-dim); font-size: 10px; line-height: 1.5; }
</style>
