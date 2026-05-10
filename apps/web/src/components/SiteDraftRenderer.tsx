import type { PublishedSnapshot, SiteDraft } from '@/lib/api'
import { Button } from '@/components/ui/button'
import { buildSiteThemeStyle } from '@/lib/site-theme'
import { preview, text } from '@/lib/styles'
import { cn } from '@/lib/utils'

type RenderableSite = Pick<SiteDraft, 'theme' | 'navigation' | 'pages'> & {
  site: {
    name: string
    seo?: {
      description?: string
    }
  }
}

type RoutablePage = {
  id: string
  slug: string
}

export function SiteDraftRenderer({
  site,
  eyebrow = 'Site render',
  showPageMeta = true,
  selectedPageId,
  linkMode = 'anchors',
  siteSlug,
}: {
  site: SiteDraft | PublishedSnapshot | RenderableSite
  eyebrow?: string
  showPageMeta?: boolean
  selectedPageId?: string
  linkMode?: 'anchors' | 'published'
  siteSlug?: string
}) {
  const renderedPages = selectedPageId
    ? site.pages.filter((page) => page.id === selectedPageId)
    : site.pages
  const pageAnchors = new Map(
    site.pages.map((page) => [page.id, pageAnchor(page.slug, page.id)]),
  )
  const pageById = new Map(site.pages.map((page) => [page.id, page]))
  const slugToPage = new Map(site.pages.map((page) => [page.slug, page]))

  return (
    <div className={preview.shell} style={buildSiteThemeStyle(site.theme)}>
      <header className={cn(preview.frame, preview.header)}>
        <div>
          <p className={text.eyebrow}>{eyebrow}</p>
          <h1 className="max-w-[10ch] font-serif text-[clamp(2.8rem,7vw,5.8rem)] font-bold leading-[0.96] text-[var(--site-foreground)]">
            {site.site.name}
          </h1>
          {site.site.seo?.description ? (
            <p className="text-[color-mix(in_oklch,var(--site-foreground)_82%,var(--site-background))]">{site.site.seo.description}</p>
          ) : null}
        </div>
        <nav className={preview.nav} aria-label="Site navigation">
          {site.navigation.primary.map((item) => (
            <a
              key={`${item.label}-${item.pageId ?? item.href ?? ''}`}
              className={preview.navLink}
              href={resolveNavigationHref(
                item,
                pageAnchors,
                pageById,
                slugToPage,
                linkMode,
                siteSlug,
              )}
            >
              {item.label}
            </a>
          ))}
        </nav>
      </header>

      {renderedPages.map((page) => (
        <article key={page.id} id={pageAnchor(page.slug, page.id)} className={preview.page}>
          {showPageMeta ? (
            <div className={preview.pageMeta}>
              <span>{page.title}</span>
              <small className="text-[color-mix(in_oklch,var(--site-foreground)_62%,var(--site-background))]">{page.slug}</small>
            </div>
          ) : null}
          <div className={preview.pageStack}>
            {page.blocks
              .filter((block) => !block.settings?.hidden)
              .map((block) => {
                switch (block.type) {
                  case 'hero':
                    return (
                      <HeroBlock
                        key={block.id}
                        props={block.props}
                        resolveHref={(href) =>
                          resolvePageHref(href, slugToPage, linkMode, siteSlug)
                        }
                      />
                    )
                  case 'text_section':
                    return <TextSectionBlock key={block.id} props={block.props} />
                  case 'features_grid':
                    return <FeaturesGridBlock key={block.id} props={block.props} />
                  case 'cta_band':
                    return (
                      <CTABandBlock
                        key={block.id}
                        props={block.props}
                        resolveHref={(href) =>
                          resolvePageHref(href, slugToPage, linkMode, siteSlug)
                        }
                      />
                    )
                  case 'image_text':
                    return <ImageTextBlock key={block.id} props={block.props} />
                  default:
                    return (
                      <section key={block.id} className={preview.panel}>
                        <p className={text.eyebrow}>Unsupported block</p>
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

function HeroBlock({
  props,
  resolveHref,
}: {
  props: Record<string, unknown>
  resolveHref: (href: string) => string
}) {
  const primary = asObject(props.primaryCta)
  return (
    <section className={cn(preview.panel, preview.hero)}>
      {asText(props.eyebrow) ? <p className={text.eyebrow}>{asText(props.eyebrow)}</p> : null}
      <h2 className="max-w-[12ch] font-serif text-[clamp(2rem,4vw,3.2rem)] font-bold leading-[0.96] text-[var(--site-foreground)]">{asText(props.headline)}</h2>
      {asText(props.subheadline) ? <p className="text-[color-mix(in_oklch,var(--site-foreground)_82%,var(--site-background))]">{asText(props.subheadline)}</p> : null}
      {primary ? (
        <div className={preview.actionRow}>
          <Button asChild variant="plain" className={preview.button}>
            <a href={resolveHref(asText(primary.href) || '#')}>
              {asText(primary.label) ?? 'Continue'}
            </a>
          </Button>
        </div>
      ) : null}
    </section>
  )
}

function TextSectionBlock({ props }: { props: Record<string, unknown> }) {
  return (
    <section className={preview.panel}>
      <h3 className="font-serif text-[1.6rem] font-bold leading-[0.96] text-[var(--site-foreground)]">{asText(props.heading)}</h3>
      <p className="text-[color-mix(in_oklch,var(--site-foreground)_82%,var(--site-background))]">{asText(props.body)}</p>
    </section>
  )
}

function FeaturesGridBlock({ props }: { props: Record<string, unknown> }) {
  return (
    <section className={preview.panel}>
      <div className={preview.sectionHeading}>
        <h3 className="font-serif text-[1.6rem] font-bold leading-[0.96] text-[var(--site-foreground)]">{asText(props.heading)}</h3>
        {asText(props.intro) ? <p className="text-[color-mix(in_oklch,var(--site-foreground)_82%,var(--site-background))]">{asText(props.intro)}</p> : null}
      </div>
      <div className={preview.features}>
        {asArray(props.items).map((item, index) => {
          const value = asObject(item)
          return (
            <article key={index} className={preview.feature}>
              <h4 className="mb-2.5 font-serif text-[1.15rem] font-bold leading-[0.96] text-[var(--site-foreground)]">{asText(value?.title)}</h4>
              <p className="text-[color-mix(in_oklch,var(--site-foreground)_82%,var(--site-background))]">{asText(value?.body)}</p>
            </article>
          )
        })}
      </div>
    </section>
  )
}

function CTABandBlock({
  props,
  resolveHref,
}: {
  props: Record<string, unknown>
  resolveHref: (href: string) => string
}) {
  const cta = asObject(props.cta)
  return (
    <section className={cn(preview.panel, preview.actionRow)}>
      <div>
        <h3 className="font-serif text-[1.6rem] font-bold leading-[0.96] text-[var(--site-foreground)]">{asText(props.heading)}</h3>
        <p className="text-[color-mix(in_oklch,var(--site-foreground)_82%,var(--site-background))]">{asText(props.body)}</p>
      </div>
      {cta ? (
        <Button
          asChild
          variant="plain"
          className={cn(preview.button, preview.ghostButton)}
        >
          <a href={resolveHref(asText(cta.href) || '#')}>
            {asText(cta.label) ?? 'Open'}
          </a>
        </Button>
      ) : null}
    </section>
  )
}

function ImageTextBlock({ props }: { props: Record<string, unknown> }) {
  return (
    <section className={cn(preview.panel, preview.split)}>
      <div>
        <h3 className="font-serif text-[1.6rem] font-bold leading-[0.96] text-[var(--site-foreground)]">{asText(props.heading)}</h3>
        <p className="text-[color-mix(in_oklch,var(--site-foreground)_82%,var(--site-background))]">{asText(props.body)}</p>
      </div>
      <div className={preview.imagePlaceholder}>
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

function resolveNavigationHref(
  item: { pageId?: string; href?: string },
  pageAnchors: Map<string, string>,
  pageById: Map<string, RoutablePage>,
  slugToPage: Map<string, RoutablePage>,
  linkMode: 'anchors' | 'published',
  siteSlug?: string,
) {
  if (item.pageId && pageAnchors.has(item.pageId)) {
    if (linkMode === 'published' && siteSlug) {
      const page = pageById.get(item.pageId)
      if (page) {
        return buildPublishedPageHref(siteSlug, page.slug)
      }
    }
    return `#${pageAnchors.get(item.pageId)}`
  }
  return resolvePageHref(item.href ?? '#', slugToPage, linkMode, siteSlug)
}

function resolvePageHref(
  href: string,
  slugToPage: Map<string, RoutablePage>,
  linkMode: 'anchors' | 'published',
  siteSlug?: string,
) {
  if (!href.startsWith('/')) {
    return href
  }
  const page = slugToPage.get(href)
  if (!page) {
    return href
  }
  if (linkMode === 'published' && siteSlug) {
    return buildPublishedPageHref(siteSlug, page.slug)
  }
  return `#${pageAnchor(page.slug, page.id)}`
}

function buildPublishedPageHref(siteSlug: string, pageSlug: string) {
  if (pageSlug === '/') {
    return `/public/${siteSlug}`
  }
  return `/public/${siteSlug}${pageSlug}`
}

function pageAnchor(slug: string, pageId: string) {
  if (slug === '/') {
    return 'page-home'
  }
  const cleaned = slug
    .replaceAll('/', '-')
    .replace(/[^a-zA-Z0-9_-]/g, '')
    .replace(/^-+/, '')
  if (!cleaned) {
    return `page-${pageId}`
  }
  return `page-${cleaned}`
}
