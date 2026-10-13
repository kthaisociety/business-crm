// The frontend talks only to its own backend, on the same origin (docs/adr/0001). No URLs are baked
// in at build time.

import type { Activity, Deal, DealType } from './data/deals'

export type User = {
  sub: string
  email: string
  name: string
}

// A failed API call. The message is the server's reason when it gave one (400s say what was wrong).
export class ApiError extends Error {
  readonly status: number
  constructor(status: number, message: string) {
    super(message)
    this.status = status
  }
}

async function call<T>(method: string, path: string, body?: unknown, signal?: AbortSignal): Promise<T> {
  const res = await fetch(path, {
    method,
    signal,
    headers: body === undefined ? undefined : { 'Content-Type': 'application/json' },
    body: body === undefined ? undefined : JSON.stringify(body),
  })
  if (!res.ok) {
    let reason = ''
    try {
      reason = (await res.json()).error ?? ''
    } catch {
      // Not JSON: fall back to the status alone.
    }
    throw new ApiError(res.status, reason ? `${reason} (${res.status})` : `Request failed (${res.status})`)
  }
  return res.json()
}

export async function fetchMe(signal?: AbortSignal): Promise<User> {
  const res = await fetch('/api/me', { signal })
  if (!res.ok) {
    throw new Error(`Loading the signed-in user failed (${res.status})`)
  }
  return res.json()
}

export const fetchDeals = (signal?: AbortSignal) => call<Deal[]>('GET', '/api/deals', undefined, signal)

export type NewLeadInput = {
  company: string
  dealType: DealType
  semester: string
  contactName: string
  contactEmail: string
  contactRole: string
  nextStep: string
}

export const createDeal = (lead: NewLeadInput) => call<Deal>('POST', '/api/deals', lead)

// The fields the deal panel edits; the API rejects any other.
export type DealPatch = Partial<Pick<Deal, 'status' | 'nextStep' | 'nextStepDue' | 'feeSEK' | 'invoiceStatus'>>

export const updateDeal = (id: string, patch: DealPatch) =>
  call<Deal>('PATCH', `/api/deals/${encodeURIComponent(id)}`, patch)

export const addNote = (id: string, body: string) =>
  call<Activity>('POST', `/api/deals/${encodeURIComponent(id)}/notes`, { body })
