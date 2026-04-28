<script setup>
import AppTopBar from './AppTopBar.vue'
import AppSidebar from './AppSidebar.vue'
import LogDock from './LogDock.vue'
import ShortcutHelp from './ShortcutHelp.vue'
</script>

<template>
  <div class="shell">
    <AppTopBar />
    <div class="body">
      <AppSidebar />
      <div class="main-col">
        <main class="content">
          <router-view v-slot="{ Component }">
            <transition name="route-fade" mode="out-in">
              <component :is="Component" />
            </transition>
          </router-view>
        </main>
        <LogDock />
      </div>
    </div>
    <ShortcutHelp />
  </div>
</template>

<style scoped>
.shell {
  display: flex;
  flex-direction: column;
  height: 100vh;
  width: 100vw;
  background: var(--surface-0);
  color: var(--text-primary);
}
.body {
  display: flex;
  flex: 1;
  min-height: 0;
}
.main-col {
  display: flex;
  flex-direction: column;
  flex: 1;
  min-width: 0;
}
.content {
  flex: 1;
  min-height: 0;
  overflow: auto;
  padding: var(--sp-5) var(--sp-6);
  display: flex;
  flex-direction: column;
  align-items: center;
}
.content > :deep(*) {
  width: 100%;
  max-width: var(--content-max);
}
</style>
