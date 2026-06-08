import { describe, expect, it } from 'vitest'
import { buildSiteThemeFromSelection, buildSiteThemeStyle } from './site-theme'

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
          scale: 'expressive',
        },
        layout: {
          sectionPaddingX: '24px',
          sectionPaddingY: '96px',
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

    expect(cssVars['--font-headingWeight']).toBe('700')
    expect(cssVars['--size-heroHeading']).toContain('6.6rem')
    expect(cssVars['--color-buttonBackground']).toBe('#f3ead8')
    expect(cssVars['--color-buttonForeground']).toBe('#151215')
    expect(cssVars['--color-imageBorder']).toBe('#ff8cad')
    expect(cssVars['--image-tallBackground']).toContain('linear-gradient')
  })

  it('builds a live preview theme from a constrained selection', () => {
    const theme = buildSiteThemeFromSelection(
      {
        version: 'theme.v1',
        tokens: {
          colors: {
            background: '#f7f3ea',
            foreground: '#2c2721',
            primary: '#426b5c',
          },
          typography: {},
          layout: {},
          shape: {},
        },
      },
      {
        palette: 'after-hours',
        fontPreset: 'humanist',
        typeScale: 'compact',
        sectionSpacing: 'airy',
        contentWidth: 'wide',
        radius: 'sharp',
        buttonStyle: 'thread-outline',
        imageStyle: 'woven-tint',
      },
      {
        palettes: [
          {
            id: 'after-hours',
            label: 'After Hours',
            previewColors: {
              background: '#151314',
              foreground: '#f1e8d8',
              primary: '#d58f4f',
            },
          },
        ],
        fontPresets: [],
        typeScales: [],
        sectionSpacings: [],
        contentWidths: [],
        radii: [],
        buttonStyles: [],
        imageStyles: [],
      },
    )

    expect(theme.tokens.colors.background).toBe('#151314')
    expect(theme.tokens.typography.headingFont).toBe('Trebuchet MS')
    expect(theme.tokens.typography.scale).toBe('compact')
    expect(theme.tokens.layout.sectionPaddingY).toBe('120px')
    expect(theme.tokens.layout.contentWidth).toBe('860px')
    expect(theme.tokens.shape.radius).toBe('0px')
    expect(theme.tokens.shape.buttonStyle).toBe('thread-outline')

    const cssVars = buildSiteThemeStyle(theme) as Record<string, string>
    expect(cssVars['--radius-panel']).toBe('0px')
    expect(cssVars['--radius-inner']).toBe('0px')
    expect(cssVars['--font-heading']).toContain('Trebuchet MS')
  })
})
