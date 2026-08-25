import { useCallback, useState, useRef } from 'react'
import { api, type StatsCount, type StatsSummary, type StatsSeriesPoint } from './api'
import { usePolling } from './hooks/usePolling'
import { SkeletonStat, SkeletonChart, SkeletonTable } from './components/Skeleton'
import { IconActivity, IconRefresh } from './components/Icons'

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

function RequestChart({ series }: { series: StatsSeriesPoint[] }) {
  const width = 640
  const height = 160
  const padTop = 14
  const padBottom = 22
  const padLeft = 2
  const padRight = 2
  const innerHeight = height - padTop - padBottom

  const maxRequests = Math.max(...series.map((p) => p.requests), 1)

  const xFor = (i: number) =>
    padLeft + (i / Math.max(series.length - 1, 1)) * (width - padLeft - padRight)
  const yFor = (v: number) => padTop + innerHeight - (v / maxRequests) * innerHeight

  const linePoints = series.map((p, i) => `${xFor(i)},${yFor(p.requests)}`).join(' ')
  const areaPoints = `${padLeft},${padTop + innerHeight} ${linePoints} ${xFor(
    series.length - 1
  )},${padTop + innerHeight}`

  const firstLabel = new Date(series[0].time).toLocaleTimeString([], {
    hour: '2-digit',
    minute: '2-digit',
  })
  const lastLabel = new Date(series[series.length - 1].time).toLocaleTimeString([], {
    hour: '2-digit',
    minute: '2-digit',
  })

  return (
    <svg
      className="request-chart"
      viewBox={`0 0 ${width} ${height}`}
      preserveAspectRatio="none"
      role="img"
      aria-label={`Request volume across ${series.length} data points, peaking at ${maxRequests} requests`}
    >
      {[0, 0.5, 1].map((g) => {
        const y = padTop + innerHeight * (1 - g)
        return (
          <line key={g} x1={padLeft} x2={width - padRight} y1={y} y2={y} className="chart-grid-line" />
        )
      })}

      <polygon points={areaPoints} className="chart-area-fill" />
      <polyline points={linePoints} className="chart-line" />

      {series.map((p, i) => (
        <circle
          key={`hit-${p.time}-${i}`}
          cx={xFor(i)}
          cy={yFor(p.requests)}
          r={8}
          className="chart-hit-target"
        >
          <title>
            {`${new Date(p.time).toLocaleString()} — ${p.requests} requests, ${p.errors} errors, ${p.cache_hits} cache hits`}
          </title>
        </circle>
      ))}

      {series.map((p, i) =>
        p.errors > 0 ? (
          <circle
            key={`err-${p.time}-${i}`}
            cx={xFor(i)}
            cy={yFor(p.requests)}
            r={3}
            className="chart-error-dot"
          />
        ) : null
      )}

      <text x={padLeft} y={height - 6} className="chart-axis-label">
        {firstLabel}
      </text>
      <text x={width - padRight} y={height - 6} textAnchor="end" className="chart-axis-label">
        {lastLabel}
      </text>
      <text x={padLeft} y={padTop + 10} className="chart-axis-label chart-axis-label--muted">
        {maxRequests.toLocaleString()} req peak
      </text>
    </svg>
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

  if (!hasLoaded && !stats) {
    return (
      <section className="card">
        <div className="card-header">
          <div>
            <span className="eyebrow">Observability</span>
            <h2 className="card-title card-title-row">
              <IconActivity size={16} className="card-icon" />
              Monitoring
            </h2>
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
          <h2 className="card-title card-title-row">
            <IconActivity size={16} className="card-icon" />
            Monitoring
          </h2>
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
            className="btn btn-ghost btn-with-icon"
            onClick={() => load(new AbortController().signal, false)}
            disabled={loading}
          >
            <IconRefresh size={14} className={loading ? 'icon-spin' : ''} />
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
              <div className="chart-container">
                <RequestChart series={stats.series} />
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