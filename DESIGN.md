---
name: Snaelda
description: A warm, crafted prompt-to-site builder for small owner-led businesses.
colors:
  antique-paper: "#f9f7f2"
  paper-card: "#fdfbf7"
  warm-pebble: "#e5e0d8"
  woven-muted: "#ece6de"
  soft-border: "#d8cfc8"
  ink-plum: "#30312e"
  muted-plum: "#6c6661"
  root-ink: "#2f1400"
  saffron-thread: "#f4a261"
  saffron-hover: "#eca068"
  teal-thread: "#2a9d8f"
  thread-mauve: "#dabed6"
  berry-red: "#b6534f"
  berry-hover: "#c15c57"
  sky-ribbon: "#6ca9e8"
  coral-ribbon: "#e76f51"
  wood-thread: "#b78656"
  night-workshop: "#131411"
  charcoal-moss: "#1f201d"
  dark-raised: "#2a2a27"
  dark-field: "#343532"
  warm-paper-text: "#e4e2dd"
  dark-paper-muted: "#cfc3ca"
  dark-border: "#4c454a"
  dark-saffron: "#ffb780"
  dark-saffron-hover: "#fca967"
  dark-teal: "#6fd8c8"
  dark-danger: "#ffb4ab"
typography:
  display:
    fontFamily: "Literata, Iowan Old Style, Palatino Linotype, serif"
    fontSize: "clamp(3rem, 8vw, 5.6rem)"
    fontWeight: 600
    lineHeight: 0.95
    letterSpacing: "-0.025em"
  headline:
    fontFamily: "Be Vietnam Pro, Avenir Next, Segoe UI, sans-serif"
    fontSize: "clamp(1.75rem, 3.4vw, 3rem)"
    fontWeight: 900
    lineHeight: 0.96
  title:
    fontFamily: "Be Vietnam Pro, Avenir Next, Segoe UI, sans-serif"
    fontSize: "1.35rem"
    fontWeight: 900
    lineHeight: 1.08
  body:
    fontFamily: "Be Vietnam Pro, Avenir Next, Segoe UI, sans-serif"
    fontSize: "1rem"
    fontWeight: 400
    lineHeight: 1.5
  label:
    fontFamily: "Be Vietnam Pro, Avenir Next, Segoe UI, sans-serif"
    fontSize: "0.75rem"
    fontWeight: 700
    lineHeight: 1.2
    letterSpacing: "0.1em"
rounded:
  checkbox: "6px"
  control-sm: "10px"
  control: "12px"
  field: "14px"
  panel: "16px"
  hero: "18px"
  modal: "20px"
  pill: "999px"
spacing:
  xs: "4px"
  sm: "8px"
  md: "12px"
  lg: "16px"
  xl: "20px"
  "2xl": "24px"
  "3xl": "32px"
components:
  button-primary:
    backgroundColor: "{colors.saffron-thread}"
    textColor: "{colors.root-ink}"
    rounded: "{rounded.pill}"
    padding: "10px 16px"
    height: "44px"
    typography: "{typography.label}"
  button-primary-dark:
    backgroundColor: "{colors.dark-saffron}"
    textColor: "{colors.root-ink}"
    rounded: "{rounded.pill}"
    padding: "10px 16px"
    height: "44px"
  button-outline:
    backgroundColor: "{colors.woven-muted}"
    textColor: "{colors.ink-plum}"
    rounded: "{rounded.pill}"
    padding: "10px 16px"
    height: "44px"
  input-field:
    backgroundColor: "{colors.woven-muted}"
    textColor: "{colors.ink-plum}"
    rounded: "{rounded.field}"
    padding: "12px 16px"
    height: "48px"
  panel:
    backgroundColor: "{colors.paper-card}"
    textColor: "{colors.ink-plum}"
    rounded: "{rounded.field}"
    padding: "24px"
  nav-pill:
    backgroundColor: "{colors.woven-muted}"
    textColor: "{colors.ink-plum}"
    rounded: "{rounded.pill}"
    padding: "8px 12px"
    height: "40px"
---

# Design System: Snaelda

## 1. Overview

**Creative North Star: "The Threaded Workshop"**

Snaelda's interface should feel like a small, capable workshop where a useful website is being spun into shape. It is warm, tactile, and slightly odd in a good way, but the working surfaces stay predictable because users are trying to publish a real site, not admire a decorative brand world.

The system carries the logo's yarn and spindle motif through color, rounded shape, soft tonal layers, and ribbon-like accents. Light mode is creamy and low-stress. Dark mode is required and should feel like a warmer night workshop: plum-biased near-black surfaces, brighter saffron and teal threads, strong contrast, and no stark white.

This visual system rejects the product's stated anti-references: cold enterprise CMS, loud startup hype machine, blank generic SaaS dashboard, luxury design portfolio tool, corporate blue default styling, hard black-on-white starkness, heavy futurism, excessive gloss, generic card grids, and interfaces that make small website publishing feel complicated.

**Key Characteristics:**

- Warm product restraint: a useful builder first, with craft cues used for orientation and state.
- Saffron momentum: primary actions and publishing movement use saffron, not generic blue.
- Teal certainty: focus, success, active navigation, and healthy progress use teal.
- Mauve structure: selected, elevated, or threaded surfaces use mauve and ribbon tints.
- Soft geometry: rounded controls, pill actions, and layered panels carry the handmade feel.
- Required dark mode: dark surfaces must use warm near-black and plum bias, never flat neutral black.

## 2. Colors

The palette is a disciplined yarn basket: antique paper and plum ink hold the surface, saffron moves the user forward, teal confirms state, and mauve makes selected or elevated surfaces feel threaded.

### Primary

- **Saffron Thread** (`saffron-thread`): the main action and momentum color. Use it for primary buttons, generate/publish moments, progress emphasis, and important CTAs.
- **Dark Saffron** (`dark-saffron`): the dark-mode primary. It should be brighter and a little sharper than light saffron so it keeps its charge against Night Workshop.

### Secondary

- **Teal Thread** (`teal-thread`): focus, success, active route borders, completion states, and supportive emphasis. Teal is functional first, decorative second.
- **Dark Teal** (`dark-teal`): dark-mode focus and success. Use it for accessible focus rings and completed generation steps.

### Tertiary

- **Thread Mauve** (`thread-mauve`): selected states, focus glows, ribbon highlights, raised surface tinting, and gentle brand texture. It should do more structural work than blue.
- **Berry Red** (`berry-red`): destructive or critical emphasis only. It is not a general accent.

### Neutral

- **Antique Paper** (`antique-paper`): light-mode page background and calm workspace foundation.
- **Paper Card** (`paper-card`): light-mode raised card and popover surfaces.
- **Warm Pebble** (`warm-pebble`): secondary light surfaces, empty states, and soft containers.
- **Soft Border** (`soft-border`): light-mode dividers, fields, and panel borders.
- **Ink Plum** (`ink-plum`): primary text in light mode. It replaces hard black.
- **Muted Plum** (`muted-plum`): secondary text and helper copy in light mode.
- **Night Workshop** (`night-workshop`): dark-mode page background and landing foundation.
- **Charcoal Moss** (`charcoal-moss`): dark-mode cards and dense product panels.
- **Dark Raised** (`dark-raised`): dark-mode hover and nested surface layer.
- **Dark Field** (`dark-field`): dark-mode input field and high-density control surface.
- **Warm Paper Text** (`warm-paper-text`): dark-mode primary text. It replaces pure white.
- **Dark Border** (`dark-border`): dark-mode dividers and control strokes.

### Named Rules

**The No Corporate Blue Rule.** Blue may appear only as a logo ribbon or illustration cue. Product actions, selection, and focus must use saffron, teal, or mauve.

**The Two Active Threads Rule.** Any product screen should lead with one dominant action accent and one supporting state accent. Extra ribbon colors belong in illustration, onboarding detail, or tiny decorative moments.

**The Warm Dark Rule.** Dark mode is a warm, plum-biased workshop. Prohibited: pure black, stark white, cool gray dashboards, and neon-on-black futurism.

## 3. Typography

**Display Font:** Literata, with Iowan Old Style and Palatino Linotype fallbacks.
**Body Font:** Be Vietnam Pro, with Avenir Next, Segoe UI, and sans-serif fallbacks.
**Label/Mono Font:** Be Vietnam Pro. There is no separate mono voice in the current product UI.

**Character:** Be Vietnam Pro carries the product's speed and clarity. Literata is reserved for brand-forward moments, landing headlines, and generated-site editorial texture; it should not appear in dense UI labels, form controls, tables, or builder chrome.

### Hierarchy

- **Display** (600, `clamp(3rem, 8vw, 5.6rem)`, 0.95): landing-page hero copy and rare brand-forward public moments only.
- **Headline** (900, `clamp(1.75rem, 3.4vw, 3rem)`, 0.96): empty states, error pages, and major product section introductions.
- **Title** (900, `1.35rem`, 1.08): product panel headings, builder section titles, progress-card headings, and compact page titles.
- **Body** (400, `1rem`, 1.5): working copy, helper text, descriptions, and route content. Keep prose to about 65 to 68 characters where it is explanatory.
- **Label** (700, `0.75rem`, `0.1em`, uppercase): eyebrows, field labels, metadata, and step labels. Use sparingly so it feels like workshop labeling, not shouting.

### Named Rules

**The Literata Reserve Rule.** Literata is for moments that need brand warmth or public-site character. Never use it for buttons, dense builder labels, menus, or form controls.

**The Compact Tool Rule.** Product UI uses fixed, readable type sizes. Fluid display type is allowed on landing and public-site surfaces, not inside sidebars or working panels.

## 4. Elevation

Snaelda uses a hybrid of tonal layering and soft shadows. Panels are mostly separated by warm surface color, borders, and radius; shadows appear for menus, modals, landing prompt surfaces, inline editor controls, and high-priority floating UI. Depth should feel like paper and thread stacked on a table, not glass or glossy plastic.

### Shadow Vocabulary

- **Soft Workshop Shadow** (`--shadow-soft`): broad menu, popover, and elevated card shadow. Use for floating UI that must sit clearly above the page.
- **Tight Workshop Shadow** (`--shadow-tight`): smaller elevated panels and compact hover surfaces.
- **Primary Button Shadow** (`0 10px 24px oklch(7% 0.022 336 / 0.26)`): only for primary momentum actions.
- **Inline Editor Shadow** (`0 14px 36px oklch(8% 0.02 336 / 0.55)`): dark floating editing tools over generated-site content.
- **Modal Shadow** (`0 28px 100px oklch(7% 0.03 340 / 0.5)`): high-priority overlays such as new-site creation.

### Named Rules

**The Flat Until Floating Rule.** Resting product panels are primarily tonal and bordered. Use shadows when an element floats above content, opens over another surface, or responds to a primary interaction.

**The No Glass Default Rule.** Backdrop blur is allowed for sticky headers, overlays, and inline editing controls. Decorative glass cards are prohibited.

## 5. Components

### Buttons

Buttons are tactile, rounded, and direct. Primary actions feel like the strand that moves the work forward.

- **Shape:** pill buttons for standard actions (`999px`), compact square icon actions use rounded control corners (`10px` to `12px`).
- **Primary:** saffron background with root-ink text, bold label, icon gap, and a minimum touch target of `44px`.
- **Hover / Focus:** primary buttons lift by `1px` on hover and shift to the hover saffron token. Focus uses the global teal outline, never a blue browser default.
- **Secondary / Outline:** teal or warm surface backgrounds with border states. Outline buttons hover toward surface-3 and teal borders.
- **Ghost / Plain:** reserved for low-emphasis links, menus, and text-like actions. Do not make destructive actions look like quiet plain links unless there is strong surrounding context.

### Chips

Chips are small prompt and filter affordances, not decorative tags.

- **Style:** rounded pill, warm surface background, faint border, bold small text.
- **State:** selected chips use saffron tint or mauve tint with clear contrast. Hover may lift by `1px`, but chips should not become primary buttons.

### Cards / Containers

Containers should feel like layered paper and woven panels, not a repetitive SaaS card grid.

- **Corner Style:** standard panels use gently curved corners (`14px` to `16px`); feature or modal containers may use `18px` to `20px`.
- **Background:** light mode uses Paper Card and Warm Pebble; dark mode uses Charcoal Moss and Dark Raised.
- **Shadow Strategy:** resting panels rely on tonal layer plus border. Menus, overlays, and floating editor tools use the Elevation vocabulary.
- **Border:** borders are warm and low-contrast. Use teal, saffron, or mauve borders only for active, selected, focus, success, or warning states.
- **Internal Padding:** product panels use `24px` on desktop and `16px` on small screens.

### Inputs / Fields

Fields are quiet and substantial, with a tactile surface that invites editing.

- **Style:** `48px` minimum height, `14px` radius, warm surface background, soft border, `16px` horizontal padding.
- **Focus:** teal border plus the global focus outline. Focus also shifts the field background to the next surface layer.
- **Error / Disabled:** error text uses the destructive token and clear language. Disabled fields keep the same shape and reduce opacity to `50%`.

### Navigation

Navigation uses familiar product patterns because the builder is a working tool.

- **Style:** sticky top bar with warm translucent surface, subtle backdrop blur, and a single bottom border.
- **Primary nav links:** pill shape, bold small text, lucide icons where useful, muted by default, surface-2 active state.
- **Site actions:** preview, analytics, and edit use inline-link pill buttons. The active route earns a teal border and raised surface.
- **Mobile:** top-bar content can wrap or grid, but navigation shape and active states must stay consistent.

### Builder Panels

Builder panels are the core product surface.

- **Style:** bordered paper-like panels with `14px` to `16px` radius and dense but readable spacing.
- **AI progress:** generation steps use circular status marks, saffron for active work, teal for completed work, and skeletons instead of centered spinners.
- **Reprompt history:** checkpoint cards use mauve-tinted surface and clear action grouping. Keep revert and diff actions visibly secondary.
- **Inline editor tools:** floating dark controls sit over generated content with warm paper text, tiny uppercase labels, and shadows. They are allowed to feel sharper than the rest of the UI because they hover over another design.

### Public-Site Renderer

The generated site preview has its own theme variables but should still feel compatible with Snaelda.

- **Style:** full-width rendered pages, not framed inside decorative cards unless the route is explicitly a preview shell.
- **Buttons:** generated-site buttons use local public-site color variables, `var(--radius-inner)`, and a `1px` hover lift.
- **Layout:** content uses generous section padding and max widths around `1180px`. Public pages can be more editorial than the product builder.
- **Controls:** Snaelda chrome around previews remains product-styled and must not inherit arbitrary generated-site typography.

## 6. Do's and Don'ts

### Do:

- **Do** use Saffron Thread for the main action on a screen and Teal Thread for focus, success, and active state.
- **Do** keep dark mode warm and sharper: Night Workshop, Charcoal Moss, bright saffron, bright teal, and warm paper text.
- **Do** keep publishing close by making draft, preview, refine, publish, and domain actions easy to spot.
- **Do** use mauve as the structural ribbon for selected, elevated, and threaded states.
- **Do** use skeleton states for generation and loading inside content surfaces.
- **Do** keep panels dense enough for repeated work, with predictable navigation, visible focus states, and `44px` minimum touch targets.
- **Do** let the logo and yarn motif guide small details: ribbon tints, thread language, soft curves, and kinetic but calm motion.

### Don't:

- **Don't** make Snaelda feel like a cold enterprise CMS.
- **Don't** make the product a loud startup hype machine.
- **Don't** default to a blank generic SaaS dashboard.
- **Don't** make screens feel like a luxury design portfolio tool.
- **Don't** use corporate blue default styling for primary actions, active routes, or focus.
- **Don't** use hard black-on-white starkness, pure black, pure white, or cool gray neutrality.
- **Don't** use heavy futurism, excessive gloss, decorative glassmorphism, or neon-on-black styling.
- **Don't** build generic card grids as the default answer for every product section.
- **Don't** make small website publishing feel complicated through configuration theater, too many modes, or unclear status.
- **Don't** turn the brand into generic SaaS minimalism, overbrand every screen with loud color, make copy too cute to be clear, or make the UI feel childish.
- **Don't** use side-stripe borders, gradient text, repeated icon-heading-text card grids, or decorative motion that does not convey state.
