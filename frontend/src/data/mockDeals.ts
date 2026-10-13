// The deal model mirrors the business team's deal log (one row per deal, with status, next step, fee
// and invoice), so the log can move into the app. Seed data is made up (docs/pcr/0005): fictional
// companies and people, @example.com emails, invented fees. Dates are relative to today, so follow-up
// nudges look the same whenever the app runs.

export type Status = 'lead' | 'discussing' | 'agreement' | 'signed' | 'finished' | 'canceled'

// Board columns, in pipeline order. Canceled deals are shown only on request.
export const STATUSES: { id: Status; label: string }[] = [
  { id: 'lead', label: 'Lead' },
  { id: 'discussing', label: 'Discussing' },
  { id: 'agreement', label: 'Agreement' },
  { id: 'signed', label: 'Signed' },
  { id: 'finished', label: 'Finished' },
  { id: 'canceled', label: 'Canceled' },
]

export type DealType = 'Event' | 'Partnership' | 'Job Post'
export const DEAL_TYPES: DealType[] = ['Event', 'Partnership', 'Job Post']

export type EventType =
  | 'Workshop'
  | 'Hackathon'
  | 'After-Work'
  | 'Lunch Lecture'
  | 'Networking event'
  | 'Panel discussion'
  | 'Company visit'
  | 'Study-trip'
  | 'Job post'
  | 'Other'
  | 'TBA'

export type Audience = 'Internal' | 'External' | 'Unclear'

export type InvoiceStatus = 'not_sent' | 'waiting' | 'paid' | 'cancelled'

export const INVOICE_STATUSES: { id: InvoiceStatus; label: string }[] = [
  { id: 'not_sent', label: 'Not sent' },
  { id: 'waiting', label: 'Waiting for payment' },
  { id: 'paid', label: 'Paid' },
  { id: 'cancelled', label: 'Cancelled' },
]

export type Person = { name: string; initials: string; color: string }

export type Contact = { name: string; email: string; role: string }

export type Activity = {
  id: string
  type: 'email' | 'meeting' | 'note'
  // Where the entry came from. Email sync and Fathom meeting notes will add entries of their own.
  source: 'manual' | 'email_sync' | 'fathom'
  title: string
  timestamp: string
  body: string
  bulletPoints?: string[]
}

export type Deal = {
  id: string
  company: string
  dealType: DealType
  eventType: EventType
  audience: Audience
  semester: string // "HT 2026" (autumn) or "VT 2026" (spring)
  status: Status
  startDate: string | null
  endDate: string | null
  location: string
  owner: Person | null
  support: Person[]
  contact: Contact | null
  nextStep: string
  nextStepDue: string | null
  feeSEK: number | null // null: not agreed yet; 0: free event
  invoiceStatus: InvoiceStatus | null
  invoiceDue: string | null
  links: { drive?: string; contract?: string; writeUp?: string }
  rating: number | null // 1–5, after the deal is finished
  notes: string
  activities: Activity[]
}

const DAY = 24 * 60 * 60 * 1000

// A deal with no touchpoint for this many days gets a follow-up nudge.
export const FOLLOW_UP_AFTER_DAYS = 4

export const isOpen = (deal: Deal) => deal.status !== 'finished' && deal.status !== 'canceled'

export function lastTouchpoint(deal: Deal): string | null {
  return deal.activities.reduce<string | null>((latest, a) => (!latest || a.timestamp > latest ? a.timestamp : latest), null)
}

export function daysSince(iso: string, now = Date.now()): number {
  return Math.max(0, Math.floor((now - new Date(iso).getTime()) / DAY))
}

// Today's date as YYYY-MM-DD, in local time.
export function today(now = new Date()): string {
  return new Date(now.getTime() - now.getTimezoneOffset() * 60_000).toISOString().slice(0, 10)
}

// Whole calendar days from one YYYY-MM-DD date to another. Counted on dates, not elapsed hours, so a
// clock change (a 23- or 25-hour day) can't shift the count.
export function calendarDaysBetween(from: string, to: string): number {
  return Math.round((Date.parse(`${to}T00:00:00Z`) - Date.parse(`${from}T00:00:00Z`)) / DAY)
}

export type Nudge = { kind: 'overdue' | 'inactive'; days: number }

// Why an open deal needs attention, if it does: an overdue next step comes first, then inactivity.
export function nudgeFor(deal: Deal, now = Date.now()): Nudge | null {
  if (!isOpen(deal)) return null
  const todayDate = today(new Date(now))
  if (deal.nextStepDue && deal.nextStepDue < todayDate) {
    return { kind: 'overdue', days: calendarDaysBetween(deal.nextStepDue, todayDate) }
  }
  const last = lastTouchpoint(deal)
  const idle = last ? daysSince(last, now) : 0
  return idle >= FOLLOW_UP_AFTER_DAYS ? { kind: 'inactive', days: idle } : null
}

// Signed or finished: the company has committed to the fee.
export const isConfirmed = (deal: Deal) => deal.status === 'signed' || deal.status === 'finished'

// A confirmed fee no one has recorded as paid (or cancelled). An invoice status left unset counts too:
// nothing has been invoiced yet.
export const isUnpaid = (deal: Deal) =>
  isConfirmed(deal) && (deal.feeSEK ?? 0) > 0 && deal.invoiceStatus !== 'paid' && deal.invoiceStatus !== 'cancelled'

export function statusLabel(status: Status): string {
  return STATUSES.find((s) => s.id === status)?.label ?? status
}

export function invoiceLabel(deal: Deal): string | null {
  if (deal.feeSEK === 0) return 'Free event'
  return INVOICE_STATUSES.find((s) => s.id === deal.invoiceStatus)?.label ?? null
}

// Newest first: "HT 2026" (autumn) comes after "VT 2026" (spring).
export function compareSemesters(a: string, b: string): number {
  const key = (s: string) => {
    const [term, year] = s.split(' ')
    return Number(year) * 2 + (term === 'HT' ? 1 : 0)
  }
  return key(b) - key(a)
}

// Helpers for seed dates relative to today.
const daysFromNow = (n: number) => today(new Date(Date.now() + n * DAY))
const hoursAgo = (n: number) => new Date(Date.now() - n * 60 * 60 * 1000).toISOString()
const daysAgo = (n: number) => hoursAgo(n * 24)

const alva: Person = { name: 'Alva Lindqvist', initials: 'AL', color: 'bg-violet-100 text-violet-700' }
const oskar: Person = { name: 'Oskar Berg', initials: 'OB', color: 'bg-sky-100 text-sky-700' }
const maja: Person = { name: 'Maja Holm', initials: 'MH', color: 'bg-amber-100 text-amber-700' }
const noah: Person = { name: 'Noah Ek', initials: 'NE', color: 'bg-rose-100 text-rose-700' }
const iris: Person = { name: 'Iris Wallin', initials: 'IW', color: 'bg-teal-100 text-teal-700' }

const base = {
  startDate: null,
  endDate: null,
  location: '',
  support: [],
  contact: null,
  nextStep: '',
  nextStepDue: null,
  feeSEK: null,
  invoiceStatus: null,
  invoiceDue: null,
  links: {},
  rating: null,
  notes: '',
} satisfies Partial<Deal>

const note = (id: string, title: string, at: string, body = ''): Activity => ({
  id,
  type: 'note',
  source: 'manual',
  title,
  timestamp: at,
  body,
})

export const mockDeals: Deal[] = [
  {
    ...base,
    id: 'fjallbacka',
    company: 'Fjällbacka Ventures',
    dealType: 'Event',
    eventType: 'After-Work',
    audience: 'External',
    semester: 'HT 2026',
    status: 'discussing',
    owner: alva,
    support: [noah],
    contact: { name: 'Clara Nord', email: 'clara.nord@example.com', role: 'Community Lead' },
    nextStep: 'Send the one-pager: dinner or hackathon sponsorship',
    nextStepDue: daysFromNow(2),
    notes: 'Deciding between a founders dinner and sponsoring the autumn hackathon.',
    activities: [
      {
        id: 'fjallbacka-1',
        type: 'meeting',
        source: 'manual',
        title: 'Intro call',
        timestamp: daysAgo(6),
        body: 'Good fit; they want visibility with founders.',
        bulletPoints: ['Prefer an evening format', 'Budget decided per event', 'Want a one-pager to take internally'],
      },
      note('fjallbacka-2', 'Drafted one-pager', daysAgo(1), 'Draft shared with Noah for review.'),
    ],
  },
  {
    ...base,
    id: 'kvarnholmen',
    company: 'Kvarnholmen AI',
    dealType: 'Event',
    eventType: 'After-Work',
    audience: 'External',
    semester: 'HT 2026',
    status: 'agreement',
    startDate: '2026-11-05',
    location: 'KTH Innovation',
    owner: oskar,
    support: [iris],
    contact: { name: 'Hugo Sjöberg', email: 'hugo.sjoberg@example.com', role: 'Partner' },
    nextStep: 'Send contract for signing; get KTHAIS added as event co-host',
    nextStepDue: daysFromNow(-2),
    feeSEK: 20_000,
    notes: 'Agreed on format and fee on the last call.',
    activities: [
      {
        id: 'kvarnholmen-1',
        type: 'email',
        source: 'manual',
        title: 'Confirmed fee and date',
        timestamp: daysAgo(3),
        body: '20,000 SEK, after-work on 5 November at KTH Innovation.',
      },
    ],
  },
  {
    ...base,
    id: 'granit',
    company: 'Granit Consulting',
    dealType: 'Event',
    eventType: 'Lunch Lecture',
    audience: 'External',
    semester: 'HT 2026',
    status: 'signed',
    startDate: '2026-12-04',
    location: 'Granit office',
    owner: maja,
    support: [oskar],
    contact: { name: 'Lina Dahl', email: 'lina.dahl@example.com', role: 'Recruitment Manager' },
    nextStep: 'Send invoice (30-day terms) and plan logistics for 4 Dec',
    nextStepDue: daysFromNow(5),
    feeSEK: 40_000,
    invoiceStatus: 'not_sent',
    links: { contract: 'https://drive.example.com/granit-contract' },
    notes: 'Signed: 40,000 SEK, and Granit covers food and venue. 30-day payment terms.',
    activities: [note('granit-1', 'Contract signed', daysAgo(2), 'Both parties signed.')],
  },
  {
    ...base,
    id: 'stromma',
    company: 'Strömma Robotics',
    dealType: 'Event',
    eventType: 'Hackathon',
    audience: 'External',
    semester: 'HT 2026',
    status: 'agreement',
    startDate: '2026-11-09',
    endDate: '2026-11-13',
    owner: alva,
    contact: { name: 'Viktor Lund', email: 'viktor.lund@example.com', role: 'Head of Engineering' },
    nextStep: 'Agree the challenge brief',
    nextStepDue: daysFromNow(4),
    feeSEK: 40_000,
    invoiceStatus: 'not_sent',
    notes: 'Co-hosted robotics hackathon; another company may join as a second sponsor.',
    activities: [note('stromma-1', 'Shared draft brief', daysAgo(2))],
  },
  {
    ...base,
    id: 'lumen',
    company: 'Lumen Health',
    dealType: 'Event',
    eventType: 'Hackathon',
    audience: 'Unclear',
    semester: 'HT 2026',
    status: 'lead',
    owner: noah,
    support: [maja],
    contact: { name: 'Ella Berg', email: 'ella.berg@example.com', role: 'CTO' },
    nextStep: 'Ask a board member for an intro after the exam period',
    notes: 'Healthcare hackathon idea.',
    activities: [note('lumen-1', 'Added as a lead', daysAgo(6), 'Heard about them at a meetup.')],
  },
  {
    ...base,
    id: 'saltsjo',
    company: 'Saltsjö Insurance',
    dealType: 'Job Post',
    eventType: 'Job post',
    audience: 'External',
    semester: 'HT 2026',
    status: 'discussing',
    owner: iris,
    contact: { name: 'Per Ström', email: 'per.strom@example.com', role: 'Talent Partner' },
    nextStep: 'Offer a job post or thesis listing',
    nextStepDue: daysFromNow(1),
    notes: 'Thesis and part-time AI roles in insurance.',
    activities: [note('saltsjo-1', 'Replied to their questions', daysAgo(1))],
  },
  {
    ...base,
    id: 'stadsnat',
    company: 'Stadsnät Labs',
    dealType: 'Partnership',
    eventType: 'TBA',
    audience: 'Unclear',
    semester: 'HT 2026',
    status: 'lead',
    owner: oskar,
    contact: { name: 'Sara Kvist', email: 'sara.kvist@example.com', role: 'Innovation Manager' },
    nextStep: 'Await reply to follow-up email',
    notes: 'Met at a career fair. Want to reach students; open to an event or exposure.',
    activities: [
      {
        id: 'stadsnat-1',
        type: 'email',
        source: 'manual',
        title: 'Follow-up email',
        timestamp: daysAgo(9),
        body: 'Followed up on the career-fair conversation.',
      },
    ],
  },
  {
    ...base,
    id: 'ekdahl',
    company: 'Ekdahl Advokatbyrå',
    dealType: 'Partnership',
    eventType: 'Other',
    audience: 'Internal',
    semester: 'HT 2026',
    status: 'discussing',
    owner: maja,
    support: [noah],
    contact: { name: 'Jonas Falk', email: 'jonas.falk@example.com', role: 'Associate' },
    nextStep: 'Agree project scope',
    nextStepDue: daysFromNow(3),
    feeSEK: 0,
    notes: 'No fee: legal advice and contract templates in exchange for a legal-AI project.',
    activities: [note('ekdahl-1', 'Scope email received', daysAgo(2))],
  },
  {
    ...base,
    id: 'vinga',
    company: 'Vinga Voice',
    dealType: 'Job Post',
    eventType: 'Job post',
    audience: 'External',
    semester: 'HT 2026',
    status: 'lead',
    owner: iris,
    contact: { name: 'Adam Roos', email: 'adam.roos@example.com', role: 'Founder' },
    nextStep: 'Reply and offer a job post',
    nextStepDue: daysFromNow(0),
    notes: 'Internships and jobs in AI voice.',
    activities: [note('vinga-1', 'Inbound email', hoursAgo(20), 'Asked about posting internships.')],
  },
  {
    ...base,
    id: 'codewise',
    company: 'Codewise',
    dealType: 'Event',
    eventType: 'Workshop',
    audience: 'External',
    semester: 'HT 2026',
    status: 'finished',
    startDate: '2026-09-03',
    location: 'KTH Innovation',
    owner: alva,
    support: [iris],
    feeSEK: 0,
    rating: 4,
    notes: 'Free workshop; good turnout.',
    activities: [note('codewise-1', 'Event done', '2026-09-03T18:00:00Z')],
  },
  {
    ...base,
    id: 'brygga',
    company: 'Brygga Capital',
    dealType: 'Event',
    eventType: 'Lunch Lecture',
    audience: 'Unclear',
    semester: 'HT 2026',
    status: 'canceled',
    owner: noah,
    invoiceStatus: 'cancelled',
    notes: 'They pulled out after a reorganisation.',
    activities: [note('brygga-1', 'Canceled', '2026-09-20T10:00:00Z')],
  },
  {
    ...base,
    id: 'polar',
    company: 'Polar Systems',
    dealType: 'Event',
    eventType: 'Hackathon',
    audience: 'External',
    semester: 'VT 2026',
    status: 'finished',
    startDate: '2026-02-07',
    endDate: '2026-02-08',
    location: 'KTH Innovation',
    owner: oskar,
    support: [alva, maja],
    feeSEK: 30_000,
    invoiceStatus: 'waiting',
    invoiceDue: daysFromNow(-10),
    rating: 5,
    notes: 'Theme: agents.',
    activities: [note('polar-1', 'Invoice sent', '2026-03-01T09:00:00Z')],
  },
  {
    ...base,
    id: 'kista',
    company: 'Kista Quantum',
    dealType: 'Job Post',
    eventType: 'Job post',
    audience: 'External',
    semester: 'VT 2026',
    status: 'finished',
    startDate: '2026-02-25',
    owner: iris,
    feeSEK: 12_000,
    invoiceStatus: 'paid',
    activities: [note('kista-1', 'Payment received', '2026-03-20T09:00:00Z')],
  },
  {
    ...base,
    id: 'granit-vt',
    company: 'Granit Consulting',
    dealType: 'Event',
    eventType: 'Panel discussion',
    audience: 'External',
    semester: 'VT 2026',
    status: 'finished',
    startDate: '2026-01-26',
    location: 'Granit office',
    owner: maja,
    feeSEK: 15_000,
    invoiceStatus: 'paid',
    rating: 4,
    activities: [note('granit-vt-1', 'Panel held', '2026-01-26T19:00:00Z')],
  },
  {
    ...base,
    id: 'arkipelag',
    company: 'Arkipelag Analytics',
    dealType: 'Event',
    eventType: 'Hackathon',
    audience: 'External',
    semester: 'HT 2025',
    status: 'finished',
    startDate: '2025-09-22',
    endDate: '2025-10-02',
    location: 'KTH Innovation',
    owner: alva,
    feeSEK: 20_000,
    invoiceStatus: 'paid',
    rating: 5,
    activities: [note('arkipelag-1', 'Payment received', '2025-11-01T09:00:00Z')],
  },
]
