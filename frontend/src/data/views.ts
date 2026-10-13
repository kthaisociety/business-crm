// Companies, contacts and semester reports, worked out from the deals. Until companies and contacts get
// tables of their own (internal/database/migrations), a company is every deal with the same name and a
// contact is every deal with the same contact email.
import {
  STATUSES,
  compareSemesters,
  isConfirmed,
  isOpen,
  isUnpaid,
  lastTouchpoint,
  nudgeFor,
  type Deal,
  type DealType,
  type Person,
  type Status,
} from './deals'

const latestOf = (dates: (string | null)[]) =>
  dates.reduce<string | null>((latest, d) => (d && (!latest || d > latest) ? d : latest), null)

const sumFees = (deals: Deal[]) => deals.reduce((sum, d) => sum + (d.feeSEK ?? 0), 0)

// Newest semester first; within one, the order deals were added.
const newestFirst = (a: Deal, b: Deal) => compareSemesters(a.semester, b.semester)

export type Company = {
  name: string
  deals: Deal[] // newest semester first
  open: number
  confirmedSEK: number
  owners: Person[]
  lastTouchpoint: string | null
}

export function companiesFrom(deals: Deal[]): Company[] {
  const groups = new Map<string, Deal[]>()
  for (const deal of deals) {
    const key = deal.company.trim().toLowerCase()
    groups.set(key, [...(groups.get(key) ?? []), deal])
  }
  return [...groups.values()]
    .map((group) => {
      const sorted = [...group].sort(newestFirst)
      const owners = new Map<string, Person>()
      for (const d of sorted) if (d.owner) owners.set(d.owner.id, d.owner)
      return {
        name: sorted[0].company,
        deals: sorted,
        open: sorted.filter(isOpen).length,
        confirmedSEK: sumFees(sorted.filter(isConfirmed)),
        owners: [...owners.values()],
        lastTouchpoint: latestOf(sorted.map(lastTouchpoint)),
      }
    })
    .sort((a, b) => a.name.localeCompare(b.name))
}

export type ContactRow = {
  name: string
  email: string
  role: string
  company: string
  deals: Deal[] // newest semester first
  lastTouchpoint: string | null
}

export function contactsFrom(deals: Deal[]): ContactRow[] {
  const groups = new Map<string, Deal[]>()
  for (const deal of deals) {
    if (!deal.contact) continue
    // Without an email, the same name at the same company is the same person.
    const key = deal.contact.email.trim().toLowerCase() || `${deal.contact.name}|${deal.company}`.toLowerCase()
    groups.set(key, [...(groups.get(key) ?? []), deal])
  }
  return [...groups.values()]
    .map((group) => {
      const sorted = [...group].sort(newestFirst)
      const latest = sorted[0]
      return {
        name: latest.contact!.name || latest.contact!.email,
        email: latest.contact!.email,
        role: latest.contact!.role,
        company: latest.company,
        deals: sorted,
        lastTouchpoint: latestOf(sorted.map(lastTouchpoint)),
      }
    })
    .sort((a, b) => a.name.localeCompare(b.name))
}

export type Bar = { label: string; value: number }

export type SemesterReport = {
  semester: string
  deals: number
  open: number
  won: number // signed or finished
  canceled: number
  // Won out of decided (won + canceled), or null before anything is decided.
  winRate: number | null
  confirmedSEK: number
  paidSEK: number
  unpaidSEK: number
  pipelineSEK: number // fees agreed on deals still open
  byStatus: Bar[]
  confirmedByType: Bar[]
  byOwner: Bar[]
  needsFollowUp: Deal[]
}

export function reportFor(semester: string, all: Deal[], now = Date.now()): SemesterReport {
  const deals = all.filter((d) => d.semester === semester)
  const won = deals.filter(isConfirmed)
  const canceled = deals.filter((d) => d.status === 'canceled')
  const decided = won.length + canceled.length

  const count = (pick: (d: Deal) => string) => {
    const counts = new Map<string, number>()
    for (const d of deals) counts.set(pick(d), (counts.get(pick(d)) ?? 0) + 1)
    return counts
  }
  const statusCounts = count((d) => d.status)
  const ownerCounts = count((d) => d.owner?.name ?? 'Unassigned')
  const types: DealType[] = ['Event', 'Partnership', 'Job Post']

  return {
    semester,
    deals: deals.length,
    open: deals.filter(isOpen).length,
    won: won.length,
    canceled: canceled.length,
    winRate: decided ? won.length / decided : null,
    confirmedSEK: sumFees(won),
    paidSEK: sumFees(won.filter((d) => d.invoiceStatus === 'paid')),
    unpaidSEK: sumFees(won.filter(isUnpaid)),
    pipelineSEK: sumFees(deals.filter(isOpen).filter((d) => !isConfirmed(d))),
    byStatus: STATUSES.map((s) => ({ label: s.label, value: statusCounts.get(s.id as Status) ?? 0 })),
    confirmedByType: types.map((t) => ({ label: t, value: sumFees(won.filter((d) => d.dealType === t)) })),
    byOwner: [...ownerCounts].map(([label, value]) => ({ label, value })).sort((a, b) => b.value - a.value || a.label.localeCompare(b.label)),
    needsFollowUp: deals.filter((d) => nudgeFor(d, now) !== null),
  }
}
