import { describe, expect, it } from 'vitest'
import { buildSiteThemeStyle } from './site-theme'

describe('buildSiteThemeStyle', () => {
  it('maps button and image theme tokens into CSS variables', () => {
    const style = buildSiteThemeStyle({
      version: 'theme.v1',
      tokens: {
        colors: {
          background: '#151215',
          foreground: '#f3ead8',
          surface: '#231c24',
          surfaceMuted: '#302333',
          primary: '#8ee2d1',
          secondary: '#8fc6ff',
          accent: '#ff8cad',
          border: '#5a3e57',
          muted: '#caa778',
          ring: '#f3b547',
        },
        typography: {
          headingFont: 'Iowan Old Style',
          bodyFont: 'Avenir Next',
        },
        layout: {
          sectionSpacing: '96px',
          contentWidth: '720px',
        },
        shape: {
          radius: '28px',
          buttonStyle: 'ink-solid',
          imageStyle: 'paper-cut',
        },
      },
    })

    const cssVars = style as Record<string, string>

    expect(cssVars['--site-button-background']).toBe('#f3ead8')
    expect(cssVars['--site-button-foreground']).toBe('#151215')
    expect(cssVars['--site-image-border']).toBe('#ff8cad')
    expect(cssVars['--site-image-tall-background']).toContain('linear-gradient')
  })
})
