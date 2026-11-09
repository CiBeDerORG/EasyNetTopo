<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { ElMessage } from 'element-plus'
import { validateAppConfig, appConfigInput, isConfigFieldVisible } from '../validation'
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
const errors = ref<Record<string, string>>({})
const inputFields = computed(() => props.fields.filter(field => field.custom_input !== false))
const visibleFields = computed(() => inputFields.value.filter(field => isConfigFieldVisible(field, values.value)))

function resetValues() {
  errors.value = {}
  values.value = Object.fromEntries(inputFields.value.map(field => [
    field.key,
    String(field.default ?? '')
  ]))
}
watch(() => props.modelValue, open => { if (open) resetValues() })
watch(() => props.fields, () => { if (props.modelValue) resetValues() }, { deep: true })
watch(values, () => {
  if (Object.keys(errors.value).length > 0) {
    errors.value = validateAppConfig(props.fields, values.value)
  }
}, { deep: true })

function submit() {
  if (props.saving) return
  errors.value = validateAppConfig(props.fields, values.value)
  const firstError = Object.values(errors.value)[0]
  if (firstError) {
    ElMessage.warning('请检查安装配置：' + firstError)
    return
  }
  emit('submit', { revision: props.revision, config: appConfigInput(props.fields, values.value) })
}
</script>

<template>
  <el-dialog :model-value="modelValue" :title="'安装配置 · ' + (app?.name || '')"
    width="min(600px, calc(100vw - 28px))" append-to-body :close-on-click-modal="false"
    @update:model-value="emit('update:modelValue', $event)">
    <p class="config-intro">以下参数由应用清单定义。密钥只会提交给 EasyNetTopo 服务端，并通过已配对 Agent 发送给目标主机。</p>
    <el-form v-if="visibleFields.length" label-position="top">
      <el-form-item v-for="field in visibleFields" :key="field.key" :label="field.label" :required="field.required" :error="errors[field.key]">
        <el-select v-if="field.type === 'boolean'" v-model="values[field.key]" :disabled="saving || field.editable === false" class="full-width">
          <el-option v-if="!field.required" label="未设置" value="" />
          <el-option label="是" value="true" />
          <el-option label="否" value="false" />
        </el-select>
        <el-select
          v-else-if="field.options?.length"
          v-model="values[field.key]"
          :disabled="saving || field.editable === false"
          class="full-width"
          :placeholder="field.required ? '请选择' : '选填'"
        >
          <el-option
            v-for="option in field.options"
            :key="option.value"
            :label="option.label"
            :value="option.value"
          />
        </el-select>
        <el-input
          v-else
          v-model="values[field.key]"
          :disabled="saving || field.editable === false"
          :type="field.type === 'secret' ? 'password' : field.type === 'number' || field.type === 'port' ? 'number' : 'text'"
          :show-password="field.type === 'secret'"
          :autocomplete="field.type === 'secret' ? 'new-password' : 'off'"
          :placeholder="field.required ? '请填写' + field.label : '选填'"
        />
        <div v-if="field.description || field.minimum !== undefined || field.maximum !== undefined || field.min_length !== undefined || field.max_length !== undefined" class="field-help">
          {{ field.description }}
          <span v-if="field.minimum !== undefined || field.maximum !== undefined">
            {{ field.minimum !== undefined ? '最小值 ' + field.minimum : '' }}
            {{ field.maximum !== undefined ? ' 最大值 ' + field.maximum : '' }}
          </span>
          <span v-if="field.min_length !== undefined || field.max_length !== undefined">
            {{ field.min_length !== undefined ? '最短 ' + field.min_length + ' 字符' : '' }}
            {{ field.max_length !== undefined ? ' 最长 ' + field.max_length + ' 字符' : '' }}
          </span>
        </div>
      </el-form-item>
    </el-form>
    <el-empty v-else description="该应用无需额外配置" :image-size="70" />
    <template #footer>
      <el-button @click="emit('update:modelValue', false)">取消</el-button>
      <el-button type="primary" :loading="saving" :disabled="saving" @click="submit">确认安装</el-button>
    </template>
  </el-dialog>
</template>

<style scoped>
.config-intro { margin: 0 0 18px; color: var(--ent-muted); font-size: 11px; line-height: 1.7; }
.full-width { width: 100%; }
.field-help { margin-top: 5px; color: var(--ent-dim); font-size: 10px; line-height: 1.5; }
</style>
