import { BarChart3, Building2, KanbanSquare, Mail, Users } from 'lucide-react'
import type { User } from '../../api'

type Props = {
  user: User | null
  userError: string | null
}

// Only the pipeline exists so far; the other tabs show what's coming.
const tabs = [
  { label: 'Pipeline', icon: KanbanSquare, ready: true },
  { label: 'Companies', icon: Building2, ready: false },
  { label: 'Contacts', icon: Users, ready: false },
  { label: 'Reports', icon: BarChart3, ready: false },
]

export default function Sidebar({ user, userError }: Props) {
  return (
    <aside className="hidden w-56 shrink-0 flex-col border-r border-gray-200 bg-white p-3 md:flex">
      <nav aria-label="Main" className="flex flex-col gap-0.5">
        {tabs.map(({ label, icon: Icon, ready }) => (
          <button
            key={label}
            type="button"
            disabled={!ready}
            aria-current={ready ? 'page' : undefined}
            className={`flex items-center gap-2.5 rounded-md px-2.5 py-1.5 text-left text-sm ${
              ready ? 'bg-gray-100 font-semibold text-gray-900' : 'cursor-not-allowed text-gray-400'
            }`}
          >
            <Icon size={16} aria-hidden />
            <span className="flex-1">{label}</span>
            {!ready && <span className="text-[10px] font-medium">Soon</span>}
          </button>
        ))}
      </nav>

      <div className="mt-6 rounded-lg border border-indigo-100 bg-indigo-50/60 p-3">
        <div className="flex items-center gap-1.5 text-xs font-semibold text-indigo-700">
          <Mail size={14} aria-hidden /> Email and meeting sync are coming
        </div>
        <p className="mt-1.5 text-[11px] leading-relaxed text-indigo-900/70">
          Soon, emails with a contact and Fathom meeting notes will land on the deal's timeline on their own.
          Until then, add a quick note after each touchpoint: it resets the follow-up timer.
        </p>
      </div>

      <div className="mt-auto border-t border-gray-100 pt-3 text-[11px] text-gray-500">
        {user && <p>Signed in as {user.email}</p>}
        {userError && <p className="text-red-600">{userError}</p>}
        {!user && !userError && <p>Loading…</p>}
      </div>
    </aside>
  )
}
