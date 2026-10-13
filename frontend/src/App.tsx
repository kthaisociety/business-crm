import { useCallback, useEffect, useState } from 'react'
import { X } from 'lucide-react'
import { addNote, createDeal, fetchDeals, fetchMe, updateDeal, type DealPatch, type NewLeadInput, type User } from './api'
import DealDetailDrawer from './components/drawer/DealDetailDrawer'
import NewLeadDialog from './components/drawer/NewLeadDialog'
import Header from './components/layout/Header'
import Sidebar from './components/layout/Sidebar'
import PipelineView from './components/PipelineView'
import { compareSemesters, currentSemester, isConfirmed, isOpen, isUnpaid, type Deal } from './data/deals'
import { useMinuteTick } from './hooks'

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

const sumFees = (deals: Deal[]) => deals.reduce((sum, deal) => sum + (deal.feeSEK ?? 0), 0)
const messageOf = (err: unknown) => (err instanceof Error ? err.message : String(err))

export default function App() {
  useMinuteTick()
  const [user, setUser] = useState<User | null>(null)
  const [userError, setUserError] = useState<string | null>(null)
  const [deals, setDeals] = useState<Deal[] | null>(null)
  const [loadError, setLoadError] = useState<string | null>(null)
  const [saveError, setSaveError] = useState<string | null>(null)
  const [semester, setSemester] = useState(() => currentSemester())
  const [selectedId, setSelectedId] = useState<string | null>(null)
  const [query, setQuery] = useState('')
  const [addingLead, setAddingLead] = useState(false)

  useEffect(() => {
    const controller = new AbortController()
    fetchMe(controller.signal)
      .then(setUser)
      .catch((err: unknown) => {
        if (controller.signal.aborted) return
        setUserError(messageOf(err))
      })
    return () => controller.abort()
  }, [])

  const loadDeals = useCallback((signal?: AbortSignal) => {
    fetchDeals(signal)
      .then(setDeals)
      .catch((err: unknown) => {
        if (signal?.aborted) return
        setLoadError(`Couldn't load deals: ${messageOf(err)}`)
      })
  }, [])

  useEffect(() => {
    const controller = new AbortController()
    loadDeals(controller.signal)
    return () => controller.abort()
  }, [loadDeals])

  const all = deals ?? []
  // The current semester is always offered, even before it has deals.
  const semesters = [...new Set([currentSemester(), ...all.map((deal) => deal.semester)])].sort(compareSemesters)

  const replaceDeal = (id: string, change: (deal: Deal) => Deal) =>
    setDeals((list) => list && list.map((deal) => (deal.id === id ? change(deal) : deal)))

  // Edits show at once and save in the background. If saving fails, the edited fields go back to what
  // they were and the error shows above the board.
  const saveDeal = (id: string, patch: DealPatch) => {
    const before = all.find((deal) => deal.id === id)
    if (!before) return
    const previous = Object.fromEntries(Object.keys(patch).map((key) => [key, before[key as keyof DealPatch]]))
    replaceDeal(id, (deal) => ({ ...deal, ...patch }))
    setSaveError(null)
    updateDeal(id, patch).catch((err: unknown) => {
      replaceDeal(id, (deal) => ({ ...deal, ...previous }))
      setSaveError(`Couldn't save ${before.company}: ${messageOf(err)}`)
    })
  }

  // Resolves once the note is saved; the drawer keeps the text if it rejects.
  const saveNote = async (id: string, text: string) => {
    const note = await addNote(id, text)
    replaceDeal(id, (deal) => ({ ...deal, activities: [...deal.activities, note] }))
  }

  // A new lead is owned by whoever adds it (the server records the signed-in user), so adding waits
  // until we know who that is.
  const addLead = async (lead: Omit<NewLeadInput, 'semester'>) => {
    if (!user) return
    const deal = await createDeal({ ...lead, semester })
    setDeals((list) => [...(list ?? []), deal])
    setAddingLead(false)
    setSelectedId(deal.id)
  }

  const inSemester = all.filter((deal) => deal.semester === semester)
  const selected = all.find((deal) => deal.id === selectedId)

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
          {saveError && (
            <p role="alert" className="mx-6 mt-4 flex items-center justify-between gap-3 rounded-lg border border-red-200 bg-red-50 px-3 py-2 text-xs text-red-700">
              {saveError}
              <button type="button" aria-label="Dismiss" onClick={() => setSaveError(null)} className="text-red-500 hover:text-red-800">
                <X size={14} aria-hidden />
              </button>
            </p>
          )}
          {loadError ? (
            <div role="alert" className="m-6 rounded-lg border border-red-200 bg-red-50 p-4 text-sm text-red-700">
              {loadError}
              <button
                type="button"
                onClick={() => {
                  setLoadError(null)
                  loadDeals()
                }}
                className="ml-3 font-semibold underline">
                Retry
              </button>
            </div>
          ) : deals === null ? (
            <p className="m-6 text-sm text-gray-500">Loading deals…</p>
          ) : (
            <PipelineView deals={inSemester.filter((deal) => matches(deal, query))} onSelect={setSelectedId} />
          )}
        </main>
      </div>

      {selected && (
        <DealDetailDrawer
          key={selected.id}
          deal={selected}
          otherDeals={all
            .filter((deal) => deal.company === selected.company && deal.id !== selected.id)
            .sort((a, b) => compareSemesters(a.semester, b.semester))}
          onClose={() => setSelectedId(null)}
          onChange={(patch) => saveDeal(selected.id, patch)}
          onAddNote={(text) => saveNote(selected.id, text)}
          onSelect={(id) => {
            const other = all.find((deal) => deal.id === id)
            if (other) setSemester(other.semester)
            setSelectedId(id)
          }}
        />
      )}
      {addingLead && user && <NewLeadDialog semester={semester} onCreate={addLead} onClose={() => setAddingLead(false)} />}
    </div>
  )
}
