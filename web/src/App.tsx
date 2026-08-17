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

	const handleAuthError = useCallback(() => {
		clearSession()
		setRole(null)
		setShowAdminLogin(false)
	}, [])

	const loadNetworks = useCallback(async () => {
		try {
			const list = await api.listBlockchainConfigs()
			setNetworks(list || []) // FIX: Guard against null
		} catch {
			setNetworks([])
		}
	}, [])

	const loadDashboard = useCallback(async (currentRole: Role) => {
		setLoadError('')
		try {
			const [blockList, netList] = await Promise.all([
				api.listBlocks(),
				currentRole === 'admin' ? api.listBlockchainConfigs() : Promise.resolve([]),
			])
			setBlocks(blockList || []) // FIX: Guard against null
			if (currentRole === 'admin') {
				setNetworks(netList || []) // FIX: Guard against null
			}
		} catch (err) {
			if (err instanceof AuthError) {
				if (currentRole === 'viewer') {
					setLoadError('This deployment requires the admin secret to read chain data.')
				} else {
					handleAuthError()
				}
				return
			}
			setLoadError(err instanceof Error ? err.message : 'Failed to load dashboard data')
		}

		if (currentRole !== 'admin') return

		try {
			const tenantList = await api.listTenants()
			setTenants(tenantList || []) // FIX: Guard against null
		} catch (err) {
			if (err instanceof AuthError) {
				handleAuthError()
				return
			}
			setLoadError(err instanceof Error ? err.message : 'Failed to load dashboard data')
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

	const latestBlock = blocks.length > 0 ? blocks[0].number : null
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
				<NodeStatusSection />
				{role === 'admin' && (
					<BlockchainSection networks={networks} onNetworksChanged={loadNetworks} />
				)}
				{role === 'admin' && (
					<TenantsSection
						tenants={tenants}
						networks={networks}
						onTenantCreated={tenant => setTenants(prev => [tenant, ...prev])}
					/>
				)}
				{role === 'admin' && <UsageSection />}
				<BlocksSection blocks={blocks} />
			</main>
		</div>
	)
}