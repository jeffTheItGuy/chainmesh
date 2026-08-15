import { useState, type FormEvent } from 'react'
import { api, type Usage } from '../lib/api'

export default function UsageSection() {
  const [apiKey, setApiKey] = useState('')
  const [usage, setUsage] = useState<Usage[] | null>(null)
  const [error, setError] = useState('')
  const [loading, setLoading] = useState(false)

  const lookup = async (e: FormEvent) => {
    e.preventDefault()
    if (!apiKey) return
    setLoading(true)
    setError('')
    try {
      const data = await api.getUsage(apiKey)
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
      <p className="card-hint">Paste a tenant's API key to see today's request volume.</p>

      <form onSubmit={lookup} className="inline-form inline-form-row">
        <input
          className="input"
          value={apiKey}
          onChange={e => setApiKey(e.target.value)}
          placeholder="bm_live_…"
        />
        <button type="submit" className="btn btn-primary" disabled={!apiKey || loading}>
          {loading ? 'Looking up…' : 'Look up'}
        </button>
      </form>

      {error && <div className="alert alert-error">{error}</div>}

      {usage && (
        usage.length === 0 ? (
          <div className="empty-state">No usage recorded for this key today.</div>
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
    </section>
  )
}
