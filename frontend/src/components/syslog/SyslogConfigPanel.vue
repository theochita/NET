<script setup>
import { reactive, watch } from 'vue'
import Panel from '../ui/Panel.vue'

const props = defineProps({
  config:   { type: Object, default: () => ({}) },
  disabled: { type: Boolean, default: false },
})
const emit = defineEmits(['update:config'])

const cfg = reactive({ Interface: '', Port: 514 })

watch(() => props.config, (incoming) => {
  if (incoming && Object.keys(incoming).length > 0) Object.assign(cfg, incoming)
}, { immediate: true, deep: true })

let saveTimer = null
function scheduleSave() {
  clearTimeout(saveTimer)
  saveTimer = setTimeout(() => emit('update:config', { ...cfg }), 500)
}
</script>

<template>
  <Panel title="Config" :subtitle="disabled ? 'Read-only while server is running' : ''">
    <div v-if="disabled" class="ro">
      <div><span class="k">Port</span><span class="v">{{ cfg.Port || 514 }}</span></div>
      <div><span class="k">Protocol</span><span class="v">UDP</span></div>
    </div>
    <div v-else class="form">
      <div class="lbl">Port</div>
      <div class="ctl">
        <input
          type="number" min="1" max="65535"
          v-model.number="cfg.Port"
          placeholder="514"
          @input="e => { cfg.Port = Math.min(65535, Math.max(1, Number(e.target.value) || 514)); scheduleSave() }"
        />
      </div>
      <div class="lbl">Protocol</div>
      <div class="ctl"><span class="mono muted">UDP</span></div>
    </div>
  </Panel>
</template>

<style scoped>
.ro {
  display: flex;
  flex-direction: column;
  gap: var(--sp-2);
}
.ro > div {
  display: flex;
  justify-content: space-between;
  padding: var(--sp-1) 0;
  border-bottom: 1px solid var(--border-subtle);
  gap: var(--sp-4);
}
.ro .k { color: var(--text-secondary); font-size: var(--fs-12); }
.ro .v { color: var(--text-primary); font-family: var(--font-mono); font-size: var(--fs-12); }
.form {
  display: grid;
  grid-template-columns: 80px 1fr;
  align-items: center;
  gap: var(--sp-2) var(--sp-4);
}
.lbl { font-size: var(--fs-12); color: var(--text-secondary); }
.ctl { display: flex; align-items: center; gap: var(--sp-2); }
</style>
