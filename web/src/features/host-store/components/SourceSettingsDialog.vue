<script setup lang="ts">
import { reactive, ref, watch } from 'vue'
import { Delete, Plus, Refresh } from '@element-plus/icons-vue'
import type { CatalogSource } from '../types'

defineProps<{
  modelValue: boolean
  sources: CatalogSource[]
  saving: boolean
  busySourceId: string
  deletingSourceId: string
}>()
const emit = defineEmits<{
  'update:modelValue': [value: boolean]
  create: [payload: { name: string; url: string; branch: string }]
  refresh: [source: CatalogSource]
  delete: [source: CatalogSource]
}>()

const showCreate = ref(false)
const form = reactive({
  name: 'Deploy',
  url: 'https://github.com/CiBeDerORG/deploy.git',
  branch: 'main'
})

watch(showCreate, open => {
  if (!open) {
    form.name = 'Deploy'
    form.url = 'https://github.com/CiBeDerORG/deploy.git'
    form.branch = 'main'
  }
})
</script>

<template>
  <el-dialog :model-value="modelValue" title="主机商店设置 · 应用源" width="min(760px, calc(100vw - 28px))"
    append-to-body :close-on-click-modal="false" @update:model-value="emit('update:modelValue', $event)">
    <p class="intro">管理应用目录的 Git 来源。只有手动同步时才会读取仓库并更新应用目录；删除来源不会卸载主机上已安装的应用。</p>

    <div v-if="sources.length" class="source-list">
      <article v-for="source in sources" :key="source.id" class="source-row">
        <div class="source-info">
          <strong>{{ source.name }}</strong>
          <span>{{ source.url }}</span>
          <small>分支 {{ source.branch }} · {{ source.current_revision ? '当前版本 ' + source.current_revision.slice(0, 12) : '尚未同步' }}</small>
          <small v-if="source.last_error" class="source-error">{{ source.last_error }}</small>
        </div>
        <div class="source-actions">
          <el-tooltip content="手动同步" placement="top">
            <el-button text circle :loading="busySourceId === source.id" :disabled="!!busySourceId || !!deletingSourceId" aria-label="同步应用源" @click="emit('refresh', source)">
              <el-icon><Refresh /></el-icon>
            </el-button>
          </el-tooltip>
          <el-tooltip content="删除来源" placement="top">
            <el-button text circle type="danger" :loading="deletingSourceId === source.id" :disabled="!!busySourceId || !!deletingSourceId" aria-label="删除应用源" @click="emit('delete', source)">
              <el-icon><Delete /></el-icon>
            </el-button>
          </el-tooltip>
        </div>
      </article>
    </div>
    <el-empty v-else description="尚未添加 Git 应用源" :image-size="80" />

    <el-button v-if="!showCreate" class="add-source" @click="showCreate = true">
      <el-icon><Plus /></el-icon>新增应用源
    </el-button>

    <div v-if="showCreate" class="create-source">
      <h4>新增 Git 应用源</h4>
      <el-form label-position="top">
        <el-form-item label="来源名称" required>
          <el-input v-model="form.name" maxlength="80" placeholder="例如：Deploy 官方应用源" />
        </el-form-item>
        <el-form-item label="Git 仓库 URL" required>
          <el-input v-model="form.url" spellcheck="false" autocomplete="off" placeholder="https://github.com/owner/repository.git" />
        </el-form-item>
        <el-form-item label="分支 / Ref" required>
          <el-input v-model="form.branch" placeholder="main" />
        </el-form-item>
      </el-form>
      <p class="field-note">支持 SSH、HTTPS、HTTP Git 地址。HTTP 明文来源须由服务端管理员显式授权，禁止在 URL 中填写账号或密码。</p>
      <div class="form-actions">
        <el-button @click="showCreate = false">取消</el-button>
        <el-button type="primary" :loading="saving" :disabled="!form.name.trim() || !form.url.trim() || !form.branch.trim()" @click="emit('create', { name: form.name.trim(), url: form.url.trim(), branch: form.branch.trim() })">
          添加来源
        </el-button>
      </div>
    </div>
  </el-dialog>
</template>

<style scoped>
.intro { margin: 0 0 17px; color: var(--ent-muted); font-size: 12px; line-height: 1.7; }
.source-list { display: grid; gap: 9px; }
.source-row {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 14px;
  padding: 13px;
  border: 1px solid #2a445c;
  border-radius: 10px;
  background: #0e2135;
}
.source-info { display: grid; min-width: 0; gap: 5px; }
.source-info strong { font-size: 12px; }
.source-info span { overflow-wrap: anywhere; color: #87a2b9; font-size: 11px; }
.source-info small { color: var(--ent-dim); font-size: 10px; }
.source-info .source-error { color: var(--ent-red); }
.source-actions { display: flex; flex: none; gap: 3px; }
.add-source { margin-top: 15px; }
.create-source { margin-top: 18px; padding: 16px; border: 1px solid #31516a; border-radius: 12px; background: #0b1a2b; }
.create-source h4 { margin: 0 0 14px; font-size: 13px; }
.field-note { color: var(--ent-muted); font-size: 11px; line-height: 1.6; }
.form-actions { display: flex; justify-content: flex-end; gap: 8px; margin-top: 12px; }
@media (max-width: 560px) { .source-row { align-items: flex-start; } }
</style>
