import { useState, type FormEvent } from 'react'
import { api, type Tenant, type CreatedTenant, type BlockchainConfig } from './api'

interface TenantsSectionProps {
  tenants: Tenant[]
  networks: BlockchainConfig[]
  onTenantCreated: (tenant: CreatedTenant) => void
}

export default function TenantsSection({ tenants, networks, onTenantCreated }: TenantsSectionProps) {
  const [showCreate, setShowCreate] = useState(false)
  const [name, setName] = useState('')
  const [quota, setQuota] = useState(1000)
  const [networkId, setNetworkId] = useState('')
  const [created, setCreated] = useState<CreatedTenant | null>(null)
  const [error, setError] = useState('')
  const [submitting, setSubmitting] = useState(false)

  const submit = async (e: FormEvent) => {
    e.preventDefault()
    if (!name) return
    setSubmitting(true)
    setError('')
    try {
      const tenant = await api.createTenant(name, quota, networkId || undefined)
      setCreated(tenant)
      onTenantCreated(tenant)
      setName('')
      setQuota(1000)
      setNetworkId('')
      setShowCreate(false)
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Could not create tenant')
    } finally {
      setSubmitting(false)
    }
  }

  const copyKey = (key: string) => navigator.clipboard.writeText(key)

  return (
    <section className="card">
      <div className="card-header">
        <div>
          <span className="eyebrow">Access</span>
          <h2 className="card-title">Tenants</h2>
        </div>
        <button className="btn btn-primary" onClick={() => setShowCreate(v => !v)}>
          {showCreate ? 'Cancel' : 'New tenant'}
        </button>
      </div>

      {created && (
        <div className="alert alert-success">
          <strong>{created.name}</strong> created at {created.quota_rpm} req/min.
          <div className="key-row">
            <code className="key-value">{created.api_key}</code>
            <button className="btn btn-ghost btn-sm" onClick={() => copyKey(created.api_key)}>Copy</button>
          </div>
          <p className="alert-note">This key is shown once. Store it now — it won't be shown again.</p>
          <button className="btn btn-ghost btn-sm" onClick={() => setCreated(null)}>Dismiss</button>
        </div>
      )}

      {showCreate && (
        <form onSubmit={submit} className="inline-form">
          <div className="field">
            <label className="label" htmlFor="tenant-name">Name</label>
            <input id="tenant-name" className="input" value={name} onChange={e => setName(e.target.value)} placeholder="Client name" />
          </div>
          <div className="field">
            <label className="label" htmlFor="tenant-quota">Quota (req/min)</label>
            <input id="tenant-quota" type="number" min={1} className="input" value={quota} onChange={e => setQuota(Number(e.target.value))} />
          </div>
          <div className="field">
            <label className="label" htmlFor="tenant-network">Blockchain Network</label>
            <select
              id="tenant-network"
              className="input"
              value={networkId}
              onChange={e => setNetworkId(e.target.value)}
            >
              <option value="">Default (auto-assign)</option>
              {networks.filter(n => n.enabled).map(n => (
                <option key={n.id} value={n.id}>{n.name} {n.chain_id ? `(Chain ${n.chain_id})` : ''}</option>
              ))}
            </select>
          </div>
          {error && <div className="alert alert-error">{error}</div>}
          <button type="submit" className="btn btn-primary" disabled={!name || submitting}>
            {submitting ? 'Creating…' : 'Create tenant'}
          </button>
        </form>
      )}

      {tenants.length === 0 ? (
        <div className="empty-state">No tenants yet. Create one to issue an API key.</div>
      ) : (
        <div className="table-wrap">
          <table className="table">
            <thead>
              <tr>
                <th>Name</th>
                <th>Network</th>
                <th>Quota</th>
                <th>Created</th>
              </tr>
            </thead>
            <tbody>
              {tenants.map(t => (
                <tr key={t.id}>
                  <td>{t.name}</td>
                  <td className="mono">
                    {networks.find(n => n.id === t.blockchain_network_id)?.name || 'Default'}
                  </td>
                  <td className="mono">{t.quota_rpm} rpm</td>
                  <td className="muted">{new Date(t.created_at).toLocaleDateString()}</td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      )}
    </section>
  )
}
