import { act, cleanup, fireEvent, render, screen, within } from '@testing-library/react'
import { afterEach, beforeEach, expect, test, vi } from 'vitest'
import App from './App'

beforeEach(() => {
  vi.stubGlobal(
    'fetch',
    vi.fn().mockResolvedValue(
      new Response(JSON.stringify({ sub: 'dev', email: 'dev@kthais.com', name: 'Dev User' })),
    ),
  )
})

afterEach(() => {
  cleanup()
  vi.unstubAllGlobals()
  vi.useRealTimers()
})

const column = (label: string) => screen.getByRole('region', { name: label })
const card = (company: string) => within(screen.getByRole('main')).getByRole('button', { name: new RegExp(company) })
const open = (company: string) => {
  fireEvent.click(card(company))
  return screen.getByRole('dialog', { name: company })
}

test('shows the signed-in user', async () => {
  render(<App />)

  expect(await screen.findByText('Signed in as dev@kthais.com')).toBeInTheDocument()
})

test('shows an error when no one is signed in', async () => {
  vi.stubGlobal('fetch', vi.fn().mockResolvedValue(new Response('', { status: 401 })))

  render(<App />)

  expect(await screen.findByText('Loading the signed-in user failed (401)')).toBeInTheDocument()
})

test('shows the latest semester by status, with its totals', () => {
  render(<App />)

  expect(screen.getByLabelText('Semester')).toHaveValue('HT 2026')
  expect(within(column('Agreement')).getByText('Kvarnholmen AI')).toBeInTheDocument()
  expect(within(column('Signed')).getByText('Granit Consulting')).toBeInTheDocument()
  expect(screen.queryByText('Polar Systems')).not.toBeInTheDocument()
  expect(screen.getByText('Open Deals:')).toHaveTextContent('Open Deals: 9')
  expect(screen.getByText('Confirmed:')).toHaveTextContent('Confirmed: 40,000 SEK')
  expect(screen.getByText('Unpaid:')).toHaveTextContent('Unpaid: 40,000 SEK')
})

test('switching semester shows that semester’s deals', () => {
  render(<App />)

  fireEvent.change(screen.getByLabelText('Semester'), { target: { value: 'VT 2026' } })

  expect(within(column('Finished')).getByText('Polar Systems')).toBeInTheDocument()
  expect(screen.queryByText('Kvarnholmen AI')).not.toBeInTheDocument()
  expect(screen.getByText('Unpaid:')).toHaveTextContent('Unpaid: 30,000 SEK')
})

test('nudges overdue next steps and inactive deals', () => {
  render(<App />)

  expect(card('Kvarnholmen AI')).toHaveTextContent('Next step overdue: 2 days')
  expect(card('Lumen Health')).toHaveTextContent('Follow-up due: 6 days inactive')
  expect(card('Saltsjö Insurance')).not.toHaveTextContent(/overdue|Follow-up due/)
})

test('a quick note logs to the timeline and clears the inactivity nudge', () => {
  render(<App />)
  const drawer = open('Lumen Health')

  fireEvent.change(within(drawer).getByLabelText('Quick note'), { target: { value: 'Got the intro, call on Friday' } })
  fireEvent.click(within(drawer).getByRole('button', { name: 'Save Note' }))

  expect(within(drawer).getByText('Got the intro, call on Friday')).toBeInTheDocument()
  expect(within(drawer).getByLabelText('Quick note')).toHaveValue('')
  expect(card('Lumen Health')).not.toHaveTextContent('Follow-up due')
})

test('moving the due date clears the overdue nudge', () => {
  render(<App />)
  const drawer = open('Kvarnholmen AI')

  fireEvent.change(within(drawer).getByLabelText('Due'), { target: { value: '2099-01-01' } })

  expect(card('Kvarnholmen AI')).not.toHaveTextContent('overdue')
})

test('changing the status moves the card to that column', () => {
  render(<App />)
  open('Lumen Health')

  fireEvent.change(screen.getByLabelText('Status'), { target: { value: 'discussing' } })
  fireEvent.click(screen.getByRole('button', { name: 'Close' }))

  expect(screen.queryByRole('dialog')).not.toBeInTheDocument()
  expect(within(column('Discussing')).getByText('Lumen Health')).toBeInTheDocument()
  expect(within(column('Lead')).queryByText('Lumen Health')).not.toBeInTheDocument()
})

test('Send Email opens a mail to the deal contact', () => {
  render(<App />)
  const drawer = open('Granit Consulting')

  expect(within(drawer).getByRole('link', { name: 'Send Email' })).toHaveAttribute(
    'href',
    expect.stringMatching(/^mailto:lina\.dahl@example\.com\?subject=/),
  )
})

test('the drawer links to other deals with the same company', () => {
  render(<App />)
  const drawer = open('Granit Consulting')

  fireEvent.click(within(drawer).getByRole('button', { name: /VT 2026 · Panel discussion/ }))

  expect(screen.getByLabelText('Semester')).toHaveValue('VT 2026')
  expect(screen.getByRole('dialog', { name: 'Granit Consulting' })).toHaveTextContent('Panel discussion')
})

test('search filters by company, contact and notes', () => {
  render(<App />)

  fireEvent.change(screen.getByLabelText('Search'), { target: { value: 'founders' } })

  expect(screen.getByText('Fjällbacka Ventures')).toBeInTheDocument()
  expect(screen.queryByText('Lumen Health')).not.toBeInTheDocument()
})

test('filters for follow-ups and shows canceled deals on request', () => {
  render(<App />)
  expect(screen.queryByText('Brygga Capital')).not.toBeInTheDocument()

  fireEvent.click(screen.getByRole('button', { name: 'Show canceled' }))
  expect(within(column('Canceled')).getByText('Brygga Capital')).toBeInTheDocument()

  fireEvent.click(screen.getByRole('button', { name: 'Needs follow-up' }))
  expect(screen.getByText('Lumen Health')).toBeInTheDocument()
  expect(screen.queryByText('Saltsjö Insurance')).not.toBeInTheDocument()
})

test('the list view shows the deal log as a table', () => {
  render(<App />)

  fireEvent.click(screen.getByRole('button', { name: 'List' }))

  const row = screen.getByRole('row', { name: /Granit Consulting/ })
  expect(row).toHaveTextContent('Signed')
  expect(row).toHaveTextContent('40,000 SEK')
  expect(row).toHaveTextContent('Not sent')
})

test('a new lead lands in Lead, owned by the signed-in user', async () => {
  render(<App />)
  await screen.findByText('Signed in as dev@kthais.com')
  fireEvent.click(screen.getByRole('button', { name: 'New Lead' }))
  const form = screen.getByRole('dialog', { name: 'New lead' })

  fireEvent.change(within(form).getByLabelText('Company'), { target: { value: 'Nordljus AB' } })
  fireEvent.change(within(form).getByLabelText('Next step'), { target: { value: 'Send intro email' } })
  fireEvent.click(within(form).getByRole('button', { name: 'Add lead' }))

  expect(screen.getByRole('dialog', { name: 'Nordljus AB' })).toHaveTextContent('Dev User')
  expect(within(column('Lead')).getByText('Nordljus AB')).toBeInTheDocument()
})

test('a signed fee with no invoice status counts as unpaid', () => {
  render(<App />)
  open('Kvarnholmen AI')

  fireEvent.change(screen.getByLabelText('Status'), { target: { value: 'signed' } })

  expect(screen.getByText('Confirmed:')).toHaveTextContent('Confirmed: 60,000 SEK')
  expect(screen.getByText('Unpaid:')).toHaveTextContent('Unpaid: 60,000 SEK')
})

test('New Lead waits for the signed-in user', async () => {
  let signIn: (res: Response) => void = () => {}
  vi.stubGlobal('fetch', vi.fn(() => new Promise<Response>((resolve) => (signIn = resolve))))
  render(<App />)

  expect(screen.getByRole('button', { name: 'New Lead' })).toBeDisabled()

  signIn(new Response(JSON.stringify({ sub: 'dev', email: 'dev@kthais.com', name: 'Dev User' })))
  await screen.findByText('Signed in as dev@kthais.com')
  expect(screen.getByRole('button', { name: 'New Lead' })).toBeEnabled()
})

test('New Lead stays disabled when sign-in fails', async () => {
  vi.stubGlobal('fetch', vi.fn().mockResolvedValue(new Response('', { status: 500 })))
  render(<App />)

  await screen.findByText('Loading the signed-in user failed (500)')
  expect(screen.getByRole('button', { name: 'New Lead' })).toBeDisabled()
})

test('a negative fee is ignored', () => {
  render(<App />)
  const drawer = open('Granit Consulting')

  fireEvent.change(within(drawer).getByLabelText('Fee (SEK)'), { target: { value: '-1000' } })

  expect(screen.getByText('Confirmed:')).toHaveTextContent('Confirmed: 40,000 SEK')
})

test('Send Email needs a contact email', async () => {
  render(<App />)
  await screen.findByText('Signed in as dev@kthais.com')
  fireEvent.click(screen.getByRole('button', { name: 'New Lead' }))
  const form = screen.getByRole('dialog', { name: 'New lead' })
  fireEvent.change(within(form).getByLabelText('Company'), { target: { value: 'Nordljus AB' } })
  fireEvent.change(within(form).getByLabelText('Contact name'), { target: { value: 'Kim Ek' } })
  fireEvent.click(within(form).getByRole('button', { name: 'Add lead' }))

  const drawer = screen.getByRole('dialog', { name: 'Nordljus AB' })
  expect(within(drawer).queryByRole('link', { name: 'Send Email' })).not.toBeInTheDocument()
  expect(within(drawer).getByText('No contact email')).toBeInTheDocument()
})

test('the drawer takes focus, keeps Tab inside, and returns focus on Escape', () => {
  render(<App />)
  card('Lumen Health').focus()
  const drawer = open('Lumen Health')
  expect(drawer).toHaveFocus()

  const close = within(drawer).getByRole('button', { name: 'Close' })
  fireEvent.keyDown(document, { key: 'Tab', shiftKey: true })
  expect(drawer).not.toHaveFocus()
  expect(drawer).toContainElement(document.activeElement as HTMLElement)

  close.focus()
  fireEvent.keyDown(document, { key: 'Escape' })
  expect(screen.queryByRole('dialog')).not.toBeInTheDocument()
  expect(card('Lumen Health')).toHaveFocus()
})

test('the New Lead form focuses Company first', async () => {
  render(<App />)
  await screen.findByText('Signed in as dev@kthais.com')
  fireEvent.click(screen.getByRole('button', { name: 'New Lead' }))

  expect(screen.getByLabelText('Company')).toHaveFocus()
})

test('nudges update when the tab becomes visible again', () => {
  vi.useFakeTimers({ toFake: ['Date', 'setInterval', 'clearInterval'] })
  render(<App />)
  expect(card('Vinga Voice')).not.toHaveTextContent('overdue')

  vi.setSystemTime(Date.now() + 2 * 24 * 60 * 60 * 1000)
  act(() => {
    document.dispatchEvent(new Event('visibilitychange'))
  })

  expect(card('Vinga Voice')).toHaveTextContent('Next step overdue: 2 days')
})
