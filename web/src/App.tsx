import { useEffect, useState } from 'react'

interface Tenant {
  id: string
  name: string
  quota_rpm: number
  created_at: string
}

interface Block {
  number: number
  hash: string
  parent_hash: string
  timestamp: string
  tx_count: number
}

interface Usage {
  tenant_id: string
  method: string
  count: number
  bytes_in: number
  period: string
}

interface CreatedTenant {
  id: string
  name: string
  api_key: string
  quota_rpm: number
  created_at: string
}

function App() {
  const [health, setHealth] = useState('checking...')
  const [tenants, setTenants] = useState<Tenant[]>([])
  const [blocks, setBlocks] = useState<Block[]>([])
  const [usage, setUsage] = useState<Usage[]>([])
  const [error, setError] = useState('')
  const [newName, setNewName] = useState('')
  const [newQuota, setNewQuota] = useState(1000)
  const [created, setCreated] = useState<CreatedTenant | null>(null)
  const [adminSecret, setAdminSecret] = useState('')
  const [showCreate, setShowCreate] = useState(false)

  useEffect(() => {
    fetch('/api/health')
      .then(r => r.json())
      .then(d => setHealth(d.status))
      .catch(() => setHealth('down'))

    fetch('/api/tenants')
      .then(r => r.json())
      .then(d => setTenants(Array.isArray(d) ? d : []))
      .catch(err => setError(err.message))

    fetch('/api/blocks')
      .then(r => r.json())
      .then(d => setBlocks(Array.isArray(d) ? d : []))
      .catch(err => console.error(err))
  }, [])

  const loadUsage = (apiKey: string) => {
    fetch(`/api/usage?api_key=${encodeURIComponent(apiKey)}`)
      .then(r => r.json())
      .then(d => setUsage(Array.isArray(d) ? d : []))
      .catch(err => console.error(err))
  }

  const createTenant = () => {
    setError('')
    fetch('/api/tenants', {
      method: 'POST',
      headers: {
        'Content-Type': 'application/json',
        ...(adminSecret && { 'X-Admin-Secret': adminSecret })
      },
      body: JSON.stringify({ name: newName, quota_rpm: newQuota })
    })
      .then(r => {
        if (!r.ok) throw new Error(`HTTP ${r.status}`)
        return r.json()
      })
      .then((d: CreatedTenant) => {
        setCreated(d)
        setNewName('')
        setNewQuota(1000)
        // Refresh tenant list
        fetch('/api/tenants')
          .then(r => r.json())
          .then(d => setTenants(Array.isArray(d) ? d : []))
      })
      .catch(err => setError(err.message))
  }

  const copyKey = (key: string) => {
    navigator.clipboard.writeText(key)
  }

  return (
    <div style={{ padding: 40, fontFamily: 'system-ui, sans-serif', maxWidth: 1200, margin: '0 auto' }}>
      <h1 style={{ color: '#2563eb' }}>BlockMesh Admin</h1>

      <div style={{ background: '#f8fafc', borderRadius: 8, padding: 20, margin: '16px 0' }}>
        <h2>System Health</h2>
        <p>Admin API: <code style={{ background: '#e2e8f0', padding: '2px 6px', borderRadius: 4 }}>{health}</code></p>
      </div>

      <div style={{ background: '#f8fafc', borderRadius: 8, padding: 20, margin: '16px 0' }}>
        <h2>Gateway</h2>
        <p>Proxy endpoint: <code style={{ background: '#e2e8f0', padding: '2px 6px', borderRadius: 4 }}>POST /v1/</code></p>
        <p>Auth header: <code style={{ background: '#e2e8f0', padding: '2px 6px', borderRadius: 4 }}>Authorization: Bearer demo-key</code></p>
      </div>

      {error && <div style={{ color: '#dc2626', background: '#fef2f2', padding: 12, borderRadius: 8, margin: '16px 0' }}>{error}</div>}

      {created && (
        <div style={{ background: '#dcfce7', borderRadius: 8, padding: 20, margin: '16px 0', border: '1px solid #86efac' }}>
          <h3>✅ Tenant Created</h3>
          <p><strong>Name:</strong> {created.name}</p>
          <p><strong>Quota:</strong> {created.quota_rpm} RPM</p>
          <p>
            <strong>API Key:</strong>{' '}
            <code style={{ background: '#fff', padding: '2px 6px', borderRadius: 4, fontSize: 12 }}>{created.api_key}</code>{' '}
            <button onClick={() => copyKey(created.api_key)} style={{ cursor: 'pointer', marginLeft: 8 }}>Copy</button>
          </p>
          <p style={{ fontSize: 12, color: '#166534' }}>Copy this now — it will not be shown again.</p>
          <button onClick={() => setCreated(null)} style={{ cursor: 'pointer' }}>Dismiss</button>
        </div>
      )}

      <div style={{ background: '#f8fafc', borderRadius: 8, padding: 20, margin: '16px 0' }}>
        <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center' }}>
          <h2>Tenants</h2>
          <button onClick={() => setShowCreate(!showCreate)} style={{ cursor: 'pointer' }}>
            {showCreate ? 'Cancel' : '+ Create Tenant'}
          </button>
        </div>

        {showCreate && (
          <div style={{ background: '#fff', borderRadius: 8, padding: 16, margin: '12px 0', border: '1px solid #e2e8f0' }}>
            <div style={{ marginBottom: 8 }}>
              <label>Name: </label>
              <input value={newName} onChange={e => setNewName(e.target.value)} placeholder="Client Name" />
            </div>
            <div style={{ marginBottom: 8 }}>
              <label>Quota (RPM): </label>
              <input type="number" value={newQuota} onChange={e => setNewQuota(Number(e.target.value))} min={1} />
            </div>
            <div style={{ marginBottom: 8 }}>
              <label>Admin Secret: </label>
              <input 
                type="password" 
                value={adminSecret} 
                onChange={e => setAdminSecret(e.target.value)} 
                placeholder="Leave blank if not set"
              />
              <span style={{ fontSize: 12, color: '#64748b', marginLeft: 8 }}>Only needed if ADMIN_SECRET is configured</span>
            </div>
            <button onClick={createTenant} disabled={!newName} style={{ cursor: 'pointer' }}>Create</button>
          </div>
        )}

        <table style={{ width: '100%', borderCollapse: 'collapse' }}>
          <thead>
            <tr style={{ borderBottom: '2px solid #e2e8f0', textAlign: 'left' }}>
              <th style={{ padding: 8 }}>Name</th>
              <th style={{ padding: 8 }}>Quota (RPM)</th>
              <th style={{ padding: 8 }}>Created</th>
              <th style={{ padding: 8 }}>Actions</th>
            </tr>
          </thead>
          <tbody>
            {tenants.map(t => (
              <tr key={t.id} style={{ borderBottom: '1px solid #e2e8f0' }}>
                <td style={{ padding: 8 }}>{t.name}</td>
                <td style={{ padding: 8 }}>{t.quota_rpm}</td>
                <td style={{ padding: 8 }}>{new Date(t.created_at).toLocaleDateString()}</td>
                <td style={{ padding: 8 }}>
                  <button onClick={() => loadUsage(t.api_key)} style={{ cursor: 'pointer' }}>View Usage</button>
                </td>
              </tr>
            ))}
          </tbody>
        </table>
      </div>

      {usage.length > 0 && (
        <div style={{ background: '#f8fafc', borderRadius: 8, padding: 20, margin: '16px 0' }}>
          <h2>Usage Report</h2>
          <table style={{ width: '100%', borderCollapse: 'collapse' }}>
            <thead>
              <tr style={{ borderBottom: '2px solid #e2e8f0', textAlign: 'left' }}>
                <th style={{ padding: 8 }}>Method</th>
                <th style={{ padding: 8 }}>Count</th>
                <th style={{ padding: 8 }}>Bytes In</th>
                <th style={{ padding: 8 }}>Period</th>
              </tr>
            </thead>
            <tbody>
              {usage.map((u, i) => (
                <tr key={i} style={{ borderBottom: '1px solid #e2e8f0' }}>
                  <td style={{ padding: 8 }}>{u.method}</td>
                  <td style={{ padding: 8 }}>{u.count}</td>
                  <td style={{ padding: 8 }}>{u.bytes_in}</td>
                  <td style={{ padding: 8 }}>{new Date(u.period).toLocaleString()}</td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      )}

      <div style={{ background: '#f8fafc', borderRadius: 8, padding: 20, margin: '16px 0' }}>
        <h2>Recent Blocks</h2>
        <table style={{ width: '100%', borderCollapse: 'collapse' }}>
          <thead>
            <tr style={{ borderBottom: '2px solid #e2e8f0', textAlign: 'left' }}>
              <th style={{ padding: 8 }}>Number</th>
              <th style={{ padding: 8 }}>Hash</th>
              <th style={{ padding: 8 }}>Tx Count</th>
              <th style={{ padding: 8 }}>Time</th>
            </tr>
          </thead>
          <tbody>
            {blocks.map(b => (
              <tr key={b.hash} style={{ borderBottom: '1px solid #e2e8f0' }}>
                <td style={{ padding: 8 }}>{b.number}</td>
                <td style={{ padding: 8, fontFamily: 'monospace', fontSize: 12 }}>{b.hash.slice(0, 20)}...</td>
                <td style={{ padding: 8 }}>{b.tx_count}</td>
                <td style={{ padding: 8 }}>{new Date(b.timestamp).toLocaleString()}</td>
              </tr>
            ))}
          </tbody>
        </table>
      </div>
    </div>
  )
}

export default App
