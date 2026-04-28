<script setup>
import { ref, computed, onMounted, onUnmounted, nextTick } from 'vue'

const props = defineProps({
  modelValue: { type: String, default: '' },
  options:    { type: Array, default: () => [] },
})
const emit = defineEmits(['update:modelValue'])

const open = ref(false)
const root = ref(null)
const menuStyle = ref({})

const label = computed(() => props.modelValue || props.options[0] || '')

function select(v) { emit('update:modelValue', v); open.value = false }

async function toggle() {
  open.value = !open.value
  if (open.value) {
    await nextTick()
    const r = root.value.getBoundingClientRect()
    menuStyle.value = {
      position: 'fixed',
      top: r.bottom + 3 + 'px',
      left: r.left + 'px',
      minWidth: r.width + 'px',
      zIndex: 9999,
    }
  }
}

function onOutside(e) { if (root.value && !root.value.contains(e.target)) open.value = false }
onMounted(() => document.addEventListener('mousedown', onOutside))
onUnmounted(() => document.removeEventListener('mousedown', onOutside))
</script>

<template>
  <div ref="root" class="sel" :class="{ open }" @click="toggle">
    <span class="val">{{ label }}</span>
    <span class="arrow">▾</span>
  </div>
  <Teleport to="body">
    <ul v-if="open" class="menu" :style="menuStyle">
      <li
        v-for="o in options" :key="o"
        :class="{ active: o === modelValue }"
        @mousedown.prevent="select(o)"
      >{{ o }}</li>
    </ul>
  </Teleport>
</template>

<style scoped>
.sel {
  position: relative;
  display: inline-flex;
  align-items: center;
  justify-content: space-between;
  gap: var(--sp-2);
  height: 28px;
  padding: 0 var(--sp-3);
  background: var(--surface-0);
  border: 1px solid var(--border-subtle);
  border-radius: var(--r-md);
  font-size: var(--fs-12);
  font-family: var(--font-ui);
  color: var(--text-primary);
  cursor: pointer;
  user-select: none;
  outline: none;
  box-sizing: border-box;
}
.sel:hover, .sel.open { border-color: var(--accent); }
.arrow { font-size: 10px; color: var(--text-tertiary); transition: transform .15s; }
.sel.open .arrow { transform: rotate(180deg); }
</style>

<style>
.menu {
  background: var(--surface-2);
  border: 1px solid var(--border-subtle);
  border-radius: var(--r-md);
  padding: var(--sp-1) 0;
  margin: 0;
  list-style: none;
  box-shadow: 0 4px 12px rgba(0,0,0,.25);
}
.menu li {
  padding: var(--sp-1) var(--sp-3);
  font-size: var(--fs-12);
  font-family: var(--font-ui);
  color: var(--text-primary);
  cursor: pointer;
  white-space: nowrap;
}
.menu li:hover { background: var(--surface-3); }
.menu li.active { color: var(--accent); }
</style>
