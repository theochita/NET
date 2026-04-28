<script setup>
import { ref, computed, onMounted, onUnmounted } from 'vue'
import SyslogHeader       from './SyslogHeader.vue'
import SyslogConfigPanel  from './SyslogConfigPanel.vue'
import SyslogMessagesPanel from './SyslogMessagesPanel.vue'
import StatCard           from '../ui/StatCard.vue'
import { useServerStatus } from '../../stores/useServerStatus.js'

const isRunning  = ref(false)
const config     = ref({})
const messages   = ref([])   // ring snapshot — used only for stat card derivation
const totalCount = ref(0)    // session total from SyslogMessageCount()

const { refresh } = useServerStatus()
let pollTimer = null

onMounted(async () => {
  await loadConfig()
  isRunning.value = await window['go']['main']['App']['IsSyslogRunning']()
  refresh()
  await refreshStats()
  pollTimer = setInterval(refreshStats, 2000)
})

onUnmounted(() => {
  clearInterval(pollTimer)
})

async function loadConfig() {
  const loaded = await window['go']['main']['App']['GetSyslogConfig']()
  config.value = loaded ?? {}
}

function saveConfig(cfg) {
  config.value = { ...cfg }
  window['go']['main']['App']['SaveSyslogConfig'](cfg)
}

async function refreshStats() {
  try {
    const [msgs, count] = await Promise.all([
      window['go']['main']['App']['GetSyslogMessages'](),
      window['go']['main']['App']['SyslogMessageCount'](),
    ])
    messages.value   = msgs ?? []
    totalCount.value = typeof count === 'number' ? count : Number(count) || 0
  } catch { /* silent */ }
}

const uniqueHostCount = computed(() => new Set(messages.value.map((m) => m.Hostname)).size)
const errorCount      = computed(() => messages.value.filter((m) => m.Severity <= 3).length)
const lastTime        = computed(() => {
  if (!messages.value.length) return '—'
  const d = new Date(messages.value[0].ReceivedAt)
  const h = String(d.getUTCHours()).padStart(2, '0')
  const m = String(d.getUTCMinutes()).padStart(2, '0')
  const s = String(d.getUTCSeconds()).padStart(2, '0')
  return `${h}:${m}:${s}`
})
</script>

<template>
  <div class="dash">
    <SyslogHeader v-model="isRunning" :config="config" @update:config="saveConfig" />

    <div class="stats">
      <StatCard label="Messages received" :value="totalCount" sublabel="this session" />
      <StatCard label="Unique hosts"      :value="uniqueHostCount" />
      <StatCard
        label="Errors / Criticals"
        :value="errorCount"
        :trend-variant="errorCount > 0 ? 'danger' : 'neutral'"
      />
      <StatCard label="Last message" :value="lastTime" />
    </div>

    <div class="panels">
      <SyslogConfigPanel :config="config" :disabled="isRunning" @update:config="saveConfig" />
      <SyslogMessagesPanel />
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
.panels { display: flex; flex-direction: column; gap: var(--sp-4); }
@media (max-width: 900px) {
  .stats { grid-template-columns: repeat(2, 1fr); }
}
</style>
