import { Fragment, useState } from 'react'
import { ChevronDown, ChevronRight } from 'lucide-react'
import type { CompanyInput } from '../../api'
import { statusLabel, type Company, type Contact, type Deal } from '../../data/deals'
import { companiesFrom } from '../../data/views'
import { formatSEK } from '../../format'
import CompanyDialog from '../dialogs/CompanyDialog'
import ContactDialog from '../dialogs/ContactDialog'
import Fee from '../Fee'
import OwnerAvatar from '../OwnerAvatar'
import type { ContactActions } from './ContactsPage'
import { AddButton, EditButton, PageTitle, TouchedAgo, cell, headCell, table } from './common'

export type CompanyActions = {
  create: (input: CompanyInput) => Promise<void>
  update: (id: string, input: CompanyInput) => Promise<void>
  remove: (id: string) => Promise<void>
}

type Props = {
  companies: Company[]
  contacts: Contact[]
  deals: Deal[] // every semester's
  query: string
  onSelect: (id: string) => void
  actions: CompanyActions
  contactActions: ContactActions
}

type Editing = { company?: Company } | { contactAt: string } | null

export default function CompaniesPage({ companies, contacts, deals, query, onSelect, actions, contactActions }: Props) {
  const [expanded, setExpanded] = useState<string | null>(null)
  const [editing, setEditing] = useState<Editing>(null)
  const close = () => setEditing(null)
  const q = query.trim().toLowerCase()
  const rows = companiesFrom(companies, contacts, deals).filter((r) => !q || r.company.name.toLowerCase().includes(q))

  return (
    <div className="min-h-0 flex-1 overflow-auto px-6 pt-4 pb-6">
      <div className="flex items-start justify-between gap-3">
        <PageTitle title="Companies" subtitle="Every company the team works with, across all semesters." />
        <AddButton label="New company" onClick={() => setEditing({})} />
      </div>
      <table className={table}>
        <thead className="bg-gray-50 text-[11px] text-gray-500">
          <tr>
            {['Company', 'Deals', 'Open', 'Latest', 'Owners', 'Confirmed', 'Last touchpoint', ''].map((h, i) => (
              <th key={i} className={headCell}>
                {h}
              </th>
            ))}
          </tr>
        </thead>
        <tbody>
          {rows.map(({ company, contacts: people, deals: companyDeals, open, confirmedSEK, owners, lastTouchpoint }) => {
            const isOpen = expanded === company.id
            const latest = companyDeals[0]
            const Chevron = isOpen ? ChevronDown : ChevronRight
            return (
              <Fragment key={company.id}>
                <tr className="hover:bg-gray-50">
                  <td className={cell}>
                    <button
                      type="button"
                      aria-expanded={isOpen}
                      onClick={() => setExpanded(isOpen ? null : company.id)}
                      className="flex items-center gap-1 font-semibold text-gray-900 hover:text-indigo-700"
                    >
                      <Chevron size={14} className="text-gray-400" aria-hidden />
                      {company.name}
                    </button>
                  </td>
                  <td className={cell}>{companyDeals.length}</td>
                  <td className={cell}>{open || '—'}</td>
                  <td className={`${cell} whitespace-nowrap text-gray-600`}>
                    {latest ? `${latest.semester} · ${statusLabel(latest.status)}` : <span className="text-gray-400">No deals yet</span>}
                  </td>
                  <td className={cell}>
                    {owners.length === 0 ? (
                      '—'
                    ) : (
                      <div className="flex -space-x-0.5">
                        {owners.map((p) => (
                          <OwnerAvatar key={p.id} owner={p} />
                        ))}
                      </div>
                    )}
                  </td>
                  <td className={`${cell} whitespace-nowrap`}>{confirmedSEK ? formatSEK(confirmedSEK) : '—'}</td>
                  <td className={cell}>
                    <TouchedAgo at={lastTouchpoint} />
                  </td>
                  <td className={`${cell} text-right`}>
                    <EditButton label={`Edit ${company.name}`} onClick={() => setEditing({ company })} />
                  </td>
                </tr>
                {isOpen && (
                  <tr>
                    <td colSpan={8} className="border-b border-gray-100 bg-gray-50/60 px-3 py-3">
                      <div className="grid gap-4 pl-5 md:grid-cols-2">
                        <div>
                          <h3 className="text-[11px] font-semibold text-gray-500">Deals</h3>
                          {companyDeals.length === 0 && <p className="mt-1 text-xs text-gray-400">None yet</p>}
                          <ul aria-label={`Deals with ${company.name}`} className="mt-1 space-y-1">
                            {companyDeals.map((deal) => (
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
                        </div>
                        <div>
                          <div className="flex items-center justify-between">
                            <h3 className="text-[11px] font-semibold text-gray-500">Contacts</h3>
                            <button
                              type="button"
                              onClick={() => setEditing({ contactAt: company.id })}
                              className="text-[11px] font-semibold text-indigo-700 hover:underline"
                            >
                              Add contact
                            </button>
                          </div>
                          {people.length === 0 && <p className="mt-1 text-xs text-gray-400">None yet</p>}
                          <ul aria-label={`Contacts at ${company.name}`} className="mt-1 space-y-1 text-xs text-gray-700">
                            {people.map((p) => (
                              <li key={p.id}>
                                <span className="font-medium">{p.name || p.email}</span>
                                {p.role && <span className="text-gray-500"> · {p.role}</span>}
                                {p.name && p.email && <span className="text-gray-400"> · {p.email}</span>}
                              </li>
                            ))}
                          </ul>
                        </div>
                      </div>
                      {company.notes && <p className="mt-3 pl-5 text-xs whitespace-pre-line text-gray-600">{company.notes}</p>}
                    </td>
                  </tr>
                )}
              </Fragment>
            )
          })}
          {rows.length === 0 && (
            <tr>
              <td colSpan={8} className="px-3 py-8 text-center text-gray-400">
                {q ? 'No companies match the search' : 'No companies yet'}
              </td>
            </tr>
          )}
        </tbody>
      </table>

      {editing && 'contactAt' in editing && (
        <ContactDialog
          companies={companies}
          companyId={editing.contactAt}
          dealCount={0}
          onSave={async (fields) => {
            await contactActions.create(fields)
            close()
          }}
          onDelete={async () => {}}
          onClose={close}
        />
      )}
      {editing && !('contactAt' in editing) && (
        <CompanyDialog
          company={editing.company}
          dealCount={editing.company ? deals.filter((d) => d.companyId === editing.company!.id).length : 0}
          onSave={async (input) => {
            if (editing.company) await actions.update(editing.company.id, input)
            else await actions.create(input)
            close()
          }}
          onDelete={async () => {
            await actions.remove(editing.company!.id)
            close()
          }}
          onClose={close}
        />
      )}
    </div>
  )
}
