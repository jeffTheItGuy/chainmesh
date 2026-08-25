import { describe, it, expect, vi } from 'vitest'
import { api, AuthError, NetworkError, verifySecret } from './api'

describe('api client', () => {
  it('throws AuthError on 401', async () => {
    globalThis.fetch = vi.fn(() =>
      Promise.resolve({ status: 401, ok: false } as Response)
    )
    await expect(api.health()).rejects.toThrow(AuthError)
  })

  it('throws AuthError on 403', async () => {
    globalThis.fetch = vi.fn(() =>
      Promise.resolve({ status: 403, ok: false } as Response)
    )
    await expect(api.health()).rejects.toThrow(AuthError)
  })

  it('throws NetworkError when verifySecret fetch fails', async () => {
    globalThis.fetch = vi.fn(() => Promise.reject(new Error('network down')))
    await expect(verifySecret('sekrit')).rejects.toThrow(NetworkError)
  })
})