import { useEffect, useState } from 'react'

// Skeleton page: proves the web → api → postgres/redis wiring. Replace with the real screens.
export default function App() {
  const [health, setHealth] = useState<Record<string, boolean> | 'loading' | 'unreachable'>('loading')

  useEffect(() => {
    fetch('/api/healthz')
      .then((r) => r.json())
      .then(setHealth)
      .catch(() => setHealth('unreachable'))
  }, [])

  return (
    <main className="mx-auto max-w-[480px] p-4 text-ink">
      <h1 className="text-2xl font-bold">روشن روزگار · Roshan Rozgar</h1>
      {typeof health === 'string' ? (
        <p className="text-muted">API: {health}</p>
      ) : (
        <ul>
          {Object.entries(health).map(([name, up]) => (
            <li key={name} className={up ? 'text-go' : 'text-stop'}>
              {name}: {up ? 'up' : 'down'}
            </li>
          ))}
        </ul>
      )}
    </main>
  )
}
