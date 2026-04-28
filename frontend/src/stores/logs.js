import { ref, computed } from 'vue'

const MAX_ENTRIES = 200

function createLogStore(eventName) {
  const entries = ref([])
  const lastEntry = computed(() => entries.value[entries.value.length - 1] ?? null)

  function clear() {
    entries.value = []
  }

  window.runtime.EventsOn(eventName, (entry) => {
    entries.value.push(entry)
    if (entries.value.length > MAX_ENTRIES) entries.value.shift()
  })

  return { entries, lastEntry, clear }
}

export const dhcpLog = createLogStore('dhcp:log')
export const tftpLog = createLogStore('tftp:log')
