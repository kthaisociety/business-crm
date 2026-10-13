import type { Deal } from '../data/deals'
import { formatSEK } from '../format'

export default function Fee({ deal }: { deal: Deal }) {
  if (deal.feeSEK === null) return <span className="text-xs text-gray-400">Fee TBD</span>
  if (deal.feeSEK === 0) return <span className="text-xs text-gray-500">No fee</span>
  return <span className="text-xs font-semibold text-gray-900">{formatSEK(deal.feeSEK)}</span>
}
