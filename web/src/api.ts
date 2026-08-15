import { getStoredSecret } from './auth'

export interface Tenant {
  id: string
  name: string
  quota_rpm: number
  created_at: string
}

export interface CreatedTenant extends Tenant {
  api_key: string
}

export interface Usage {
  tenant_id: string
  method: string
  count: number
  bytes_in: number
  period: string
}

export interface Block {
  number: number
  hash: string
  parent_hash: string
  timestamp: string
  tx_count: number
}

export class AuthError extends Error {
  constructor() {
    super('Admin secret rejected')
    this.name = 'AuthError'
  }
}

async function request<T>(path: string, options: RequestInit = {}): Promise<T> {
  const secret = getStoredSecret()
  const res = await fetch(path, {
    ...options,
    headers: {
      'Content-Type': 'application/json',
      ...(secret ? { 'X-Admin-Secret': secret } : {}),
      ...(options.headers ?? {}),
    },
  })

  if (res.status === 401 || res.status === 403) {
    throw new AuthError()
  }
  if (!res.ok) {
    const body = await res.json().catch(() => ({}))
    throw new Error(body.error ?? `Request failed (${res.status})`)
  }
  return res.json() as Promise<T>
}

// Used only from the login screen, against a secret that hasn't been
// stored yet - lets us confirm it's valid before committing to it.
export async function verifySecret(secret: string): Promise<boolean> {
  const res = await fetch('/api/tenants', {
    headers: { 'X-Admin-Secret': secret },
  })
  return res.ok
}

export const api = {
  health: () => request<{ status: string }>('/api/health'),
  listTenants: () => request<Tenant[]>('/api/tenants'),
  createTenant: (name: string, quotaRpm: number) =>
    request<CreatedTenant>('/api/tenants', {
      method: 'POST',
      body: JSON.stringify({ name, quota_rpm: quotaRpm }),
    }),
  getUsage: (apiKey: string, day?: string) =>
    request<Usage[]>(
      `/api/usage?api_key=${encodeURIComponent(apiKey)}${day ? `&day=${day}` : ''}`
    ),
  listBlocks: () => request<Block[]>('/api/blocks'),
}
