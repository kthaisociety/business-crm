import { ArrowRight } from 'lucide-react'
import { nudgeFor, type Deal } from '../../data/mockDeals'
import { formatDate, formatDateRange } from '../../format'
import { DealTypeBadge, InvoiceBadge, NudgeBadge } from '../badges'
import OwnerAvatar from '../OwnerAvatar'
import Fee from '../Fee'

type Props = {
  deal: Deal
  onSelect: (id: string) => void
}

export default function DealCard({ deal, onSelect }: Props) {
  const nudge = nudgeFor(deal)
  const when = formatDateRange(deal.startDate, deal.endDate)

  return (
    <button
      type="button"
      onClick={() => onSelect(deal.id)}
      className="w-full rounded-lg border border-gray-200 bg-white p-3 text-left shadow-sm transition hover:border-indigo-200 hover:shadow focus-visible:outline-2 focus-visible:outline-indigo-500"
    >
      <div className="flex items-start justify-between gap-2">
        <h3 className="text-sm font-semibold text-gray-900">{deal.company}</h3>
        <DealTypeBadge dealType={deal.dealType} />
      </div>
      <p className="mt-0.5 text-[11px] text-gray-500">
        {deal.eventType}
        {when && ` · ${when}`}
      </p>

      {nudge && (
        <div className="mt-2">
          <NudgeBadge nudge={nudge} />
        </div>
      )}

      {deal.nextStep && (
        <p className="mt-2 flex gap-1.5 text-xs text-gray-700">
          <ArrowRight size={12} className="mt-0.5 shrink-0 text-gray-400" aria-hidden />
          <span className="line-clamp-2">
            {deal.nextStep}
            {deal.nextStepDue && <span className="text-gray-400"> · {formatDate(deal.nextStepDue)}</span>}
          </span>
        </p>
      )}

      <div className="mt-3 flex items-center justify-between gap-2">
        <div className="flex items-center gap-1.5">
          <Fee deal={deal} />
          {(deal.status === 'signed' || deal.status === 'finished') && <InvoiceBadge deal={deal} />}
        </div>
        <div className="flex -space-x-1.5">
          {[deal.owner, ...deal.support].filter((p) => p !== null).map((person) => (
            <OwnerAvatar key={person.name} owner={person} />
          ))}
        </div>
      </div>
    </button>
  )
}
