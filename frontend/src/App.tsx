import { useCallback, useEffect, useRef, useState } from 'react'
import { X } from 'lucide-react'
import { Navigate, Route, Routes } from 'react-router'
import {
  addNote,
  createCompany,
  createContact,
  createDeal,
  deleteCompany,
  deleteContact,
  fetchCompanies,
  fetchContacts,
  fetchDeals,
  fetchMe,
  updateCompany,
  updateContact,
  updateDeal,
  type DealPatch,
  type NewLeadInput,
  type User,
} from './api'
import DealDetailDrawer from './components/drawer/DealDetailDrawer'
import NewLeadDialog from './components/drawer/NewLeadDialog'
import Header from './components/layout/Header'
import Sidebar, { MobileNav } from './components/layout/Sidebar'
import CompaniesPage, { type CompanyActions } from './components/pages/CompaniesPage'
import ContactsPage, { type ContactActions } from './components/pages/ContactsPage'
import ReportsPage from './components/pages/ReportsPage'
import PipelineView from './components/PipelineView'
import { compareSemesters, currentSemester, isConfirmed, isOpen, isUnpaid, type Company, type Contact, type Deal } from './data/deals'
import { messageOf } from './format'
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

type Data = { deals: Deal[]; companies: Company[]; contacts: Contact[] }

const fetchAll = async (signal?: AbortSignal): Promise<Data> => {
  const [deals, companies, contacts] = await Promise.all([fetchDeals(signal), fetchCompanies(signal), fetchContacts(signal)])
  return { deals, companies, contacts }
}

export default function App() {
  useMinuteTick()
  const [user, setUser] = useState<User | null>(null)
  const [userError, setUserError] = useState<string | null>(null)
  const [data, setData] = useState<Data | null>(null)
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

  const load = useCallback((signal?: AbortSignal) => {
    fetchAll(signal)
      .then(setData)
      .catch((err: unknown) => {
        if (signal?.aborted) return
        setLoadError(`Couldn't load deals: ${messageOf(err)}`)
      })
  }, [])

  useEffect(() => {
    const controller = new AbortController()
    load(controller.signal)
    return () => controller.abort()
  }, [load])

  // After a change that shows in more than one place (a company's name is on its deals, a new lead may
  // add a company and contact), everything reloads.
  const reload = () =>
    fetchAll()
      .then(setData)
      .catch((err: unknown) => setSaveError(`Couldn't refresh: ${messageOf(err)}`))

  const all = data?.deals ?? []
  const companies = data?.companies ?? []
  const contacts = data?.contacts ?? []
  // The current semester is always offered, even before it has deals.
  const semesters = [...new Set([currentSemester(), ...all.map((deal) => deal.semester)])].sort(compareSemesters)

  const replaceDeal = (id: string, change: (deal: Deal) => Deal) =>
    setData((d) => d && { ...d, deals: d.deals.map((deal) => (deal.id === id ? change(deal) : deal)) })

  // Saves to one deal go out one at a time, in the order they were made, so the server ends up with the
  // last edit. `confirmed` holds each deal as the server last confirmed it while its saves are pending.
  const saveQueue = useRef(new Map<string, Promise<void>>())
  const confirmed = useRef(new Map<string, Deal>())

  // Edits show at once and save in the background. If a save fails, each field it changed goes back to
  // the last confirmed value, unless a newer edit has changed that field since; the error shows above the
  // board.
  const saveDeal = (id: string, patch: DealPatch) => {
    const before = all.find((deal) => deal.id === id)
    if (!before) return
    if (!confirmed.current.has(id)) confirmed.current.set(id, before)
    const { contactId, ...fields } = patch
    const local: Partial<Deal> = { ...fields }
    if (contactId !== undefined) local.contact = contacts.find((c) => c.id === contactId) ?? null
    replaceDeal(id, (deal) => ({ ...deal, ...local }))
    setSaveError(null)

    // The fields this edit set locally (a contactId shows as the contact itself).
    const keys = Object.keys(local) as (keyof Deal)[]
    const send = async () => {
      try {
        confirmed.current.set(id, await updateDeal(id, patch))
      } catch (err) {
        const last = confirmed.current.get(id)!
        replaceDeal(id, (deal) => {
          const reverted = { ...deal }
          for (const key of keys) if (deal[key] === local[key]) Object.assign(reverted, { [key]: last[key] })
          return reverted
        })
        setSaveError(`Couldn't save ${before.company}: ${messageOf(err)}`)
      }
    }
    const queued = (saveQueue.current.get(id) ?? Promise.resolve()).then(send)
    saveQueue.current.set(id, queued)
    void queued.then(() => {
      // The last save in line clears the deal's queue.
      if (saveQueue.current.get(id) === queued) {
        saveQueue.current.delete(id)
        confirmed.current.delete(id)
      }
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
    setData((d) => d && { ...d, deals: [...d.deals, deal] })
    setAddingLead(false)
    setSelectedId(deal.id)
    void reload()
  }

  // Each resolves once saved and the app has reloaded; a rejection carries the server's reason to the form.
  const companyActions: CompanyActions = {
    create: async (input) => {
      await createCompany(input)
      await reload()
    },
    update: async (id, input) => {
      await updateCompany(id, input)
      await reload()
    },
    remove: async (id) => {
      await deleteCompany(id)
      await reload()
    },
  }
  const contactActions: ContactActions = {
    create: async (fields) => {
      await createContact(fields)
      await reload()
    },
    update: async (id, fields) => {
      await updateContact(id, fields)
      await reload()
    },
    remove: async (id) => {
      await deleteContact(id)
      await reload()
    },
  }

  // Opens a deal from any page, switching to its semester so the board behind shows it.
  const openDeal = (id: string) => {
    const deal = all.find((d) => d.id === id)
    if (deal) setSemester(deal.semester)
    setSelectedId(id)
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
        // Adding waits for the deal list, so a late first load can't replace a lead added meanwhile.
        canAddLead={user !== null && data !== null}
        onNewLead={() => setAddingLead(true)}
      />
      <MobileNav />
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
                  load()
                }}
                className="ml-3 font-semibold underline"
              >
                Retry
              </button>
            </div>
          ) : data === null ? (
            <p className="m-6 text-sm text-gray-500">Loading deals…</p>
          ) : (
            <Routes>
              <Route
                index
                element={
                  <PipelineView
                    deals={inSemester.filter((deal) => matches(deal, query))}
                    onSelect={setSelectedId}
                    onMove={(id, status) => saveDeal(id, { status })}
                  />
                }
              />
              <Route path="companies" element={<CompaniesPage companies={companies} contacts={contacts} deals={all} query={query} onSelect={openDeal} actions={companyActions} contactActions={contactActions} />} />
              <Route path="contacts" element={<ContactsPage companies={companies} contacts={contacts} deals={all} query={query} onSelect={openDeal} actions={contactActions} />} />
              <Route path="reports" element={<ReportsPage deals={all} semester={semester} onSelect={openDeal} />} />
              <Route path="*" element={<Navigate to="/" replace />} />
            </Routes>
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
          companyContacts={contacts.filter((c) => c.companyId === selected.companyId)}
          onChange={(patch) => saveDeal(selected.id, patch)}
          onAddNote={(text) => saveNote(selected.id, text)}
          onSelect={openDeal}
        />
      )}
      {addingLead && user && <NewLeadDialog semester={semester} companies={companies} contacts={contacts} onCreate={addLead} onClose={() => setAddingLead(false)} />}
    </div>
  )
}
