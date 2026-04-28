<script setup>
defineProps({
  variant: {
    type: String,
    default: 'neutral',
    validator: (v) => ['neutral', 'success', 'warn', 'danger', 'info'].includes(v),
  },
  label: { type: String, required: true },
})
</script>

<template>
  <span class="chip" :data-variant="variant">
    <span class="dot" aria-hidden="true"></span>
    <span class="label">{{ label }}</span>
  </span>
</template>

<style scoped>
.chip {
  display: inline-flex;
  align-items: center;
  gap: var(--sp-2);
  padding: 3px 10px;
  border-radius: 999px;
  font-size: var(--fs-11);
  font-weight: 600;
  letter-spacing: 0.03em;
  line-height: 1;
  font-family: var(--font-ui);
}
.dot {
  width: 6px;
  height: 6px;
  border-radius: 50%;
}
.chip[data-variant="neutral"] {
  background: var(--surface-3);
  color: var(--text-secondary);
}
.chip[data-variant="neutral"] .dot {
  background: var(--text-tertiary);
}
.chip[data-variant="success"] {
  background: var(--success-subtle);
  color: var(--success);
}
.chip[data-variant="success"] .dot {
  background: var(--success);
  box-shadow: 0 0 0 3px var(--success-subtle);
}
.chip[data-variant="warn"] {
  background: var(--warn-subtle);
  color: var(--warn);
}
.chip[data-variant="warn"] .dot {
  background: var(--warn);
}
.chip[data-variant="danger"] {
  background: var(--danger-subtle);
  color: var(--danger);
}
.chip[data-variant="danger"] .dot {
  background: var(--danger);
}
.chip[data-variant="info"] {
  background: var(--accent-subtle);
  color: var(--accent);
}
.chip[data-variant="info"] .dot {
  background: var(--accent);
}
</style>
