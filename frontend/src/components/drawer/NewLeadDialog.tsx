import { useRef, useState, type FormEvent } from 'react'
import { X } from 'lucide-react'
import type { NewLeadInput } from '../../api'
import { DEAL_TYPES, type DealType } from '../../data/deals'
import { useModal } from '../../hooks'

type Props = {
  semester: string
  // Resolves once the lead is saved; the form stays open with the error if it rejects.
  onCreate: (lead: Omit<NewLeadInput, 'semester'>) => Promise<void>
  onClose: () => void
}

const input =
  'mt-1 w-full rounded-lg border border-gray-200 px-2.5 py-1.5 text-xs text-gray-800 focus:border-indigo-300 focus:outline-none'

export default function NewLeadDialog({ semester, onCreate, onClose }: Props) {
  const [company, setCompany] = useState('')
  const [dealType, setDealType] = useState<DealType>('Event')
  const [contactName, setContactName] = useState('')
  const [contactEmail, setContactEmail] = useState('')
  const [contactRole, setContactRole] = useState('')
  const [nextStep, setNextStep] = useState('')
  const [saving, setSaving] = useState(false)
  const [error, setError] = useState<string | null>(null)

  // The dialog stays open while saving: closing it then would let a late result close a newer form.
  const close = () => {
    if (!saving) onClose()
  }
  const dialog = useRef<HTMLFormElement>(null)
  useModal(dialog, close)

  const submit = async (e: FormEvent) => {
    e.preventDefault()
    if (!company.trim() || saving) return
    setSaving(true)
    setError(null)
    try {
      await onCreate({
        company: company.trim(),
        dealType,
        contactName: contactName.trim(),
        contactEmail: contactEmail.trim(),
        contactRole: contactRole.trim(),
        nextStep: nextStep.trim(),
      })
    } catch (err) {
      setError(`Couldn't add the lead: ${err instanceof Error ? err.message : String(err)}`)
      setSaving(false)
    }
  }

  return (
    <div className="fixed inset-0 z-40 flex items-start justify-center p-4 pt-24">
      <button type="button" aria-label="Close new lead" tabIndex={-1} onClick={close} className="absolute inset-0 bg-gray-900/20" />
      <form
        ref={dialog}
        tabIndex={-1}
        role="dialog"
        aria-modal="true"
        aria-label="New lead"
        onSubmit={submit}
        className="relative w-full max-w-sm rounded-xl focus:outline-none border border-gray-200 bg-white p-5 shadow-xl"
      >
        <div className="flex items-center justify-between">
          <h2 className="font-display text-sm font-bold text-gray-900">New lead · {semester}</h2>
          <button type="button" onClick={close} aria-label="Close" className="rounded-md p-1 text-gray-400 hover:bg-gray-100">
            <X size={16} aria-hidden />
          </button>
        </div>
        <div className="mt-4 space-y-3 text-xs font-medium text-gray-600">
          <label className="block">
            Company
            <input required data-autofocus value={company} onChange={(e) => setCompany(e.target.value)} className={input} />
          </label>
          <label className="block">
            Deal type
            <select value={dealType} onChange={(e) => setDealType(e.target.value as DealType)} className={input}>
              {DEAL_TYPES.map((t) => (
                <option key={t}>{t}</option>
              ))}
            </select>
          </label>
          <div className="grid grid-cols-2 gap-3">
            <label className="block">
              Contact name
              <input value={contactName} onChange={(e) => setContactName(e.target.value)} className={input} />
            </label>
            <label className="block">
              Contact email
              <input type="email" value={contactEmail} onChange={(e) => setContactEmail(e.target.value)} className={input} />
            </label>
          </div>
          <label className="block">
            Contact role
            <input
              value={contactRole}
              onChange={(e) => setContactRole(e.target.value)}
              className={input}
              placeholder="e.g. Talent Acquisition Lead"
            />
          </label>
          <label className="block">
            Next step
            <input value={nextStep} onChange={(e) => setNextStep(e.target.value)} className={input} placeholder="e.g. Send intro email" />
          </label>
        </div>
        {error && (
          <p role="alert" className="mt-4 text-xs text-red-600">
            {error}
          </p>
        )}
        <button type="submit" disabled={saving} className="mt-5 w-full rounded-lg bg-indigo-600 py-2 text-xs font-semibold text-white hover:bg-indigo-700 disabled:opacity-50">
          {saving ? 'Adding…' : 'Add lead'}
        </button>
      </form>
    </div>
  )
}
