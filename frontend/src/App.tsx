import { useEffect, useState } from 'react'
import { fetchMe, type User } from './api'
import DealDetailDrawer from './components/drawer/DealDetailDrawer'
import NewLeadDialog, { type NewLead } from './components/drawer/NewLeadDialog'
import Header from './components/layout/Header'
import Sidebar from './components/layout/Sidebar'
import PipelineView from './components/PipelineView'
import { compareSemesters, isConfirmed, isOpen, isUnpaid, mockDeals, type Deal, type Person } from './data/mockDeals'
import { initialsOf } from './format'
import { useMinuteTick } from './hooks'

// Deals live in memory for now: reloading the page resets them to the seed data.

let nextId = 0
const newId = (prefix: string) => `${prefix}-${Date.now()}-${nextId++}`

function matches(deal: Deal, query: string): boolean {
  const q = query.trim().toLowerCase()
  if (!q) return true
  return [
    deal.company,
    deal.notes,
    deal.nextStep,
    deal.contact?.name ?? '',
    deal.contact?.email ?? '',
    deal.owner?.name ?? '',
    ...deal.support.map((p) => p.name),
    ...deal.activities.flatMap((a) => [a.title, a.body, ...(a.bulletPoints ?? [])]),
  ].some((text) => text.toLowerCase().includes(q))
}

function personFor(user: User): Person {
  const name = user.name || user.email
  return { name, initials: initialsOf(name), color: 'bg-emerald-100 text-emerald-700' }
}

const sumFees = (deals: Deal[]) => deals.reduce((sum, deal) => sum + (deal.feeSEK ?? 0), 0)

export default function App() {
  useMinuteTick()
  const [user, setUser] = useState<User | null>(null)
  const [userError, setUserError] = useState<string | null>(null)
  const [deals, setDeals] = useState<Deal[]>(mockDeals)
  const semesters = [...new Set(deals.map((deal) => deal.semester))].sort(compareSemesters)
  const [semester, setSemester] = useState(semesters[0])
  const [selectedId, setSelectedId] = useState<string | null>(null)
  const [query, setQuery] = useState('')
  const [addingLead, setAddingLead] = useState(false)

  useEffect(() => {
    const controller = new AbortController()
    fetchMe(controller.signal)
      .then(setUser)
      .catch((err: unknown) => {
        if (controller.signal.aborted) return
        setUserError(err instanceof Error ? err.message : String(err))
      })
    return () => controller.abort()
  }, [])

  const updateDeal = (id: string, change: (deal: Deal) => Deal) =>
    setDeals((all) => all.map((deal) => (deal.id === id ? change(deal) : deal)))

  const addNote = (id: string, text: string) =>
    updateDeal(id, (deal) => ({
      ...deal,
      activities: [
        ...deal.activities,
        { id: newId('note'), type: 'note', source: 'manual', title: 'Quick note', timestamp: new Date().toISOString(), body: text },
      ],
    }))

  // A new lead is owned by whoever adds it, so adding waits until we know who that is.
  const addLead = (lead: NewLead) => {
    if (!user) return
    const deal: Deal = {
      id: newId('deal'),
      company: lead.company,
      dealType: lead.dealType,
      eventType: lead.dealType === 'Job Post' ? 'Job post' : 'TBA',
      audience: 'Unclear',
      semester,
      status: 'lead',
      startDate: null,
      endDate: null,
      location: '',
      owner: personFor(user),
      support: [],
      contact: lead.contactName || lead.contactEmail ? { name: lead.contactName, email: lead.contactEmail, role: '' } : null,
      nextStep: lead.nextStep,
      nextStepDue: null,
      feeSEK: null,
      invoiceStatus: null,
      invoiceDue: null,
      links: {},
      rating: null,
      notes: '',
      activities: [
        { id: newId('note'), type: 'note', source: 'manual', title: 'Added as a lead', timestamp: new Date().toISOString(), body: '' },
      ],
    }
    setDeals((all) => [...all, deal])
    setAddingLead(false)
    setSelectedId(deal.id)
  }

  const inSemester = deals.filter((deal) => deal.semester === semester)
  const selected = deals.find((deal) => deal.id === selectedId)

  return (
    <div className="flex h-screen flex-col bg-gray-50 font-sans text-gray-900">
      <Header
        semesters={semesters}
        semester={semester}
        onSemesterChange={setSemester}
        query={query}
        onQueryChange={setQuery}
        openDeals={inSemester.filter(isOpen).length}
        confirmedSEK={sumFees(inSemester.filter(isConfirmed))}
        unpaidSEK={sumFees(inSemester.filter(isUnpaid))}
        canAddLead={user !== null}
        onNewLead={() => setAddingLead(true)}
      />
      <div className="flex min-h-0 flex-1">
        <Sidebar user={user} userError={userError} />
        <main className="flex min-w-0 flex-1 flex-col">
          <PipelineView deals={inSemester.filter((deal) => matches(deal, query))} onSelect={setSelectedId} />
        </main>
      </div>

      {selected && (
        <DealDetailDrawer
          key={selected.id}
          deal={selected}
          otherDeals={deals
            .filter((deal) => deal.company === selected.company && deal.id !== selected.id)
            .sort((a, b) => compareSemesters(a.semester, b.semester))}
          onClose={() => setSelectedId(null)}
          onChange={(patch) => updateDeal(selected.id, (deal) => ({ ...deal, ...patch }))}
          onAddNote={(text) => addNote(selected.id, text)}
          onSelect={(id) => {
            const other = deals.find((deal) => deal.id === id)
            if (other) setSemester(other.semester)
            setSelectedId(id)
          }}
        />
      )}
      {addingLead && user && <NewLeadDialog semester={semester} onCreate={addLead} onClose={() => setAddingLead(false)} />}
    </div>
  )
}
