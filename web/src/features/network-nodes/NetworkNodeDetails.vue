<script setup lang="ts">
import { computed } from 'vue'
import type { NetworkRow } from './data'
import type { NodeAttributes } from './attributes'

const props = defineProps<{ row: NetworkRow; attributes?: NodeAttributes }>()
defineEmits<{ edit: [] }>()
const locationLabel = computed(() => {
  const item = props.attributes?.location
  return item ? [item.country, item.province, item.city, item.county].filter(Boolean).join(' · ') || '未设置' : '未设置'
})
</script>

<template>
  <div class="node-details" @click.stop @keydown.stop>
    <div class="attribute-toolbar">
      <strong>节点属性</strong>
      <el-button size="small" :disabled="row.id === '—'" @click.stop="$emit('edit')">编辑节点属性</el-button>
    </div>
    <dl class="node-detail attributes-detail">
      <div><dt>节点别名</dt><dd>{{ attributes?.alias || '未设置' }}</dd></div>
      <div><dt>是否有公网 IP</dt><dd>{{ attributes ? (attributes.has_public_ip ? '是' : '否') : '未确认' }}</dd></div>
      <div><dt>已保存公网 IP</dt><dd>{{ attributes?.public_ip || '—' }}</dd></div>
      <div><dt>对端候选公网 IP</dt><dd>{{ row.candidatePublicIP || '—' }}</dd></div>
      <div><dt>节点位置</dt><dd>{{ locationLabel }}</dd></div>
      <div><dt>经纬度</dt><dd>{{ attributes?.location.latitude == null ? '—' : attributes.location.latitude + ', ' + attributes.location.longitude }}</dd></div>
    </dl>
    <dl class="node-detail">
      <div><dt>节点 ID</dt><dd>{{ row.id }}</dd></div>
      <div><dt>实例 ID</dt><dd>{{ row.instanceID }}</dd></div>
      <div><dt>连接报告</dt><dd>{{ row.connection }}</dd></div>
      <div><dt>下一跳节点</dt><dd>{{ row.nextHop }}</dd></div>
      <div><dt>路由开销</dt><dd>{{ row.cost }}</dd></div>
      <div><dt>直连延迟</dt><dd>{{ row.directLatency }}</dd></div>
      <div><dt>累计接收</dt><dd>{{ row.rx }}</dd></div>
      <div><dt>累计发送</dt><dd>{{ row.tx }}</dd></div>
      <div><dt>UDP NAT 类型</dt><dd>{{ row.natType }}</dd></div>
      <div><dt>连接丢包率</dt><dd>{{ row.loss }}</dd></div>
      <div v-if="row.kind === '入口实例'"><dt>公网地址</dt><dd>{{ row.publicIP }}</dd></div>
      <div v-if="row.kind === '入口实例'"><dt>监听地址</dt><dd>{{ row.listeners }}</dd></div>
    </dl>
    <section v-if="row.kind !== '入口实例'" class="connections" aria-label="连接明细">
      <h3>连接明细 <span>（{{ row.connections.length }} 条）</span></h3>
      <p v-if="!row.connections.length" class="connection-note">未报告直连连接，暂无连接明细。</p>
      <template v-else>
        <p class="connection-note">列表丢包率优先取默认连接，未报告默认连接时取首条有效连接。</p>
        <div class="connection-list">
          <article v-for="(conn, index) in row.connections" :key="conn.id + ':' + index" class="connection-item">
            <div class="connection-heading">
              <strong>连接 {{ index + 1 }} · {{ conn.protocol }}</strong>
              <el-tag v-if="conn.isDefault" size="small" type="info">默认连接</el-tag>
            </div>
            <dl class="node-detail connection-detail">
              <div><dt>连接 ID</dt><dd>{{ conn.id }}</dd></div>
              <div><dt>本地地址</dt><dd>{{ conn.localAddress }}</dd></div>
              <div><dt>对端地址</dt><dd>{{ conn.remoteAddress }}</dd></div>
              <div><dt>连接延迟</dt><dd>{{ conn.latency }}</dd></div>
              <div><dt>丢包率</dt><dd>{{ conn.loss }}</dd></div>
              <div><dt>累计接收</dt><dd>{{ conn.rx }}</dd></div>
              <div><dt>累计发送</dt><dd>{{ conn.tx }}</dd></div>
            </dl>
          </article>
        </div>
      </template>
    </section>
  </div>
</template>

<style scoped>
.node-details { padding: 16px 22px 20px; cursor: auto; color: var(--ent-text); background: var(--ent-detail-bg); }
.node-details .attribute-toolbar strong { color: var(--ent-detail-accent); font-size: 14px; }
.node-details .node-detail { background: var(--ent-detail-section); border: 1px solid var(--ent-detail-line); border-radius: 10px; padding: 13px 15px; }
.node-details .attributes-detail { background: var(--ent-detail-attributes); border-color: var(--ent-detail-line); }
.attribute-toolbar { display: flex; justify-content: space-between; align-items: center; gap: 12px; margin-bottom: 14px; }
.attributes-detail { margin-bottom: 14px !important; }
.node-detail { display: grid; grid-template-columns: repeat(auto-fit, minmax(180px, 1fr)); gap: 10px 20px; margin: 0; }
.node-detail div { min-width: 0; }
.node-detail dt, .connection-note, h3 span { color: var(--ent-detail-label); font-size: 12px; }
.node-detail dt { font-weight: 550; }
.node-detail dd { margin: 5px 0 0; overflow-wrap: anywhere; color: var(--ent-detail-value); font-weight: 600; }
.connections { margin-top: 18px; }
h3 { margin: 0; font-size: 14px; }
.connection-list { display: grid; gap: 10px; }
.connection-item { padding: 12px; border: 1px solid var(--ent-detail-line); border-radius: 10px; background: var(--ent-detail-section); }
.connection-item .connection-detail { background: transparent; border: 0; padding: 0; }
.connection-heading { display: flex; align-items: center; flex-wrap: wrap; gap: 8px; margin-bottom: 12px; }
.connection-heading strong { color: var(--ent-detail-accent); }
@media (max-width: 760px) {
  .node-details { margin-top: 14px; padding: 14px; border-top: 2px solid var(--ent-detail-accent); border-radius: 10px; }
  .node-detail { grid-template-columns: minmax(0, 1fr); }
}
</style>
