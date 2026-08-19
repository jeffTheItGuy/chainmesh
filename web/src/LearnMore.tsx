interface LearnMoreProps {
  onBack: () => void
}

export default function LearnMore({ onBack }: LearnMoreProps) {
  return (
    <div className="login-shell">
      <div className="gate-card">
        <button
          type="button"
          className="btn btn-ghost btn-sm login-back"
          onClick={onBack}
        >
          ← Back
        </button>

        <div className="login-mark">
          <span className="login-mark-dot" />
          BlockMesh
        </div>

        <h1 className="login-title">What is BlockMesh?</h1>
        <p className="login-sub">
          A production-grade, self-hosted API gateway that sits between your
          applications and blockchain RPC nodes.
        </p>

        <div className="gate-options">
          <div className="gate-option" style={{ cursor: 'default' }}>
            <span className="gate-option-title">🧠 Smart Routing</span>
            <span className="gate-option-sub">
              Health-aware failover across multiple RPC endpoints. Automatically
              routes to the fastest healthy node and retries on failure.
            </span>
          </div>

          <div className="gate-option" style={{ cursor: 'default' }}>
            <span className="gate-option-title">🔐 Secure Multi-Tenancy</span>
            <span className="gate-option-sub">
              SHA-256 hashed API keys, per-tenant rate limits (RPS/RPM/daily),
              key rotation, and constant-time authentication.
            </span>
          </div>

          <div className="gate-option" style={{ cursor: 'default' }}>
            <span className="gate-option-title">⚡ Domain-Aware Caching</span>
            <span className="gate-option-sub">
              Redis-backed caching with method-specific TTLs — e.g.,
              eth_chainId for 24h, eth_blockNumber for 2s.
            </span>
          </div>

          <div className="gate-option" style={{ cursor: 'default' }}>
            <span className="gate-option-title">📊 Real-Time Observability</span>
            <span className="gate-option-sub">
              Async telemetry, pre-aggregated stats views, Prometheus metrics,
              and a live dashboard for monitoring requests and node health.
            </span>
          </div>

          <div className="gate-option" style={{ cursor: 'default' }}>
            <span className="gate-option-title">🔗 Multi-Network Support</span>
            <span className="gate-option-sub">
              Manage Ethereum, Sepolia, Polygon, and other networks via UI or
              Admin API without restarting the gateway.
            </span>
          </div>
        </div>

        <div
          className="mt-16"
          style={{ display: 'flex', flexDirection: 'column', gap: 8 }}
        >
          <a
            href="https://github.com/jeffTheItGuy/chainmesh/tree/main/docs"
            target="_blank"
            rel="noopener noreferrer"
            className="btn btn-primary w-full"
            style={{ display: 'block', textAlign: 'center', textDecoration: 'none' }}
          >
            Read full documentation →
          </a>
          <p
            className="alert-note"
            style={{ textAlign: 'center', marginBottom: 0 }}
          >
            Covers deployment, architecture, API reference, and troubleshooting.
          </p>
        </div>
      </div>
    </div>
  )
}