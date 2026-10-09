import { useEffect, useState } from 'react'
import { fetchMe, type User } from './api'

type State =
  | { status: 'loading' }
  | { status: 'error'; message: string }
  | { status: 'ready'; user: User }

export default function App() {
  const [state, setState] = useState<State>({ status: 'loading' })

  useEffect(() => {
    const controller = new AbortController()
    fetchMe(controller.signal)
      .then((user) => setState({ status: 'ready', user }))
      .catch((err: unknown) => {
        if (controller.signal.aborted) return
        setState({ status: 'error', message: err instanceof Error ? err.message : String(err) })
      })
    return () => controller.abort()
  }, [])

  return (
    <main className="mx-auto max-w-5xl p-6">
      <h1 className="text-2xl font-semibold">Business CRM</h1>
      {state.status === 'loading' && <p className="mt-4 text-neutral-500">Loading…</p>}
      {state.status === 'error' && <p className="mt-4 text-red-600">{state.message}</p>}
      {state.status === 'ready' && (
        <p className="mt-1 text-neutral-500">Signed in as {state.user.email}</p>
      )}
    </main>
  )
}
