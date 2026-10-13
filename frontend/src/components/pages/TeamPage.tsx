import { useState } from 'react'
import type { MemberInput } from '../../api'
import { ROLES, isOpen, type Deal, type Person, type Role } from '../../data/deals'
import { messageOf } from '../../format'
import FormDialog, { DeleteButton, inputClass } from '../dialogs/FormDialog'
import OwnerAvatar from '../OwnerAvatar'
import { AddButton, PageTitle, cell, headCell, table } from './common'

export type TeamActions = {
  add: (member: MemberInput) => Promise<void>
  setRole: (id: string, role: Role) => Promise<void>
  remove: (id: string) => Promise<void>
}

type Props = {
  members: Person[]
  deals: Deal[] // every semester's
  meId: string | undefined
  isAdmin: boolean
  actions: TeamActions
}

const roleLabel = (role: Role | undefined) => ROLES.find((r) => r.id === role)?.label ?? 'Member'

export default function TeamPage({ members, deals, meId, isAdmin, actions }: Props) {
  const [adding, setAdding] = useState(false)
  const [error, setError] = useState<string | null>(null)

  const changeRole = (id: string, role: Role) => {
    setError(null)
    actions.setRole(id, role).catch((err: unknown) => setError(messageOf(err)))
  }

  return (
    <div className="min-h-0 flex-1 overflow-auto px-6 pt-4 pb-6">
      <div className="flex items-start justify-between gap-3">
        <PageTitle
          title="Team"
          subtitle={
            isAdmin
              ? 'The business team. Add people before they sign in, so leads can be assigned to them.'
              : 'The business team. Admins add people and change roles.'
          }
        />
        {isAdmin && <AddButton label="Add member" onClick={() => setAdding(true)} />}
      </div>
      {error && (
        <p role="alert" className="mb-3 rounded-lg border border-red-200 bg-red-50 px-3 py-2 text-xs text-red-700">
          {error}
        </p>
      )}
      <table className={table}>
        <thead className="bg-gray-50 text-[11px] text-gray-500">
          <tr>
            {['Name', 'Email', 'Role', 'Open deals', 'All deals', ''].map((h, i) => (
              <th key={i} className={headCell}>
                {h}
              </th>
            ))}
          </tr>
        </thead>
        <tbody>
          {members.map((member) => {
            const owned = deals.filter((d) => d.owner?.id === member.id)
            const supports = deals.some((d) => d.support.some((p) => p.id === member.id))
            return (
              <tr key={member.id} className="hover:bg-gray-50">
                <td className={cell}>
                  <span className="flex items-center gap-2 font-semibold text-gray-900">
                    <OwnerAvatar owner={member} />
                    {member.name}
                    {member.id === meId && <span className="font-normal text-gray-400">(you)</span>}
                  </span>
                </td>
                <td className={`${cell} text-gray-500`}>{member.email}</td>
                <td className={cell}>
                  {isAdmin ? (
                    <select
                      aria-label={`Role of ${member.name}`}
                      value={member.role ?? 'member'}
                      onChange={(e) => changeRole(member.id, e.target.value as Role)}
                      className="rounded-md border border-gray-200 bg-white px-2 py-1 text-xs"
                    >
                      {ROLES.map((r) => (
                        <option key={r.id} value={r.id}>
                          {r.label}
                        </option>
                      ))}
                    </select>
                  ) : (
                    roleLabel(member.role)
                  )}
                </td>
                <td className={cell}>{owned.filter(isOpen).length || '—'}</td>
                <td className={cell}>{owned.length || '—'}</td>
                <td className={`${cell} text-right`}>
                  {isAdmin && member.id !== meId && owned.length === 0 && !supports && (
                    <DeleteButton what={member.name} onDelete={() => actions.remove(member.id)} />
                  )}
                </td>
              </tr>
            )
          })}
        </tbody>
      </table>
      <p className="mt-2 text-[11px] text-gray-400">
        Someone who owns or supports deals stays on the team until their deals are reassigned.
      </p>

      {adding && (
        <AddMemberDialog
          onAdd={async (member) => {
            await actions.add(member)
            setAdding(false)
          }}
          onClose={() => setAdding(false)}
        />
      )}
    </div>
  )
}

function AddMemberDialog({ onAdd, onClose }: { onAdd: (member: MemberInput) => Promise<void>; onClose: () => void }) {
  const [name, setName] = useState('')
  const [email, setEmail] = useState('')
  const [role, setRole] = useState<Role>('member')
  return (
    <FormDialog
      title="Add member"
      submitLabel="Add member"
      onSubmit={() => onAdd({ name: name.trim(), email: email.trim(), role })}
      onClose={onClose}
    >
      <label className="block">
        Name
        <input required data-autofocus value={name} onChange={(e) => setName(e.target.value)} className={inputClass} />
      </label>
      <label className="block">
        Email
        <input required type="email" value={email} onChange={(e) => setEmail(e.target.value)} className={inputClass} />
      </label>
      <label className="block">
        Role
        <select value={role} onChange={(e) => setRole(e.target.value as Role)} className={inputClass}>
          {ROLES.map((r) => (
            <option key={r.id} value={r.id}>
              {r.label}
            </option>
          ))}
        </select>
      </label>
    </FormDialog>
  )
}
