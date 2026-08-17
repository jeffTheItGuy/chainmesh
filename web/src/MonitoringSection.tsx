import { useCallback, useEffect, useState } from 'react'
import { api, type StatsCount, type StatsSummary } from './api'

type Range = '15m' | '1h' | '24h'

function CountTable({ title, items }: { title: string; items: StatsCount[] }) {
  return (
    <div className="table-wrap">
      <div className="eyebrow" style={{ marginBottom: 8 }}>
        {title}
      </div>

      {items.length === 0 ? (
        <div className="empty-state">No data yet.</div>
      ) : (
        <table className="table">
          <thead>
            <tr>
              <th>Name</th>
              <th style={{ textAlign: 'right' }}>Count</th>
            </tr>
          </thead>
          <tbody>
            {items.map(item => (
              <tr key={item.name}>
                <td className="mono">{item.name}</td>
                <td style={{ textAlign: 'right' }}>{item.count.toLocaleString()}</td>
              </tr>
            ))}
          </tbody>
        </table>
      )}
    </div>
  )
}

export default function MonitoringSection() {
  const [range, setRange] = useState<Range>('1h')
  const [stats, setStats] = useState<StatsSummary | null>(null)
  const [error, setError] = useState('')
  const [loading, setLoading] = useState(false)

  const load = useCallback(async () => {
    setLoading(true)
    setError('')

    try {
      const data = await api.getStatsSummary(range)
      setStats(data)
    } catch (err) {
      setStats(null)
      setError(err instanceof Error ? err.message : 'Failed to load stats')
    } finally {
      setLoading(false)
    }
  }, [range])

  useEffect(() => {
    load()
    const interval = setInterval(load, 15000)
    return () => clearInterval(interval)
  }, [load])

  const cacheHitRate =
    stats && stats.totals.requests > 0
      ? ((stats.totals.cache_hits / stats.totals.requests) * 100).toFixed(1)
      : '0.0'

  const maxRequests = stats
    ? Math.max(...stats.series.map(point => point.requests), 1)
    : 1

  return (
    <section className="card">
      <div className="card-header">
        <div>
          <span className="eyebrow">Observability</span>
          <h2 className="card-title">Monitoring</h2>
        </div>

        <div style={{ display: 'flex', gap: 8, alignItems: 'center' }}>
          <select
            className="input"
            value={range}
            onChange={e => setRange(e.target.value as Range)}
          >
            <option value="15m">Last 15 minutes</option>
            <option value="1h">Last hour</option>
            <option value="24h">Last 24 hours</option>
          </select>

          <button className="btn btn-ghost" onClick={load} disabled={loading}>
            {loading ? 'Refreshing…' : 'Refresh'}
          </button>
        </div>
      </div>

      {error && <div className="alert alert-error">{error}</div>}

      {stats && (
        <>
          <div
            style={{
              display: 'grid',
              gridTemplateColumns: 'repeat(auto-fit, minmax(160px, 1fr))',
              gap: 16,
            }}
          >
            <div className="stat-card">
              <span className="stat-label">Requests</span>
              <span className="stat-value">{stats.totals.requests.toLocaleString()}</span>
            </div>

            <div className="stat-card">
              <span className="stat-label">Errors</span>
              <span className="stat-value">{stats.totals.errors.toLocaleString()}</span>
            </div>

            <div className="stat-card">
              <span className="stat-label">Cache hit rate</span>
              <span className="stat-value">{cacheHitRate}%</span>
            </div>

            <div className="stat-card">
              <span className="stat-label">Avg latency</span>
              <span className="stat-value stat-mono">
                {Math.round(stats.latency.avg_ms)}ms
              </span>
            </div>

            <div className="stat-card">
              <span className="stat-label">p95 latency</span>
              <span className="stat-value stat-mono">
                {Math.round(stats.latency.p95_ms)}ms
              </span>
            </div>
          </div>

          <div>
            <div className="eyebrow" style={{ marginBottom: 8 }}>
              Request volume
            </div>

            {stats.series.length === 0 ? (
              <div className="empty-state">No requests recorded for this range.</div>
            ) : (
              <div
                style={{
                  display: 'flex',
                  alignItems: 'flex-end',
                  gap: 2,
                  height: 120,
                  padding: 12,
                  background: 'var(--bg)',
                  border: '1px solid var(--border)',
                  borderRadius: 8,
                }}
              >
                {stats.series.map((point, index) => {
                  const height = Math.max(2, (point.requests / maxRequests) * 100)
                  const label = new Date(point.time).toLocaleString()

                  return (
                    <div
                      key={`${point.time}-${index}`}
                      title={`${label} — ${point.requests} requests, ${point.errors} errors`}
                      style={{
                        flex: 1,
                        minWidth: 2,
                        height: `${height}%`,
                        borderRadius: 2,
                        background: point.errors > 0 ? 'var(--danger)' : 'var(--accent)',
                        opacity: 0.85,
                      }}
                    />
                  )
                })}
              </div>
            )}
          </div>

          <div
            style={{
              display: 'grid',
              gridTemplateColumns: 'repeat(auto-fit, minmax(240px, 1fr))',
              gap: 16,
            }}
          >
            <CountTable title="Top methods" items={stats.top_methods} />
            <CountTable title="Top networks" items={stats.top_networks} />
            <CountTable title="Top statuses" items={stats.top_statuses} />
          </div>
        </>
      )}
    </section>
  )
}