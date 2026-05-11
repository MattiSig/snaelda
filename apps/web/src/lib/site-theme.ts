import type { CSSProperties } from 'react'
import type { SiteDraft } from '@/lib/api'

type ThemeConfig = SiteDraft['theme']

export function buildSiteThemeStyle(theme: ThemeConfig): CSSProperties {
  const colors = theme.tokens.colors ?? {}
  const typography = theme.tokens.typography ?? {}
  const layout = theme.tokens.layout ?? {}
  const shape = theme.tokens.shape ?? {}
  const radius = asString(shape.radius) || '28px'
  const buttonStyle = asString(shape.buttonStyle) || 'ribbon-fill'
  const imageStyle = asString(shape.imageStyle) || 'woven-tint'
  const buttonVars = resolveButtonVars(buttonStyle, colors)
  const imageVars = resolveImageVars(imageStyle, colors)

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
    ...buttonVars,
    ...imageVars,
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

function resolveButtonVars(style: string, colors: Record<string, string>) {
  const background = colors.background ?? '#191119'
  const foreground = colors.foreground ?? colors.text ?? '#f3ead8'
  const primary = colors.primary ?? '#86d8cf'
  const surface = colors.surface ?? '#241a24'
  const surfaceMuted = colors.surfaceMuted ?? '#302333'
  const border = colors.border ?? '#5a3e57'

  switch (style) {
    case 'thread-outline':
      return {
        '--site-button-background': surface,
        '--site-button-foreground': primary,
        '--site-button-border': primary,
        '--site-button-shadow': 'none',
        '--site-button-ghost-background': surfaceMuted,
        '--site-button-ghost-foreground': foreground,
        '--site-button-ghost-border': border,
      }
    case 'ink-solid':
      return {
        '--site-button-background': foreground,
        '--site-button-foreground': background,
        '--site-button-border': foreground,
        '--site-button-shadow': `0 12px 24px ${withAlpha(foreground, '1f')}`,
        '--site-button-ghost-background': surfaceMuted,
        '--site-button-ghost-foreground': foreground,
        '--site-button-ghost-border': foreground,
      }
    default:
      return {
        '--site-button-background': primary,
        '--site-button-foreground': background,
        '--site-button-border': primary,
        '--site-button-shadow': `0 12px 24px ${withAlpha(primary, '2b')}`,
        '--site-button-ghost-background': surfaceMuted,
        '--site-button-ghost-foreground': foreground,
        '--site-button-ghost-border': border,
      }
  }
}

function resolveImageVars(style: string, colors: Record<string, string>) {
  const surface = colors.surface ?? '#241a24'
  const surfaceMuted = colors.surfaceMuted ?? '#302333'
  const primary = colors.primary ?? '#86d8cf'
  const secondary = colors.secondary ?? '#89b9f0'
  const accent = colors.accent ?? '#ff8a9d'
  const border = colors.border ?? '#5a3e57'
  const background = colors.background ?? '#191119'

  switch (style) {
    case 'soft-frame':
      return {
        '--site-image-background': surface,
        '--site-image-border': border,
        '--site-image-shadow': 'none',
        '--site-image-tall-background': `linear-gradient(180deg, ${surface} 0%, ${surfaceMuted} 100%)`,
        '--site-image-caption-background': withAlpha(background, '66'),
      }
    case 'paper-cut':
      return {
        '--site-image-background': `color-mix(in oklch, ${surface} 88%, ${accent})`,
        '--site-image-border': accent,
        '--site-image-shadow': `0 16px 32px ${withAlpha(accent, '18')}`,
        '--site-image-tall-background': `linear-gradient(155deg, color-mix(in oklch, ${surface} 84%, ${accent}) 0%, color-mix(in oklch, ${surfaceMuted} 88%, ${secondary}) 58%, color-mix(in oklch, ${surface} 90%, ${primary}) 100%)`,
        '--site-image-caption-background': withAlpha(surface, 'b8'),
      }
    default:
      return {
        '--site-image-background': `color-mix(in oklch, ${surface} 88%, ${secondary})`,
        '--site-image-border': border,
        '--site-image-shadow': `0 16px 32px ${withAlpha(primary, '18')}`,
        '--site-image-tall-background': `linear-gradient(160deg, color-mix(in oklch, ${surface} 84%, ${primary}) 0%, ${surfaceMuted} 55%, color-mix(in oklch, ${surface} 90%, ${accent}) 100%)`,
        '--site-image-caption-background': withAlpha(surface, 'b8'),
      }
  }
}
