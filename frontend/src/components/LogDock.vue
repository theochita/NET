<script setup>
import { ref, computed, onMounted, onUnmounted, nextTick, watch } from 'vue'

const MAX_LINES = 500

const expanded = ref(false)
const activeTab = ref('dhcp')
const paused = ref(false)
const height = ref(240)
const resizing = ref(false)

const dhcp = ref([])
const tftp = ref([])

const scrollerDhcp = ref(null)
const scrollerTftp = ref(null)

function addEntry(target, entry) {
  const store = target === 'dhcp' ? dhcp : tftp
  store.value.push({
    time: entry.Timestamp ? new Date(entry.Timestamp) : new Date(),
    level: (entry.Level || 'INFO').toUpperCase(),
    message: entry.Message || '',
  })
  if (store.value.length > MAX_LINES) {
    store.value.splice(0, store.value.length - MAX_LINES)
  }
  if (!paused.value && expanded.value && activeTab.value === target) {
    nextTick(() => scrollToBottom(target))
  }
}

function scrollToBottom(target) {
  const el = target === 'dhcp' ? scrollerDhcp.value : scrollerTftp.value
  if (el) el.scrollTop = el.scrollHeight
}

function toggle() {
  expanded.value = !expanded.value
  if (expanded.value) {
    nextTick(() => scrollToBottom(activeTab.value))
  }
}

function onKey(e) {
  if (e.key === 'Escape' && expanded.value) {
    expanded.value = false
  }
}

function clearActive() {
  if (activeTab.value === 'dhcp') dhcp.value = []
  else tftp.value = []
}

function copyActive() {
  const store = activeTab.value === 'dhcp' ? dhcp.value : tftp.value
  const text = store
    .map((e) => `${fmtTime(e.time)}  ${e.level}  ${e.message}`)
    .join('\n')
  navigator.clipboard?.writeText(text)
}

function fmtTime(d) {
  const h = String(d.getHours()).padStart(2, '0')
  const m = String(d.getMinutes()).padStart(2, '0')
  const s = String(d.getSeconds()).padStart(2, '0')
  return `${h}:${m}:${s}`
}

function startResize(e) {
  resizing.value = true
  const startY = e.clientY
  const startH = height.value
  const maxH = Math.floor(window.innerHeight * 0.6)
  function onMove(ev) {
    const dy = startY - ev.clientY
    height.value = Math.min(maxH, Math.max(80, startH + dy))
  }
  function onUp() {
    resizing.value = false
    document.removeEventListener('mousemove', onMove)
    document.removeEventListener('mouseup', onUp)
  }
  document.addEventListener('mousemove', onMove)
  document.addEventListener('mouseup', onUp)
}

const onDhcp = (e) => addEntry('dhcp', e)
const onTftp = (e) => addEntry('tftp', e)

onMounted(() => {
  window.runtime?.EventsOn('dhcp:log', onDhcp)
  window.runtime?.EventsOn('tftp:log', onTftp)
  window.addEventListener('keydown', onKey)
})

onUnmounted(() => {
  window.runtime?.EventsOff('dhcp:log')
  window.runtime?.EventsOff('tftp:log')
  window.removeEventListener('keydown', onKey)
})

watch([expanded, activeTab], () => {
  if (expanded.value) nextTick(() => scrollToBottom(activeTab.value))
})

const dhcpCount = computed(() => dhcp.value.length)
const tftpCount = computed(() => tftp.value.length)
</script>

<template>
  <div class="dock" :class="{ expanded }" :style="expanded ? { height: height + 'px' } : {}">
    <div v-if="expanded" class="resize-grip" @mousedown.prevent="startResize"></div>

    <div class="strip" @click="toggle" :aria-expanded="expanded" role="button" tabindex="0" @keydown.enter="toggle" @keydown.space.prevent="toggle">
      <span class="chev">{{ expanded ? '▾' : '▸' }}</span>
      <span class="title">Logs</span>
      <span class="counts">
        <span class="pill">DHCP · {{ dhcpCount }}</span>
        <span class="pill">TFTP · {{ tftpCount }}</span>
      </span>
      <span class="hint" v-if="!expanded">click to expand</span>
    </div>

    <div v-if="expanded" class="body">
      <div class="toolbar">
        <div class="tabs">
          <button
            class="tab"
            :class="{ active: activeTab === 'dhcp' }"
            @click.stop="activeTab = 'dhcp'"
          >DHCP</button>
          <button
            class="tab"
            :class="{ active: activeTab === 'tftp' }"
            @click.stop="activeTab = 'tftp'"
          >TFTP</button>
        </div>
        <div class="tools">
          <label class="pause">
            <input type="checkbox" v-model="paused" />
            Pause
          </label>
          <button class="tool" @click.stop="copyActive">Copy</button>
          <button class="tool" @click.stop="clearActive">Clear</button>
        </div>
      </div>

      <div v-show="activeTab === 'dhcp'" ref="scrollerDhcp" class="scroller">
        <div v-for="(e, i) in dhcp" :key="i" class="line" :data-level="e.level">
          <span class="t">{{ fmtTime(e.time) }}</span>
          <span class="lv">{{ e.level }}</span>
          <span class="msg">{{ e.message }}</span>
        </div>
        <div v-if="dhcp.length === 0" class="empty">No DHCP events yet.</div>
      </div>

      <div v-show="activeTab === 'tftp'" ref="scrollerTftp" class="scroller">
        <div v-for="(e, i) in tftp" :key="i" class="line" :data-level="e.level">
          <span class="t">{{ fmtTime(e.time) }}</span>
          <span class="lv">{{ e.level }}</span>
          <span class="msg">{{ e.message }}</span>
        </div>
        <div v-if="tftp.length === 0" class="empty">No TFTP events yet.</div>
      </div>
    </div>
  </div>
</template>

<style scoped>
.dock {
  background: var(--surface-1);
  border-top: 1px solid var(--border-subtle);
  display: flex;
  flex-direction: column;
  flex-shrink: 0;
  position: relative;
  transition: height var(--t-med);
}
.dock:not(.expanded) {
  height: var(--dock-h-collapsed);
}
.resize-grip {
  position: absolute;
  top: -3px;
  left: 0;
  right: 0;
  height: 6px;
  cursor: ns-resize;
  z-index: 2;
}
.strip {
  height: var(--dock-h-collapsed);
  display: flex;
  align-items: center;
  gap: var(--sp-3);
  padding: 0 var(--sp-4);
  cursor: pointer;
  user-select: none;
  color: var(--text-secondary);
  font-size: var(--fs-12);
  flex-shrink: 0;
}
.strip:hover { color: var(--text-primary); }
.chev {
  font-size: var(--fs-11);
  width: 10px;
  display: inline-flex;
  justify-content: center;
}
.title {
  color: var(--text-primary);
  font-weight: 500;
  letter-spacing: 0.02em;
}
.counts {
  display: flex;
  gap: var(--sp-2);
  margin-left: var(--sp-2);
}
.pill {
  background: var(--surface-3);
  padding: 1px var(--sp-2);
  border-radius: var(--r-sm);
  font-family: var(--font-mono);
  font-size: var(--fs-11);
  color: var(--text-secondary);
}
.hint {
  margin-left: auto;
  color: var(--text-tertiary);
  font-size: var(--fs-11);
}
.body {
  display: flex;
  flex-direction: column;
  flex: 1;
  min-height: 0;
  border-top: 1px solid var(--border-subtle);
}
.toolbar {
  display: flex;
  justify-content: space-between;
  align-items: center;
  padding: var(--sp-2) var(--sp-4);
  border-bottom: 1px solid var(--border-subtle);
}
.tabs { display: flex; gap: var(--sp-1); }
.tab {
  background: transparent;
  border: none;
  padding: var(--sp-1) var(--sp-3);
  color: var(--text-secondary);
  font-family: var(--font-ui);
  font-size: var(--fs-12);
  border-radius: var(--r-sm);
  cursor: pointer;
}
.tab:hover { background: var(--surface-3); color: var(--text-primary); }
.tab.active { background: var(--surface-3); color: var(--text-primary); }
.tools { display: flex; align-items: center; gap: var(--sp-3); font-size: var(--fs-11); color: var(--text-secondary); }
.pause { display: inline-flex; align-items: center; gap: var(--sp-1); cursor: pointer; }
.tool {
  background: transparent;
  border: 1px solid var(--border-subtle);
  color: var(--text-secondary);
  padding: 2px var(--sp-2);
  border-radius: var(--r-sm);
  cursor: pointer;
  font-size: var(--fs-11);
}
.tool:hover { color: var(--text-primary); border-color: var(--border-strong); }
.scroller {
  flex: 1;
  overflow: auto;
  padding: var(--sp-2) var(--sp-4);
  font-family: var(--font-mono);
  font-size: var(--fs-12);
  line-height: 1.6;
}
.line {
  display: grid;
  grid-template-columns: 70px 60px 1fr;
  gap: var(--sp-2);
  color: var(--text-primary);
}
.line .t { color: var(--text-tertiary); }
.line .lv { color: var(--text-secondary); font-weight: 600; }
.line[data-level="ERROR"] .lv { color: var(--danger); }
.line[data-level="WARN"] .lv,
.line[data-level="WARNING"] .lv { color: var(--warn); }
.line[data-level="PACKET"] .lv { color: var(--accent); }
.empty {
  color: var(--text-tertiary);
  text-align: center;
  padding: var(--sp-6);
  font-family: var(--font-ui);
}
</style>
