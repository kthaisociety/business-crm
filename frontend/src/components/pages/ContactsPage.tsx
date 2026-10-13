import { Mail } from 'lucide-react'
import type { Deal } from '../../data/deals'
import { contactsFrom } from '../../data/views'
import { PageTitle, TouchedAgo, cell, headCell, table } from './common'

type Props = {
  deals: Deal[] // every semester's
  query: string
  onSelect: (id: string) => void
}

export default function ContactsPage({ deals, query, onSelect }: Props) {
  const q = query.trim().toLowerCase()
  const contacts = contactsFrom(deals).filter(
    (c) => !q || [c.name, c.email, c.company, c.role].some((text) => text.toLowerCase().includes(q)),
  )

  return (
    <div className="min-h-0 flex-1 overflow-auto px-6 pt-4 pb-6">
      <PageTitle title="Contacts" subtitle="The people at each company, from every deal. Open one to see their latest deal." />
      <table className={table}>
        <thead className="bg-gray-50 text-[11px] text-gray-500">
          <tr>
            {['Name', 'Company', 'Role', 'Email', 'Deals', 'Last touchpoint'].map((h) => (
              <th key={h} className={headCell}>
                {h}
              </th>
            ))}
          </tr>
        </thead>
        <tbody>
          {contacts.map((contact) => (
            <tr key={`${contact.email}|${contact.name}|${contact.company}`} className="hover:bg-gray-50">
              <td className={cell}>
                <button
                  type="button"
                  onClick={() => onSelect(contact.deals[0].id)}
                  className="font-semibold text-gray-900 hover:text-indigo-700"
                >
                  {contact.name}
                </button>
              </td>
              <td className={cell}>{contact.company}</td>
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
              <td className={cell}>{contact.deals.length}</td>
              <td className={cell}>
                <TouchedAgo at={contact.lastTouchpoint} />
              </td>
            </tr>
          ))}
          {contacts.length === 0 && (
            <tr>
              <td colSpan={6} className="px-3 py-8 text-center text-gray-400">
                {q ? 'No contacts match the search' : 'No contacts yet: add one to a deal'}
              </td>
            </tr>
          )}
        </tbody>
      </table>
    </div>
  )
}
