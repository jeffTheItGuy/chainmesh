import type { Block } from './api'

interface BlocksSectionProps {
  blocks: Block[]
  hasLoaded: boolean
}

export default function BlocksSection({ blocks, hasLoaded }: BlocksSectionProps) {
  if (!hasLoaded) {
    return <div className="empty-state">Loading blocks…</div>
  }

  return (
    <section className="card">
      <div className="card-header">
        <div>
          <span className="eyebrow">Chain data</span>
          <h2 className="card-title">Recent blocks</h2>
        </div>
      </div>
      {blocks.length === 0 ? (
        <div className="empty-state">No blocks ingested yet.</div>
      ) : (
        <div className="table-wrap">
          <table className="table">
            <thead>
              <tr>
                <th>Network</th>
                <th>Number</th>
                <th>Hash</th>
                <th>Txs</th>
                <th>Time</th>
              </tr>
            </thead>
            <tbody>
              {blocks.map(b => (
                <tr key={`${b.network_id}-${b.hash}`}>
                  <td className="mono muted">{b.network_name || b.network_id?.slice(0, 8) || '—'}</td>
                  <td className="mono">#{b.number.toLocaleString()}</td>
                  <td className="mono muted truncate" title={b.hash}>{b.hash}</td>
                  <td>{b.tx_count}</td>
                  <td className="muted">{new Date(b.timestamp).toLocaleTimeString()}</td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      )}
    </section>
  )
}