<script setup>
import { ref, nextTick } from 'vue'

const props = defineProps({
  values: { type: Array, default: () => [] },
  placeholder: { type: String, default: '' },
})
const emit = defineEmits(['update:values'])

const text = ref('')
const editingIndex = ref(-1)
const editingValue = ref('')
const editInputRef = ref(null)

function add() {
  const v = text.value.trim()
  if (!v) return
  if (props.values.includes(v)) { text.value = ''; return }
  emit('update:values', [...props.values, v])
  text.value = ''
}

function remove(v) {
  emit('update:values', props.values.filter((x) => x !== v))
}

function onKeydown(e) {
  if (e.key === 'Enter' || e.key === ',') { e.preventDefault(); add() }
  if (e.key === 'Backspace' && !text.value && props.values.length) {
    remove(props.values[props.values.length - 1])
  }
}

function startEdit(i) {
  editingIndex.value = i
  editingValue.value = props.values[i]
  nextTick(() => editInputRef.value?.focus())
}

function commitEdit() {
  if (editingIndex.value < 0) return
  const v = editingValue.value.trim()
  if (v) {
    const copy = [...props.values]
    copy[editingIndex.value] = v
    emit('update:values', copy)
  }
  editingIndex.value = -1
}

function onEditKeydown(e) {
  if (e.key === 'Enter') { e.preventDefault(); commitEdit() }
  if (e.key === 'Escape') { e.preventDefault(); editingIndex.value = -1 }
}
</script>

<template>
  <div class="pills">
    <span class="pill" v-for="(v, i) in values" :key="i">
      <template v-if="editingIndex === i">
        <input
          ref="editInputRef"
          v-model="editingValue"
          class="pill-edit"
          @keydown="onEditKeydown"
          @blur="commitEdit"
        />
      </template>
      <template v-else>
        <span class="pill-label" @dblclick="startEdit(i)">{{ v }}</span>
        <button type="button" @click="remove(v)" :aria-label="`Remove ${v}`">×</button>
      </template>
    </span>
    <input
      v-model="text"
      :placeholder="values.length === 0 ? placeholder : ''"
      @keydown="onKeydown"
      @blur="add"
    />
  </div>
</template>

<style scoped>
.pills {
  display: flex;
  flex-wrap: wrap;
  gap: 4px;
  align-items: center;
  width: 100%;
  background: var(--surface-0);
  border: 1px solid var(--border-subtle);
  border-radius: var(--r-md);
  padding: 2px 6px;
  min-height: 28px;
  cursor: text;
}
.pills:focus-within { border-color: var(--accent); }
.pill {
  display: inline-flex;
  align-items: center;
  gap: 4px;
  background: var(--surface-3);
  border: 1px solid var(--border-strong);
  border-radius: var(--r-sm);
  padding: 0 6px;
  font-size: var(--fs-11);
  color: var(--text-primary);
  font-family: var(--font-mono);
}
.pill-label {
  cursor: default;
  user-select: none;
}
.pill-label:hover { text-decoration: underline dotted; cursor: text; }
.pill-edit {
  border: none;
  outline: none;
  background: transparent;
  font-size: var(--fs-11);
  font-family: var(--font-mono);
  color: var(--text-primary);
  width: 100px;
  padding: 0;
}
.pill button {
  background: none;
  border: none;
  color: var(--text-tertiary);
  cursor: pointer;
  font-size: 12px;
  line-height: 1;
  padding: 0;
}
.pill button:hover { color: var(--danger); }
input {
  flex: 1;
  min-width: 80px;
  background: transparent;
  border: none;
  outline: none;
  color: var(--text-primary);
  font-size: var(--fs-12);
  font-family: var(--font-ui);
  padding: 2px 0;
}
</style>
