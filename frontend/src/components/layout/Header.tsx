import { Plus, Search } from 'lucide-react'
import { formatSEK } from '../../format'

type Props = {
  semesters: string[]
  semester: string
  onSemesterChange: (semester: string) => void
  query: string
  onQueryChange: (query: string) => void
  openDeals: number
  confirmedSEK: number
  unpaidSEK: number
  canAddLead: boolean
  onNewLead: () => void
}

export default function Header(props: Props) {
  const { semesters, semester, onSemesterChange, query, onQueryChange, openDeals, confirmedSEK, unpaidSEK } = props
  const { canAddLead, onNewLead } = props

  return (
    <header className="z-20 flex h-14 shrink-0 items-center justify-between border-b border-gray-200 bg-white px-6">
      <div className="flex items-center gap-4">
        <div className="flex items-center gap-2.5 border-r border-gray-200 pr-4">
          <div className="flex h-8 w-8 items-center justify-center rounded-lg border border-indigo-100 bg-indigo-50 text-sm font-bold text-indigo-600 shadow-sm">
            AI
          </div>
          <div>
            <h1 className="font-display text-sm leading-tight font-bold text-gray-900">Lucid</h1>
            <p className="text-[11px] font-medium text-gray-500">Corporate Outreach CRM</p>
          </div>
        </div>
        <label className="flex items-center gap-2 rounded-md border border-gray-200 bg-gray-50 py-0.5 pl-2.5 text-xs font-semibold text-gray-700">
          <span className="h-2 w-2 rounded-full bg-emerald-500" />
          <span className="sr-only">Semester</span>
          <select
            value={semester}
            onChange={(e) => onSemesterChange(e.target.value)}
            className="bg-transparent py-0.5 pr-1 font-semibold focus:outline-none"
          >
            {semesters.map((s) => (
              <option key={s}>{s}</option>
            ))}
          </select>
        </label>
        <label className="hidden w-72 items-center gap-2 rounded-lg border border-gray-200 bg-gray-50 px-3 py-1.5 text-gray-400 focus-within:border-indigo-300 md:flex">
          <Search size={14} aria-hidden />
          <input
            type="search"
            aria-label="Search"
            value={query}
            onChange={(e) => onQueryChange(e.target.value)}
            placeholder="Search companies, contacts, notes..."
            className="w-full border-none bg-transparent text-xs text-gray-800 placeholder-gray-400 focus:outline-none"
          />
        </label>
      </div>
      <div className="flex items-center gap-4">
        <div className="hidden items-center gap-2 xl:flex">
          <div className="rounded-md border border-gray-200 bg-gray-50 px-3 py-1 text-xs text-gray-600">
            Open Deals: <strong className="font-semibold text-gray-900">{openDeals}</strong>
          </div>
          <div className="rounded-md border border-emerald-100 bg-emerald-50 px-3 py-1 text-xs text-emerald-800">
            Confirmed: <strong className="font-bold">{formatSEK(confirmedSEK)}</strong>
          </div>
          <div className="rounded-md border border-orange-100 bg-orange-50 px-3 py-1 text-xs text-orange-800">
            Unpaid: <strong className="font-bold">{formatSEK(unpaidSEK)}</strong>
          </div>
        </div>
        <button
          type="button"
          onClick={onNewLead}
          disabled={!canAddLead}
          title={canAddLead ? undefined : 'Available once you are signed in'}
          className="flex items-center gap-1.5 rounded-lg bg-indigo-600 px-3.5 py-1.5 text-xs font-semibold text-white transition hover:bg-indigo-700 disabled:cursor-not-allowed disabled:opacity-50"
        >
          <Plus size={14} aria-hidden /> New Lead
        </button>
      </div>
    </header>
  )
}
