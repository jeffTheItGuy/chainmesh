import { useCallback, useState, useRef } from 'react'
import { api, type StatsCount, type StatsSummary } from './api'
import { usePolling } from './hooks/usePolling'
import { SkeletonStat, SkeletonChart, SkeletonTable } from './components/Skeleton'

type Range = '15m' | '1h' | '24h'

function CountTable({ title, items }: { title: string; items: StatsCount[] }) {
  return (
    <div className="table-wrap">
      <div className="eyebrow mb-8">{title}</div>
      {items.length === 0 ? (
        <div className="empty-state">No data yet.</div>
      ) : (
        <table className="table">
          <thead>
            <tr>
              <th>Name</th>
              <th className="text-right">Count</th>
            </tr>
          </thead>
          <tbody>
            {items.map((item) => (
              <tr key={item.name}>
                <td className="mono">{item.name}</td>
                <td className="text-right">{item.count.toLocaleString()}</td>
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
  const [hasLoaded, setHasLoaded] = useState(false)

  const reqId = useRef(0)

  const load = useCallback(
    async (signal: AbortSignal, background: boolean) => {
      const id = ++reqId.current
      if (!background) setLoading(true)
      setError('')
      try {
        const data = await api.getStatsSummary(range)
        if (signal.aborted) return
        if (id === reqId.current) {
          setStats(data)
          setHasLoaded(true)
        }
      } catch (err) {
        if (signal.aborted) return
        if (id === reqId.current) {
          setStats(null)
          setError(err instanceof Error ? err.message : 'Failed to load stats')
          setHasLoaded(true)
        }
      } finally {
        if (signal.aborted) return
        if (id === reqId.current && !background) setLoading(false)
      }
    },
    [range]
  )

  usePolling(load, 15000, [range])

  const cacheHitRate =
    stats && stats.totals.requests > 0
      ? ((stats.totals.cache_hits / stats.totals.requests) * 100).toFixed(1)
      : '0.0'

  const maxRequests = stats ? Math.max(...stats.series.map((point) => point.requests), 1) : 1

  if (!hasLoaded && !stats) {
    return (
      <section className="card">
        <div className="card-header">
          <div>
            <span className="eyebrow">Observability</span>
            <h2 className="card-title">Monitoring</h2>
          </div>
        </div>
        <div className="grid grid-cols-auto-160 gap-16">
          <SkeletonStat />
          <SkeletonStat />
          <SkeletonStat />
          <SkeletonStat />
          <SkeletonStat />
        </div>
        <SkeletonChart />
        <div className="grid grid-cols-auto-240 gap-16">
          <SkeletonTable rows={4} />
          <SkeletonTable rows={4} />
          <SkeletonTable rows={4} />
        </div>
      </section>
    )
  }

  return (
    <section className="card">
      <div className="card-header">
        <div>
          <span className="eyebrow">Observability</span>
          <h2 className="card-title">Monitoring</h2>
        </div>
        <div className="flex items-center gap-8">
          <select
            className="input"
            value={range}
            onChange={(e) => setRange(e.target.value as Range)}
            aria-label="Time range"
          >
            <option value="15m">Last 15 minutes</option>
            <option value="1h">Last hour</option>
            <option value="24h">Last 24 hours</option>
          </select>
          <button
            className="btn btn-ghost"
            onClick={() => load(new AbortController().signal, false)}
            disabled={loading}
          >
            {loading ? 'Refreshing…' : 'Refresh'}
          </button>
        </div>
      </div>
      {error && (
        <div className="alert alert-error" role="alert">
          {error}
        </div>
      )}
      {stats && (
        <>
          <div className="grid grid-cols-auto-160 gap-16">
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
              <span className="stat-value stat-mono">{Math.round(stats.latency.avg_ms)}ms</span>
            </div>
            <div className="stat-card">
              <span className="stat-label">p95 latency</span>
              <span className="stat-value stat-mono">
                {Math.round(stats.latency.p95_ms)}ms
              </span>
            </div>
          </div>
          <div>
            <div className="eyebrow mb-8">Request volume</div>
            {stats.series.length === 0 ? (
              <div className="empty-state">No requests recorded for this range.</div>
            ) : (
              <div
                className="chart-container"
                role="img"
                aria-label={`Request volume chart showing ${stats.series.length} data points`}
              >
                {stats.series.map((point, index) => {
                  const height = Math.max(2, (point.requests / maxRequests) * 100)
                  const label = new Date(point.time).toLocaleString()
                  return (
                    <div
                      key={`${point.time}-${index}`}
                      title={`${label} — ${point.requests} requests, ${point.errors} errors`}
                      className={`chart-bar ${point.errors > 0 ? 'chart-bar--error' : ''}`}
                      style={{ height: `${height}%` }}
                    />
                  )
                })}
              </div>
            )}
          </div>
          <div className="grid grid-cols-auto-240 gap-16">
            <CountTable title="Top methods" items={stats.top_methods} />
            <CountTable title="Top networks" items={stats.top_networks} />
            <CountTable title="Top statuses" items={stats.top_statuses} />
          </div>
        </>
      )}
    </section>
  )
}
