import { useState, type FormEvent } from 'react'
import { verifySecret } from './api'
import { storeSecret } from './auth'

interface LoginProps {
  onAuthenticated: () => void
}

export default function Login({ onAuthenticated }: LoginProps) {
  const [secret, setSecret] = useState('')
  const [checking, setChecking] = useState(false)
  const [error, setError] = useState('')

  const submit = async (e: FormEvent) => {
    e.preventDefault()
    if (!secret) return
    setChecking(true)
    setError('')
    try {
      const ok = await verifySecret(secret)
      if (!ok) {
        setError('That secret was rejected. Check it and try again.')
        return
      }
      storeSecret(secret)
      onAuthenticated()
    } catch {
      setError('Could not reach the admin API. Is it running?')
    } finally {
      setChecking(false)
    }
  }

  return (
    <div className="login-shell">
      <div className="login-card">
        <div className="login-mark">
          <span className="login-mark-dot" />
          BlockMesh
        </div>
        <h1 className="login-title">Sign in to the console</h1>
        <p className="login-sub">Enter the admin secret configured on this deployment.</p>

        <form onSubmit={submit} className="login-form">
          <label className="label" htmlFor="secret">Admin secret</label>
          <input
            id="secret"
            type="password"
            className="input"
            value={secret}
            onChange={e => setSecret(e.target.value)}
            placeholder="ADMIN_SECRET"
            autoFocus
            autoComplete="current-password"
          />
          {error && <div className="alert alert-error">{error}</div>}
          <button type="submit" className="btn btn-primary" disabled={checking || !secret}>
            {checking ? 'Checking…' : 'Enter console'}
          </button>
        </form>
      </div>
    </div>
  )
}
