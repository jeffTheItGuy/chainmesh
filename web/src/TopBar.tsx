interface TopBarProps {
  healthStatus: string
  onLogout: () => void
}

export default function TopBar({ healthStatus, onLogout }: TopBarProps) {
  const healthy = healthStatus === 'ok'
  return (
    <header className="topbar">
      <div className="topbar-mark">
        <span className="topbar-mark-dot" />
        <span className="topbar-mark-text">BlockMesh</span>
        <span className="topbar-eyebrow">Gateway Console</span>
      </div>
      <div className="topbar-right">
        <span className={`status-pill ${healthy ? 'status-pill-ok' : 'status-pill-down'}`}>
          <span className="status-pill-dot" />
          {healthy ? 'Admin API online' : `Admin API ${healthStatus}`}
        </span>
        <button className="btn btn-ghost" onClick={onLogout}>Log out</button>
      </div>
    </header>
  )
}
