<script setup>
import { ref, computed, onMounted, watch } from 'vue'
import StatusChip from '../ui/StatusChip.vue'

const props = defineProps({
  modelValue: { type: Boolean, default: false },
  config:     { type: Object, default: () => ({}) },
})
const emit = defineEmits(['update:modelValue', 'update:config'])

const interfaces        = ref([])
const selectedInterface = ref('')
const loading           = ref(false)
const boundIP           = ref('')
const startError        = ref('')

onMounted(async () => {
  try {
    interfaces.value = await window['go']['main']['App']['GetInterfaces']() || []
  } catch (e) {
    console.error('Failed to load interfaces:', e)
  }
})

watch(() => props.config.Interface, (iface) => {
  if (iface !== undefined && iface !== selectedInterface.value) selectedInterface.value = iface
}, { immediate: true })

function onInterfaceChange() {
  emit('update:config', { ...props.config, Interface: selectedInterface.value })
}

async function toggle() {
  loading.value = true
  startError.value = ''
  try {
    if (props.modelValue) {
      await window['go']['main']['App']['StopSyslog']()
      boundIP.value = ''
      emit('update:modelValue', false)
    } else {
      await window['go']['main']['App']['StartSyslog']()
      const iface = interfaces.value.find((i) => i.Name === selectedInterface.value)
      boundIP.value = iface?.IPs?.[0] ?? '0.0.0.0'
      emit('update:modelValue', true)
    }
  } catch (e) {
    startError.value = (typeof e === 'string' ? e : e?.message) || 'Failed to start Syslog server'
    emit('update:modelValue', false)
  } finally {
    loading.value = false
  }
}

const variant = computed(() => {
  if (loading.value) return 'warn'
  return props.modelValue ? 'success' : 'neutral'
})
const statusLabel = computed(() => {
  if (loading.value) return props.modelValue ? 'STOPPING' : 'STARTING'
  return props.modelValue ? 'RUNNING' : 'STOPPED'
})
const bindLine = computed(() => {
  const port    = props.config.Port || 514
  const ifLabel = selectedInterface.value || 'all interfaces'
  if (props.modelValue) return `${ifLabel} · ${boundIP.value || '0.0.0.0'} :${port}`
  return ifLabel
})
</script>

<template>
  <header class="hdr-wrap">
    <div class="hdr">
      <div class="left">
        <h1 class="title">Syslog Server</h1>
        <div class="bind">{{ bindLine }}</div>
      </div>
      <div class="right">
        <select
          v-if="!modelValue"
          v-model="selectedInterface"
          class="select"
          @change="onInterfaceChange"
        >
          <option value="">All interfaces</option>
          <option v-for="iface in interfaces" :key="iface.Name" :value="iface.Name">
            {{ iface.Name }} ({{ iface.IPs.join(', ') }})
          </option>
        </select>
        <StatusChip :variant="variant" :label="statusLabel" />
        <button
          class="btn"
          :class="modelValue ? 'danger' : 'primary'"
          :disabled="loading"
          @click="toggle"
        >
          {{ loading ? '…' : modelValue ? 'Stop' : 'Start' }}
        </button>
      </div>
    </div>
    <div v-if="startError" class="error-banner">{{ startError }}</div>
  </header>
</template>

<style scoped>
.hdr-wrap {
  padding-bottom: var(--sp-4);
  border-bottom: 1px solid var(--border-subtle);
  margin-bottom: var(--sp-5);
}
.hdr {
  display: flex;
  justify-content: space-between;
  align-items: center;
  gap: var(--sp-3);
}
.title {
  font-size: var(--fs-18);
  font-weight: 600;
  color: var(--text-primary);
  margin: 0;
  line-height: 1.2;
}
.bind {
  font-family: var(--font-mono);
  font-size: var(--fs-12);
  color: var(--text-secondary);
  margin-top: 2px;
}
.right {
  display: flex;
  align-items: center;
  gap: var(--sp-3);
}
.error-banner {
  font-size: var(--fs-12);
  font-family: var(--font-mono);
  color: var(--danger, #e53e3e);
  background: color-mix(in srgb, var(--danger, #e53e3e) 10%, transparent);
  border: 1px solid color-mix(in srgb, var(--danger, #e53e3e) 30%, transparent);
  border-radius: 4px;
  padding: 4px 8px;
  margin-top: var(--sp-2);
  width: 100%;
}
</style>
