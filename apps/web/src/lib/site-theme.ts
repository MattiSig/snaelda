import type { CSSProperties } from 'react'
import type { SiteDraft } from '@/lib/api'

type ThemeConfig = SiteDraft['theme']

export function buildSiteThemeStyle(theme: ThemeConfig): CSSProperties {
  const colors = theme.tokens.colors ?? {}
  const typography = theme.tokens.typography ?? {}
  const layout = theme.tokens.layout ?? {}
  const shape = theme.tokens.shape ?? {}
  const radius = asString(shape.radius) || '28px'

  return {
    '--site-background': colors.background ?? '#191119',
    '--site-foreground': colors.foreground ?? colors.text ?? '#f3ead8',
    '--site-surface': colors.surface ?? '#241a24',
    '--site-surface-muted': colors.surfaceMuted ?? '#302333',
    '--site-primary': colors.primary ?? '#86d8cf',
    '--site-secondary': colors.secondary ?? '#89b9f0',
    '--site-accent': colors.accent ?? '#ff8a9d',
    '--site-border': colors.border ?? '#5a3e57',
    '--site-muted': colors.muted ?? '#caa778',
    '--site-ring': colors.ring ?? '#f2bd63',
    '--site-glow-primary': withAlpha(colors.primary ?? '#86d8cf', '1f'),
    '--site-glow-accent': withAlpha(colors.accent ?? '#ff8a9d', '1f'),
    '--site-font-heading': resolveFontStack(asString(typography.headingFont)),
    '--site-font-body': resolveFontStack(asString(typography.bodyFont)),
    '--site-section-spacing': asString(layout.sectionSpacing) || '96px',
    '--site-content-width': asString(layout.contentWidth) || '720px',
    '--site-radius-panel': radius,
    '--site-radius-inner': innerRadius(radius),
  } as CSSProperties
}

function asString(value: unknown) {
  return typeof value === 'string' ? value : ''
}

function resolveFontStack(value: string) {
  switch (value) {
    case 'Iowan Old Style':
      return '"Iowan Old Style", "Palatino Linotype", "Book Antiqua", Georgia, serif'
    case 'Avenir Next':
      return '"Avenir Next", "Segoe UI", "Helvetica Neue", sans-serif'
    default:
      return value || '"Avenir Next", "Segoe UI", "Helvetica Neue", sans-serif'
  }
}

function innerRadius(radius: string) {
  return `calc(${radius} - 6px)`
}

function withAlpha(color: string, alphaHex: string) {
  const normalized = color.trim()
  if (/^#[0-9a-fA-F]{6}$/.test(normalized)) {
    return `${normalized}${alphaHex}`
  }
  if (/^#[0-9a-fA-F]{3}$/.test(normalized)) {
    const expanded = normalized
      .slice(1)
      .split('')
      .map((part) => part + part)
      .join('')
    return `#${expanded}${alphaHex}`
  }
  return color
}
