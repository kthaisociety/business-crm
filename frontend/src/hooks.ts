import { useEffect, useRef, useState, type RefObject } from 'react'

const FOCUSABLE =
  'a[href], button:not([disabled]), input:not([disabled]), select:not([disabled]), textarea:not([disabled]), [tabindex]:not([tabindex="-1"])'

// Makes `ref` behave as a modal dialog: focus moves into it (to its [data-autofocus] field, if any),
// Tab and Shift+Tab stay inside it, Escape calls onClose, and focus returns to whatever opened it.
export function useModal(ref: RefObject<HTMLElement | null>, onClose: () => void) {
  // Callers pass a new onClose each render; reading it from a ref keeps the effect (and the opener it
  // remembers) from resetting on every render.
  const close = useRef(onClose)
  useEffect(() => {
    close.current = onClose
  })

  // What had focus when the dialog first rendered, to return focus to on close.
  const [opener] = useState(() => document.activeElement as HTMLElement | null)

  useEffect(() => {
    const dialog = ref.current
    ;(dialog?.querySelector<HTMLElement>('[data-autofocus]') ?? dialog)?.focus()

    const onKey = (e: KeyboardEvent) => {
      if (e.key === 'Escape') {
        close.current()
        return
      }
      if (e.key !== 'Tab' || !dialog) return
      const items = [...dialog.querySelectorAll<HTMLElement>(FOCUSABLE)]
      if (items.length === 0) return
      const first = items[0]
      const last = items[items.length - 1]
      const active = document.activeElement
      if (!dialog.contains(active)) {
        e.preventDefault()
        first.focus()
      } else if (e.shiftKey && (active === first || active === dialog)) {
        e.preventDefault()
        last.focus()
      } else if (!e.shiftKey && active === last) {
        e.preventDefault()
        first.focus()
      }
    }
    document.addEventListener('keydown', onKey)
    return () => {
      document.removeEventListener('keydown', onKey)
      if (opener?.isConnected) opener.focus()
    }
  }, [ref, opener])
}

// Re-renders every minute and whenever the tab becomes visible again, so anything computed from the
// current time (follow-up nudges, "days ago") stays current on a board left open overnight.
export function useMinuteTick() {
  const [, setTick] = useState(0)
  useEffect(() => {
    const tick = () => setTick((n) => n + 1)
    const onVisible = () => {
      if (document.visibilityState === 'visible') tick()
    }
    const timer = window.setInterval(tick, 60_000)
    document.addEventListener('visibilitychange', onVisible)
    return () => {
      window.clearInterval(timer)
      document.removeEventListener('visibilitychange', onVisible)
    }
  }, [])
}

// A field's text while it's being edited, saved only when editing ends. It follows `value` whenever that
// changes from outside, e.g. when a failed save puts the old value back.
export function useDraft(value: string): [string, (draft: string) => void] {
  const [draft, setDraft] = useState(value)
  const [seen, setSeen] = useState(value)
  if (value !== seen) {
    setSeen(value)
    setDraft(value)
  }
  return [draft, setDraft]
}
