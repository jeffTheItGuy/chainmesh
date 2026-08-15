import { useEffect, useState, useCallback } from 'react'
import { api, AuthError, type Tenant, type Block } from './lib/api'
import { getStoredSecret, clearSecret } from './lib/auth'
import Login from './components/Login'
import TopBar from './components/TopBar'
import StatsStrip from './components/StatsStrip'
import TenantsSection from './components/TenantsSection'
import UsageSection from './components/UsageSection'
import BlocksSection from './components/BlocksSection'

export default function App() {
  const [authed, setAuthed] = useState<boolean>(() => !!getStoredSecret())
  const [healthStatus, setHealthStatus] = useState('checking')
  const [tenants, setTenants] = useState<Tenant[]>([])
  const [blocks, setBlocks] = useState<Block[]>([])
  const [loadError, setLoadError] = useState('')

  const handleAuthError = useCallback(() => {
    clearSecret()
    setAuthed(false)
  }, [])

  const loadDashboard = useCallback(async () => {
    setLoadError('')
    try {
      const health = await api.health()
      setHealthStatus(health.status)
    } catch {
      setHealthStatus('unreachable')
    }
    try {
      const [tenantList, blockList] = await Promise.all([
        api.listTenants(),
        api.listBlocks(),
      ])
      setTenants(tenantList)
      setBlocks(blockList)
    } catch (err) {
      if (err instanceof AuthError) {
        handleAuthError()
        return
      }
      setLoadError(err instanceof Error ? err.message : 'Failed to load dashboard data')
    }
  }, [handleAuthError])

  useEffect(() => {
    if (authed) loadDashboard()
  }, [authed, loadDashboard])

  if (!authed) {
    return <Login onAuthenticated={() => setAuthed(true)} />
  }

  const latestBlock = blocks.length > 0 ? blocks[0].number : null
  const totalTxCount = blocks.reduce((sum, b) => sum + b.tx_count, 0)

  return (
    <div className="shell">
      <TopBar healthStatus={healthStatus} onLogout={handleAuthError} />
      <main className="content">
        <StatsStrip
          tenantCount={tenants.length}
          latestBlock={latestBlock}
          totalTxCount={totalTxCount}
        />
        {loadError && <div className="alert alert-error">{loadError}</div>}
        <TenantsSection
          tenants={tenants}
          onTenantCreated={tenant => setTenants(prev => [tenant, ...prev])}
        />
        <UsageSection />
        <BlocksSection blocks={blocks} />
      </main>
    </div>
  )
}
