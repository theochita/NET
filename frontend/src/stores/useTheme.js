import { ref, watch } from 'vue'

const STORAGE_KEY = 'net.theme'

function initial() {
  const saved = localStorage.getItem(STORAGE_KEY)
  if (saved === 'dark' || saved === 'light') return saved
  if (window.matchMedia?.('(prefers-color-scheme: dark)').matches) return 'dark'
  return 'light'
}

const theme = ref(initial())

function apply(value) {
  document.documentElement.setAttribute('data-theme', value)
}

apply(theme.value)

watch(theme, (value) => {
  apply(value)
  try {
    localStorage.setItem(STORAGE_KEY, value)
  } catch {
    // storage unavailable (private mode etc.) — not fatal
  }
})

export function useTheme() {
  return {
    theme,
    toggle() {
      theme.value = theme.value === 'dark' ? 'light' : 'dark'
    },
    set(value) {
      if (value === 'dark' || value === 'light') theme.value = value
    },
  }
}
