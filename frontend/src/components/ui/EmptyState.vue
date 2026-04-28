<script setup>
defineProps({
  title: { type: String, required: true },
  description: { type: String, default: '' },
})
</script>

<template>
  <div class="empty">
    <div class="icon" aria-hidden="true">
      <slot name="icon">
        <svg width="28" height="28" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.5" stroke-linecap="round" stroke-linejoin="round">
          <path d="M3 7h18M3 12h18M3 17h18" />
        </svg>
      </slot>
    </div>
    <h4 class="title">{{ title }}</h4>
    <p v-if="description" class="desc">{{ description }}</p>
    <div class="action"><slot name="action" /></div>
  </div>
</template>

<style scoped>
.empty {
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  gap: var(--sp-2);
  padding: var(--sp-6) var(--sp-4);
  text-align: center;
  color: var(--text-secondary);
}
.icon {
  color: var(--text-tertiary);
  margin-bottom: var(--sp-1);
}
.title {
  font-size: var(--fs-13);
  font-weight: 600;
  color: var(--text-primary);
  margin: 0;
}
.desc {
  font-size: var(--fs-12);
  color: var(--text-secondary);
  margin: 0;
  max-width: 320px;
}
.action:not(:empty) {
  margin-top: var(--sp-2);
}
</style>
