<script setup>
import { ref, computed, onMounted, onUnmounted } from 'vue'
import DHCPHeader from './DHCPHeader.vue'
import DHCPScopePanel from './DHCPScopePanel.vue'
import DHCPOptionsPanel from './DHCPOptionsPanel.vue'
import DHCPLeasesPanel from './DHCPLeasesPanel.vue'
import StatCard from '../ui/StatCard.vue'
import { useServerStatus } from '../../stores/useServerStatus.js'

const isRunning = ref(false)
const config = ref({})
const leaseCount = ref(0)
const now = ref(Date.now())

const { dhcpStartedAt, refresh } = useServerStatus()

let nowTimer = null
let leasePollTimer = null

onMounted(async () => {
  await loadConfig()
  isRunning.value = await window['go']['main']['App']['IsDHCPRunning']()
  refresh()
  nowTimer = setInterval(() => { now.value = Date.now() }, 1000)
  leasePollTimer = setInterval(fetchLeaseCount, 2000)
  fetchLeaseCount()
})

onUnmounted(() => {
  clearInterval(nowTimer)
  clearInterval(leasePollTimer)
})

async function loadConfig() {
  const loaded = await window['go']['main']['App']['GetConfig']()
  config.value = loaded ?? {}
}

async function fetchLeaseCount() {
  try {
    const leases = await window['go']['main']['App']['GetLeases']()
    leaseCount.value = (leases ?? []).length
  } catch { /* silent */ }
}

function saveConfig(cfg) {
  config.value = { ...cfg }
  window['go']['main']['App']['SaveConfig'](cfg)
}

const uptime = computed(() => {
  if (!isRunning.value || !dhcpStartedAt.value) return '—'
  const started = new Date(dhcpStartedAt.value).getTime()
  const s = Math.max(0, Math.floor((now.value - started) / 1000))
  const h = Math.floor(s / 3600)
  const m = Math.floor((s % 3600) / 60)
  const ss = s % 60
  return `${h}:${String(m).padStart(2, '0')}:${String(ss).padStart(2, '0')}`
})

const poolSize = computed(() => {
  const s = ipToInt(config.value.PoolStart)
  const e = ipToInt(config.value.PoolEnd)
  if (s === null || e === null) return 0
  return Math.max(0, e - s + 1)
})

const poolUsagePct = computed(() => {
  if (poolSize.value === 0) return 0
  return Math.round((leaseCount.value / poolSize.value) * 100)
})

const poolSublabel = computed(() => poolSize.value > 0 ? `${leaseCount.value} / ${poolSize.value}` : 'no pool set')

function ipToInt(ip) {
  if (!ip) return null
  const parts = ip.split('.').map(Number)
  if (parts.length !== 4 || parts.some((p) => isNaN(p) || p < 0 || p > 255)) return null
  return (parts[0] << 24 | parts[1] << 16 | parts[2] << 8 | parts[3]) >>> 0
}
</script>

<template>
  <div class="dash">
    <DHCPHeader v-model="isRunning" :config="config" @update:config="saveConfig" />

    <div class="stats">
      <StatCard label="Active leases" :value="leaseCount" />
      <StatCard
        label="Pool usage"
        :value="poolSize > 0 ? `${poolUsagePct}%` : '—'"
        :progress="poolUsagePct"
        :sublabel="poolSublabel"
      />
      <StatCard label="Uptime" :value="uptime" :sublabel="isRunning ? 'running' : 'stopped'" />
      <StatCard label="Interface" :value="config.Interface || '—'" />
    </div>

    <div class="panels">
      <DHCPScopePanel :config="config" :disabled="isRunning" @update:config="saveConfig" />
      <DHCPOptionsPanel :config="config" :disabled="isRunning" @update:config="saveConfig" />
      <DHCPLeasesPanel />
    </div>
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
.panels {
  display: flex;
  flex-direction: column;
  gap: var(--sp-4);
}
@media (max-width: 900px) {
  .stats { grid-template-columns: repeat(2, 1fr); }
}
</style>
