const SECRET_KEY = 'blockmesh_admin_secret'
const VIEWER_KEY = 'blockmesh_viewer_session'

export type Role = 'admin' | 'viewer' | null

// Safely access sessionStorage in case the browser blocks it
function safeGetItem(key: string): string | null {
    try { return sessionStorage.getItem(key) } catch { return null }
}
function safeSetItem(key: string, value: string): void {
    try { sessionStorage.setItem(key, value) } catch {}
}
function safeRemoveItem(key: string): void {
    try { sessionStorage.removeItem(key) } catch {}
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