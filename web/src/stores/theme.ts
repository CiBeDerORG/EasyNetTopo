export type ThemeMode = 'system' | 'light' | 'dark'

const storageKey = 'easynettopo-theme-mode'
const darkMedia = '(prefers-color-scheme: dark)'

export function readThemeMode(): ThemeMode {
  try {
    const stored = window.localStorage.getItem(storageKey)
    return stored === 'light' || stored === 'dark' ? stored : 'system'
  } catch {
    return 'system'
  }
}

export function applyThemeMode(mode: ThemeMode) {
  if (typeof document === 'undefined' || typeof window === 'undefined') return
  const dark = mode === 'dark' || (mode === 'system' && window.matchMedia(darkMedia).matches)
  document.documentElement.dataset.theme = dark ? 'dark' : 'light'
  document.documentElement.style.colorScheme = dark ? 'dark' : 'light'
}

export function setThemeMode(mode: ThemeMode) {
  try {
    if (mode === 'system') window.localStorage.removeItem(storageKey)
    else window.localStorage.setItem(storageKey, mode)
  } catch {
    // 存储受限时，仍允许本次会话切换主题。
  }
  applyThemeMode(mode)
}

export function initializeTheme() {
  applyThemeMode(readThemeMode())
  const media = window.matchMedia(darkMedia)
  media.addEventListener('change', () => {
    if (readThemeMode() === 'system') applyThemeMode('system')
  })
}
