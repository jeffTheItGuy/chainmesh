import { useCallback, useState, useRef, useEffect } from 'react'
import { api, type NetworkHealth } from './api'
import { usePolling } from './hooks/usePolling'

export default function NodeStatusSection() {
  const [health, setHealth] = useState<NetworkHealth[]>([])
  const [error, setError] = useState('')
  const [loading, setLoading] = useState(false)
  const [hasLoaded, setHasLoaded] = useState(false)

  const reqId = useRef(0)
  const mounted = useRef(true)

  // FIX: Reset mounted on every mount. 
  // Without this, React StrictMode's double-mount strands the flag at `false` 
  // after the first cleanup, causing all subsequent API responses to be ignored.
  useEffect(() => {
    mounted.current = true
    return () => { 
      mounted.current = false 
    }
  }, [])

  const load = useCallback(async (background = false) => {
    const id = ++reqId.current
    if (!background) setLoading(true)
    setError('')
    try {
      const data = await api.getNodeHealth()
      if (!mounted.current) return
      if (id === reqId.current) {
        setHealth(data || [])
        setHasLoaded(true)
      }
    } catch (err) {
      if (!mounted.current) return
      if (id === reqId.current) {
        setHealth([])
        setError(err instanceof Error ? err.message : 'Failed to load node health')
        setHasLoaded(true)
      }
    } finally {
      if (!mounted.current) return
      if (id === reqId.current && !background) setLoading(false)
    }
  }, [])

  usePolling(load, 10000)

  if (!hasLoaded) {
    return <div className="empty-state">Loading node status…</div>
  }

  return (
    <section className="card">
      <div className="card-header">
        <div>
          <span className="eyebrow">Infrastructure</span>
          <h2 className="card-title">Node status</h2>
        </div>
        <button className="btn btn-ghost" onClick={() => load(false)} disabled={loading}>
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
                      <td className="mono muted truncate" title={node.url}>
                        {node.url.replace(/^https?:\/\//, '')}
                      </td>
                      <td>
                        {node.healthy ? (
                          <span style={{ color: 'var(--success)' }}>Healthy</span>
                        ) : (
                          <span style={{ color: 'var(--danger)' }}>Down</span>
                        )}
                      </td>
                      <td className="mono">{node.latency_ms}ms</td>
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