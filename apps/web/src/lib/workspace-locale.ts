import { useSyncExternalStore } from 'react'
import { coerceLocale, type Locale } from '@/lib/locale'

// The authenticated workspace's locale, published so the root document can drive
// `<html lang>` for the builder surface. It is client-only: the `/app` shell
// fetches the session after hydration, so the server snapshot is always null and
// SSR falls back to the visitor locale until the session resolves.
let currentWorkspaceLocale: Locale | null = null
const listeners = new Set<() => void>()

export function setWorkspaceLocale(value: unknown): void {
  const next = coerceLocale(value)
  if (next === currentWorkspaceLocale) {
    return
  }
  currentWorkspaceLocale = next
  for (const listener of listeners) {
    listener()
  }
}

function subscribe(onChange: () => void): () => void {
  listeners.add(onChange)
  return () => {
    listeners.delete(onChange)
  }
}

export function useWorkspaceLocale(): Locale | null {
  return useSyncExternalStore(
    subscribe,
    () => currentWorkspaceLocale,
    () => null,
  )
}
