import { Fragment, useState } from 'react'
import { ChevronDown, ChevronRight } from 'lucide-react'
import { statusLabel, type Deal } from '../../data/deals'
import { companiesFrom } from '../../data/views'
import { formatSEK } from '../../format'
import Fee from '../Fee'
import OwnerAvatar from '../OwnerAvatar'
import { PageTitle, TouchedAgo, cell, headCell, table } from './common'

type Props = {
  deals: Deal[] // every semester's
  query: string
  onSelect: (id: string) => void
}

export default function CompaniesPage({ deals, query, onSelect }: Props) {
  const [expanded, setExpanded] = useState<string | null>(null)
  const q = query.trim().toLowerCase()
  const companies = companiesFrom(deals).filter((c) => !q || c.name.toLowerCase().includes(q))

  return (
    <div className="min-h-0 flex-1 overflow-auto px-6 pt-4 pb-6">
      <PageTitle title="Companies" subtitle="Every company the team has had a deal with, across all semesters." />
      <table className={table}>
        <thead className="bg-gray-50 text-[11px] text-gray-500">
          <tr>
            {['Company', 'Deals', 'Open', 'Latest', 'Owners', 'Confirmed', 'Last touchpoint'].map((h) => (
              <th key={h} className={headCell}>
                {h}
              </th>
            ))}
          </tr>
        </thead>
        <tbody>
          {companies.map((company) => {
            const open = expanded === company.name
            const latest = company.deals[0]
            const Chevron = open ? ChevronDown : ChevronRight
            return (
              <Fragment key={company.name}>
                <tr className="hover:bg-gray-50">
                  <td className={cell}>
                    <button
                      type="button"
                      aria-expanded={open}
                      onClick={() => setExpanded(open ? null : company.name)}
                      className="flex items-center gap-1 font-semibold text-gray-900 hover:text-indigo-700"
                    >
                      <Chevron size={14} className="text-gray-400" aria-hidden />
                      {company.name}
                    </button>
                  </td>
                  <td className={cell}>{company.deals.length}</td>
                  <td className={cell}>{company.open || '—'}</td>
                  <td className={`${cell} whitespace-nowrap text-gray-600`}>
                    {latest.semester} · {statusLabel(latest.status)}
                  </td>
                  <td className={cell}>
                    <div className="flex -space-x-1.5">
                      {company.owners.map((p) => (
                        <OwnerAvatar key={p.id} owner={p} />
                      ))}
                    </div>
                  </td>
                  <td className={`${cell} whitespace-nowrap`}>
                    {company.confirmedSEK ? formatSEK(company.confirmedSEK) : '—'}
                  </td>
                  <td className={cell}>
                    <TouchedAgo at={company.lastTouchpoint} />
                  </td>
                </tr>
                {open && (
                  <tr>
                    <td colSpan={7} className="border-b border-gray-100 bg-gray-50/60 px-3 py-2">
                      <ul aria-label={`Deals with ${company.name}`} className="space-y-1 pl-5">
                        {company.deals.map((deal) => (
                          <li key={deal.id} className="flex items-center gap-3">
                            <button
                              type="button"
                              onClick={() => onSelect(deal.id)}
                              className="text-xs font-medium text-indigo-700 hover:underline"
                            >
                              {deal.semester} · {deal.eventType} · {statusLabel(deal.status)}
                            </button>
                            <Fee deal={deal} />
                          </li>
                        ))}
                      </ul>
                    </td>
                  </tr>
                )}
              </Fragment>
            )
          })}
          {companies.length === 0 && (
            <tr>
              <td colSpan={7} className="px-3 py-8 text-center text-gray-400">
                {q ? 'No companies match the search' : 'No companies yet: add a lead to start'}
              </td>
            </tr>
          )}
        </tbody>
      </table>
    </div>
  )
}
