const STORAGE_KEY = 'blockmesh_admin_secret'

// Session-only by design: closing the tab clears it, so there's nothing
// long-lived sitting in the browser. This is a UX gate, not the security
// boundary - the admin API enforces the real check on every request
// regardless of what the frontend does or doesn't send.
export function getStoredSecret(): string | null {
  return sessionStorage.getItem(STORAGE_KEY)
}

export function storeSecret(secret: string): void {
  sessionStorage.setItem(STORAGE_KEY, secret)
}

export function clearSecret(): void {
  sessionStorage.removeItem(STORAGE_KEY)
}
