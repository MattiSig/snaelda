import { describe, expect, it } from 'vitest'
import { LOCALES, t, translator } from './i18n'

describe('i18n catalogs', () => {
  it('exposes both Iceland-phase locales with is first', () => {
    expect(LOCALES).toEqual(['is', 'en'])
  })

  it('returns locale-specific copy for a shared key', () => {
    expect(t('en', 'landing.hero.titleAccent')).toBe('A real one.')
    expect(t('is', 'landing.hero.titleAccent')).toBe('Alvöru vefur.')
  })

  it('never leaks English into the Icelandic landing hero', () => {
    expect(t('is', 'landing.hero.subtitle')).not.toMatch(/[A-Za-z]+ing\b/)
    expect(t('is', 'landing.hero.subtitle')).toContain('Snælda')
  })

  it('binds a locale via translator', () => {
    const tr = translator('is')
    expect(tr('landing.nav.login')).toBe('Skrá inn')
  })
})
