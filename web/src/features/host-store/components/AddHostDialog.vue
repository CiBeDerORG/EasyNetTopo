<script setup lang="ts">
import { reactive, watch } from 'vue'

const props = defineProps<{ modelValue: boolean; saving: boolean }>()
const emit = defineEmits<{
  'update:modelValue': [value: boolean]
  submit: [payload: { name: string; endpoint: string }]
}>()

const form = reactive({ name: '', endpoint: '' })

watch(() => props.modelValue, open => {
  if (open) {
    form.name = ''
    form.endpoint = ''
  }
})

function submit() {
  const name = form.name.trim()
  const endpoint = form.endpoint.trim()
  if (!name || !endpoint) return
  emit('submit', { name, endpoint })
}
</script>

<template>
  <el-dialog :model-value="modelValue" title="添加主机" width="min(520px, calc(100vw - 28px))"
    append-to-body :close-on-click-modal="false" @update:model-value="emit('update:modelValue', $event)">
    <el-form label-position="top" @submit.prevent="submit">
      <el-form-item label="主机名称" required>
        <el-input v-model="form.name" maxlength="100" placeholder="例如：fnOS-NAS-01" autofocus />
      </el-form-item>
      <el-form-item label="Agent 地址（包含端口）" required>
        <el-input v-model="form.endpoint" maxlength="200" placeholder="https://192.168.1.20:7014" />
      </el-form-item>
      <el-alert title="支持 HTTPS 地址，也支持可信网络内显式使用 http://。添加后还需要核验 Agent UUID、TLS 指纹并完成配对。" type="info" :closable="false" />
    </el-form>
    <template #footer>
      <el-button @click="emit('update:modelValue', false)">取消</el-button>
      <el-button type="primary" :loading="saving" :disabled="!form.name.trim() || !form.endpoint.trim()" @click="submit">添加主机</el-button>
    </template>
  </el-dialog>
</template>
