<script setup>
import { useServerStatus } from '../stores/useServerStatus.js'

const { runningCount } = useServerStatus()
</script>

<template>
  <header class="topbar">
    <div class="brand">
      <span class="glyph">◆</span>
      <span class="name">NET</span>
      <span class="tagline">Network engineering toolkit</span>
    </div>
    <div class="right">
      <span class="running-chip" v-if="runningCount > 0">
        <span class="dot"></span>
        {{ runningCount }} {{ runningCount === 1 ? 'server' : 'servers' }} running
      </span>
    </div>
  </header>
</template>

<style scoped>
.topbar {
  height: var(--topbar-h);
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 0 var(--sp-4);
  background: var(--surface-1);
  border-bottom: 1px solid var(--border-subtle);
  flex-shrink: 0;
  -webkit-app-region: drag; /* Wails: drag the app window from here */
}
.brand {
  display: flex;
  align-items: center;
  gap: var(--sp-2);
  font-size: var(--fs-11);
  color: var(--text-secondary);
  letter-spacing: 0.03em;
}
.glyph {
  color: var(--accent);
  font-size: var(--fs-13);
}
.name {
  color: var(--text-primary);
  font-weight: 600;
  letter-spacing: 0.08em;
}
.tagline {
  color: var(--text-tertiary);
  padding-left: var(--sp-2);
  border-left: 1px solid var(--border-subtle);
  margin-left: var(--sp-2);
}
.right {
  -webkit-app-region: no-drag;
}
.running-chip {
  display: inline-flex;
  align-items: center;
  gap: var(--sp-2);
  font-size: var(--fs-11);
  color: var(--success);
  background: var(--success-subtle);
  padding: 2px var(--sp-2);
  border-radius: 999px;
}
.running-chip .dot {
  width: 6px;
  height: 6px;
  border-radius: 50%;
  background: var(--success);
  box-shadow: 0 0 0 3px var(--success-subtle);
}
</style>
