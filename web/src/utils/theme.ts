export type AppTheme = 'light' | 'dark'

const THEME_COOKIE = 'theme'
const THEME_STORAGE_KEY = 'theme'

export function normalizeTheme(value: string | null | undefined): AppTheme {
  return value === 'dark' ? 'dark' : 'light'
}

export function readStoredTheme(): AppTheme {
  const cookieTheme = typeof document === 'undefined' ? null : getCookie(THEME_COOKIE)
  const localTheme = typeof localStorage === 'undefined' ? null : localStorage.getItem(THEME_STORAGE_KEY)

  return normalizeTheme(cookieTheme || localTheme)
}

export function applyTheme(theme: AppTheme) {
  if (typeof document === 'undefined') return

  document.documentElement.setAttribute('data-theme', theme)
  document.documentElement.classList.toggle('dark', theme === 'dark')
  document.documentElement.style.colorScheme = theme
}

export function persistTheme(theme: AppTheme) {
  if (typeof localStorage !== 'undefined') {
    localStorage.setItem(THEME_STORAGE_KEY, theme)
  }
  if (typeof document !== 'undefined') {
    setCookie(THEME_COOKIE, theme, 365)
  }
}

export function setStoredTheme(theme: AppTheme) {
  applyTheme(theme)
  persistTheme(theme)
}

function setCookie(name: string, value: string, daysUp: number) {
  const expires = new Date()
  expires.setTime(expires.getTime() + daysUp * 24 * 60 * 60 * 1000)
  document.cookie = `${name}=${value};expires=${expires.toUTCString()};path=/`
}

function getCookie(name: string) {
  const match = document.cookie.match(new RegExp(`(?:^|; )${name}=([^;]+)`))
  return match?.[1] ? decodeURIComponent(match[1]) : null
}
