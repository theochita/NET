<script setup>
defineProps({
  label: { type: String, required: true },
  value: { type: [String, Number], default: '—' },
  sublabel: { type: String, default: '' },
  progress: { type: Number, default: null }, // 0..100, or null for no bar
  trend: { type: String, default: '' },       // e.g. "+3 today"
  trendVariant: {
    type: String,
    default: 'neutral',
    validator: (v) => ['neutral', 'success', 'warn', 'danger'].includes(v),
  },
})
</script>

<template>
  <div class="stat-card">
    <div class="label">{{ label }}</div>
    <div class="value">{{ value }}</div>
    <div v-if="progress !== null" class="progress">
      <div class="fill" :style="{ width: `${Math.max(0, Math.min(100, progress))}%` }"></div>
    </div>
    <div v-if="sublabel" class="sub">{{ sublabel }}</div>
    <div v-if="trend" class="trend" :data-variant="trendVariant">{{ trend }}</div>
  </div>
</template>

<style scoped>
.stat-card {
  background: var(--surface-2);
  border: 1px solid var(--border-subtle);
  border-radius: var(--r-lg);
  padding: var(--sp-3) var(--sp-4);
  display: flex;
  flex-direction: column;
  gap: var(--sp-1);
  min-height: 84px;
}
.label {
  font-size: var(--fs-11);
  text-transform: uppercase;
  letter-spacing: 0.05em;
  color: var(--text-secondary);
  font-weight: 500;
}
.value {
  font-family: var(--font-mono);
  font-size: var(--fs-18);
  color: var(--text-primary);
  font-weight: 600;
  font-variant-numeric: tabular-nums;
  line-height: 1.1;
  margin-top: 2px;
}
.progress {
  height: 4px;
  border-radius: 2px;
  background: var(--surface-3);
  overflow: hidden;
  margin-top: var(--sp-2);
}
.progress .fill {
  height: 100%;
  background: var(--accent);
  transition: width var(--t-med);
}
.sub {
  font-size: var(--fs-11);
  color: var(--text-tertiary);
  font-family: var(--font-mono);
}
.trend {
  font-size: var(--fs-11);
  font-weight: 500;
}
.trend[data-variant="success"] { color: var(--success); }
.trend[data-variant="warn"] { color: var(--warn); }
.trend[data-variant="danger"] { color: var(--danger); }
.trend[data-variant="neutral"] { color: var(--text-tertiary); }
</style>
