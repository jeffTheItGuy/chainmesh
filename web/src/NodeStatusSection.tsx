import { useCallback, useEffect, useState } from 'react'
import { api, type NetworkHealth } from './api'

export default function NodeStatusSection() {
  const [health, setHealth] = useState<NetworkHealth[]>([])
  const [error, setError] = useState('')
  const [loading, setLoading] = useState(false)

  const load = useCallback(async () => {
    setLoading(true)
    setError('')

    try {
      const data = await api.getNodeHealth()
      setHealth(data)
    } catch (err) {
      setHealth([])
      setError(err instanceof Error ? err.message : 'Failed to load node health')
    } finally {
      setLoading(false)
    }
  }, [])

  useEffect(() => {
    load()
    const interval = setInterval(load, 10000)
    return () => clearInterval(interval)
  }, [load])

  const latencyMs = (latency: number) => {
    // Go time.Duration marshals as nanoseconds.
    return Math.max(0, Math.round(latency / 1_000_000))
  }

  return (
    <section className="card">
      <div className="card-header">
        <div>
          <span className="eyebrow">Infrastructure</span>
          <h2 className="card-title">Node status</h2>
        </div>

        <button className="btn btn-ghost" onClick={load} disabled={loading}>
          {loading ? 'Refreshing…' : 'Refresh'}
        </button>
      </div>

      {error && <div className="alert alert-error">{error}</div>}

      {health.length === 0 ? (
        <div className="empty-state">No node health data available.</div>
      ) : (
        <div style={{ display: 'flex', flexDirection: 'column', gap: 16 }}>
          {health.map(network => (
            <div key={network.network_id} className="table-wrap">
              <div className="eyebrow" style={{ marginBottom: 8 }}>
                Network {network.network_id.slice(0, 8)}
              </div>

              <table className="table">
                <thead>
                  <tr>
                    <th>Endpoint</th>
                    <th>Status</th>
                    <th>Latency</th>
                    <th>Fails</th>
                    <th>Requests</th>
                  </tr>
                </thead>
                <tbody>
                  {network.nodes.map(node => (
                    <tr key={node.url}>
                      <td className="mono muted">
                        {node.url.replace(/^https:\/\//, '').slice(0, 32)}…
                      </td>
                      <td>
                        {node.healthy ? (
                          <span style={{ color: 'var(--success)' }}>Healthy</span>
                        ) : (
                          <span style={{ color: 'var(--danger)' }}>Down</span>
                        )}
                      </td>
                      <td className="mono">{latencyMs(node.latency)}ms</td>
                      <td>{node.consecutive_fails}</td>
                      <td>{node.total_requests}</td>
                    </tr>
                  ))}
                </tbody>
              </table>
            </div>
          ))}
        </div>
      )}
    </section>
  )
}