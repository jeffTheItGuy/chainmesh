import { useState, type FormEvent } from 'react'
import { api, type BlockchainConfig, type TestConnectionResult } from './api'

interface BlockchainSectionProps {
  networks: BlockchainConfig[]
  onNetworksChanged: () => void
}

export default function BlockchainSection({ networks, onNetworksChanged }: BlockchainSectionProps) {
  const safeNetworks = networks || []
  const [editing, setEditing] = useState<BlockchainConfig | null>(null)
  const [showForm, setShowForm] = useState(false)
  const [name, setName] = useState('')
  const [rpc1, setRpc1] = useState('')
  const [rpc2, setRpc2] = useState('')
  const [chainId, setChainId] = useState('')
  const [enabled, setEnabled] = useState(true)
  const [testResult, setTestResult] = useState<TestConnectionResult | null>(null)
  const [error, setError] = useState('')
  const [loading, setLoading] = useState(false)
  const [testing, setTesting] = useState(false)

  const resetForm = () => {
    setName('')
    setRpc1('')
    setRpc2('')
    setChainId('')
    setEnabled(true)
    setTestResult(null)
    setError('')
    setEditing(null)
    setShowForm(false)
  }

  const startEdit = (cfg: BlockchainConfig) => {
    setEditing(cfg)
    setName(cfg.name)
    setRpc1(cfg.rpc_endpoint_1)
    setRpc2(cfg.rpc_endpoint_2 ?? '')
    setChainId(cfg.chain_id ?? '')
    setEnabled(cfg.enabled)
    setShowForm(true)
    setTestResult(null)
    setError('')
  }

  const test = async () => {
    if (!rpc1) return
    setTesting(true)
    setError('')
    setTestResult(null)
    try {
      const result = await api.testBlockchainConnection({ rpc_endpoint_1: rpc1, rpc_endpoint_2: rpc2 })
      setTestResult(result)
      // FIX: Auto-fill Chain ID from successful test
      if (result.connected && result.chain_id) {
        setChainId(result.chain_id)
      }
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Test failed')
    } finally {
      setTesting(false)
    }
  }

  const submit = async (e: FormEvent) => {
    e.preventDefault()
    if (!name || !rpc1) return
    setLoading(true)
    setError('')
    try {
      const payload = {
        name,
        rpc_endpoint_1: rpc1,
        rpc_endpoint_2: rpc2,
        chain_id: chainId,
        enabled,
      }
      if (editing) {
        await api.updateBlockchainConfig(editing.id, payload)
      } else {
        await api.createBlockchainConfig(payload)
      }
      onNetworksChanged()
      resetForm()
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Save failed')
    } finally {
      setLoading(false)
    }
  }

  const remove = async (id: string) => {
    if (!confirm('Delete this network? Tenants using it will fall back to the default.')) return
    try {
      await api.deleteBlockchainConfig(id)
      onNetworksChanged()
    } catch (err) {
      alert(err instanceof Error ? err.message : 'Delete failed')
    }
  }

  return (
    <section className="card">
      <div className="card-header">
        <div>
          <span className="eyebrow">Infrastructure</span>
          <h2 className="card-title">Blockchain Networks</h2>
        </div>
        <button className="btn btn-primary" onClick={() => { resetForm(); setShowForm(v => !v) }}>
          {showForm ? 'Cancel' : 'Add network'}
        </button>
      </div>
      {showForm && (
        <form onSubmit={submit} className="inline-form">
          <div className="field">
            <label className="label" htmlFor="bc-name">Network Name</label>
            <input id="bc-name" className="input" value={name} onChange={e => setName(e.target.value)} placeholder="Ethereum Mainnet" required />
          </div>
          <div className="field">
            <label className="label" htmlFor="bc-rpc1">RPC Endpoint 1</label>
            <input id="bc-rpc1" type="url" className="input" value={rpc1} onChange={e => setRpc1(e.target.value)} placeholder="https://..." required />
          </div>
          <div className="field">
            <label className="label" htmlFor="bc-rpc2">RPC Endpoint 2 (optional)</label>
            <input id="bc-rpc2" type="url" className="input" value={rpc2} onChange={e => setRpc2(e.target.value)} placeholder="https://..." />
          </div>
          <div className="field">
            <label className="label" htmlFor="bc-chain">Chain ID (optional)</label>
            <input id="bc-chain" className="input" value={chainId} onChange={e => setChainId(e.target.value)} placeholder="1" />
          </div>
          <div className="field" style={{ flexDirection: 'row', alignItems: 'center', gap: 8 }}>
            <input id="bc-enabled" type="checkbox" checked={enabled} onChange={e => setEnabled(e.target.checked)} />
            <label className="label" htmlFor="bc-enabled" style={{ margin: 0 }}>Enabled</label>
          </div>
          {testResult && (
            <div className={`alert ${testResult.connected ? 'alert-success' : 'alert-error'}`}>
              {testResult.connected ? <>Connected — Chain ID: {testResult.chain_id}</> : <>Connection failed — {testResult.error}</>}
            </div>
          )}
          {error && <div className="alert alert-error">{error}</div>}
          <div style={{ display: 'flex', gap: '12px' }}>
            <button type="button" className="btn btn-ghost" onClick={test} disabled={!rpc1 || testing}>
              {testing ? 'Testing…' : 'Test Connection'}
            </button>
            <button type="submit" className="btn btn-primary" disabled={!name || !rpc1 || loading}>
              {loading ? 'Saving…' : (editing ? 'Update network' : 'Add network')}
            </button>
          </div>
        </form>
      )}
      {safeNetworks.length === 0 ? (
        <div className="empty-state">No networks configured. Add one to enable the gateway.</div>
      ) : (
        <div className="table-wrap">
          <table className="table">
            <thead>
              <tr>
                <th>Name</th>
                <th>Chain ID</th>
                <th>RPC 1</th>
                <th>Enabled</th>
                <th style={{ textAlign: 'right' }}>Actions</th>
              </tr>
            </thead>
            <tbody>
              {safeNetworks.map(n => (
                <tr key={n.id}>
                  <td>{n.name}</td>
                  <td className="mono">{n.chain_id || '—'}</td>
                  <td className="mono muted truncate" title={n.rpc_endpoint_1}>
                    {n.rpc_endpoint_1.replace(/^https?:\/\//, '')}
                  </td>
                  <td>{n.enabled ? 'Yes' : 'No'}</td>
                  <td style={{ textAlign: 'right' }}>
                    <button className="btn btn-ghost btn-sm" onClick={() => startEdit(n)}>Edit</button>
                    <button className="btn btn-ghost btn-sm" onClick={() => remove(n.id)} style={{ marginLeft: 8 }}>Delete</button>
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      )}
    </section>
  )
}