<script setup>
import { computed } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { useTheme } from '../stores/useTheme.js'
import { useServerStatus } from '../stores/useServerStatus.js'

const route = useRoute()
const router = useRouter()
const { theme, toggle } = useTheme()
const { dhcpRunning, tftpRunning, syslogRunning } = useServerStatus()

const items = computed(() => [
  { key: 'dhcp',   label: 'DHCP Server',   to: '/dhcp',   running: dhcpRunning.value },
  { key: 'tftp',   label: 'TFTP Server',   to: '/tftp',   running: tftpRunning.value },
  { key: 'syslog', label: 'Syslog Server', to: '/syslog', running: syslogRunning.value },
])

const activeKey = computed(() => route.path.replace(/^\//, ''))
const version = import.meta.env.VITE_APP_VERSION || 'dev'

function go(to) { router.push(to) }
</script>

<template>
  <aside class="sidebar">
    <div class="brand">NET</div>

    <nav class="nav">
      <button
        v-for="it in items"
        :key="it.key"
        class="nav-item"
        :class="{ active: activeKey === it.key }"
        @click="go(it.to)"
      >
        <span class="dot" :class="{ on: it.running }" aria-hidden="true"></span>
        <span class="label">{{ it.label }}</span>
      </button>
    </nav>

    <div class="foot">
      <button class="theme-btn" @click="toggle" :aria-label="`Switch to ${theme === 'dark' ? 'light' : 'dark'} theme`">
        <svg v-if="theme === 'dark'" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.8" stroke-linecap="round" stroke-linejoin="round">
          <circle cx="12" cy="12" r="4"/><path d="M12 2v2M12 20v2M2 12h2M20 12h2M4.9 4.9l1.4 1.4M17.7 17.7l1.4 1.4M4.9 19.1l1.4-1.4M17.7 6.3l1.4-1.4"/>
        </svg>
        <svg v-else viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.8" stroke-linecap="round" stroke-linejoin="round">
          <path d="M21 12.8A9 9 0 1 1 11.2 3a7 7 0 0 0 9.8 9.8z"/>
        </svg>
        <span>{{ theme === 'dark' ? 'Light' : 'Dark' }}</span>
      </button>
      <div class="version">v{{ version }}</div>
    </div>
  </aside>
</template>

<style scoped>
.sidebar {
  width: var(--sidebar-w);
  background: var(--surface-1);
  border-right: 1px solid var(--border-subtle);
  display: flex;
  flex-direction: column;
  flex-shrink: 0;
  padding: var(--sp-3) var(--sp-2);
}
.brand {
  color: var(--text-primary);
  font-weight: 700;
  font-size: var(--fs-11);
  text-transform: uppercase;
  letter-spacing: 0.1em;
  padding: var(--sp-2) var(--sp-3) var(--sp-4);
}
.nav {
  display: flex;
  flex-direction: column;
  gap: 2px;
  flex: 1;
}
.nav-item {
  display: flex;
  align-items: center;
  gap: var(--sp-2);
  padding: var(--sp-2) var(--sp-3);
  border: none;
  background: transparent;
  color: var(--text-secondary);
  font-size: var(--fs-13);
  font-family: var(--font-ui);
  text-align: left;
  border-radius: var(--r-md);
  cursor: pointer;
  transition: background var(--t-fast), color var(--t-fast);
}
.nav-item:hover {
  background: var(--surface-3);
  color: var(--text-primary);
}
.nav-item.active {
  background: var(--surface-3);
  color: var(--text-primary);
  font-weight: 500;
}
.dot {
  width: 6px;
  height: 6px;
  border-radius: 50%;
  background: var(--text-tertiary);
  flex-shrink: 0;
  transition: background var(--t-fast), box-shadow var(--t-fast);
}
.dot.on {
  background: var(--success);
  box-shadow: 0 0 0 3px var(--success-subtle);
}
.foot {
  display: flex;
  flex-direction: column;
  gap: var(--sp-2);
  padding: var(--sp-2) var(--sp-3);
  border-top: 1px solid var(--border-subtle);
  margin-top: var(--sp-3);
}
.theme-btn {
  display: flex;
  align-items: center;
  gap: var(--sp-2);
  background: transparent;
  border: none;
  color: var(--text-secondary);
  font-size: var(--fs-12);
  font-family: var(--font-ui);
  padding: var(--sp-1) 0;
  cursor: pointer;
}
.theme-btn:hover { color: var(--text-primary); }
.theme-btn svg { width: 14px; height: 14px; }
.version {
  font-size: var(--fs-11);
  color: var(--text-tertiary);
  font-family: var(--font-mono);
}
</style>
