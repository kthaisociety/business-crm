// An in-memory stand-in for the Go API, behind a stubbed fetch (docs/pcr/0002: tests never call real
// services). It starts from the fixtures and answers the same routes the app calls, with the server's
// rules: unique company names and contact emails, a deal's contact at its company, and no deleting a
// company or contact that deals point at.
import { vi } from 'vitest'
import type { Activity, Company, Contact, Deal } from '../data/deals'
import { fixtureCompanies, fixtureContacts, fixtureDeals } from './fixtures'

export const devUser = { sub: 'dev', email: 'dev@kthais.com', name: 'Dev User' }

type Failure = { method: string; path: RegExp; status: number; error: string; once: boolean }

export type FakeApi = {
  deals: Deal[]
  companies: Company[]
  contacts: Contact[]
  requests: { method: string; path: string; body: unknown }[]
  // Make matching requests fail until cleared with `failures.length = 0`, or only the next one.
  failures: Failure[]
  fail: (method: string, path: RegExp, status?: number, error?: string) => void
  failOnce: (method: string, path: RegExp, status?: number, error?: string) => void
  // Hold matching requests unanswered until release() is called, to test what happens meanwhile.
  hold: (method: string, path: RegExp) => { release: () => Promise<void> }
}

const json = (body: unknown, status = 200) =>
  new Response(JSON.stringify(body), { status, headers: { 'Content-Type': 'application/json' } })
const error = (status: number, message: string) => json({ error: message }, status)

let nextId = 0
const id = (prefix: string) => `${prefix}-new-${nextId++}`

type Body = Record<string, unknown>

export function stubApi(options: { me?: () => Promise<Response> } = {}): FakeApi {
  const api: FakeApi = {
    deals: structuredClone(fixtureDeals),
    companies: structuredClone(fixtureCompanies),
    contacts: structuredClone(fixtureContacts),
    requests: [],
    failures: [],
    fail(method, path, status = 500, message = 'something went wrong') {
      api.failures.push({ method, path, status, error: message, once: false })
    },
    failOnce(method, path, status = 500, message = 'something went wrong') {
      api.failures.push({ method, path, status, error: message, once: true })
    },
    hold(method, path) {
      let release: () => void = () => {}
      const gate = new Promise<void>((resolve) => (release = resolve))
      holds.push({ method, path, gate })
      return {
        release: async () => {
          release()
          await gate
        },
      }
    },
  }
  const holds: { method: string; path: RegExp; gate: Promise<void> }[] = []
  const owner = { id: 'm-dev', name: devUser.name, email: devUser.email }
  const sameText = (a: string, b: string) => a.trim().toLowerCase() === b.trim().toLowerCase()
  const emailTaken = (email: string, except?: string) =>
    email !== '' && api.contacts.some((c) => c.id !== except && sameText(c.email, email))

  // Deals carry their company's name and their contact, as the server's JSON does.
  const refreshDeals = () => {
    for (const d of api.deals) {
      d.company = api.companies.find((c) => c.id === d.companyId)?.name ?? d.company
      if (d.contact) d.contact = api.contacts.find((c) => c.id === d.contact!.id) ?? null
    }
  }

  const handle = async (method: string, path: string, body: Body): Promise<Response> => {
    const held = holds.find((h) => h.method === method && h.path.test(path))
    if (held) await held.gate
    const failure = api.failures.find((f) => f.method === method && f.path.test(path))
    if (failure?.once) api.failures.splice(api.failures.indexOf(failure), 1)
    if (failure) return error(failure.status, failure.error)

    if (method === 'GET' && path === '/api/me') return options.me ? options.me() : json(devUser)
    if (method === 'GET' && path === '/api/deals') return json(api.deals)
    if (method === 'GET' && path === '/api/companies') return json(api.companies)
    if (method === 'GET' && path === '/api/contacts') return json(api.contacts)

    if (method === 'POST' && path === '/api/deals') {
      let company = api.companies.find((c) => c.id === body.companyId || (body.company && sameText(c.name, body.company as string)))
      if (!company) {
        company = { id: id('c'), name: (body.company as string).trim(), website: '', notes: '' }
        api.companies.push(company)
      }
      let contact: Contact | null = null
      if (body.contactId) {
        contact = api.contacts.find((c) => c.id === body.contactId) ?? null
        if (contact?.companyId !== company.id) return error(400, `that contact isn't at ${company.name}`)
      } else if (body.contact) {
        const input = body.contact as Omit<Contact, 'id' | 'companyId'>
        contact = api.contacts.find((c) => input.email && sameText(c.email, input.email)) ?? null
        if (contact && contact.companyId !== company.id) return error(409, `${input.email} is already a contact elsewhere`)
        if (!contact) {
          contact = { id: id('p'), companyId: company.id, ...input }
          api.contacts.push(contact)
        }
      }
      const deal: Deal = {
        id: id('deal'),
        companyId: company.id,
        company: company.name,
        dealType: body.dealType as Deal['dealType'],
        eventType: body.dealType === 'Job Post' ? 'Job post' : 'TBA',
        audience: 'Unclear',
        semester: body.semester as string,
        status: 'lead',
        startDate: null,
        endDate: null,
        location: '',
        owner,
        support: [],
        contact,
        nextStep: (body.nextStep as string) ?? '',
        nextStepDue: null,
        feeSEK: null,
        invoiceStatus: null,
        invoiceDue: null,
        links: {},
        rating: null,
        notes: '',
        createdAt: new Date().toISOString(),
        activities: [
          { id: id('a'), type: 'note', source: 'manual', title: 'Added as a lead', timestamp: new Date().toISOString(), body: '' },
        ],
      }
      api.deals.push(deal)
      return json(deal, 201)
    }

    if (method === 'POST' && path === '/api/companies') {
      if (api.companies.some((c) => sameText(c.name, body.name as string))) {
        return error(409, `there is already a company called ${body.name}`)
      }
      const company = { id: id('c'), website: '', notes: '', ...body } as Company
      api.companies.push(company)
      return json(company, 201)
    }
    if (method === 'POST' && path === '/api/contacts') {
      if (emailTaken(body.email as string)) return error(409, `${body.email} is already a contact`)
      const contact = { id: id('p'), ...body } as Contact
      api.contacts.push(contact)
      return json(contact, 201)
    }

    const company = path.match(/^\/api\/companies\/([^/]+)$/)
    if (company) {
      const found = api.companies.find((c) => c.id === decodeURIComponent(company[1]))
      if (!found) return error(404, 'company not found')
      if (method === 'PATCH') {
        if (body.name && api.companies.some((c) => c !== found && sameText(c.name, body.name as string))) {
          return error(409, `there is already a company called ${body.name}`)
        }
        Object.assign(found, body)
        refreshDeals()
        return json(found)
      }
      if (method === 'DELETE') {
        const count = api.deals.filter((d) => d.companyId === found.id).length
        if (count) return error(409, `the company has ${count} deals; it can only be deleted without any`)
        api.companies = api.companies.filter((c) => c !== found)
        api.contacts = api.contacts.filter((c) => c.companyId !== found.id)
        return new Response(null, { status: 204 })
      }
    }

    const contact = path.match(/^\/api\/contacts\/([^/]+)$/)
    if (contact) {
      const found = api.contacts.find((c) => c.id === decodeURIComponent(contact[1]))
      if (!found) return error(404, 'contact not found')
      const count = api.deals.filter((d) => d.contact?.id === found.id).length
      if (method === 'PATCH') {
        if (body.email !== undefined && emailTaken(body.email as string, found.id)) return error(409, `${body.email} is already a contact`)
        if (body.companyId && body.companyId !== found.companyId && count) return error(409, 'the contact is on deals with their current company')
        Object.assign(found, body)
        refreshDeals()
        return json(found)
      }
      if (method === 'DELETE') {
        if (count) return error(409, `the contact is on ${count} deals; change their contact first`)
        api.contacts = api.contacts.filter((c) => c !== found)
        return new Response(null, { status: 204 })
      }
    }

    const match = path.match(/^\/api\/deals\/([^/]+)(\/notes)?$/)
    const deal = match && api.deals.find((d) => d.id === decodeURIComponent(match[1]))
    if (match && !deal) return error(404, 'deal not found')
    if (deal && method === 'PATCH' && !match[2]) {
      const { contactId, ...fields } = body
      if (contactId !== undefined) {
        const next = contactId === null ? null : api.contacts.find((c) => c.id === contactId)
        if (next === undefined || (next && next.companyId !== deal.companyId)) return error(400, 'that contact isn’t at the company')
        deal.contact = next
      }
      Object.assign(deal, fields)
      return json(deal)
    }
    if (deal && method === 'POST' && match[2]) {
      const note: Activity = {
        id: id('a'),
        type: 'note',
        source: 'manual',
        title: 'Quick note',
        timestamp: new Date().toISOString(),
        body: body.body as string,
      }
      deal.activities.push(note)
      return json(note, 201)
    }
    return error(404, 'not found')
  }

  vi.stubGlobal(
    'fetch',
    vi.fn(async (input: string, init?: RequestInit) => {
      const method = init?.method ?? 'GET'
      const body = init?.body ? JSON.parse(init.body as string) : undefined
      api.requests.push({ method, path: input, body })
      return handle(method, input, body ?? {})
    }),
  )
  return api
}
