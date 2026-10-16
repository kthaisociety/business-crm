import { useState } from 'react'
import type { NewLeadInput } from '../../api'
import { DEAL_TYPES, type Company, type Contact, type DealType, type Person } from '../../data/deals'
import FormDialog, { inputClass } from '../dialogs/FormDialog'

type Props = {
  semester: string
  companies: Company[]
  contacts: Contact[]
  members: Person[]
  meId: string | undefined
  onCreate: (lead: Omit<NewLeadInput, 'semester'>) => Promise<void>
  onClose: () => void
}

const NO_CONTACT = ''
const NEW_CONTACT = 'new'

// Adds a lead. Typing a company that exists picks it, along with its contacts; any other name adds a new
// company with the lead.
export default function NewLeadDialog({ semester, companies, contacts, members, meId, onCreate, onClose }: Props) {
  const [companyName, setCompanyName] = useState('')
  const [ownerId, setOwnerId] = useState(meId ?? '') // '': no one yet
  const [dealType, setDealType] = useState<DealType>('Event')
  const [choice, setChoice] = useState<string | null>(null) // null: the default for the company
  const [contactName, setContactName] = useState('')
  const [contactEmail, setContactEmail] = useState('')
  const [contactRole, setContactRole] = useState('')
  const [nextStep, setNextStep] = useState('')

  const typed = companyName.trim()
  const company = companies.find((c) => c.name.toLowerCase() === typed.toLowerCase())
  const companyContacts = company ? contacts.filter((c) => c.companyId === company.id) : []
  // A contact picked at one company doesn't carry over when the company changes.
  const valid = choice === NO_CONTACT || choice === NEW_CONTACT || companyContacts.some((c) => c.id === choice)
  const contactChoice = choice !== null && valid ? choice : companyContacts.length ? NO_CONTACT : NEW_CONTACT

  const submit = async () => {
    const owner = { ownerId: ownerId || null, dealType, nextStep: nextStep.trim() }
    const lead: Omit<NewLeadInput, 'semester'> = company ? { ...owner, companyId: company.id } : { ...owner, company: typed }
    if (contactChoice === NEW_CONTACT && (contactName.trim() || contactEmail.trim())) {
      lead.contact = { name: contactName.trim(), email: contactEmail.trim(), role: contactRole.trim() }
    } else if (contactChoice !== NEW_CONTACT && contactChoice !== NO_CONTACT) {
      lead.contactId = contactChoice
    }
    await onCreate(lead)
  }

  return (
    <FormDialog title={`New lead · ${semester}`} submitLabel="Add lead" onSubmit={submit} onClose={onClose}>
      <div>
        <label className="block">
          Company
          <input
            required
            data-autofocus
            list="company-names"
            aria-describedby="company-hint"
            value={companyName}
            onChange={(e) => setCompanyName(e.target.value)}
            className={inputClass}
          />
        </label>
        <datalist id="company-names">
          {companies.map((c) => (
            <option key={c.id} value={c.name} />
          ))}
        </datalist>
        <p id="company-hint" className="mt-1 text-[11px] font-normal text-gray-400">
          {!typed
            ? 'Pick one from the list, or type a new name'
            : company
              ? `Existing company, ${companyContacts.length} ${companyContacts.length === 1 ? 'contact' : 'contacts'}`
              : `${typed} is new and will be added`}
        </p>
      </div>
      <label className="block">
        Deal type
        <select value={dealType} onChange={(e) => setDealType(e.target.value as DealType)} className={inputClass}>
          {DEAL_TYPES.map((t) => (
            <option key={t}>{t}</option>
          ))}
        </select>
      </label>
      <label className="block">
        Contact
        <select value={contactChoice} onChange={(e) => setChoice(e.target.value)} className={inputClass}>
          <option value={NO_CONTACT}>No contact yet</option>
          {companyContacts.map((c) => (
            <option key={c.id} value={c.id}>
              {c.name || c.email}
              {c.role ? ` · ${c.role}` : ''}
            </option>
          ))}
          <option value={NEW_CONTACT}>New contact…</option>
        </select>
      </label>
      {contactChoice === NEW_CONTACT && (
        <div className="space-y-3 rounded-lg border border-gray-100 bg-gray-50/60 p-3">
          <div className="grid grid-cols-2 gap-3">
            <label className="block">
              Contact name
              <input value={contactName} onChange={(e) => setContactName(e.target.value)} className={inputClass} />
            </label>
            <label className="block">
              Contact email
              <input type="email" value={contactEmail} onChange={(e) => setContactEmail(e.target.value)} className={inputClass} />
            </label>
          </div>
          <label className="block">
            Contact role
            <input
              value={contactRole}
              onChange={(e) => setContactRole(e.target.value)}
              className={inputClass}
              placeholder="e.g. Talent Acquisition Lead"
            />
          </label>
        </div>
      )}
      <label className="block">
        Owner
        <select value={ownerId} onChange={(e) => setOwnerId(e.target.value)} className={inputClass}>
          <option value="">Unassigned: anyone can claim it</option>
          {members.map((m) => (
            <option key={m.id} value={m.id}>
              {m.id === meId ? `Me (${m.name})` : m.name}
            </option>
          ))}
        </select>
      </label>
      <label className="block">
        Next step
        <input value={nextStep} onChange={(e) => setNextStep(e.target.value)} className={inputClass} placeholder="e.g. Send intro email" />
      </label>
    </FormDialog>
  )
}
