import { BarChart3, Building2, KanbanSquare, Mail, UserCog, Users } from 'lucide-react'
import { NavLink } from 'react-router'
import type { User } from '../../api'
import { canSeeReports } from '../../data/deals'

type Props = {
  user: User | null
  userError: string | null
}

const pages = [
  { label: 'Pipeline', icon: KanbanSquare, path: '/' },
  { label: 'Companies', icon: Building2, path: '/companies' },
  { label: 'Contacts', icon: Users, path: '/contacts' },
  { label: 'Reports', icon: BarChart3, path: '/reports', boardOnly: true },
  { label: 'Team', icon: UserCog, path: '/team' },
]

// The pages this user's role opens.
const pagesFor = (user: User | null) => pages.filter((p) => !p.boardOnly || canSeeReports(user?.role))

const linkClass = ({ isActive }: { isActive: boolean }) =>
  `flex items-center gap-2.5 rounded-md px-2.5 py-1.5 text-sm ${
    isActive ? 'bg-gray-100 font-semibold text-gray-900' : 'text-gray-600 hover:bg-gray-50 hover:text-gray-900'
  }`

export default function Sidebar({ user, userError }: Props) {
  return (
    <aside className="hidden w-56 shrink-0 flex-col border-r border-gray-200 bg-white p-3 md:flex">
      <nav aria-label="Main" className="flex flex-col gap-0.5">
        {pagesFor(user).map(({ label, icon: Icon, path }) => (
          <NavLink key={path} to={path} end className={linkClass}>
            <Icon size={16} aria-hidden />
            {label}
          </NavLink>
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
        {user?.role && <p className="mt-0.5 text-gray-400 capitalize">{user.role}</p>}
        {userError && <p className="text-red-600">{userError}</p>}
        {!user && !userError && <p>Loading…</p>}
      </div>
    </aside>
  )
}

// The sidebar is hidden on narrow screens; this row of tabs takes its place.
export function MobileNav({ user }: { user: User | null }) {
  return (
    <nav aria-label="Pages" className="flex gap-1 overflow-x-auto border-b border-gray-200 bg-white px-4 py-2 md:hidden">
      {pagesFor(user).map(({ label, path }) => (
        <NavLink
          key={path}
          to={path}
          end
          className={({ isActive }) =>
            `shrink-0 rounded-md px-2.5 py-1 text-xs ${isActive ? 'bg-gray-100 font-semibold text-gray-900' : 'text-gray-600'}`
          }
        >
          {label}
        </NavLink>
      ))}
    </nav>
  )
}
