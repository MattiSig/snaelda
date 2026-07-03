import { describe, expect, it } from 'vitest'
import {
  coerceLocale,
  localeFromAcceptLanguage,
  ogLocale,
  resolveLocale,
} from './locale'

describe('coerceLocale', () => {
  it('accepts supported locales and rejects everything else', () => {
    expect(coerceLocale('is')).toBe('is')
    expect(coerceLocale('en')).toBe('en')
    expect(coerceLocale('sv')).toBeNull()
    expect(coerceLocale(undefined)).toBeNull()
    expect(coerceLocale(42)).toBeNull()
  })
})

describe('localeFromAcceptLanguage', () => {
  it('matches on the primary subtag', () => {
    expect(localeFromAcceptLanguage('is-IS,is;q=0.9')).toBe('is')
    expect(localeFromAcceptLanguage('en-US,en;q=0.8')).toBe('en')
  })

  it('honors quality weights over listed order', () => {
    expect(localeFromAcceptLanguage('en;q=0.4, is;q=0.9')).toBe('is')
  })

  it('skips unsupported languages and returns null when none match', () => {
    expect(localeFromAcceptLanguage('fr-FR,de;q=0.7')).toBeNull()
    expect(localeFromAcceptLanguage('')).toBeNull()
    expect(localeFromAcceptLanguage(null)).toBeNull()
  })
})

describe('resolveLocale', () => {
  it('prefers the explicit param over every other signal', () => {
    expect(
      resolveLocale({ param: 'en', stored: 'is', acceptLanguage: 'is' }),
    ).toBe('en')
  })

  it('falls back through stored then Accept-Language', () => {
    expect(resolveLocale({ stored: 'en', acceptLanguage: 'is' })).toBe('en')
    expect(resolveLocale({ acceptLanguage: 'en-GB' })).toBe('en')
  })

  it('defaults to Icelandic for the Iceland phase', () => {
    expect(resolveLocale({})).toBe('is')
    expect(resolveLocale({ param: 'sv', acceptLanguage: 'fr' })).toBe('is')
  })
})

describe('ogLocale', () => {
  it('maps locales to Open Graph values', () => {
    expect(ogLocale('is')).toBe('is_IS')
    expect(ogLocale('en')).toBe('en_US')
  })
})
