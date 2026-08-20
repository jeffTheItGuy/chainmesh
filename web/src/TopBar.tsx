interface TopBarProps {
  role: 'admin' | 'viewer'
  onLogout: () => void
}

export default function TopBar({ role, onLogout }: TopBarProps) {
  return (
    <header className="topbar">
      <div className="topbar-mark">
        <img
          src="/logo/icon.svg"
          alt=""
          className="topbar-logo"
        />
        <span className="topbar-mark-text">ChainMesh</span>
        <span className="topbar-eyebrow">Gateway Console</span>
      </div>
      <div className="topbar-right">
        <span className="topbar-eyebrow">{role === 'admin' ? 'Admin' : 'Viewer'}</span>
        <button className="btn btn-ghost" onClick={onLogout}>
          Log out
        </button>
      </div>
    </header>
  )
}