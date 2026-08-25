import { render, screen, fireEvent } from '@testing-library/react'
import { describe, it, expect, vi } from 'vitest'
import TenantsSection from './TenantsSection'
import { ToastProvider } from './components/ToastProvider'

describe('TenantsSection', () => {
  it('toggles from create to edit mode and pre-fills fields', () => {
    const tenant = {
      id: 't1',
      name: 'Acme',
      quota_rpm: 500,
      quota_rps: 5,
      quota_daily: 50000,
      plan: 'pro',
      created_at: new Date().toISOString(),
    }

    render(
      <ToastProvider>
        <TenantsSection
          tenants={[tenant]}
          networks={[]}
          hasLoaded={true}
          onTenantCreated={vi.fn()}
          onTenantDeleted={vi.fn()}
          onTenantUpdated={vi.fn()}
        />
      </ToastProvider>
    )

    fireEvent.click(screen.getByRole('button', { name: /edit/i }))
    expect(screen.getByDisplayValue('Acme')).toBeInTheDocument()
    expect(screen.getByDisplayValue('Pro')).toBeInTheDocument()
  })
})