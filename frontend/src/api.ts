// The frontend talks only to its own backend, on the same origin (docs/adr/0001). No URLs are baked
// in at build time.

export type User = {
  sub: string
  email: string
  name: string
}

export async function fetchMe(signal?: AbortSignal): Promise<User> {
  const res = await fetch('/api/me', { signal })
  if (!res.ok) {
    throw new Error(`Loading the signed-in user failed (${res.status})`)
  }
  return res.json()
}
