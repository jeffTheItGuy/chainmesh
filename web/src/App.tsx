import { useEffect, useState } from 'react'

function App() {
  const [health, setHealth] = useState('checking...')

  useEffect(() => {
    fetch('/api/health')
      .then(r => r.json())
      .then(d => setHealth(d.status))
      .catch(() => setHealth('down'))
  }, [])

  return (
    <div style={{ padding: 40, fontFamily: 'sans-serif' }}>
      <h1>BlockMesh Admin</h1>
      <p>Admin API: {health}</p>
    </div>
  )
}

export default App
