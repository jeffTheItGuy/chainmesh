interface RoleGateProps {
  onViewer: () => void
  onAdmin: () => void
}

export default function RoleGate({ onViewer, onAdmin }: RoleGateProps) {
  return (
    <div className="login-shell">
      <div className="gate-card">
        <div className="login-mark">
          <span className="login-mark-dot" />
          BlockMesh
        </div>
        <h1 className="login-title">How do you want to continue?</h1>
        <p className="login-sub">
          Viewers can browse chain data and stats with no credentials. Admins get
          tenant management and usage metering.
        </p>

        <div className="gate-options">
          <button type="button" className="gate-option" onClick={onViewer}>
            <span className="gate-option-title">Continue as viewer</span>
            <span className="gate-option-sub">Read-only dashboard — no login required</span>
          </button>
          <button type="button" className="gate-option" onClick={onAdmin}>
            <span className="gate-option-title">Admin sign in</span>
            <span className="gate-option-sub">Requires the admin secret</span>
          </button>
        </div>
      </div>
    </div>
  )
}
