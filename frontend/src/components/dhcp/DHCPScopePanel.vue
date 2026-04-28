<script setup>
import { reactive, computed, watch } from 'vue'
import Panel from '../ui/Panel.vue'
import PillInput from '../ui/PillInput.vue'

const props = defineProps({
  config:   { type: Object, default: () => ({}) },
  disabled: { type: Boolean, default: false },
})
const emit = defineEmits(['update:config'])

const cfg = reactive({
  Interface:    '',
  PoolStart:    '',
  PoolEnd:      '',
  LeaseTime:    86400,
  Router:       '',
  Mask:         '',
  DNS:          [],
  NTP:          [],
  DomainName:   '',
  DomainSearch: [],
  WINS:         [],
  BootFile:     '',
  TFTPServer:   '',
  Options:      [],
})

watch(() => props.config, (incoming) => {
  if (incoming && Object.keys(incoming).length > 0) Object.assign(cfg, incoming)
}, { immediate: true, deep: true })

const leaseHuman = computed(() => {
  const h = Math.floor(cfg.LeaseTime / 3600)
  const m = Math.floor((cfg.LeaseTime % 3600) / 60)
  if (h === 0) return `${m}m`
  if (m === 0) return `${h}h`
  return `${h}h ${m}m`
})

let saveTimer = null
function scheduleSave() {
  clearTimeout(saveTimer)
  saveTimer = setTimeout(() => emit('update:config', { ...cfg }), 500)
}

function setList(key, val) {
  cfg[key] = val
  scheduleSave()
}
</script>

<template>
  <Panel title="Scope" :subtitle="disabled ? 'Read-only while server is running' : ''">

    <!-- read-only view while running -->
    <div v-if="disabled" class="ro">
      <div><span class="k">Pool</span><span class="v">{{ cfg.PoolStart || '—' }} – {{ cfg.PoolEnd || '—' }}</span></div>
      <div><span class="k">Lease</span><span class="v">{{ leaseHuman }}</span></div>
      <div><span class="k">Router</span><span class="v">{{ cfg.Router || '—' }}</span></div>
      <div><span class="k">Mask</span><span class="v">{{ cfg.Mask || '—' }}</span></div>
      <div><span class="k">DNS</span><span class="v">{{ cfg.DNS?.join(', ') || '—' }}</span></div>
      <div><span class="k">NTP</span><span class="v">{{ cfg.NTP?.join(', ') || '—' }}</span></div>
      <div><span class="k">WINS</span><span class="v">{{ cfg.WINS?.join(', ') || '—' }}</span></div>
      <div><span class="k">Domain</span><span class="v">{{ cfg.DomainName || '—' }}</span></div>
      <div><span class="k">Domain search</span><span class="v">{{ cfg.DomainSearch?.join(', ') || '—' }}</span></div>
      <div><span class="k">Boot file</span><span class="v">{{ cfg.BootFile || '—' }}</span></div>
      <div><span class="k">TFTP server</span><span class="v">{{ cfg.TFTPServer || '—' }}</span></div>
    </div>

    <!-- editable form -->
    <div v-else class="form">
      <div class="lbl">Pool start</div>
      <div class="ctl">
        <input v-model="cfg.PoolStart" placeholder="192.168.1.100" @input="scheduleSave" />
      </div>

      <div class="lbl">Pool end</div>
      <div class="ctl">
        <input v-model="cfg.PoolEnd" placeholder="192.168.1.200" @input="scheduleSave" />
      </div>

      <div class="lbl">Lease ({{ leaseHuman }})</div>
      <div class="ctl">
        <input
          type="number" min="60" max="604800" step="3600"
          v-model.number="cfg.LeaseTime"
          @input="scheduleSave"
        />
        <span class="unit">seconds</span>
      </div>

      <div class="lbl">Router</div>
      <div class="ctl">
        <input v-model="cfg.Router" placeholder="192.168.1.1" @input="scheduleSave" />
      </div>

      <div class="lbl">Subnet mask</div>
      <div class="ctl">
        <input v-model="cfg.Mask" placeholder="255.255.255.0" @input="scheduleSave" />
      </div>

      <div class="lbl">DNS</div>
      <div class="ctl">
        <PillInput
          :values="cfg.DNS || []"
          placeholder="IP then Enter"
          @update:values="(v) => setList('DNS', v)"
        />
      </div>

      <div class="lbl">NTP</div>
      <div class="ctl">
        <PillInput
          :values="cfg.NTP || []"
          placeholder="IP then Enter"
          @update:values="(v) => setList('NTP', v)"
        />
      </div>

      <div class="lbl">WINS</div>
      <div class="ctl">
        <PillInput
          :values="cfg.WINS || []"
          placeholder="IP then Enter"
          @update:values="(v) => setList('WINS', v)"
        />
      </div>

      <div class="lbl">Domain name</div>
      <div class="ctl">
        <input v-model="cfg.DomainName" placeholder="corp.local" @input="scheduleSave" />
      </div>

      <div class="lbl">Domain search</div>
      <div class="ctl">
        <PillInput
          :values="cfg.DomainSearch || []"
          placeholder="domain then Enter"
          @update:values="(v) => setList('DomainSearch', v)"
        />
      </div>

      <div class="lbl">Boot file</div>
      <div class="ctl">
        <input v-model="cfg.BootFile" placeholder="pxelinux.0" @input="scheduleSave" />
      </div>

      <div class="lbl">TFTP server</div>
      <div class="ctl">
        <input v-model="cfg.TFTPServer" placeholder="192.168.1.1" @input="scheduleSave" />
      </div>
    </div>

  </Panel>
</template>

<style scoped>
.ro {
  display: grid;
  grid-template-columns: repeat(2, 1fr);
  gap: var(--sp-2) var(--sp-5);
  max-width: 640px;
}
.ro > div {
  display: flex;
  justify-content: space-between;
  padding: var(--sp-1) 0;
  border-bottom: 1px solid var(--border-subtle);
  gap: var(--sp-4);
}
.ro .k { color: var(--text-secondary); font-size: var(--fs-12); }
.ro .v { color: var(--text-primary); font-family: var(--font-mono); font-size: var(--fs-12); text-align: right; }
</style>
