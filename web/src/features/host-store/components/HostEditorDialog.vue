<script setup lang="ts">
import { computed, reactive, ref, watch } from 'vue'
import { ElMessage } from 'element-plus'
import type { Host } from '../types'

const props = defineProps<{
  modelValue: boolean
  host: Host | null
  saving: boolean
  pairing: boolean
}>()
const emit = defineEmits<{
  'update:modelValue': [value: boolean]
  save: [payload: { name: string; endpoint: string }]
  pair: [payload: { agent_id: string; spki_sha256: string; pairing_code: string }]
}>()

const form = reactive({ name: '', endpoint: '', agentId: '', fingerprint: '', pairingCode: '' })
const pairSections = ref<string[]>([])
const endpointDirty = computed(() =>
  !!props.host && form.endpoint.trim() !== props.host.endpoint.trim()
)

function reset() {
  form.name = props.host?.name || ''
  form.endpoint = props.host?.endpoint || ''
  form.agentId = props.host?.agent_id || ''
  form.fingerprint = props.host?.spki_sha256 || ''
  form.pairingCode = ''
  pairSections.value = props.host?.agent_id ? [] : ['pair']
}
watch(() => props.modelValue, open => { if (open) reset() })
watch(() => props.host, () => { if (props.modelValue) reset() })

function save() {
  const name = form.name.trim()
  const endpoint = form.endpoint.trim()
  if (name && endpoint) emit('save', { name, endpoint })
}
function pair() {
  if (!props.host || props.saving || props.pairing) return
  if (endpointDirty.value) {
    ElMessage.warning('Agent 地址已修改，请先保存主机资料，再执行配对。')
    return
  }
  const agentId = form.agentId.trim()
  const fingerprint = form.fingerprint.trim()
  const pairingCode = form.pairingCode.trim()
  if (agentId && fingerprint && pairingCode) {
    emit('pair', { agent_id: agentId, spki_sha256: fingerprint, pairing_code: pairingCode })
  }
}
</script>

<template>
  <el-dialog :model-value="modelValue" :title="host ? '编辑主机' : '主机信息'" width="min(620px, calc(100vw - 28px))"
    append-to-body :close-on-click-modal="false" @update:model-value="emit('update:modelValue', $event)">
    <template v-if="host">
      <el-form label-position="top">
        <el-form-item label="主机名称" required>
          <el-input v-model="form.name" maxlength="100" :disabled="saving || pairing" />
        </el-form-item>
        <el-form-item label="Agent 地址（主机名/IP:端口或完整 URL）" required>
          <el-input v-model="form.endpoint" maxlength="255" :disabled="saving || pairing" />
        </el-form-item>
        <el-alert title="保存主机资料只修改名称和连接地址。现有 Agent ID、TLS 指纹、Token 与配对时间保持不变；只有主动执行重新配对才会更新授权。" type="info" :closable="false" />
        <el-alert v-if="endpointDirty" class="endpoint-warning" title="Agent 地址已修改但尚未保存。请先保存修改，待主机地址更新后再配对，避免请求发送到旧地址。" type="warning" :closable="false" show-icon />
      </el-form>

      <el-divider />
      <el-collapse v-model="pairSections">
        <el-collapse-item name="pair">
          <template #title>
            <strong>{{ host.agent_id ? '重新配对 Agent' : '首次配对 Agent' }}</strong>
          </template>
          <el-alert title="请从目标主机可信终端获取 Agent UUID、TLS SPKI SHA-256 指纹和一次性配对码。不要仅信任网络返回的指纹。" type="warning" :closable="false" />
          <el-form label-position="top" class="pair-form">
            <el-form-item label="Agent UUID" required>
              <el-input v-model="form.agentId" autocomplete="off" :disabled="saving || pairing" />
            </el-form-item>
            <el-form-item label="TLS SPKI SHA-256（64 位十六进制）" required>
              <el-input v-model="form.fingerprint" autocomplete="off" spellcheck="false" :disabled="saving || pairing" />
            </el-form-item>
            <el-form-item label="一次性配对码" required>
              <el-input v-model="form.pairingCode" type="password" show-password autocomplete="off" :disabled="saving || pairing" />
            </el-form-item>
            <el-alert v-if="host.agent_id" title="重新配对成功会替换 Agent 当前唯一有效控制端授权。请确认确实需要重新配对。" type="warning" :closable="false" />
          </el-form>
          <div class="pair-footer">
            <el-button type="primary" :loading="pairing" :disabled="saving || pairing || endpointDirty || !form.agentId.trim() || !form.fingerprint.trim() || !form.pairingCode.trim()" @click="pair">
              安全配对
            </el-button>
          </div>
        </el-collapse-item>
      </el-collapse>
    </template>
    <el-empty v-else description="主机信息不可用" />

    <template #footer>
      <el-button @click="emit('update:modelValue', false)">关闭</el-button>
      <el-button type="primary" :loading="saving" :disabled="!host || saving || pairing || !form.name.trim() || !form.endpoint.trim()" @click="save">保存修改</el-button>
    </template>
  </el-dialog>
</template>

<style scoped>
.pair-form { margin-top: 16px; }
.endpoint-warning { margin-top: 12px; }
.pair-footer { display: flex; justify-content: flex-end; padding-top: 4px; }
</style>
