<script setup>
import { ElDropdown, ElDropdownMenu, ElDropdownItem } from 'element-plus'
import { MoreFilled } from '@element-plus/icons-vue'

defineProps({
  items: {
    // [{ key, label, danger?: bool, disabled?: bool }]
    type: Array,
    required: true,
  },
})
const emit = defineEmits(['select'])
function onCommand(key) { emit('select', key) }
</script>

<template>
  <ElDropdown trigger="click" @command="onCommand">
    <button class="kebab" aria-label="More actions" type="button">
      <MoreFilled />
    </button>
    <template #dropdown>
      <ElDropdownMenu>
        <ElDropdownItem
          v-for="it in items"
          :key="it.key"
          :command="it.key"
          :disabled="it.disabled"
        >
          <span :class="{ danger: it.danger }">{{ it.label }}</span>
        </ElDropdownItem>
      </ElDropdownMenu>
    </template>
  </ElDropdown>
</template>

<style scoped>
.kebab {
  width: 24px;
  height: 24px;
  display: inline-flex;
  align-items: center;
  justify-content: center;
  background: transparent;
  border: none;
  color: var(--text-secondary);
  border-radius: var(--r-sm);
  cursor: pointer;
  font-size: var(--fs-15);
  padding: 0;
}
.kebab:hover {
  background: var(--surface-3);
  color: var(--text-primary);
}
.danger { color: var(--danger); }
</style>
