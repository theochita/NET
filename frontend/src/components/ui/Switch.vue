<script setup>
defineProps({
  modelValue: { type: Boolean, default: false },
  disabled:   { type: Boolean, default: false },
})
const emit = defineEmits(['update:modelValue'])
</script>

<template>
  <label class="sw" :aria-disabled="disabled">
    <input
      type="checkbox"
      :checked="modelValue"
      :disabled="disabled"
      @change="(e) => !disabled && emit('update:modelValue', e.target.checked)"
    />
    <span class="track"></span>
    <span class="knob"></span>
  </label>
</template>

<style scoped>
.sw {
  position: relative;
  display: inline-block;
  width: 34px;
  height: 18px;
  flex-shrink: 0;
  cursor: pointer;
}
.sw[aria-disabled="true"] { opacity: 0.5; cursor: not-allowed; }
input { display: none; }
.track {
  position: absolute;
  inset: 0;
  background: var(--surface-3);
  border: 1px solid var(--border-subtle);
  border-radius: 999px;
  transition: background var(--t-fast), border-color var(--t-fast);
}
.knob {
  position: absolute;
  top: 2px;
  left: 2px;
  width: 12px;
  height: 12px;
  background: var(--text-secondary);
  border-radius: 50%;
  transition: all var(--t-fast);
  pointer-events: none;
}
input:checked + .track { background: var(--success); border-color: var(--success); }
input:checked + .track + .knob { left: 18px; background: #fff; }
</style>
