import { useState } from 'react'
import type { CompanyInput } from '../../api'
import type { Company } from '../../data/deals'
import FormDialog, { DeleteButton, inputClass } from './FormDialog'

type Props = {
  company?: Company // editing this one; otherwise adding
  dealCount: number
  onSave: (input: CompanyInput) => Promise<void>
  onDelete: () => Promise<void>
  onClose: () => void
}

export default function CompanyDialog({ company, dealCount, onSave, onDelete, onClose }: Props) {
  const [name, setName] = useState(company?.name ?? '')
  const [website, setWebsite] = useState(company?.website ?? '')
  const [notes, setNotes] = useState(company?.notes ?? '')

  return (
    <FormDialog
      title={company ? `Edit ${company.name}` : 'New company'}
      submitLabel={company ? 'Save' : 'Add company'}
      onSubmit={() => onSave({ name: name.trim(), website: website.trim(), notes })}
      onClose={onClose}
      extra={
        company &&
        (dealCount === 0 ? (
          <DeleteButton what="company" onDelete={onDelete} />
        ) : (
          <span className="text-[11px] text-gray-400">Has deals, so it can't be deleted</span>
        ))
      }
    >
      <label className="block">
        Name
        <input required data-autofocus value={name} onChange={(e) => setName(e.target.value)} className={inputClass} />
      </label>
      <label className="block">
        Website
        <input type="url" value={website} onChange={(e) => setWebsite(e.target.value)} placeholder="https://" className={inputClass} />
      </label>
      <label className="block">
        Notes
        <textarea value={notes} onChange={(e) => setNotes(e.target.value)} rows={3} className={`${inputClass} resize-none`} />
      </label>
    </FormDialog>
  )
}
