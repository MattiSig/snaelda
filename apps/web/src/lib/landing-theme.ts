import type { CSSProperties } from 'react'

// The warm near-black, ribbon-accented palette shared by the public marketing
// surfaces (landing + re-spin demo). Kept in one place so the two entry points
// into the funnel feel like the same product (BRANDING.md dark-mode direction).
export const landingTheme = {
  backgroundColor: '#131411',
  color: '#e4e2dd',
  '--background': '#131411',
  '--surface-0': '#131411',
  '--surface-1': '#1f201d',
  '--surface-2': '#2a2a27',
  '--surface-3': '#343532',
  '--paper': '#e4e2dd',
  '--paper-muted': '#cfc3ca',
  '--ink': '#131411',
  '--border': '#4c454a',
  '--thread-mauve': '#dabed6',
  '--thread-gold': '#f4a261',
  '--thread-teal': '#6fd8c8',
  '--thread-coral': '#f07a98',
  '--thread-violet': '#b58ad0',
} as CSSProperties
