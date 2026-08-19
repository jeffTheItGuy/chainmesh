const SECRET_KEY = 'blockmesh_admin_secret'
const VIEWER_KEY = 'blockmesh_viewer_session'

export type Role = 'admin' | 'viewer' | null

function safeGetItem(key: string): string | null {
  if (typeof window === 'undefined') return null
  try {
    return window.sessionStorage.getItem(key)
  } catch (e) {
    console.warn('Storage access denied for key:', key, e)
    return null
  }
}

function safeSetItem(key: string, value: string): void {
  if (typeof window === 'undefined') return
  try {
    window.sessionStorage.setItem(key, value)
  } catch (e) {
    console.warn('Storage write denied for key:', key, e)
  }
}

function safeRemoveItem(key: string): void {
  if (typeof window === 'undefined') return
  try {
    window.sessionStorage.removeItem(key)
  } catch (e) {
    console.warn('Storage removal denied for key:', key, e)
  }
}

export function getStoredSecret(): string | null {
  return safeGetItem(SECRET_KEY)
}

export function storeSecret(secret: string): void {
  safeSetItem(SECRET_KEY, secret)
  safeRemoveItem(VIEWER_KEY)
}

export function storeViewerSession(): void {
  safeSetItem(VIEWER_KEY, '1')
  safeRemoveItem(SECRET_KEY)
}

export function getRole(): Role {
  if (getStoredSecret()) return 'admin'
  if (safeGetItem(VIEWER_KEY)) return 'viewer'
  return null
}

export function clearSession(): void {
  safeRemoveItem(SECRET_KEY)
  safeRemoveItem(VIEWER_KEY)
}
