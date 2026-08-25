import { useCallback, useState, useRef } from 'react'
import { api, type NetworkHealth } from './api'
import { usePolling } from './hooks/usePolling'
import { SkeletonTable } from './components/Skeleton'
import { IconServer, IconRefresh } from './components/Icons'
import Badge from './components/Badge'
import { colorForId } from './utils/color'

export default function NodeStatusSection() {
  const [health, setHealth] = useState<NetworkHealth[]>([])
  const [error, setError] = useState('')
  const [loading, setLoading] = useState(false)
  const [hasLoaded, setHasLoaded] = useState(false)

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
  }

  if (!hasLoaded) {
    return (
      <section className="card">
        <div className="card-header">
          <div>
            <span className="eyebrow">Infrastructure</span>
            <h2 className="card-title card-title-row">
              <IconServer size={16} className="card-icon" />
              Node status
            </h2>
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
          <h2 className="card-title card-title-row">
            <IconServer size={16} className="card-icon" />
            Node status
          </h2>
        </div>
        <button className="btn btn-ghost btn-with-icon" onClick={handleRefresh} disabled={loading}>
          <IconRefresh size={14} className={loading ? 'icon-spin' : ''} />
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
              <div className="eyebrow mb-8 flex items-center gap-8">
                <span
                  className="network-dot"
                  style={{ background: colorForId(network.network_id) }}
                />
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
                        <Badge tone={node.healthy ? 'success' : 'danger'}>
                          {node.healthy ? 'Healthy' : 'Down'}
                        </Badge>
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