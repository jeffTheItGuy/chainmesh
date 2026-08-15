interface StatsStripProps {
  tenantCount: number
  latestBlock: number | null
  totalTxCount: number
}

export default function StatsStrip({ tenantCount, latestBlock, totalTxCount }: StatsStripProps) {
  return (
    <div className="stat-grid">
      <div className="stat-card">
        <span className="stat-label">Tenants</span>
        <span className="stat-value">{tenantCount}</span>
      </div>
      <div className="stat-card">
        <span className="stat-label">Latest block</span>
        <span className="stat-value stat-mono">
          {latestBlock !== null ? `#${latestBlock.toLocaleString()}` : '—'}
        </span>
      </div>
      <div className="stat-card">
        <span className="stat-label">Tx in recent blocks</span>
        <span className="stat-value">{totalTxCount.toLocaleString()}</span>
      </div>
    </div>
  )
}
