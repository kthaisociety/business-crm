import type { Deal } from '../../data/deals'
import { formatSEK } from '../../format'
import DealCard from './DealCard'

type Props = {
  label: string
  deals: Deal[]
  onSelect: (id: string) => void
}

export default function KanbanColumn({ label, deals, onSelect }: Props) {
  const total = deals.reduce((sum, deal) => sum + (deal.feeSEK ?? 0), 0)

  return (
    <section aria-label={label} className="flex w-72 shrink-0 flex-col rounded-xl bg-gray-100/70 p-2">
      <header className="flex items-center justify-between px-1.5 pt-1 pb-2.5">
        <div className="flex items-center gap-2">
          <h2 className="font-display text-xs font-bold text-gray-700">{label}</h2>
          <span className="rounded-full bg-white px-1.5 text-[11px] font-semibold text-gray-500">
            {deals.length}
          </span>
        </div>
        <span className="text-[11px] font-medium text-gray-500">{formatSEK(total)}</span>
      </header>
      <div className="flex flex-col gap-2 overflow-y-auto">
        {deals.map((deal) => (
          <DealCard key={deal.id} deal={deal} onSelect={onSelect} />
        ))}
        {deals.length === 0 && (
          <p className="rounded-lg border border-dashed border-gray-300 py-6 text-center text-[11px] text-gray-400">
            No deals
          </p>
        )}
      </div>
    </section>
  )
}
