// An in-memory stand-in for the Go API, behind a stubbed fetch (docs/pcr/0002: tests never call real
// services). It starts from the fixture deals and answers the same routes the app calls.
import { vi } from 'vitest'
import type { Activity, Deal } from '../data/deals'
import { fixtureDeals } from './fixtures'

export const devUser = { sub: 'dev', email: 'dev@kthais.com', name: 'Dev User' }

type Failure = { method: string; path: RegExp; status: number; error: string }

export type FakeApi = {
  deals: Deal[]
  requests: { method: string; path: string; body: unknown }[]
  // Make matching requests fail until cleared with `failures.length = 0`.
  failures: Failure[]
  fail: (method: string, path: RegExp, status?: number, error?: string) => void
}

const json = (body: unknown, status = 200) =>
  new Response(JSON.stringify(body), { status, headers: { 'Content-Type': 'application/json' } })

let nextId = 0

export function stubApi(options: { me?: () => Promise<Response> } = {}): FakeApi {
  const api: FakeApi = {
    deals: structuredClone(fixtureDeals),
    requests: [],
    failures: [],
    fail(method, path, status = 500, error = 'something went wrong') {
      api.failures.push({ method, path, status, error })
    },
  }
  const owner = { id: 'm-dev', name: devUser.name, email: devUser.email }

  const handle = async (method: string, path: string, body: unknown): Promise<Response> => {
    const failure = api.failures.find((f) => f.method === method && f.path.test(path))
    if (failure) return json({ error: failure.error }, failure.status)

    if (method === 'GET' && path === '/api/me') return options.me ? options.me() : json(devUser)
    if (method === 'GET' && path === '/api/deals') return json(api.deals)

    if (method === 'POST' && path === '/api/deals') {
      const lead = body as Record<string, string>
      const deal: Deal = {
        id: `deal-${nextId++}`,
        company: lead.company,
        dealType: lead.dealType as Deal['dealType'],
        eventType: lead.dealType === 'Job Post' ? 'Job post' : 'TBA',
        audience: 'Unclear',
        semester: lead.semester,
        status: 'lead',
        startDate: null,
        endDate: null,
        location: '',
        owner,
        support: [],
        contact: lead.contactName || lead.contactEmail ? { name: lead.contactName, email: lead.contactEmail, role: lead.contactRole } : null,
        nextStep: lead.nextStep,
        nextStepDue: null,
        feeSEK: null,
        invoiceStatus: null,
        invoiceDue: null,
        links: {},
        rating: null,
        notes: '',
        activities: [
          { id: `a-${nextId++}`, type: 'note', source: 'manual', title: 'Added as a lead', timestamp: new Date().toISOString(), body: '' },
        ],
      }
      api.deals.push(deal)
      return json(deal, 201)
    }

    const match = path.match(/^\/api\/deals\/([^/]+)(\/notes)?$/)
    const deal = match && api.deals.find((d) => d.id === decodeURIComponent(match[1]))
    if (match && !deal) return json({ error: 'deal not found' }, 404)
    if (deal && method === 'PATCH' && !match[2]) {
      Object.assign(deal, body)
      return json(deal)
    }
    if (deal && method === 'POST' && match[2]) {
      const note: Activity = {
        id: `a-${nextId++}`,
        type: 'note',
        source: 'manual',
        title: 'Quick note',
        timestamp: new Date().toISOString(),
        body: (body as { body: string }).body,
      }
      deal.activities.push(note)
      return json(note, 201)
    }
    return json({ error: 'not found' }, 404)
  }

  vi.stubGlobal(
    'fetch',
    vi.fn(async (input: string, init?: RequestInit) => {
      const method = init?.method ?? 'GET'
      const body = init?.body ? JSON.parse(init.body as string) : undefined
      api.requests.push({ method, path: input, body })
      return handle(method, input, body)
    }),
  )
  return api
}
