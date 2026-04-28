import { ref, computed, onUnmounted } from 'vue'

const dhcpRunning = ref(false)
const tftpRunning = ref(false)
const syslogRunning = ref(false)
const dhcpStartedAt = ref(null)
const tftpStartedAt = ref(null)
const syslogStartedAt = ref(null)

let pollTimer = null
let refCount = 0

async function poll() {
  try {
    const [d, t, sy] = await Promise.all([
      window['go']['main']['App']['IsDHCPRunning'](),
      window['go']['main']['App']['IsTFTPRunning'](),
      window['go']['main']['App']['IsSyslogRunning'](),
    ])
    dhcpRunning.value = !!d
    tftpRunning.value = !!t
    syslogRunning.value = !!sy

    const [ds, ts, sys] = await Promise.all([
      window['go']['main']['App']['DHCPStartedAt'](),
      window['go']['main']['App']['TFTPStartedAt'](),
      window['go']['main']['App']['SyslogStartedAt'](),
    ])
    // Go's time.Time zero value serialises to "0001-01-01T00:00:00Z"
    dhcpStartedAt.value = ds?.startsWith('0001-') ? null : ds
    tftpStartedAt.value = ts?.startsWith('0001-') ? null : ts
    syslogStartedAt.value = sys?.startsWith('0001-') ? null : sys
  } catch {
    // backend not ready — leave values as-is
  }
}

function ensurePolling() {
  if (pollTimer) return
  poll()
  pollTimer = setInterval(poll, 2000)
}

function stopPolling() {
  if (pollTimer) {
    clearInterval(pollTimer)
    pollTimer = null
  }
}

export function useServerStatus() {
  refCount++
  ensurePolling()

  onUnmounted(() => {
    refCount--
    if (refCount <= 0) stopPolling()
  })

  const runningCount = computed(() =>
    (dhcpRunning.value ? 1 : 0) + (tftpRunning.value ? 1 : 0) + (syslogRunning.value ? 1 : 0)
  )

  return {
    dhcpRunning,
    tftpRunning,
    syslogRunning,
    dhcpStartedAt,
    tftpStartedAt,
    syslogStartedAt,
    runningCount,
    refresh: poll,
  }
}
