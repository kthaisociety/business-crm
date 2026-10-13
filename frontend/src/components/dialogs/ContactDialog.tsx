import { useState } from 'react'
import type { ContactFields } from '../../api'
import type { Company, Contact } from '../../data/deals'
import FormDialog, { DeleteButton, inputClass } from './FormDialog'

type Props = {
  contact?: Contact // editing this one; otherwise adding
  companies: Company[]
  companyId?: string // preselected when adding from a company
  dealCount: number
  onSave: (fields: ContactFields) => Promise<void>
  onDelete: () => Promise<void>
  onClose: () => void
}

export default function ContactDialog({ contact, companies, companyId, dealCount, onSave, onDelete, onClose }: Props) {
  const [company, setCompany] = useState(contact?.companyId ?? companyId ?? companies[0]?.id ?? '')
  const [name, setName] = useState(contact?.name ?? '')
  const [email, setEmail] = useState(contact?.email ?? '')
  const [role, setRole] = useState(contact?.role ?? '')

  return (
    <FormDialog
      title={contact ? `Edit ${contact.name || contact.email}` : 'New contact'}
      submitLabel={contact ? 'Save' : 'Add contact'}
      onSubmit={() => onSave({ companyId: company, name: name.trim(), email: email.trim(), role: role.trim() })}
      onClose={onClose}
      extra={
        contact &&
        (dealCount === 0 ? (
          <DeleteButton what="contact" onDelete={onDelete} />
        ) : (
          <span className="text-[11px] text-gray-400">On deals, so it can't be deleted</span>
        ))
      }
    >
      <label className="block">
        Company
        <select
          required
          value={company}
          onChange={(e) => setCompany(e.target.value)}
          disabled={!!contact && dealCount > 0}
          title={contact && dealCount > 0 ? 'A contact on deals stays with their company' : undefined}
          className={inputClass}
        >
          {companies.map((c) => (
            <option key={c.id} value={c.id}>
              {c.name}
            </option>
          ))}
        </select>
      </label>
      <label className="block">
        Name
        <input data-autofocus value={name} onChange={(e) => setName(e.target.value)} className={inputClass} />
      </label>
      <label className="block">
        Email
        <input type="email" value={email} onChange={(e) => setEmail(e.target.value)} className={inputClass} />
      </label>
      <label className="block">
        Role
        <input value={role} onChange={(e) => setRole(e.target.value)} placeholder="e.g. Talent Acquisition Lead" className={inputClass} />
      </label>
      {!name.trim() && !email.trim() && <p className="text-[11px] text-gray-400">Give a name, an email, or both.</p>}
    </FormDialog>
  )
}
