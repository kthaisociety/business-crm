import type { Status } from '../data/deals'

// Each status's color: the dot on its board column, and its bar in Reports.
export const statusColors: Record<Status, string> = {
  lead: 'bg-slate-400',
  discussing: 'bg-sky-500',
  agreement: 'bg-amber-500',
  signed: 'bg-emerald-500',
  finished: 'bg-indigo-500',
  canceled: 'bg-rose-400',
}
