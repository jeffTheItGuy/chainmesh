export function SkeletonStat() {
  return (
    <div className="stat-card">
      <div className="skeleton skeleton-text" style={{ width: '40%' }} />
      <div className="skeleton skeleton-stat" />
    </div>
  )
}

export function SkeletonChart() {
  return <div className="skeleton skeleton-chart" />
}

export function SkeletonTable({ rows = 3 }: { rows?: number }) {
  return (
    <div>
      <div className="skeleton skeleton-title" style={{ width: '30%', marginBottom: 12 }} />
      {Array.from({ length: rows }).map((_, i) => (
        <div key={i} className="skeleton skeleton-text" style={{ marginBottom: 8 }} />
      ))}
    </div>
  )
}

export function SkeletonBlocks() {
  return (
    <section className="card">
      <div className="card-header">
        <div>
          <div className="skeleton skeleton-text" style={{ width: 80, marginBottom: 4 }} />
          <div className="skeleton skeleton-title" style={{ width: 140 }} />
        </div>
      </div>
      <SkeletonTable rows={5} />
    </section>
  )
}
