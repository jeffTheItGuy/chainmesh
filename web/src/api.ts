import { getStoredSecret } from './auth'

export interface Tenant {
  id: string
  name: string
  quota_rpm: number
  quota_rps: number
  quota_daily: number
  plan: string
  blockchain_network_id?: string
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
  network_id?: string
  network_name?: string
}

export interface BlockchainConfig {
  id: string
  name: string
  rpc_endpoint_1: string
  rpc_endpoint_2?: string
  chain_id?: string
  enabled: boolean
  created_at: string
  updated_at: string
}

export interface TestConnectionResult {
  connected: boolean
  chain_id?: string
  error?: string
}

export interface EndpointHealth {
  url: string
  healthy: boolean
  latency_ms: number
  last_check: string
  consecutive_fails: number
  total_requests: number
  total_failures: number
}

export interface NetworkHealth {
  network_id: string
  nodes: EndpointHealth[]
}

/* ------------------------------------------------------------------ */
/* Custom monitoring dashboard types                                   */
/* ------------------------------------------------------------------ */

export type StatsRange = '15m' | '1h' | '24h'

export interface StatsTotals {
  requests: number
  success: number
  errors: number
  cache_hits: number
  cache_misses: number
}

export interface StatsLatency {
  avg_ms: number
  p95_ms: number
}

export interface StatsCount {
  name: string
  count: number
}

export interface StatsSeriesPoint {
  time: string
  requests: number
  errors: number
  cache_hits: number
}

export interface StatsSummary {
  range: StatsRange
  from: string
  to: string
  totals: StatsTotals
  latency: StatsLatency
  top_methods: StatsCount[]
  top_statuses: StatsCount[]
  top_networks: StatsCount[]
  series: StatsSeriesPoint[]
}

/* ------------------------------------------------------------------ */
/* Auth                                                                */
/* ------------------------------------------------------------------ */

export class AuthError extends Error {
  constructor() {
    super('Admin secret rejected')
    this.name = 'AuthError'
  }
}

async function request<T>(path: string, options: RequestInit = {}): Promise<T> {
  const secret = getStoredSecret()
  
  const headers: Record<string, string> = {
    ...(secret ? { 'X-Admin-Secret': secret } : {}),
    ...(options.headers as Record<string, string> ?? {}),
  }

  // FIX: Only set Content-Type if there is a body
  if (options.body) {
    headers['Content-Type'] = 'application/json'
  }

  const res = await fetch(path, {
    ...options,
    headers,
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

export async function verifySecret(secret: string): Promise<boolean> {
  try {
    const res = await fetch('/api/tenants', {
      headers: { 'X-Admin-Secret': secret },
    })
    
    // FIX: Properly distinguish between bad credentials and server errors
    if (res.status === 401 || res.status === 403) return false
    if (!res.ok) throw new Error(`Server error: ${res.status}`)
    return true
  } catch (err) {
    if (err instanceof Error && err.message.startsWith('Server error')) throw err
    throw new Error('Could not reach the admin API. Is it running?')
  }
}

/* ------------------------------------------------------------------ */
/* API client                                                          */
/* ------------------------------------------------------------------ */

export const api = {
  health: () => request<{ status: string }>('/api/health'),

  /* ----------------------- Tenants ------------------------------- */
  listTenants: () => request<Tenant[]>('/api/tenants'),

  createTenant: (
    name: string,
    quotaRpm: number,
    blockchainNetworkId?: string,
    quotaRps?: number,
    quotaDaily?: number,
    plan?: string
  ) =>
    request<CreatedTenant>('/api/tenants', {
      method: 'POST',
      body: JSON.stringify({
        name,
        quota_rpm: quotaRpm,
        quota_rps: quotaRps,
        quota_daily: quotaDaily,
        plan,
        blockchain_network_id: blockchainNetworkId,
      }),
    }),

  getTenant: (id: string) => request<Tenant>(`/api/tenants/${id}`),

  updateTenant: (
    id: string,
    payload: {
      name?: string
      quota_rpm?: number
      quota_rps?: number
      quota_daily?: number
      plan?: string
      blockchain_network_id?: string
    }
  ) =>
    request<{ updated: boolean }>(`/api/tenants/${id}`, {
      method: 'PUT',
      body: JSON.stringify(payload),
    }),

  deleteTenant: (id: string) =>
    request<{ deleted: boolean }>(`/api/tenants/${id}`, {
      method: 'DELETE',
    }),

  rotateTenantKey: (id: string) =>
    request<{ api_key: string }>(`/api/tenants/${id}/rotate-key`, {
      method: 'POST',
    }),

  /* ----------------------- Usage --------------------------------- */
  // FIX: Move API key out of query string and into a header
  getUsage: (apiKey: string, day?: string) =>
    request<Usage[]>(
      `/api/usage${day ? `?day=${encodeURIComponent(day)}` : ''}`,
      { headers: { 'X-Tenant-API-Key': apiKey } }
    ),

    // Admin-only usage lookup by tenant ID
  getTenantUsage: (tenantId: string, day?: string) =>
    request<Usage[]>(
      `/api/tenants/${tenantId}/usage${day ? `?day=${encodeURIComponent(day)}` : ''}`
    ),

  /* ----------------------- Monitoring ---------------------------- */
  getStatsSummary: (range: StatsRange = '1h') =>
    request<StatsSummary>(`/api/stats/summary?range=${range}`),

  getNodeHealth: () => request<NetworkHealth[]>('/gateway/health/nodes'),

  /* ----------------------- Blocks -------------------------------- */
  listBlocks: () => request<Block[]>('/api/blocks'),

  /* ----------------------- Blockchain configs -------------------- */
  listBlockchainConfigs: () => request<BlockchainConfig[]>('/api/blockchain'),

  getBlockchainConfig: (id: string) => request<BlockchainConfig>(`/api/blockchain/${id}`),

  createBlockchainConfig: (cfg: Omit<BlockchainConfig, 'id' | 'created_at' | 'updated_at'>) =>
    request<BlockchainConfig>('/api/blockchain', {
      method: 'POST',
      body: JSON.stringify(cfg),
    }),

  updateBlockchainConfig: (
    id: string,
    cfg: Omit<BlockchainConfig, 'id' | 'created_at' | 'updated_at'>
  ) =>
    request<{ updated: boolean }>(`/api/blockchain/${id}`, {
      method: 'PUT',
      body: JSON.stringify(cfg),
    }),

  deleteBlockchainConfig: (id: string) =>
    request<{ deleted: boolean }>(`/api/blockchain/${id}`, {
      method: 'DELETE',
    }),

  testBlockchainConnection: (cfg: { rpc_endpoint_1: string; rpc_endpoint_2?: string }) =>
    request<TestConnectionResult>('/api/blockchain/test', {
      method: 'POST',
      body: JSON.stringify(cfg),
    }),
}