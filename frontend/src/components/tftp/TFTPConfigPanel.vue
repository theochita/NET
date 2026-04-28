<script setup>
import { reactive, watch } from 'vue'
import Panel from '../ui/Panel.vue'
import Switch from '../ui/Switch.vue'

const props = defineProps({
  config:   { type: Object, default: () => ({}) },
  disabled: { type: Boolean, default: false },
})
const emit = defineEmits(['update:config'])

const cfg = reactive({
  Interface:    '',
  Root:         '',
  ReadEnabled:  true,
  WriteEnabled: true,
  BlockSize:    0,
})

watch(() => props.config, (incoming) => {
  if (incoming && Object.keys(incoming).length > 0) Object.assign(cfg, incoming)
}, { immediate: true, deep: true })

let saveTimer = null
function scheduleSave() {
  clearTimeout(saveTimer)
  saveTimer = setTimeout(() => emit('update:config', { ...cfg }), 300)
}

async function pickFolder() {
  try {
    const path = await window['go']['main']['App']['PickTFTPFolder']()
    if (path) {
      cfg.Root = path
      emit('update:config', { ...cfg })
    }
  } catch (e) {
    console.error('Failed to pick folder:', e)
  }
}

async function openFolder() {
  try {
    await window['go']['main']['App']['OpenTFTPFolder']()
  } catch (e) {
    alert(String(e))
  }
}
</script>

<template>
  <Panel title="Configuration" :subtitle="disabled ? 'Read-only while server is running' : ''">
    <div class="form">

      <div class="lbl">Root directory</div>
      <div class="ctl" style="gap:6px">
        <input class="root-input" :value="cfg.Root" readonly placeholder="No folder selected" />
        <button class="btn" :disabled="disabled" @click="pickFolder">📁 Browse…</button>
        <button class="btn" @click="openFolder">📂 Open</button>
      </div>

      <div class="lbl">Read</div>
      <div class="ctl">
        <Switch
          :modelValue="cfg.ReadEnabled"
          :disabled="disabled"
          @update:modelValue="(v) => { cfg.ReadEnabled = v; scheduleSave() }"
        />
        <span class="hint">Allow clients to GET files from this server</span>
      </div>

      <div class="lbl">Write</div>
      <div class="ctl">
        <Switch
          :modelValue="cfg.WriteEnabled"
          :disabled="disabled"
          @update:modelValue="(v) => { cfg.WriteEnabled = v; scheduleSave() }"
        />
        <span class="hint">Allow clients to PUT files to this server</span>
      </div>

      <div class="lbl">Block size</div>
      <div class="ctl">
        <input
          type="number" min="0" max="65464" step="512"
          :value="cfg.BlockSize"
          :disabled="disabled"
          @input="(e) => { cfg.BlockSize = Math.max(0, Number(e.target.value) || 0); scheduleSave() }"
        />
        <span class="hint">0 = negotiate with client (RFC 2348)</span>
      </div>

    </div>
  </Panel>
</template>

<style scoped>
.root-input {
  flex: 1;
  min-width: 0;
  background: var(--surface-0);
  border: 1px solid var(--border-subtle);
  color: var(--text-secondary);
  height: 28px;
  padding: 0 var(--sp-3);
  border-radius: var(--r-md);
  font-size: var(--fs-12);
  font-family: var(--font-mono);
  outline: none;
  cursor: default;
}
</style>
