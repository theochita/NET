<script setup>
import { ref, onMounted, onUnmounted, computed } from 'vue'
import Panel from '../ui/Panel.vue'
import EmptyState from '../ui/EmptyState.vue'
import KebabMenu from '../ui/KebabMenu.vue'

const active  = ref(new Map())
const history = ref([])
const filter  = ref('')

onMounted(async () => {
  const a = await window['go']['main']['App']['GetActiveTransfers']()
  for (const t of a ?? []) active.value.set(t.ID, t)
  history.value = await window['go']['main']['App']['GetTransferHistory']() ?? []
  window.runtime?.EventsOn('tftp:transfer', onTransfer)
})
onUnmounted(() => { window.runtime?.EventsOff('tftp:transfer') })

function onTransfer(t) {
  if (t.Status === 'active') {
    active.value.set(t.ID, { ...t })
    active.value = new Map(active.value)
  } else {
    active.value.delete(t.ID)
    active.value = new Map(active.value)
    history.value = [t, ...history.value].slice(0, 50)
  }
}

async function clearHistory() {
  await window['go']['main']['App']['ClearTransferHistory']()
  history.value = []
}

const activeRows = computed(() => Array.from(active.value.values()))

const filteredHistory = computed(() => {
  if (!filter.value.trim()) return history.value
  const q = filter.value.toLowerCase()
  return history.value.filter((t) =>
    (t.Peer || '').toLowerCase().includes(q) ||
    (t.Filename || '').toLowerCase().includes(q),
  )
})

function percent(t) {
  if (!t.Size || t.Size <= 0) return null
  return Math.min(100, Math.floor((t.Bytes / t.Size) * 100))
}
function fmtBytes(n) {
  if (n < 1024) return `${n} B`
  if (n < 1024 * 1024) return `${(n / 1024).toFixed(1)} KiB`
  return `${(n / (1024 * 1024)).toFixed(2)} MiB`
}
function fmtDuration(a, b) {
  const d = (new Date(b).getTime() - new Date(a).getTime()) / 1000
  if (!isFinite(d) || d < 0) return '—'
  if (d < 1) return `${Math.round(d * 1000)} ms`
  return `${d.toFixed(2)} s`
}
function fmtTime(s) { return s ? new Date(s).toLocaleTimeString() : '—' }
</script>

<template>
  <div class="stack">

    <Panel title="Active transfers" :subtitle="`${activeRows.length} in progress`">
      <EmptyState
        v-if="activeRows.length === 0"
        title="No transfers in progress"
        description="Transfers appear here while clients are GET-ing or PUT-ing files."
      />
      <table v-else class="tbl">
        <thead>
          <tr>
            <th style="width:180px">Peer</th>
            <th>File</th>
            <th style="width:60px">Dir</th>
            <th style="min-width:220px">Progress</th>
          </tr>
        </thead>
        <tbody>
          <tr v-for="r in activeRows" :key="r.ID">
            <td><span class="mono">{{ r.Peer }}</span></td>
            <td><span class="mono">{{ r.Filename }}</span></td>
            <td>
              <span class="tag" :data-v="r.Direction === 'read' ? 'success' : 'warning'">
                {{ r.Direction === 'read' ? 'R' : 'W' }}
              </span>
            </td>
            <td>
              <div v-if="percent(r) !== null" class="prog-row">
                <div class="prog" style="flex:1"><div class="fill" :style="{ width: `${percent(r)}%` }"></div></div>
                <span class="pct">{{ percent(r) }}%</span>
              </div>
              <span v-else class="mono muted">{{ fmtBytes(r.Bytes) }} / ?</span>
            </td>
          </tr>
        </tbody>
      </table>
    </Panel>

    <Panel title="History" :subtitle="`${history.length} recent`">
      <template #actions>
        <input
          v-model="filter"
          class="input"
          placeholder="Filter peer or filename"
          style="width:220px"
        />
        <KebabMenu
          :items="[{ key: 'clear', label: 'Clear history', danger: true, disabled: history.length === 0 }]"
          @select="(k) => k === 'clear' && clearHistory()"
        />
      </template>

      <EmptyState
        v-if="history.length === 0"
        title="No completed transfers"
        description="Finished transfers appear here (most recent first, up to 50)."
      />
      <table v-else class="tbl">
        <thead>
          <tr>
            <th style="width:100px">Time</th>
            <th style="width:170px">Peer</th>
            <th>File</th>
            <th style="width:60px">Dir</th>
            <th style="width:110px">Bytes</th>
            <th style="width:100px">Duration</th>
            <th style="width:90px">Status</th>
            <th>Error</th>
          </tr>
        </thead>
        <tbody>
          <tr v-for="r in filteredHistory" :key="r.ID">
            <td><span class="mono muted">{{ fmtTime(r.EndedAt) }}</span></td>
            <td><span class="mono">{{ r.Peer }}</span></td>
            <td><span class="mono">{{ r.Filename }}</span></td>
            <td>
              <span class="tag" :data-v="r.Direction === 'read' ? 'success' : 'warning'">
                {{ r.Direction === 'read' ? 'R' : 'W' }}
              </span>
            </td>
            <td><span class="mono">{{ fmtBytes(r.Bytes) }}</span></td>
            <td><span class="mono muted">{{ fmtDuration(r.StartedAt, r.EndedAt) }}</span></td>
            <td>
              <span class="tag" :data-v="r.Status === 'ok' ? 'success' : 'danger'">{{ r.Status }}</span>
            </td>
            <td class="err">{{ r.Error }}</td>
          </tr>
        </tbody>
      </table>
    </Panel>

  </div>
</template>

<style scoped>
.stack { display: flex; flex-direction: column; gap: var(--sp-4); }
.err { color: var(--text-secondary); font-size: var(--fs-12); }
</style>
