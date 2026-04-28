<script setup>
import { ref, computed, onMounted, onUnmounted } from 'vue'
import Panel from '../ui/Panel.vue'
import KebabMenu from '../ui/KebabMenu.vue'
import EmptyState from '../ui/EmptyState.vue'

const MAX = 1000

const messages      = ref([])
const severityFilter = ref('all')   // 'all' | 'warn' | 'error' | 'crit'
const hostFilter    = ref('all')
const keyword       = ref('')

// Incoming messages are buffered here and flushed into `messages` on a timer
// so Vue only re-renders at most every 100 ms instead of on every packet.
let pending = []
let flushTimer = null

// Syslog severity: 0=Emergency … 7=Debug. Lower number = more severe.
const SEVERITY_LABELS = ['EMERG', 'ALERT', 'CRIT', 'ERR', 'WARN', 'NOTICE', 'INFO', 'DEBUG']

// Maximum severity value to display (inclusive). Filter drops msg.Severity > threshold.
const SEVERITY_THRESHOLD = { all: 7, warn: 4, error: 3, crit: 2 }

onMounted(async () => {
  try {
    const result = await window['go']['main']['App']['GetSyslogMessages']()
    messages.value = result ?? []
  } catch { /* silent — show empty state */ }
  flushTimer = setInterval(flush, 100)
  window.runtime?.EventsOn('syslog:message', onMessage)
})

onUnmounted(() => {
  window.runtime?.EventsOff('syslog:message', onMessage)
  clearInterval(flushTimer)
  pending = []
})

function onMessage(msg) {
  pending.push(msg)
}

function flush() {
  if (pending.length === 0) return
  const batch = pending
  pending = []
  // prepend newest-first and trim to MAX in one splice so Vue sees one mutation
  messages.value.unshift(...batch.reverse())
  if (messages.value.length > MAX) messages.value.length = MAX
}

const uniqueHosts = computed(() => [...new Set(messages.value.map((m) => m.Hostname))].sort())

const filtered = computed(() => {
  const threshold = SEVERITY_THRESHOLD[severityFilter.value] ?? 7
  const host = hostFilter.value
  const kw   = keyword.value.toLowerCase()
  return messages.value.filter((m) => {
    if (m.Severity > threshold) return false
    if (host !== 'all' && m.Hostname !== host) return false
    if (kw && !m.Message.toLowerCase().includes(kw)) return false
    return true
  })
})

function severityClass(sev) {
  if (sev <= 2) return 'sev-crit'
  if (sev === 3) return 'sev-err'
  if (sev === 4) return 'sev-warn'
  if (sev <= 6) return 'sev-info'
  return 'sev-debug'
}

function fmtTime(ts) {
  const d = new Date(ts)
  const h = String(d.getUTCHours()).padStart(2, '0')
  const m = String(d.getUTCMinutes()).padStart(2, '0')
  const s = String(d.getUTCSeconds()).padStart(2, '0')
  return `${h}:${m}:${s}`
}

async function confirmClear() {
  if (!window.confirm('This will remove all syslog messages from memory.')) return
  await window['go']['main']['App']['ClearSyslogMessages']()
  messages.value = []
}

const subtitle = computed(() =>
  messages.value.length === 0 ? '' : `${filtered.value.length} of ${messages.value.length}`
)
</script>

<template>
  <Panel title="Messages" :subtitle="subtitle">
    <template #actions>
      <div class="filters">
        <select v-model="severityFilter" class="filter-sel">
          <option value="all">All severities</option>
          <option value="warn">≥ Warning</option>
          <option value="error">≥ Error</option>
          <option value="crit">≥ Critical</option>
        </select>
        <select v-model="hostFilter" class="filter-sel">
          <option value="all">All hosts</option>
          <option v-for="h in uniqueHosts" :key="h" :value="h">{{ h }}</option>
        </select>
        <input v-model="keyword" class="filter-kw" placeholder="Filter message…" />
      </div>
      <KebabMenu
        :items="[{ key: 'clear', label: 'Clear all messages', danger: true, disabled: messages.length === 0 }]"
        @select="(k) => k === 'clear' && confirmClear()"
      />
    </template>

    <EmptyState
      v-if="messages.length === 0"
      title="No messages yet"
      description="Messages from network devices will appear here once the server is running."
    />

    <table v-else class="tbl">
      <thead>
        <tr>
          <th style="width:70px">Time</th>
          <th style="width:70px">Severity</th>
          <th style="width:130px">Host</th>
          <th style="width:110px">Tag</th>
          <th>Message</th>
        </tr>
      </thead>
      <tbody>
        <tr v-for="(m, i) in filtered" :key="i">
          <td><span class="mono muted">{{ fmtTime(m.ReceivedAt) }}</span></td>
          <td><span class="sev" :class="severityClass(m.Severity)">{{ SEVERITY_LABELS[m.Severity] ?? m.Severity }}</span></td>
          <td><span class="mono">{{ m.Hostname }}</span></td>
          <td><span class="mono muted small">{{ m.Tag }}</span></td>
          <td>{{ m.Message }}</td>
        </tr>
      </tbody>
    </table>
  </Panel>
</template>

<style scoped>
.filters {
  display: flex;
  gap: var(--sp-2);
  align-items: center;
}
.filter-sel {
  font-size: var(--fs-12);
  padding: 2px var(--sp-2);
  background: var(--surface-3);
  border: 1px solid var(--border-subtle);
  border-radius: var(--r-sm);
  color: var(--text-primary);
  cursor: pointer;
}
.filter-kw {
  font-size: var(--fs-12);
  padding: 2px var(--sp-2);
  background: var(--surface-3);
  border: 1px solid var(--border-subtle);
  border-radius: var(--r-sm);
  color: var(--text-primary);
  width: 150px;
}
.filter-kw::placeholder { color: var(--text-tertiary); }
.sev { font-family: var(--font-mono); font-size: var(--fs-11); font-weight: 600; }
.sev-crit   { color: var(--danger); }
.sev-err    { color: var(--danger); opacity: 0.75; }
.sev-warn   { color: var(--warn); }
.sev-info   { color: var(--accent); }
.sev-debug  { color: var(--text-tertiary); }
.small { font-size: var(--fs-11); }
</style>
