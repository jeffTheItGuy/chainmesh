import { useState, type FormEvent } from 'react'
import { api, type Tenant, type CreatedTenant, type BlockchainConfig } from './api'

interface TenantsSectionProps {
  tenants: Tenant[]
  networks: BlockchainConfig[]
  hasLoaded: boolean
  onTenantCreated: (tenant: CreatedTenant) => void
  onTenantDeleted: (id: string) => void
  onTenantUpdated: (tenant: Tenant) => void
}

export default function TenantsSection({
  tenants,
  networks,
  hasLoaded,
  onTenantCreated,
  onTenantDeleted,
  onTenantUpdated,
}: TenantsSectionProps) {
  const [showForm, setShowForm] = useState(false)
  const [editing, setEditing] = useState<Tenant | null>(null)

  const [name, setName] = useState('')
  const [quotaRpm, setQuotaRpm] = useState(1000)
  const [quotaRps, setQuotaRps] = useState(10)
  const [quotaDaily, setQuotaDaily] = useState(100000)
  const [plan, setPlan] = useState('free')
  const [networkId, setNetworkId] = useState('')

  const [created, setCreated] = useState<CreatedTenant | null>(null)
  const [rotatedKey, setRotatedKey] = useState<{ tenantName: string; apiKey: string } | null>(null)
  const [copiedKey, setCopiedKey] = useState('')
  const [error, setError] = useState('')
  const [submitting, setSubmitting] = useState(false)

  const resetForm = () => {
    setName('')
    setQuotaRpm(1000)
    setQuotaRps(10)
    setQuotaDaily(100000)
    setPlan('free')
    setNetworkId('')
    setEditing(null)
    setShowForm(false)
    setError('')
  }

  const startEdit = (t: Tenant) => {
    setEditing(t)
    setName(t.name)
    setQuotaRpm(t.quota_rpm)
    setQuotaRps(t.quota_rps)
    setQuotaDaily(t.quota_daily)
    setPlan(t.plan)
    setNetworkId(t.blockchain_network_id || '')
    setShowForm(true)
    setError('')
  }

  const submit = async (e: FormEvent) => {
    e.preventDefault()
    if (!name) return
    setSubmitting(true)
    setError('')
    try {
      if (editing) {
        const payload = {
          name,
          quota_rpm: quotaRpm,
          quota_rps: quotaRps,
          quota_daily: quotaDaily,
          plan,
          blockchain_network_id: networkId || undefined,
        }
        await api.updateTenant(editing.id, payload)
        onTenantUpdated({ ...editing, ...payload })
        resetForm()
      } else {
        const tenant = await api.createTenant(name, quotaRpm, networkId || undefined, quotaRps, quotaDaily, plan)
        setCreated(tenant)
        onTenantCreated(tenant)
        setName('')
        setQuotaRpm(1000)
        setQuotaRps(10)
        setQuotaDaily(100000)
        setPlan('free')
        setNetworkId('')
        setShowForm(false)
      }
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Operation failed')
    } finally {
      setSubmitting(false)
    }
  }

  const remove = async (id: string, tenantName: string) => {
    if (!confirm(`Delete tenant "${tenantName}"? This cannot be undone.`)) return
    try {
      await api.deleteTenant(id)
      onTenantDeleted(id)
    } catch (err) {
      alert(err instanceof Error ? err.message : 'Delete failed')
    }
  }

  const rotateKey = async (id: string, tenantName: string) => {
    if (!confirm(`Rotate the API key for "${tenantName}"? The current key will stop working immediately.`)) return
    try {
      const result = await api.rotateTenantKey(id)
      setRotatedKey({ tenantName, apiKey: result.api_key })
    } catch (err) {
      alert(err instanceof Error ? err.message : 'Key rotation failed')
    }
  }

  const copyKey = async (key: string) => {
    try {
      await navigator.clipboard.writeText(key)
      setCopiedKey(key)
      setTimeout(() => setCopiedKey(''), 2000)
    } catch {
      alert('Failed to copy. Please select and copy manually.')
    }
  }

  if (!hasLoaded) {
    return <div className="empty-state">Loading tenants…</div>
  }

  return (
    <section className="card">
      <div className="card-header">
        <div>
          <span className="eyebrow">Access</span>
          <h2 className="card-title">Tenants</h2>
        </div>
        <button className="btn btn-primary" onClick={() => { resetForm(); setShowForm(v => !v) }}>
          {showForm ? 'Cancel' : 'New tenant'}
        </button>
      </div>

      {created && (
        <div className="alert alert-success">
          <strong>{created.name}</strong> created at {created.quota_rpm} req/min.
          <div className="key-row">
            <code className="key-value">{created.api_key}</code>
            <button className="btn btn-ghost btn-sm" onClick={() => copyKey(created.api_key)}>
              {copiedKey === created.api_key ? 'Copied!' : 'Copy'}
            </button>
          </div>
          <p className="alert-note">This key is shown once. Store it now — it won't be shown again.</p>
          <button className="btn btn-ghost btn-sm" onClick={() => setCreated(null)}>Dismiss</button>
        </div>
      )}

      {rotatedKey && (
        <div className="alert alert-success">
          <strong>{rotatedKey.tenantName}</strong> — new API key:
          <div className="key-row">
            <code className="key-value">{rotatedKey.apiKey}</code>
            <button className="btn btn-ghost btn-sm" onClick={() => copyKey(rotatedKey.apiKey)}>
              {copiedKey === rotatedKey.apiKey ? 'Copied!' : 'Copy'}
            </button>
          </div>
          <p className="alert-note">This key is shown once. Store it now — it won't be shown again.</p>
          <button className="btn btn-ghost btn-sm" onClick={() => setRotatedKey(null)}>Dismiss</button>
        </div>
      )}

      {showForm && (
        <form onSubmit={submit} className="inline-form">
          <div className="field">
            <label className="label" htmlFor="tenant-name">Name</label>
            <input id="tenant-name" className="input" value={name} onChange={e => setName(e.target.value)} placeholder="Client name" required />
          </div>
          <div className="field">
            <label className="label" htmlFor="tenant-plan">Plan</label>
            <select id="tenant-plan" className="input" value={plan} onChange={e => setPlan(e.target.value)}>
              <option value="free">Free</option>
              <option value="basic">Basic</option>
              <option value="pro">Pro</option>
              <option value="enterprise">Enterprise</option>
            </select>
          </div>
          <div className="field">
            <label className="label" htmlFor="tenant-quota-rps">Quota (req/sec)</label>
            <input id="tenant-quota-rps" type="number" min={1} className="input" value={quotaRps} onChange={e => setQuotaRps(Number(e.target.value))} />
          </div>
          <div className="field">
            <label className="label" htmlFor="tenant-quota-rpm">Quota (req/min)</label>
            <input id="tenant-quota-rpm" type="number" min={1} className="input" value={quotaRpm} onChange={e => setQuotaRpm(Number(e.target.value))} />
          </div>
          <div className="field">
            <label className="label" htmlFor="tenant-quota-daily">Quota (daily)</label>
            <input id="tenant-quota-daily" type="number" min={1} className="input" value={quotaDaily} onChange={e => setQuotaDaily(Number(e.target.value))} />
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
            {submitting ? 'Saving…' : editing ? 'Update tenant' : 'Create tenant'}
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
                <th>Plan</th>
                <th>Quota</th>
                <th>Created</th>
                <th style={{ textAlign: 'right' }}>Actions</th>
              </tr>
            </thead>
            <tbody>
              {tenants.map(t => (
                <tr key={t.id}>
                  <td>{t.name}</td>
                  <td className="mono">
                    {networks.find(n => n.id === t.blockchain_network_id)?.name || 'Default'}
                  </td>
                  <td className="mono">{t.plan || 'free'}</td>
                  <td className="mono">{t.quota_rpm} rpm</td>
                  <td className="muted">{new Date(t.created_at).toLocaleDateString()}</td>
                  <td style={{ textAlign: 'right' }}>
                    <button className="btn btn-ghost btn-sm" onClick={() => startEdit(t)}>Edit</button>
                    <button className="btn btn-ghost btn-sm" onClick={() => rotateKey(t.id, t.name)} style={{ marginLeft: 4 }}>Rotate</button>
                    <button className="btn btn-ghost btn-sm" onClick={() => remove(t.id, t.name)} style={{ marginLeft: 4, color: 'var(--danger)' }}>Delete</button>
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      )}
    </section>
  )
}