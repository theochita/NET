<script setup>
import { ref, computed, watch } from 'vue'
import Panel from '../ui/Panel.vue'
import EmptyState from '../ui/EmptyState.vue'
import KebabMenu from '../ui/KebabMenu.vue'
import SelectInput from '../ui/SelectInput.vue'

const props = defineProps({
  config:   { type: Object, default: () => ({}) },
  disabled: { type: Boolean, default: false },
})
const emit = defineEmits(['update:config'])

const options = ref([])
watch(() => props.config.Options, (o) => {
  options.value = Array.isArray(o) ? [...o] : []
}, { immediate: true })

const draft = ref({ Code: '', Type: 'string', Value: '' })
const types = ['string', 'ip', 'ips', 'uint8', 'uint16', 'uint32', 'hex']

const canAdd = computed(() => {
  const code = Number(draft.value.Code)
  return code >= 1 && code <= 254 && draft.value.Value.trim().length > 0
})

function save() {
  emit('update:config', { ...props.config, Options: [...options.value] })
}

function addOption() {
  if (!canAdd.value) return
  options.value.push({ Code: Number(draft.value.Code), Type: draft.value.Type, Value: draft.value.Value })
  draft.value = { Code: '', Type: 'string', Value: '' }
  save()
}

function removeOption(idx) {
  options.value.splice(idx, 1)
  save()
}

const subtitle = computed(() => `${options.value.length} option${options.value.length === 1 ? '' : 's'}`)
</script>

<template>
  <Panel title="Custom options" :subtitle="subtitle">

    <EmptyState
      v-if="options.length === 0 && disabled"
      title="No custom options"
      description="Custom DHCP options (e.g. 66 TFTP server, 67 boot file) appear here when added."
    />

    <template v-else>
      <table v-if="options.length" class="tbl">
        <thead>
          <tr>
            <th style="width:80px">Code</th>
            <th style="width:110px">Type</th>
            <th>Value</th>
            <th v-if="!disabled" style="width:40px"></th>
          </tr>
        </thead>
        <tbody>
          <tr v-for="(o, i) in options" :key="i">
            <td><span class="mono">{{ o.Code }}</span></td>
            <td>{{ o.Type }}</td>
            <td><span class="mono">{{ o.Value }}</span></td>
            <td v-if="!disabled">
              <KebabMenu
                :items="[{ key: 'remove', label: 'Remove', danger: true }]"
                @select="(k) => k === 'remove' && removeOption(i)"
              />
            </td>
          </tr>
        </tbody>
      </table>

      <div v-if="!disabled" class="add-row">
        <input
          type="number" min="1" max="254"
          v-model="draft.Code"
          placeholder="Code"
          style="width:80px"
        />
        <SelectInput v-model="draft.Type" :options="types" style="width:110px" />
        <input
          v-model="draft.Value"
          placeholder="Value (e.g. 192.168.1.1 or pxelinux.0)"
          style="flex:1"
          @keyup.enter="addOption"
        />
        <button class="btn primary" :disabled="!canAdd" @click="addOption">+ Add</button>
      </div>
    </template>

  </Panel>
</template>
