import { AlertTriangle, Clock } from 'lucide-react'
import type { Deal, DealType, Nudge } from '../data/mockDeals'
import { invoiceLabel } from '../data/mockDeals'

const dealTypeStyles: Record<DealType, string> = {
  Event: 'bg-indigo-50 text-indigo-700 border-indigo-200',
  Partnership: 'bg-emerald-50 text-emerald-700 border-emerald-200',
  'Job Post': 'bg-amber-50 text-amber-700 border-amber-200',
}

export function DealTypeBadge({ dealType }: { dealType: DealType }) {
  return (
    <span className={`shrink-0 rounded border px-1.5 py-0.5 text-[10px] font-semibold ${dealTypeStyles[dealType]}`}>
      {dealType}
    </span>
  )
}

export function NudgeBadge({ nudge }: { nudge: Nudge }) {
  const overdue = nudge.kind === 'overdue'
  const Icon = overdue ? Clock : AlertTriangle
  return (
    <span
      className={`inline-flex items-center gap-1 rounded-md border px-2 py-0.5 text-[11px] font-semibold ${
        overdue ? 'border-red-200 bg-red-50 text-red-700' : 'border-amber-200 bg-amber-50 text-amber-800'
      }`}
    >
      <Icon size={12} aria-hidden />
      {overdue
        ? `Next step overdue: ${nudge.days} ${nudge.days === 1 ? 'day' : 'days'}`
        : `Follow-up due: ${nudge.days} days inactive`}
    </span>
  )
}

export function InvoiceBadge({ deal }: { deal: Deal }) {
  const label = invoiceLabel(deal)
  if (!label) return null
  const style =
    deal.feeSEK === 0
      ? 'text-gray-500 bg-gray-50 border-gray-200'
      : {
          not_sent: 'text-orange-700 bg-orange-50 border-orange-200',
          waiting: 'text-sky-700 bg-sky-50 border-sky-200',
          paid: 'text-emerald-700 bg-emerald-50 border-emerald-200',
          cancelled: 'text-gray-500 bg-gray-50 border-gray-200',
        }[deal.invoiceStatus ?? 'cancelled']
  return <span className={`rounded border px-1.5 py-0.5 text-[10px] font-medium ${style}`}>{label}</span>
}
