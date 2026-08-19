import { useCallback, useState, useRef } from 'react'
import { api, type NetworkHealth } from './api'
import { usePolling } from './hooks/usePolling'
import { useToast } from './components/ToastProvider'
import { SkeletonTable } from './components/Skeleton'

export default function NodeStatusSection() {
  const [health, setHealth] = useState<NetworkHealth[]>([])
  const [error, setError] = useState('')
  const [loading, setLoading] = useState(false)
  const [hasLoaded, setHasLoaded] = useState(false)
  const { showToast } = useToast()

  const reqId = useRef(0)

  const load = useCallback(
    async (signal: AbortSignal, background: boolean) => {
      const id = ++reqId.current
      if (!background) setLoading(true)
      setError('')
      try {
        const data = await api.getNodeHealth()
        if (signal.aborted) return
        if (id === reqId.current) {
          setHealth(data || [])
          setHasLoaded(true)
        }
      } catch (err) {
        if (signal.aborted) return
        if (id === reqId.current) {
          setHealth([])
          setError(err instanceof Error ? err.message : 'Failed to load node health')
          setHasLoaded(true)
        }
      } finally {
        if (signal.aborted) return
        if (id === reqId.current && !background) setLoading(false)
      }
    },
    []
  )

  usePolling(load, 10000)

  const handleRefresh = () => {
    load(new AbortController().signal, false)
    showToast('Refreshing node status...', 'success')
  }

  if (!hasLoaded) {
    return (
      <section className="card">
        <div className="card-header">
          <div>
            <span className="eyebrow">Infrastructure</span>
            <h2 className="card-title">Node status</h2>
          </div>
        </div>
        <SkeletonTable rows={4} />
      </section>
    )
  }

  return (
    <section className="card">
      <div className="card-header">
        <div>
          <span className="eyebrow">Infrastructure</span>
          <h2 className="card-title">Node status</h2>
        </div>
        <button className="btn btn-ghost" onClick={handleRefresh} disabled={loading}>
          {loading ? 'Refreshing…' : 'Refresh'}
        </button>
      </div>
      {error && (
        <div className="alert alert-error" role="alert">
          {error}
        </div>
      )}
      {health.length === 0 ? (
        <div className="empty-state">No node health data available.</div>
      ) : (
        <div className="flex flex-col gap-16">
          {health.map((network) => (
            <div key={network.network_id} className="table-wrap">
              <div className="eyebrow mb-8">
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
                  {network.nodes.map((node) => (
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
