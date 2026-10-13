import { useEffect, useState, type FormEvent, type ReactNode } from 'react'
import { ExternalLink, Mail, MessageSquare, StickyNote, Star, Users, X } from 'lucide-react'
import {
  INVOICE_STATUSES,
  STATUSES,
  daysSince,
  lastTouchpoint,
  nudgeFor,
  statusLabel,
  type Activity,
  type Deal,
  type InvoiceStatus,
  type Status,
} from '../../data/mockDeals'
import { formatDate, formatDateRange } from '../../format'
import { DealTypeBadge, NudgeBadge } from '../badges'
import OwnerAvatar from '../OwnerAvatar'

type Props = {
  deal: Deal
  otherDeals: Deal[] // earlier or parallel deals with the same company
  onClose: () => void
  onChange: (patch: Partial<Deal>) => void
  onAddNote: (text: string) => void
  onSelect: (id: string) => void
}

const activityIcons = { email: Mail, meeting: Users, note: StickyNote } satisfies Record<Activity['type'], unknown>
const sourceLabels: Record<Activity['source'], string | null> = {
  manual: null,
  email_sync: 'via email sync',
  fathom: 'via Fathom',
}

const field =
  'w-full rounded-md border border-gray-200 bg-white px-2 py-1 text-xs text-gray-800 focus:border-indigo-300 focus:outline-none'

function Row({ label, children }: { label: string; children: ReactNode }) {
  return (
    <>
      <dt className="pt-1 text-gray-500">{label}</dt>
      <dd className="min-w-0 pt-1 text-gray-900">{children}</dd>
    </>
  )
}

export default function DealDetailDrawer({ deal, otherDeals, onClose, onChange, onAddNote, onSelect }: Props) {
  const [note, setNote] = useState('')

  useEffect(() => {
    const onKey = (e: KeyboardEvent) => {
      if (e.key === 'Escape') onClose()
    }
    window.addEventListener('keydown', onKey)
    return () => window.removeEventListener('keydown', onKey)
  }, [onClose])

  const saveNote = (e: FormEvent) => {
    e.preventDefault()
    const text = note.trim()
    if (!text) return
    onAddNote(text)
    setNote('')
  }

  const nudge = nudgeFor(deal)
  const last = lastTouchpoint(deal)
  const lastDays = last ? daysSince(last) : null
  const timeline = [...deal.activities].sort((a, b) => b.timestamp.localeCompare(a.timestamp))
  const mailto = deal.contact
    ? `mailto:${deal.contact.email}?subject=${encodeURIComponent(`KTHAIS × ${deal.company}`)}`
    : undefined
  const links = [
    ['Drive', deal.links.drive],
    ['Contract', deal.links.contract],
    ['Write-up', deal.links.writeUp],
  ].filter((l): l is [string, string] => Boolean(l[1]))

  return (
    <div className="fixed inset-0 z-40 flex justify-end">
      <button type="button" aria-label="Close details" tabIndex={-1} onClick={onClose} className="absolute inset-0 bg-gray-900/20" />
      <aside
        role="dialog"
        aria-modal="true"
        aria-label={deal.company}
        className="relative flex h-full w-[440px] max-w-full flex-col border-l border-gray-200 bg-white shadow-xl"
      >
        <header className="flex items-start justify-between gap-3 border-b border-gray-200 px-5 py-4">
          <div>
            <div className="flex items-center gap-2">
              <h2 className="font-display text-base font-bold text-gray-900">{deal.company}</h2>
              <DealTypeBadge dealType={deal.dealType} />
            </div>
            <p className="mt-0.5 text-xs text-gray-500">
              {deal.eventType} · {deal.audience} · {deal.semester}
            </p>
          </div>
          <button type="button" onClick={onClose} aria-label="Close" className="rounded-md p-1 text-gray-400 hover:bg-gray-100 hover:text-gray-700">
            <X size={18} aria-hidden />
          </button>
        </header>

        <div className="flex-1 overflow-y-auto pb-6">
          <div className="flex items-center gap-2 px-5 pt-4">
            {mailto ? (
              <a
                href={mailto}
                className="flex items-center gap-1.5 rounded-lg border border-gray-200 px-3 py-1.5 text-xs font-semibold text-gray-700 hover:bg-gray-50"
              >
                <Mail size={14} aria-hidden /> Send Email
              </a>
            ) : (
              <span className="rounded-lg border border-dashed border-gray-200 px-3 py-1.5 text-xs text-gray-400">No contact yet</span>
            )}
            <label className="flex items-center gap-1.5 text-xs text-gray-500">
              Status
              <select
                value={deal.status}
                onChange={(e) => onChange({ status: e.target.value as Status })}
                className="rounded-lg border border-gray-200 bg-white px-2 py-1.5 text-xs font-semibold text-gray-800"
              >
                {STATUSES.map((s) => (
                  <option key={s.id} value={s.id}>
                    {s.label}
                  </option>
                ))}
              </select>
            </label>
          </div>

          <section aria-label="Next step" className="mx-5 mt-4 rounded-lg border border-indigo-100 bg-indigo-50/40 p-3">
            <div className="flex items-center justify-between gap-2">
              <h3 className="text-xs font-semibold text-gray-700">Next step</h3>
              {nudge && <NudgeBadge nudge={nudge} />}
            </div>
            <label className="sr-only" htmlFor="next-step">
              Next step
            </label>
            <input
              id="next-step"
              value={deal.nextStep}
              onChange={(e) => onChange({ nextStep: e.target.value })}
              placeholder="What happens next?"
              className={`${field} mt-2`}
            />
            <label className="mt-2 flex items-center gap-2 text-xs text-gray-500">
              Due
              <input
                type="date"
                value={deal.nextStepDue ?? ''}
                onChange={(e) => onChange({ nextStepDue: e.target.value || null })}
                className={`${field} w-auto`}
              />
            </label>
          </section>

          <dl className="mx-5 mt-4 grid grid-cols-[auto_1fr] items-center gap-x-6 gap-y-1.5 text-xs">
            <Row label="Owner">
              {deal.owner ? (
                <span className="flex items-center gap-2">
                  <OwnerAvatar owner={deal.owner} /> {deal.owner.name}
                </span>
              ) : (
                <span className="text-gray-400">Unassigned</span>
              )}
            </Row>
            {deal.support.length > 0 && <Row label="Support">{deal.support.map((p) => p.name).join(', ')}</Row>}
            <Row label="Contact">
              {deal.contact ? (
                <>
                  {deal.contact.name}
                  {deal.contact.role && <span className="text-gray-500"> · {deal.contact.role}</span>}
                  <br />
                  <span className="text-gray-500">{deal.contact.email}</span>
                </>
              ) : (
                <span className="text-gray-400">—</span>
              )}
            </Row>
            <Row label="Date">{formatDateRange(deal.startDate, deal.endDate) || <span className="text-gray-400">TBA</span>}</Row>
            {deal.location && <Row label="Location">{deal.location}</Row>}
            <Row label="Last touchpoint">
              {lastDays === null ? '—' : lastDays === 0 ? 'Today' : `${lastDays} ${lastDays === 1 ? 'day' : 'days'} ago`}
            </Row>
            <Row label="Fee (SEK)">
              <input
                type="number"
                aria-label="Fee (SEK)"
                min={0}
                step={1000}
                value={deal.feeSEK ?? ''}
                placeholder="Not agreed"
                onChange={(e) => onChange({ feeSEK: e.target.value === '' ? null : Number(e.target.value) })}
                className={`${field} w-32`}
              />
            </Row>
            {deal.feeSEK !== 0 && (
              <Row label="Invoice">
                <select
                  aria-label="Invoice"
                  value={deal.invoiceStatus ?? ''}
                  onChange={(e) => onChange({ invoiceStatus: (e.target.value || null) as InvoiceStatus | null })}
                  className={`${field} w-auto`}
                >
                  <option value="">—</option>
                  {INVOICE_STATUSES.map((s) => (
                    <option key={s.id} value={s.id}>
                      {s.label}
                    </option>
                  ))}
                </select>
                {deal.invoiceDue && <span className="ml-2 text-gray-500">due {formatDate(deal.invoiceDue)}</span>}
              </Row>
            )}
            {deal.rating !== null && (
              <Row label="Rating">
                <span className="flex items-center gap-0.5 text-amber-500" aria-label={`${deal.rating} out of 5`}>
                  {Array.from({ length: 5 }, (_, i) => (
                    <Star key={i} size={12} fill={i < deal.rating! ? 'currentColor' : 'none'} aria-hidden />
                  ))}
                </span>
              </Row>
            )}
            {links.length > 0 && (
              <Row label="Links">
                <span className="flex flex-wrap gap-2">
                  {links.map(([label, href]) => (
                    <a key={label} href={href} target="_blank" rel="noreferrer" className="flex items-center gap-1 text-indigo-600 hover:underline">
                      {label} <ExternalLink size={11} aria-hidden />
                    </a>
                  ))}
                </span>
              </Row>
            )}
          </dl>

          {deal.notes && <p className="mx-5 mt-4 rounded-lg bg-gray-50 p-3 text-xs text-gray-700">{deal.notes}</p>}

          {otherDeals.length > 0 && (
            <section aria-label="Other deals" className="mx-5 mt-4">
              <h3 className="text-xs font-semibold text-gray-700">Other deals with {deal.company}</h3>
              <ul className="mt-1.5 space-y-1">
                {otherDeals.map((other) => (
                  <li key={other.id}>
                    <button type="button" onClick={() => onSelect(other.id)} className="text-xs text-indigo-600 hover:underline">
                      {other.semester} · {other.eventType} · {statusLabel(other.status)}
                    </button>
                  </li>
                ))}
              </ul>
            </section>
          )}

          <form onSubmit={saveNote} className="mx-5 mt-5">
            <label htmlFor="quick-note" className="flex items-center gap-1.5 text-xs font-semibold text-gray-700">
              <MessageSquare size={14} aria-hidden /> Quick note
            </label>
            <textarea
              id="quick-note"
              value={note}
              onChange={(e) => setNote(e.target.value)}
              rows={3}
              placeholder="What happened? Call, reply, next step…"
              className="mt-1.5 w-full resize-none rounded-lg border border-gray-200 p-2.5 text-xs text-gray-800 placeholder-gray-400 focus:border-indigo-300 focus:outline-none"
            />
            <div className="mt-1.5 flex justify-end">
              <button
                type="submit"
                disabled={!note.trim()}
                className="rounded-lg bg-indigo-600 px-3 py-1.5 text-xs font-semibold text-white hover:bg-indigo-700 disabled:opacity-40"
              >
                Save Note
              </button>
            </div>
          </form>

          <section aria-label="Timeline" className="mx-5 mt-4">
            <h3 className="text-xs font-semibold text-gray-700">Timeline</h3>
            <ol className="mt-3 space-y-4 border-l border-gray-200 pl-4">
              {timeline.map((activity) => {
                const Icon = activityIcons[activity.type]
                const source = sourceLabels[activity.source]
                return (
                  <li key={activity.id} className="relative">
                    <span className="absolute top-0 -left-[25px] flex h-[17px] w-[17px] items-center justify-center rounded-full border border-gray-200 bg-white text-gray-500">
                      <Icon size={10} aria-hidden />
                    </span>
                    <div className="flex items-baseline justify-between gap-2">
                      <p className="text-xs font-semibold text-gray-900">
                        {activity.title}
                        {source && <span className="ml-1.5 font-normal text-gray-400">{source}</span>}
                      </p>
                      <time dateTime={activity.timestamp} className="shrink-0 text-[11px] text-gray-400">
                        {formatDate(activity.timestamp)}
                      </time>
                    </div>
                    {activity.body && <p className="mt-0.5 text-xs whitespace-pre-line text-gray-600">{activity.body}</p>}
                    {activity.bulletPoints && (
                      <ul className="mt-1 list-disc space-y-0.5 pl-4 text-xs text-gray-600">
                        {activity.bulletPoints.map((point) => (
                          <li key={point}>{point}</li>
                        ))}
                      </ul>
                    )}
                  </li>
                )
              })}
            </ol>
          </section>
        </div>
      </aside>
    </div>
  )
}
