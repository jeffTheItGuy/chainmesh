import { useState, type FormEvent, useEffect } from 'react'
import { api, type Usage, type Tenant } from './api'

interface UsageSectionProps {
  tenants: Tenant[]
}

export default function UsageSection({ tenants }: UsageSectionProps) {
  const [tenantId, setTenantId] = useState('')
  const [day, setDay] = useState('')
  const [usage, setUsage] = useState<Usage[] | null>(null)
  const [error, setError] = useState('')
  const [loading, setLoading] = useState(false)

  // Auto-select the first tenant when the list loads
  useEffect(() => {
    if (tenants.length > 0 && !tenantId) {
      setTenantId(tenants[0].id)
    }
  }, [tenants, tenantId])

  const lookup = async (e: FormEvent) => {
    e.preventDefault()
    if (!tenantId) return
    setLoading(true)
    setError('')
    try {
      const data = await api.getTenantUsage(tenantId, day || undefined)
      setUsage(data)
    } catch (err) {
      setUsage(null)
      setError(err instanceof Error ? err.message : 'Lookup failed')
    } finally {
      setLoading(false)
    }
  }

  return (
    <section className="card">
      <div className="card-header">
        <div>
          <span className="eyebrow">Metering</span>
          <h2 className="card-title">Usage lookup</h2>
        </div>
      </div>
      <p className="card-hint">Select a tenant to inspect their request volume.</p>
      
      {tenants.length === 0 ? (
        <div className="empty-state">Create a tenant first to view usage data.</div>
      ) : (
        <>
          <form onSubmit={lookup} className="inline-form inline-form-row">
            <select
              className="input"
              value={tenantId}
              onChange={e => setTenantId(e.target.value)}
            >
              {tenants.map(t => (
                <option key={t.id} value={t.id}>{t.name}</option>
              ))}
            </select>
            <input
              type="date"
              className="input"
              value={day}
              onChange={e => setDay(e.target.value)}
              title="Leave empty for today"
              style={{ maxWidth: 180 }}
            />
            <button type="submit" className="btn btn-primary" disabled={!tenantId || loading}>
              {loading ? 'Looking up…' : 'Look up'}
            </button>
          </form>
          {error && <div className="alert alert-error">{error}</div>}
          {usage && (
            usage.length === 0 ? (
              <div className="empty-state">No usage recorded for this tenant on the selected day.</div>
            ) : (
              <div className="table-wrap">
                <table className="table">
                  <thead>
                    <tr>
                      <th>Method</th>
                      <th>Count</th>
                      <th>Bytes in</th>
                      <th>Period</th>
                    </tr>
                  </thead>
                  <tbody>
                    {usage.map((u, i) => (
                      <tr key={i}>
                        <td className="mono">{u.method}</td>
                        <td>{u.count.toLocaleString()}</td>
                        <td>{u.bytes_in.toLocaleString()}</td>
                        <td className="muted">{new Date(u.period).toLocaleString()}</td>
                      </tr>
                    ))}
                  </tbody>
                </table>
              </div>
            )
          )}
        </>
      )}
    </section>
  )
}