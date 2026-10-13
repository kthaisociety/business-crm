import { expect, test } from 'vitest'
import { compareSemesters, mockDeals, nudgeFor, type Deal } from './mockDeals'

const now = new Date('2026-10-13T12:00:00').getTime()
const deal = (patch: Partial<Deal>): Deal => ({
  ...mockDeals[0],
  status: 'discussing',
  nextStepDue: null,
  activities: [{ id: 'a', type: 'note', source: 'manual', title: '', body: '', timestamp: '2026-10-12T12:00:00' }],
  ...patch,
})

test('an overdue next step wins over inactivity', () => {
  const old = [{ id: 'a', type: 'note' as const, source: 'manual' as const, title: '', body: '', timestamp: '2026-10-01T12:00:00' }]
  expect(nudgeFor(deal({ nextStepDue: '2026-10-10', activities: old }), now)).toEqual({ kind: 'overdue', days: 3 })
})

test('a next step due today is not overdue', () => {
  expect(nudgeFor(deal({ nextStepDue: '2026-10-13' }), now)).toBeNull()
})

test('nudges after 4 days without a touchpoint', () => {
  const at = (timestamp: string) => [{ id: 'a', type: 'note' as const, source: 'manual' as const, title: '', body: '', timestamp }]
  expect(nudgeFor(deal({ activities: at('2026-10-10T12:00:00') }), now)).toBeNull()
  expect(nudgeFor(deal({ activities: at('2026-10-09T12:00:00') }), now)).toEqual({ kind: 'inactive', days: 4 })
})

test('finished and canceled deals never nudge', () => {
  expect(nudgeFor(deal({ status: 'finished', nextStepDue: '2026-01-01' }), now)).toBeNull()
  expect(nudgeFor(deal({ status: 'canceled', nextStepDue: '2026-01-01' }), now)).toBeNull()
})

test('sorts semesters newest first', () => {
  expect(['HT 2025', 'HT 2026', 'VT 2026'].sort(compareSemesters)).toEqual(['HT 2026', 'VT 2026', 'HT 2025'])
})
