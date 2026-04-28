<script setup>
defineProps({
  title: { type: String, required: true },
  subtitle: { type: String, default: '' },
})
</script>

<template>
  <section class="panel">
    <header class="panel-head">
      <div class="titles">
        <h3 class="title">{{ title }}</h3>
        <span v-if="subtitle" class="subtitle">{{ subtitle }}</span>
      </div>
      <div class="actions">
        <slot name="actions" />
      </div>
    </header>
    <div class="panel-body">
      <slot />
    </div>
  </section>
</template>

<style scoped>
.panel {
  background: var(--surface-2);
  border: 1px solid var(--border-subtle);
  border-radius: var(--r-lg);
  overflow: hidden;
}
.panel-head {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: var(--sp-3);
  padding: var(--sp-3) var(--sp-4);
  border-bottom: 1px solid var(--border-subtle);
}
.titles {
  display: flex;
  align-items: baseline;
  gap: var(--sp-3);
  min-width: 0;
}
.title {
  font-size: var(--fs-13);
  font-weight: 600;
  color: var(--text-primary);
  margin: 0;
}
.subtitle {
  font-size: var(--fs-11);
  color: var(--text-secondary);
}
.actions {
  display: flex;
  gap: var(--sp-2);
}
.panel-body {
  padding: var(--sp-4);
}
</style>
