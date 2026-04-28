<script setup>
import { ref, onMounted, onUnmounted, computed } from 'vue'
import Panel from '../ui/Panel.vue'
import EmptyState from '../ui/EmptyState.vue'
import KebabMenu from '../ui/KebabMenu.vue'

const leases         = ref([])
const initialLoading = ref(true)
let pollTimer = null

onMounted(() => {
  fetchLeases(true)
  pollTimer = setInterval(() => fetchLeases(false), 2000)
})
onUnmounted(() => { clearInterval(pollTimer) })

async function fetchLeases(initial) {
  try {
    const result = await window['go']['main']['App']['GetLeases']()
    leases.value = (result ?? [])
      .map((l) => ({ ...l, ExpiresAt: new Date(l.ExpiresAt) }))
      .sort((a, b) => a.IP.localeCompare(b.IP, undefined, { numeric: true, sensitivity: 'base' }))
  } catch {
    // silent — show stale data until next success
  } finally {
    if (initial) initialLoading.value = false
  }
}

function ttlSeconds(expiresAt) { return Math.max(0, (expiresAt - Date.now()) / 1000) }

function ttlText(expiresAt) {
  const s  = ttlSeconds(expiresAt)
  const h  = Math.floor(s / 3600)
  const m  = Math.floor((s % 3600) / 60)
  const ss = Math.floor(s % 60)
  if (h > 0) return `${h}h ${m}m`
  if (m > 0) return `${m}m ${ss}s`
  return `${ss}s`
}

function ttlVariant(row) {
  const total = (new Date(row.ExpiresAt) - new Date(row.IssuedAt ?? Date.now())) / 1000 || 86400
  const pct   = (ttlSeconds(row.ExpiresAt) / total) * 100
  if (pct > 50) return 'ok'
  if (pct > 25) return 'warn'
  return 'danger'
}

async function copy(text) {
  try { await navigator.clipboard.writeText(text) } catch { /* silent */ }
}

async function confirmClear() {
  if (!window.confirm('This will remove all active leases. Clients will need to re-request IPs.')) return
  await window['go']['main']['App']['ClearLeases']()
  leases.value = []
}

const subtitle = computed(() => `${leases.value.length} lease${leases.value.length === 1 ? '' : 's'}`)
</script>

<template>
  <Panel title="Active leases" :subtitle="subtitle">
    <template #actions>
      <KebabMenu
        :items="[{ key: 'clear', label: 'Clear all leases', danger: true, disabled: leases.length === 0 }]"
        @select="(k) => k === 'clear' && confirmClear()"
      />
    </template>

    <EmptyState
      v-if="!initialLoading && leases.length === 0"
      title="No active leases"
      description="Clients that obtain an IP from this server will appear here."
    />

    <table v-else class="tbl">
      <thead>
        <tr>
          <th style="width:180px">MAC</th>
          <th style="width:140px">IP</th>
          <th>Hostname</th>
          <th style="width:170px">Expires</th>
          <th style="width:100px">TTL</th>
        </tr>
      </thead>
      <tbody>
        <tr v-for="(r, i) in leases" :key="i">
          <td>
            <span class="mono copyable" @click="copy(r.MAC)" :title="`Copy ${r.MAC}`">{{ r.MAC }}</span>
            <span v-if="r.ClientID" class="client-id mono muted copyable" @click="copy(r.ClientID)" :title="`Client-ID: ${r.ClientID}`">id:{{ r.ClientID }}</span>
          </td>
          <td>
            <span class="mono accent copyable" @click="copy(r.IP)" :title="`Copy ${r.IP}`">{{ r.IP }}</span>
          </td>
          <td>{{ r.Hostname }}</td>
          <td><span class="mono muted">{{ r.ExpiresAt.toLocaleString() }}</span></td>
          <td><span class="ttl" :data-variant="ttlVariant(r)">{{ ttlText(r.ExpiresAt) }}</span></td>
        </tr>
      </tbody>
    </table>

  </Panel>
</template>

<style scoped>
.client-id {
  display: block;
  font-size: var(--fs-11, 11px);
  opacity: 0.6;
}
</style>
