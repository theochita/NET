<script setup>
import { ref, computed, onMounted, onUnmounted } from 'vue'
import TFTPHeader from './TFTPHeader.vue'
import TFTPConfigPanel from './TFTPConfigPanel.vue'
import TFTPTransfersPanel from './TFTPTransfersPanel.vue'
import StatCard from '../ui/StatCard.vue'
import { useServerStatus } from '../../stores/useServerStatus.js'

const isRunning = ref(false)
const config = ref({})
const activeCount = ref(0)
const historyCount = ref(0)
const sessionBytes = ref(0)
const now = ref(Date.now())

const { tftpStartedAt, refresh } = useServerStatus()

let nowTimer = null

onMounted(async () => {
  await loadConfig()
  isRunning.value = await window['go']['main']['App']['IsTFTPRunning']()
  refresh()
  nowTimer = setInterval(() => { now.value = Date.now() }, 1000)
  await refreshCounts()
  window.runtime?.EventsOn('tftp:transfer', onTransfer)
})
onUnmounted(() => {
  clearInterval(nowTimer)
  window.runtime?.EventsOff('tftp:transfer')
})

async function refreshCounts() {
  try {
    const active = await window['go']['main']['App']['GetActiveTransfers']()
    const history = await window['go']['main']['App']['GetTransferHistory']()
    activeCount.value = (active ?? []).length
    historyCount.value = (history ?? []).length
    sessionBytes.value = (history ?? []).reduce((sum, t) => sum + (t.Bytes || 0), 0)
  } catch { /* silent */ }
}

function onTransfer(t) {
  if (t.Status !== 'active') {
    historyCount.value += 1
    sessionBytes.value += t.Bytes || 0
  }
  refreshCounts()
}

async function loadConfig() {
  const loaded = await window['go']['main']['App']['GetTFTPConfig']()
  config.value = loaded ?? {}
}

function saveConfig(cfg) {
  config.value = { ...cfg }
  window['go']['main']['App']['SaveTFTPConfig'](cfg)
}

const uptime = computed(() => {
  if (!isRunning.value || !tftpStartedAt.value) return '—'
  const started = new Date(tftpStartedAt.value).getTime()
  const s = Math.max(0, Math.floor((now.value - started) / 1000))
  const h = Math.floor(s / 3600)
  const m = Math.floor((s % 3600) / 60)
  const ss = s % 60
  return `${h}:${String(m).padStart(2, '0')}:${String(ss).padStart(2, '0')}`
})

const bytesLabel = computed(() => {
  const n = sessionBytes.value
  if (n < 1024) return `${n} B`
  if (n < 1024 * 1024) return `${(n / 1024).toFixed(1)} KiB`
  if (n < 1024 * 1024 * 1024) return `${(n / (1024 * 1024)).toFixed(2)} MiB`
  return `${(n / (1024 * 1024 * 1024)).toFixed(2)} GiB`
})

const rootShort = computed(() => {
  const r = config.value.Root
  if (!r) return '—'
  const parts = r.split('/')
  return parts.length > 3 ? `…/${parts.slice(-2).join('/')}` : r
})

async function openRoot() {
  try {
    await window['go']['main']['App']['OpenTFTPFolder']()
  } catch (e) {
    alert(String(e))
  }
}
</script>

<template>
  <div class="dash">
    <TFTPHeader v-model="isRunning" :config="config" @update:config="saveConfig" />

    <div class="stats">
      <StatCard label="Active transfers" :value="activeCount" />
      <StatCard label="Session transfers" :value="historyCount" sublabel="since app start" />
      <StatCard label="Bytes transferred" :value="bytesLabel" sublabel="session total" />
      <div class="root-card" @click="openRoot" role="button" tabindex="0" :title="config.Root">
        <div class="label">Root directory</div>
        <div class="value">{{ rootShort }}</div>
        <div class="sub">click to open</div>
      </div>
    </div>

    <div class="panels">
      <TFTPConfigPanel :config="config" :disabled="isRunning" @update:config="saveConfig" />
      <TFTPTransfersPanel />
    </div>

    <div class="uptime-note" v-if="isRunning">Uptime: <span class="mono">{{ uptime }}</span></div>
  </div>
</template>

<style scoped>
.dash { display: flex; flex-direction: column; gap: 0; }
.stats {
  display: grid;
  grid-template-columns: repeat(4, 1fr);
  gap: var(--sp-3);
  margin-bottom: var(--sp-5);
}
.root-card {
  background: var(--surface-2);
  border: 1px solid var(--border-subtle);
  border-radius: var(--r-lg);
  padding: var(--sp-3) var(--sp-4);
  cursor: pointer;
  transition: border-color var(--t-fast);
}
.root-card:hover { border-color: var(--accent); }
.root-card .label {
  font-size: var(--fs-11);
  text-transform: uppercase;
  letter-spacing: 0.05em;
  color: var(--text-secondary);
  font-weight: 500;
}
.root-card .value {
  font-family: var(--font-mono);
  font-size: var(--fs-15);
  color: var(--text-primary);
  font-weight: 500;
  margin-top: 4px;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}
.root-card .sub {
  font-size: var(--fs-11);
  color: var(--text-tertiary);
}
.panels { display: flex; flex-direction: column; gap: var(--sp-4); }
.uptime-note {
  font-size: var(--fs-11);
  color: var(--text-secondary);
  margin-top: var(--sp-4);
  text-align: right;
}
.mono { font-family: var(--font-mono); }
@media (max-width: 900px) {
  .stats { grid-template-columns: repeat(2, 1fr); }
}
</style>
