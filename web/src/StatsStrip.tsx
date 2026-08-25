import Sparkline from './components/Sparkline'

interface StatsStripProps {
  tenantCount: number | null
  latestBlock: number | null
  totalTxCount: number
  txSeries?: number[]
}

export default function StatsStrip({
  tenantCount,
  latestBlock,
  totalTxCount,
  txSeries = [],
}: StatsStripProps) {
  const trend = txSeries.length >= 2 ? txSeries[txSeries.length - 1] - txSeries[0] : 0
  const trendPositive = trend >= 0

  return (
    <div className={`stat-grid${tenantCount === null ? ' stat-grid--two' : ''}`}>
      {tenantCount !== null && (
        <div className="stat-card">
          <span className="stat-label">Tenants</span>
          <span className="stat-value">{tenantCount}</span>
        </div>
      )}
      <div className="stat-card">
        <span className="stat-label">Latest block</span>
        <span className="stat-value stat-mono">
          {latestBlock !== null ? `#${latestBlock.toLocaleString()}` : '—'}
        </span>
      </div>
      <div className="stat-card">
        <span className="stat-label">Tx in recent blocks</span>
        <span className="stat-value">{totalTxCount.toLocaleString()}</span>
        {txSeries.length >= 2 && (
          <div className="stat-trend">
            <Sparkline
              values={txSeries}
              stroke={trendPositive ? 'var(--success)' : 'var(--danger)'}
            />
            <span
              className={`stat-trend-label ${
                trendPositive ? 'stat-trend-label--up' : 'stat-trend-label--down'
              }`}
            >
              {trendPositive ? '▲' : '▼'} {Math.abs(trend).toLocaleString()}
            </span>
          </div>
        )}
      </div>
    </div>
  )
}