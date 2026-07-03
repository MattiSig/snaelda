import { createServerFn } from '@tanstack/react-start'
import { useMatches, useRouterState } from '@tanstack/react-router'
import { useEffect, useSyncExternalStore } from 'react'
import { DEFAULT_LOCALE, LOCALES, type Locale } from '@/lib/i18n'

export { DEFAULT_LOCALE } from '@/lib/i18n'
export type { Locale } from '@/lib/i18n'

/** Persisted visitor choice; survives across sessions and navigations. */
export const LOCALE_STORAGE_KEY = 'snaelda-locale'

/** `og:locale` value for each supported content locale (`sv_SE` reserved). */
const OG_LOCALES: Record<Locale, string> = {
  is: 'is_IS',
  en: 'en_US',
}

export function ogLocale(locale: Locale): string {
  return OG_LOCALES[locale] ?? OG_LOCALES[DEFAULT_LOCALE]
}

export function isLocale(value: unknown): value is Locale {
  return (
    typeof value === 'string' && (LOCALES as readonly string[]).includes(value)
  )
}

/** Narrow an unknown value to a supported locale, or `null` if unsupported. */
export function coerceLocale(value: unknown): Locale | null {
  return isLocale(value) ? value : null
}

/**
 * Pick the best supported locale from an `Accept-Language` (or `navigator.language`)
 * value, honoring quality weights and matching on the primary subtag so `is-IS`
 * resolves to `is`.
 */
export function localeFromAcceptLanguage(
  header: string | null | undefined,
): Locale | null {
  if (!header) {
    return null
  }

  const ranked = header
    .split(',')
    .map((part) => {
      const [tag, ...params] = part.trim().split(';')
      const quality = params
        .map((p) => p.trim())
        .find((p) => p.startsWith('q='))
      const q = quality ? Number.parseFloat(quality.slice(2)) : 1
      return { tag: tag.trim().toLowerCase(), q: Number.isNaN(q) ? 0 : q }
    })
    .filter((entry) => entry.tag !== '')
    .sort((a, b) => b.q - a.q)

  for (const { tag } of ranked) {
    const primary = tag.split('-')[0]
    if (isLocale(primary)) {
      return primary
    }
  }
  return null
}

/**
 * Shared resolution order for the marketing/demo surface (Spec 22):
 * explicit `?lang` → stored choice → `Accept-Language` → `is` default.
 */
export function resolveLocale(input: {
  param?: unknown
  stored?: unknown
  acceptLanguage?: string | null
}): Locale {
  return (
    coerceLocale(input.param) ??
    coerceLocale(input.stored) ??
    localeFromAcceptLanguage(input.acceptLanguage) ??
    DEFAULT_LOCALE
  )
}

/**
 * SSR-safe resolver used by route loaders so `<html lang>` and `og:locale` are
 * correct on the first byte. Reads the `?lang` override and the request's
 * `Accept-Language` header; `localStorage` is layered in on the client by
 * `useLocale`.
 */
export const resolveRequestLocale = createServerFn({ method: 'GET' }).handler(
  async (): Promise<Locale> => {
    const { getRequestHeader, getRequestUrl } = await import(
      '@tanstack/start-server-core'
    )
    const url = getRequestUrl({ xForwardedHost: true, xForwardedProto: true })
    return resolveLocale({
      param: url.searchParams.get('lang'),
      acceptLanguage: getRequestHeader('accept-language') ?? null,
    })
  },
)

/**
 * Reads the SSR-resolved locale that a loader stashed on its data, so the first
 * client render matches the server and there is no flash. Defaults to `is`.
 */
function useSsrLocale(): Locale {
  const matches = useMatches()
  const resolved = matches
    .map((match) => {
      const data = match.loaderData as { locale?: unknown } | undefined
      return coerceLocale(data?.locale)
    })
    .find((value): value is Locale => value !== null)
  return resolved ?? DEFAULT_LOCALE
}

/** Client-only signal: persisted choice, then browser language. `null` on the server. */
function getStoredLocaleSnapshot(): Locale | null {
  const stored = coerceLocale(window.localStorage.getItem(LOCALE_STORAGE_KEY))
  return stored ?? localeFromAcceptLanguage(window.navigator.language)
}

function subscribeStoredLocale(onChange: () => void): () => void {
  window.addEventListener('storage', onChange)
  return () => window.removeEventListener('storage', onChange)
}

/**
 * Resolve the active marketing-surface locale. Seeds from the SSR value to avoid
 * a hydration flash, then layers in the client-only signals (`?lang`, stored
 * choice, `navigator.language`) once mounted. An explicit `?lang` is persisted.
 */
export function useLocale(): Locale {
  const ssrLocale = useSsrLocale()
  const param = useRouterState({
    select: (state) => {
      const search = state.location.search as { lang?: unknown }
      return coerceLocale(search?.lang)
    },
  })
  // `useSyncExternalStore` reads localStorage/navigator only on the client
  // (server snapshot is `null`), so hydration matches the SSR-resolved locale.
  const stored = useSyncExternalStore(
    subscribeStoredLocale,
    getStoredLocaleSnapshot,
    () => null,
  )

  useEffect(() => {
    if (param) {
      window.localStorage.setItem(LOCALE_STORAGE_KEY, param)
    }
  }, [param])

  return param ?? stored ?? ssrLocale
}
