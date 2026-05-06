import type { CSSProperties } from 'react'
import type { SiteDraft } from '@/lib/api'

export function SiteDraftRenderer({ draft }: { draft: SiteDraft }) {
  const colors = draft.theme.tokens.colors ?? {}
  const style = {
    '--site-background': colors.background ?? '#151215',
    '--site-foreground': colors.foreground ?? colors.text ?? '#f6f2ec',
    '--site-surface': colors.surface ?? '#231c24',
    '--site-surface-muted': colors.surfaceMuted ?? '#312736',
    '--site-primary': colors.primary ?? '#8fc6ff',
    '--site-secondary': colors.secondary ?? '#8ee2d1',
    '--site-accent': colors.accent ?? '#ff8cad',
    '--site-border': colors.border ?? '#58415b',
    '--site-muted': colors.muted ?? '#b78656',
    '--site-ring': colors.ring ?? '#f3b547',
  } as CSSProperties

  return (
    <div className="site-preview" style={style}>
      <header className="site-preview__header">
        <div>
          <p className="eyebrow">Draft preview</p>
          <h1>{draft.site.name}</h1>
          {draft.site.seo?.description ? <p>{draft.site.seo.description}</p> : null}
        </div>
        <nav className="site-preview__nav" aria-label="Site navigation">
          {draft.navigation.primary.map((item) => (
            <a
              key={`${item.label}-${item.pageId ?? item.href ?? ''}`}
              href={item.href ?? '#'}
            >
              {item.label}
            </a>
          ))}
        </nav>
      </header>

      {draft.pages.map((page) => (
        <article key={page.id} className="site-preview__page">
          <div className="site-preview__page-meta">
            <span>{page.title}</span>
            <small>{page.slug}</small>
          </div>
          <div className="site-preview__page-stack">
            {page.blocks
              .filter((block) => !block.settings?.hidden)
              .map((block) => {
                switch (block.type) {
                  case 'hero':
                    return <HeroBlock key={block.id} props={block.props} />
                  case 'text_section':
                    return <TextSectionBlock key={block.id} props={block.props} />
                  case 'features_grid':
                    return <FeaturesGridBlock key={block.id} props={block.props} />
                  case 'cta_band':
                    return <CTABandBlock key={block.id} props={block.props} />
                  case 'image_text':
                    return <ImageTextBlock key={block.id} props={block.props} />
                  default:
                    return (
                      <section key={block.id} className="site-preview__panel">
                        <p className="eyebrow">Unsupported block</p>
                        <strong>{block.type}</strong>
                      </section>
                    )
                }
              })}
          </div>
        </article>
      ))}
    </div>
  )
}

function HeroBlock({ props }: { props: Record<string, unknown> }) {
  const primary = asObject(props.primaryCta)
  return (
    <section className="site-preview__hero site-preview__panel">
      {asText(props.eyebrow) ? <p className="eyebrow">{asText(props.eyebrow)}</p> : null}
      <h2>{asText(props.headline)}</h2>
      {asText(props.subheadline) ? <p>{asText(props.subheadline)}</p> : null}
      {primary ? (
        <div className="site-preview__actions">
          <a className="site-preview__button" href={asText(primary.href) ?? '#'}>
            {asText(primary.label) ?? 'Continue'}
          </a>
        </div>
      ) : null}
    </section>
  )
}

function TextSectionBlock({ props }: { props: Record<string, unknown> }) {
  return (
    <section className="site-preview__panel site-preview__copy">
      <h3>{asText(props.heading)}</h3>
      <p>{asText(props.body)}</p>
    </section>
  )
}

function FeaturesGridBlock({ props }: { props: Record<string, unknown> }) {
  return (
    <section className="site-preview__panel">
      <div className="site-preview__section-heading">
        <h3>{asText(props.heading)}</h3>
        {asText(props.intro) ? <p>{asText(props.intro)}</p> : null}
      </div>
      <div className="site-preview__features">
        {asArray(props.items).map((item, index) => {
          const value = asObject(item)
          return (
            <article key={index} className="site-preview__feature">
              <h4>{asText(value?.title)}</h4>
              <p>{asText(value?.body)}</p>
            </article>
          )
        })}
      </div>
    </section>
  )
}

function CTABandBlock({ props }: { props: Record<string, unknown> }) {
  const cta = asObject(props.cta)
  return (
    <section className="site-preview__panel site-preview__cta">
      <div>
        <h3>{asText(props.heading)}</h3>
        <p>{asText(props.body)}</p>
      </div>
      {cta ? (
        <a className="site-preview__button site-preview__button--ghost" href={asText(cta.href) ?? '#'}>
          {asText(cta.label) ?? 'Open'}
        </a>
      ) : null}
    </section>
  )
}

function ImageTextBlock({ props }: { props: Record<string, unknown> }) {
  return (
    <section className="site-preview__panel site-preview__split">
      <div>
        <h3>{asText(props.heading)}</h3>
        <p>{asText(props.body)}</p>
      </div>
      <div className="site-preview__image-placeholder">
        <span>Image slot</span>
      </div>
    </section>
  )
}

function asText(value: unknown) {
  return typeof value === 'string' ? value : ''
}

function asArray(value: unknown) {
  return Array.isArray(value) ? value : []
}

function asObject(value: unknown) {
  if (!value || typeof value !== 'object' || Array.isArray(value)) {
    return null
  }
  return value as Record<string, unknown>
}
