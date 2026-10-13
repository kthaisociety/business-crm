import { expect, test } from 'vitest'
import { fixtureDeals } from '../test/fixtures'
import { calendarDaysBetween, compareSemesters, currentSemester, isUnpaid, nudgeFor, type Deal } from './deals'

const now = new Date('2026-10-13T12:00:00').getTime()
const deal = (patch: Partial<Deal>): Deal => ({
  ...fixtureDeals[0],
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

test('counts overdue days by calendar date, across a clock change', () => {
  // Stockholm springs forward on 29 March 2026: the night of 29→30 March is 23 hours long.
  expect(calendarDaysBetween('2026-03-29', '2026-03-30')).toBe(1)
  expect(calendarDaysBetween('2026-03-28', '2026-03-31')).toBe(3)
  const justAfterMidnight = new Date('2026-03-30T00:30:00').getTime()
  expect(nudgeFor(deal({ nextStepDue: '2026-03-29' }), justAfterMidnight)).toEqual({ kind: 'overdue', days: 1 })
})

test('a confirmed fee is unpaid until marked paid or cancelled', () => {
  const signed = (invoiceStatus: Deal['invoiceStatus']) => deal({ status: 'signed', feeSEK: 20_000, invoiceStatus })
  expect(isUnpaid(signed(null))).toBe(true)
  expect(isUnpaid(signed('not_sent'))).toBe(true)
  expect(isUnpaid(signed('waiting'))).toBe(true)
  expect(isUnpaid(signed('paid'))).toBe(false)
  expect(isUnpaid(signed('cancelled'))).toBe(false)
  expect(isUnpaid(deal({ status: 'discussing', feeSEK: 20_000, invoiceStatus: null }))).toBe(false)
})

test('the current semester is VT from January and HT from July', () => {
  expect(currentSemester(new Date(2026, 0, 10))).toBe('VT 2026')
  expect(currentSemester(new Date(2026, 5, 30))).toBe('VT 2026')
  expect(currentSemester(new Date(2026, 6, 1))).toBe('HT 2026')
  expect(currentSemester(new Date(2026, 11, 31))).toBe('HT 2026')
})
