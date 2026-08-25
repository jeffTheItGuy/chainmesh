import { describe, it, expect, beforeEach } from 'vitest'
import { getRole, storeSecret, storeViewerSession, clearSession } from './auth'

describe('auth', () => {
  beforeEach(() => {
    clearSession()
  })

  it('returns admin when secret is stored', () => {
    storeSecret('sekrit')
    expect(getRole()).toBe('admin')
  })

  it('returns viewer when viewer session is stored', () => {
    storeViewerSession()
    expect(getRole()).toBe('viewer')
  })

  it('returns null when session is cleared', () => {
    storeSecret('sekrit')
    clearSession()
    expect(getRole()).toBeNull()
  })
})