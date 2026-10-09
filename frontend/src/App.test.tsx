import { render, screen } from '@testing-library/react'
import { afterEach, expect, test, vi } from 'vitest'
import App from './App'

afterEach(() => {
  vi.unstubAllGlobals()
})

test('shows the signed-in user', async () => {
  vi.stubGlobal(
    'fetch',
    vi.fn().mockResolvedValue(
      new Response(JSON.stringify({ sub: 'dev', email: 'dev@kthais.com', name: 'Dev User' })),
    ),
  )

  render(<App />)

  expect(await screen.findByText('Signed in as dev@kthais.com')).toBeInTheDocument()
})

test('shows an error when no one is signed in', async () => {
  vi.stubGlobal('fetch', vi.fn().mockResolvedValue(new Response('', { status: 401 })))

  render(<App />)

  expect(await screen.findByText('Loading the signed-in user failed (401)')).toBeInTheDocument()
})
