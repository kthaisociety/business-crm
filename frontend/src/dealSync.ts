import type { Deal } from './data/deals'

// Deal edits show at once and save in the background, while refreshes and save responses keep bringing
// the server's version of deals. This tracks local edits field by field, so a server answer can be
// applied in full except for the fields edited after its request went out, or still being saved: those
// keep the local value. A refresh that was asked for before an edit can't undo it, and an edit can't hide
// changes made elsewhere, such as a renamed company or a contact's new email.
export class LocalEdits {
  private count = 0
  private editedAt = new Map<string, number>() // `${dealId}:${field}` → the edit that last set it
  private saving = new Map<string, number>() // same key → saves of it still in flight

  // A point in the edit history: take it when a request goes out, and pass it to merge with the answer.
  now(): number {
    return this.count
  }

  // Records a local edit to these fields of a deal, and returns the edit's number.
  edit(dealId: string, fields: readonly string[]): number {
    this.count++
    for (const field of fields) this.editedAt.set(key(dealId, field), this.count)
    return this.count
  }

  startSaving(dealId: string, fields: readonly string[]): void {
    for (const field of fields) {
      const k = key(dealId, field)
      this.saving.set(k, (this.saving.get(k) ?? 0) + 1)
    }
  }

  doneSaving(dealId: string, fields: readonly string[]): void {
    for (const field of fields) {
      const k = key(dealId, field)
      const left = (this.saving.get(k) ?? 1) - 1
      if (left > 0) this.saving.set(k, left)
      else this.saving.delete(k)
    }
  }

  // Whether the edit numbered `edit` is still the latest to this field, with nothing else saving it.
  isLatest(dealId: string, field: string, edit: number): boolean {
    const k = key(dealId, field)
    return this.editedAt.get(k) === edit && !this.saving.has(k)
  }

  // The server's deal, as of `takenAt`, with the local value kept for each field edited since or still
  // saving.
  merge(server: Deal, local: Deal | undefined, takenAt: number): Deal {
    if (!local) return server
    const merged = { ...server }
    for (const field of Object.keys(local) as (keyof Deal)[]) {
      const k = key(server.id, field)
      if ((this.editedAt.get(k) ?? 0) > takenAt || this.saving.has(k)) Object.assign(merged, { [field]: local[field] })
    }
    return merged
  }
}

const key = (dealId: string, field: string) => `${dealId}:${field}`
