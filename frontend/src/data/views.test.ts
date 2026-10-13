import { expect, test } from 'vitest'
import { fixtureDeals, lastSemester, thisSemester } from '../test/fixtures'
import { companiesFrom, contactsFrom, reportFor } from './views'

test('a company gathers its deals from every semester, newest first', () => {
  const granit = companiesFrom(fixtureDeals).find((c) => c.name === 'Granit Consulting')!

  expect(granit.deals.map((d) => d.semester)).toEqual([thisSemester, lastSemester])
  expect(granit.confirmedSEK).toBe(55_000)
  expect(granit.open).toBe(1)
  expect(granit.owners.map((p) => p.name)).toEqual(['Maja Holm'])
})

test('companies with the same name in different case are one company', () => {
  const deals = [fixtureDeals[0], { ...fixtureDeals[1], company: fixtureDeals[0].company.toUpperCase() }]

  expect(companiesFrom(deals)).toHaveLength(1)
})

test('contacts come from deals, one per email', () => {
  const contacts = contactsFrom(fixtureDeals)

  expect(contacts.find((c) => c.email === 'lina.dahl@example.com')).toMatchObject({
    name: 'Lina Dahl',
    company: 'Granit Consulting',
    role: 'Recruitment Manager',
  })
  expect(new Set(contacts.map((c) => c.email)).size).toBe(contacts.length)
})

test('a semester report adds up fees, wins and follow-ups', () => {
  const report = reportFor(thisSemester, fixtureDeals)

  expect(report).toMatchObject({
    deals: 11,
    won: 2,
    canceled: 1,
    confirmedSEK: 40_000,
    paidSEK: 0,
    unpaidSEK: 40_000,
    pipelineSEK: 60_000,
  })
  expect(report.winRate).toBeCloseTo(2 / 3)
  expect(report.needsFollowUp.map((d) => d.company).sort()).toEqual(['Kvarnholmen AI', 'Lumen Health', 'Stadsnät Labs'])
  expect(report.byStatus.find((b) => b.label === 'Lead')?.value).toBe(3)
})

test('a semester with nothing decided has no win rate', () => {
  expect(reportFor('VT 2000', fixtureDeals).winRate).toBeNull()
})

test('within a semester, the most recently added deal comes first', () => {
  const older = { ...fixtureDeals[0], id: 'older', company: 'Nordljus AB', createdAt: '2026-09-01T00:00:00Z' }
  const newer = { ...older, id: 'newer', createdAt: '2026-10-01T00:00:00Z' }

  expect(companiesFrom([older, newer])[0].deals.map((d) => d.id)).toEqual(['newer', 'older'])
})

test('reports count teammates who share a name separately', () => {
  const kim1 = { id: 'm-kim1', name: 'Kim Ek', email: 'kim1@example.com' }
  const kim2 = { id: 'm-kim2', name: 'Kim Ek', email: 'kim2@example.com' }
  const deals = [
    { ...fixtureDeals[0], id: 'a', owner: kim1 },
    { ...fixtureDeals[0], id: 'b', owner: kim1 },
    { ...fixtureDeals[0], id: 'c', owner: kim2 },
  ]

  expect(reportFor(fixtureDeals[0].semester, deals).byOwner).toEqual([
    { label: 'Kim Ek (kim1@example.com)', value: 2 },
    { label: 'Kim Ek (kim2@example.com)', value: 1 },
  ])
})
