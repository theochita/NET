<script setup>
import { ref, onMounted, onUnmounted } from 'vue'

const visible = ref(false)

const shortcuts = [
  { keys: ['?'], desc: 'Toggle this help' },
  { keys: ['Esc'], desc: 'Collapse the log dock' },
  { keys: ['g', 'd'], desc: 'Go to DHCP Server' },
  { keys: ['g', 't'], desc: 'Go to TFTP Server' },
  { keys: ['g', 's'], desc: 'Go to Syslog Server' },
]

let gPending = false
let gTimer = null

function onKey(e) {
  const tag = (e.target && e.target.tagName) || ''
  if (tag === 'INPUT' || tag === 'TEXTAREA' || e.target.isContentEditable) return

  if (e.key === '?') {
    visible.value = !visible.value
    e.preventDefault()
    return
  }
  if (e.key === 'Escape' && visible.value) {
    visible.value = false
    e.preventDefault()
    return
  }
  if (e.key === 'g' && !gPending) {
    gPending = true
    clearTimeout(gTimer)
    gTimer = setTimeout(() => { gPending = false }, 800)
    return
  }
  if (gPending) {
    if (e.key === 'd') { window.location.hash = '#/dhcp'; gPending = false }
    else if (e.key === 't') { window.location.hash = '#/tftp'; gPending = false }
    else if (e.key === 's') { window.location.hash = '#/syslog'; gPending = false }
  }
}

onMounted(() => window.addEventListener('keydown', onKey))
onUnmounted(() => window.removeEventListener('keydown', onKey))
</script>

<template>
  <transition name="route-fade">
    <div v-if="visible" class="overlay" @click.self="visible = false">
      <div class="sheet" role="dialog" aria-label="Keyboard shortcuts">
        <h3>Keyboard shortcuts</h3>
        <ul>
          <li v-for="(s, i) in shortcuts" :key="i">
            <span class="keys">
              <kbd v-for="k in s.keys" :key="k">{{ k }}</kbd>
            </span>
            <span class="desc">{{ s.desc }}</span>
          </li>
        </ul>
        <div class="foot">Press <kbd>?</kbd> to close</div>
      </div>
    </div>
  </transition>
</template>

<style scoped>
.overlay {
  position: fixed;
  inset: 0;
  background: rgba(0, 0, 0, 0.5);
  display: flex;
  align-items: center;
  justify-content: center;
  z-index: 1000;
}
.sheet {
  background: var(--surface-2);
  border: 1px solid var(--border-subtle);
  border-radius: var(--r-xl);
  padding: var(--sp-5) var(--sp-6);
  min-width: 360px;
  max-width: 480px;
}
h3 { font-size: var(--fs-15); margin: 0 0 var(--sp-4); color: var(--text-primary); }
ul { list-style: none; padding: 0; margin: 0; display: flex; flex-direction: column; gap: var(--sp-2); }
li { display: flex; justify-content: space-between; align-items: center; gap: var(--sp-4); }
.keys { display: flex; gap: var(--sp-1); }
kbd {
  background: var(--surface-3);
  border: 1px solid var(--border-strong);
  border-radius: var(--r-sm);
  padding: 2px 6px;
  font-family: var(--font-mono);
  font-size: var(--fs-11);
  color: var(--text-primary);
}
.desc { color: var(--text-secondary); font-size: var(--fs-12); }
.foot {
  margin-top: var(--sp-4);
  padding-top: var(--sp-3);
  border-top: 1px solid var(--border-subtle);
  color: var(--text-tertiary);
  font-size: var(--fs-11);
  text-align: center;
}
</style>
