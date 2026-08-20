interface TopBarProps {
  role: 'admin' | 'viewer'
  showDocs?: boolean
  onToggleDocs?: () => void
  onLogout: () => void
}

export default function TopBar({ role, showDocs, onToggleDocs, onLogout }: TopBarProps) {
  return (
    <header className="topbar">
      <div className="topbar-mark">
        <img
          src="/logo/icon.svg"
          alt=""
          width={18}
          height={18}
          style={{ display: 'inline-block' }}
        />
        <span className="topbar-mark-text">ChainMesh</span>
        <span className="topbar-eyebrow">Gateway Console</span>
      </div>
      <div className="topbar-right">
        <span className="topbar-eyebrow">{role === 'admin' ? 'Admin' : 'Viewer'}</span>
        {onToggleDocs && (
          <button className="btn btn-ghost" onClick={onToggleDocs}>
            {showDocs ? 'Dashboard' : 'API Docs'}
          </button>
        )}
        <button className="btn btn-ghost" onClick={onLogout}>
          Log out
        </button>
      </div>
    </header>
  )
}
