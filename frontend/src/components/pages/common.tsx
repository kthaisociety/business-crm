import { daysSince } from '../../data/deals'

// Shared table look for the Companies and Contacts pages, matching the pipeline's list view.
export const table =
  'w-full min-w-[760px] border-separate border-spacing-0 overflow-hidden rounded-xl border border-gray-200 bg-white text-left text-xs'
export const headCell = 'border-b border-gray-200 px-3 py-2 font-semibold'
export const cell = 'border-b border-gray-100 px-3 py-2.5 text-gray-700'

export function PageTitle({ title, subtitle }: { title: string; subtitle: string }) {
  return (
    <div className="mb-3">
      <h2 className="font-display text-base font-bold text-gray-900">{title}</h2>
      <p className="text-xs text-gray-500">{subtitle}</p>
    </div>
  )
}

export function TouchedAgo({ at }: { at: string | null }) {
  if (!at) return <span className="text-gray-400">—</span>
  const days = daysSince(at)
  return <span>{days === 0 ? 'Today' : `${days} ${days === 1 ? 'day' : 'days'} ago`}</span>
}
