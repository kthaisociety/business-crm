import { useState } from 'react'
import { Mail } from 'lucide-react'
import type { ContactFields } from '../../api'
import type { Company, Contact, Deal } from '../../data/deals'
import { contactsFrom } from '../../data/views'
import ContactDialog from '../dialogs/ContactDialog'
import { AddButton, EditButton, PageTitle, TouchedAgo, cell, headCell, table } from './common'

export type ContactActions = {
  create: (fields: ContactFields) => Promise<void>
  update: (id: string, fields: ContactFields) => Promise<void>
  remove: (id: string) => Promise<void>
}

type Props = {
  companies: Company[]
  contacts: Contact[]
  deals: Deal[] // every semester's
  query: string
  onSelect: (id: string) => void
  actions: ContactActions
}

export default function ContactsPage({ companies, contacts, deals, query, onSelect, actions }: Props) {
  const [editing, setEditing] = useState<{ contact?: Contact; dealCount: number } | null>(null)
  const close = () => setEditing(null)
  const q = query.trim().toLowerCase()
  const rows = contactsFrom(contacts, companies, deals).filter(
    (r) => !q || [r.contact.name, r.contact.email, r.contact.role, r.company].some((text) => text.toLowerCase().includes(q)),
  )

  return (
    <div className="min-h-0 flex-1 overflow-auto px-6 pt-4 pb-6">
      <div className="flex items-start justify-between gap-3">
        <PageTitle title="Contacts" subtitle="The people at each company. Open one to see their latest deal." />
        <AddButton
          label="New contact"
          onClick={() => setEditing({ dealCount: 0 })}
          disabled={companies.length === 0}
          title={companies.length === 0 ? 'Add a company first' : undefined}
        />
      </div>
      <table className={table}>
        <thead className="bg-gray-50 text-[11px] text-gray-500">
          <tr>
            {['Name', 'Company', 'Role', 'Email', 'Deals', 'Last touchpoint', ''].map((h, i) => (
              <th key={i} className={headCell}>
                {h}
              </th>
            ))}
          </tr>
        </thead>
        <tbody>
          {rows.map(({ contact, company, deals: theirDeals, lastTouchpoint }) => {
            const label = contact.name || contact.email
            const edit = () => setEditing({ contact, dealCount: theirDeals.length })
            return (
              <tr key={contact.id} className="hover:bg-gray-50">
                <td className={cell}>
                  <button
                    type="button"
                    onClick={() => (theirDeals.length ? onSelect(theirDeals[0].id) : edit())}
                    className="font-semibold text-gray-900 hover:text-indigo-700"
                  >
                    {label}
                  </button>
                </td>
                <td className={cell}>{company}</td>
                <td className={`${cell} text-gray-500`}>{contact.role || '—'}</td>
                <td className={cell}>
                  {contact.email ? (
                    <a href={`mailto:${contact.email}`} className="inline-flex items-center gap-1 text-indigo-700 hover:underline">
                      <Mail size={12} aria-hidden /> {contact.email}
                    </a>
                  ) : (
                    <span className="text-gray-400">—</span>
                  )}
                </td>
                <td className={cell}>{theirDeals.length || '—'}</td>
                <td className={cell}>
                  <TouchedAgo at={lastTouchpoint} />
                </td>
                <td className={`${cell} text-right`}>
                  <EditButton label={`Edit ${label}`} onClick={edit} />
                </td>
              </tr>
            )
          })}
          {rows.length === 0 && (
            <tr>
              <td colSpan={7} className="px-3 py-8 text-center text-gray-400">
                {q ? 'No contacts match the search' : 'No contacts yet'}
              </td>
            </tr>
          )}
        </tbody>
      </table>

      {editing && (
        <ContactDialog
          contact={editing.contact}
          companies={companies}
          dealCount={editing.dealCount}
          onSave={async (fields) => {
            if (editing.contact) await actions.update(editing.contact.id, fields)
            else await actions.create(fields)
            close()
          }}
          onDelete={async () => {
            await actions.remove(editing.contact!.id)
            close()
          }}
          onClose={close}
        />
      )}
    </div>
  )
}
