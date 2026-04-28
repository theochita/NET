<script setup>
defineProps({
  width: { type: String, default: '100%' },
  height: { type: String, default: '14px' },
  radius: { type: String, default: 'var(--r-sm)' },
})
</script>

<template>
  <span class="skel" :style="{ width, height, borderRadius: radius }" aria-hidden="true"></span>
</template>

<style scoped>
.skel {
  display: inline-block;
  background: linear-gradient(90deg, var(--surface-3) 0%, var(--surface-2) 50%, var(--surface-3) 100%);
  background-size: 200% 100%;
  animation: shimmer 1.4s linear infinite;
}
@keyframes shimmer {
  0% { background-position: 200% 0; }
  100% { background-position: -200% 0; }
}
</style>
