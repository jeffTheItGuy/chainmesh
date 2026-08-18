import { useEffect, useState, useCallback } from 'react'
import { api, AuthError, type Tenant, type Block, type BlockchainConfig } from './api'
import { getRole, clearSession, storeViewerSession, type Role } from './auth'
import RoleGate from './RoleGate'
import Login from './Login'
import TopBar from './TopBar'
import StatsStrip from './StatsStrip'
import BlockchainSection from './BlockchainSection'
import TenantsSection from './TenantsSection'
import UsageSection from './UsageSection'
import BlocksSection from './BlocksSection'
import NodeStatusSection from './NodeStatusSection'
import MonitoringSection from './MonitoringSection'

export default function App() {
  const [role, setRole] = useState<Role>(() => getRole())
  const [showAdminLogin, setShowAdminLogin] = useState(false)
  const [tenants, setTenants] = useState<Tenant[]>([])
  const [blocks, setBlocks] = useState<Block[]>([])
  const [networks, setNetworks] = useState<BlockchainConfig[]>([])
  const [loadError, setLoadError] = useState('')
  const [hasLoaded, setHasLoaded] = useState(false)

  const handleAuthError = useCallback(() => {
    clearSession()
    setRole(null)
    setShowAdminLogin(false)
  }, [])

  const loadNetworks = useCallback(async () => {
    try {
      const list = await api.listBlockchainConfigs()
      setNetworks(list || [])
    } catch {
      setNetworks([])
    }
  }, [])

  const loadDashboard = useCallback(async (currentRole: Role) => {
    setLoadError('')
    setHasLoaded(false)
    try {
      const [blockList, netList] = await Promise.all([
        api.listBlocks(),
        currentRole === 'admin' ? api.listBlockchainConfigs() : Promise.resolve([]),
      ])
      setBlocks(blockList || [])
      if (currentRole === 'admin') {
        setNetworks(netList || [])
      }
    } catch (err) {
      if (err instanceof AuthError) {
        if (currentRole === 'viewer') {
          setLoadError('This deployment requires the admin secret to read chain data.')
        } else {
          handleAuthError()
        }
        setHasLoaded(true)
        return
      }
      setLoadError(err instanceof Error ? err.message : 'Failed to load dashboard data')
      setHasLoaded(true)
      return
    }

    if (currentRole !== 'admin') {
      setHasLoaded(true)
      return
    }

    try {
      const tenantList = await api.listTenants()
      setTenants(tenantList || [])
    } catch (err) {
      if (err instanceof AuthError) {
        handleAuthError()
        return
      }
      setLoadError(err instanceof Error ? err.message : 'Failed to load dashboard data')
    } finally {
      setHasLoaded(true)
    }
  }, [handleAuthError])

  useEffect(() => {
    if (role) loadDashboard(role)
  }, [role, loadDashboard])

  if (!role) {
    return showAdminLogin ? (
      <Login
        onAuthenticated={() => {
          setShowAdminLogin(false)
          setRole('admin')
        }}
        onBack={() => setShowAdminLogin(false)}
      />
    ) : (
      <RoleGate
        onViewer={() => {
          storeViewerSession()
          setRole('viewer')
        }}
        onAdmin={() => setShowAdminLogin(true)}
      />
    )
  }

  const latestBlock = blocks.length > 0 ? Math.max(...blocks.map(b => b.number)) : null
  const totalTxCount = blocks.reduce((sum, b) => sum + b.tx_count, 0)

  return (
    <div className="shell">
      <TopBar role={role} onLogout={handleAuthError} />
      <main className="content">
        <StatsStrip
          tenantCount={role === 'admin' ? tenants.length : null}
          latestBlock={latestBlock}
          totalTxCount={totalTxCount}
        />
        {loadError && <div className="alert alert-error">{loadError}</div>}

        {role === 'admin' && <MonitoringSection />}
        {role === 'admin' && <NodeStatusSection />}

        {role === 'admin' && (
          <BlockchainSection networks={networks} onNetworksChanged={loadNetworks} />
        )}
        {role === 'admin' && (
          <TenantsSection
            tenants={tenants}
            networks={networks}
            hasLoaded={hasLoaded}
            onTenantCreated={tenant => setTenants(prev => [tenant, ...prev])}
            onTenantDeleted={id => setTenants(prev => prev.filter(t => t.id !== id))}
            onTenantUpdated={updated => setTenants(prev => prev.map(t => t.id === updated.id ? updated : t))}
          />
        )}
        {role === 'admin' && <UsageSection tenants={tenants} />}
        <BlocksSection blocks={blocks} hasLoaded={hasLoaded} />
      </main>
    </div>
  )
}