import { expect, test } from 'vitest'
import { LocalEdits } from './dealSync'
import { fixtureDeals } from './test/fixtures'

const deal = fixtureDeals.find((d) => d.id === 'granit')!

test('a server answer applies in full when nothing was edited', () => {
  const edits = new LocalEdits()
  const server = { ...deal, company: 'Granit AB' }

  expect(edits.merge(server, deal, edits.now())).toEqual(server)
})

test('an edit made after the request went out is kept, and everything else comes from the server', () => {
  const edits = new LocalEdits()
  const takenAt = edits.now()
  edits.edit(deal.id, ['status'])
  const local = { ...deal, status: 'finished' as const }
  const server = { ...deal, company: 'Granit AB', contact: { ...deal.contact!, email: 'new@example.com' } }

  const merged = edits.merge(server, local, takenAt)

  expect(merged.status).toBe('finished')
  expect(merged.company).toBe('Granit AB')
  expect(merged.contact?.email).toBe('new@example.com')
})

test('a field still saving keeps its local value, even in an answer asked for after the edit', () => {
  const edits = new LocalEdits()
  edits.edit(deal.id, ['status'])
  edits.startSaving(deal.id, ['status'])
  const local = { ...deal, status: 'finished' as const }

  expect(edits.merge(deal, local, edits.now()).status).toBe('finished')

  edits.doneSaving(deal.id, ['status'])
  expect(edits.merge(deal, local, edits.now()).status).toBe(deal.status)
})

test('an edit is the latest until another edit or save of the same field', () => {
  const edits = new LocalEdits()
  const first = edits.edit(deal.id, ['status'])
  expect(edits.isLatest(deal.id, 'status', first)).toBe(true)

  edits.startSaving(deal.id, ['status'])
  expect(edits.isLatest(deal.id, 'status', first)).toBe(false)
  edits.doneSaving(deal.id, ['status'])

  edits.edit(deal.id, ['status'])
  expect(edits.isLatest(deal.id, 'status', first)).toBe(false)
  expect(edits.isLatest(deal.id, 'feeSEK', first)).toBe(false)
})
