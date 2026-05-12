import type { PublishedSnapshot, SiteDraft } from '@/lib/api'
import { Button } from '@/components/ui/button'
import { buildDraftAssetURL, buildPublishedAssetURL } from '@/lib/assets'
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
  publishedBasePath,
}: {
  site: SiteDraft | PublishedSnapshot | RenderableSite
  eyebrow?: string
  showPageMeta?: boolean
  selectedPageId?: string
  linkMode?: 'anchors' | 'published'
  siteSlug?: string
  publishedBasePath?: string
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
            <p className="text-[color-mix(in_oklch,var(--site-foreground)_82%,var(--site-background))]">
              {site.site.seo.description}
            </p>
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
                publishedBasePath,
              )}
            >
              {item.label}
            </a>
          ))}
        </nav>
      </header>

      {renderedPages.map((page) => (
        <article
          key={page.id}
          id={pageAnchor(page.slug, page.id)}
          className={preview.page}
        >
          {showPageMeta ? (
            <div className={preview.pageMeta}>
              <span>{page.title}</span>
              <small className="text-[color-mix(in_oklch,var(--site-foreground)_62%,var(--site-background))]">
                {page.slug}
              </small>
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
                          resolvePageHref(
                            href,
                            slugToPage,
                            linkMode,
                            siteSlug,
                            publishedBasePath,
                          )
                        }
                        linkMode={linkMode}
                        siteSlug={siteSlug}
                      />
                    )
                  case 'text_section':
                    return (
                      <TextSectionBlock key={block.id} props={block.props} />
                    )
                  case 'features_grid':
                    return (
                      <FeaturesGridBlock key={block.id} props={block.props} />
                    )
                  case 'cta_band':
                    return (
                      <CTABandBlock
                        key={block.id}
                        props={block.props}
                        resolveHref={(href) =>
                          resolvePageHref(
                            href,
                            slugToPage,
                            linkMode,
                            siteSlug,
                            publishedBasePath,
                          )
                        }
                      />
                    )
                  case 'image_text':
                    return (
                      <ImageTextBlock
                        key={block.id}
                        props={block.props}
                        linkMode={linkMode}
                        siteSlug={siteSlug}
                      />
                    )
                  case 'gallery':
                    return (
                      <GalleryBlock
                        key={block.id}
                        props={block.props}
                        linkMode={linkMode}
                        siteSlug={siteSlug}
                      />
                    )
                  case 'testimonials':
                    return (
                      <TestimonialsBlock
                        key={block.id}
                        props={block.props}
                        linkMode={linkMode}
                        siteSlug={siteSlug}
                      />
                    )
                  case 'pricing_packages':
                    return (
                      <PricingPackagesBlock
                        key={block.id}
                        props={block.props}
                        resolveHref={(href) =>
                          resolvePageHref(
                            href,
                            slugToPage,
                            linkMode,
                            siteSlug,
                            publishedBasePath,
                          )
                        }
                      />
                    )
                  case 'faq':
                    return <FAQBlock key={block.id} props={block.props} />
                  case 'team_profile_cards':
                    return (
                      <TeamProfileCardsBlock
                        key={block.id}
                        props={block.props}
                        resolveHref={(href) =>
                          resolvePageHref(
                            href,
                            slugToPage,
                            linkMode,
                            siteSlug,
                            publishedBasePath,
                          )
                        }
                        linkMode={linkMode}
                        siteSlug={siteSlug}
                      />
                    )
                  case 'footer':
                    return (
                      <FooterBlock
                        key={block.id}
                        props={block.props}
                        resolveHref={(href) =>
                          resolvePageHref(
                            href,
                            slugToPage,
                            linkMode,
                            siteSlug,
                            publishedBasePath,
                          )
                        }
                      />
                    )
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
  linkMode,
  siteSlug,
}: {
  props: Record<string, unknown>
  resolveHref: (href: string) => string
  linkMode: 'anchors' | 'published'
  siteSlug?: string
}) {
  const primary = asObject(props.primaryCta)
  const image = asImageRef(props.image)
  const layout = asText(props.layout) || 'centered'
  const hasImage = image !== null
  return (
    <section className={cn(preview.panel, preview.hero)}>
      <div
        className={cn(
          hasImage && layout !== 'centered'
            ? 'grid gap-6 lg:grid-cols-[minmax(0,1.05fr)_minmax(240px,0.95fr)] lg:items-center'
            : 'grid gap-6',
        )}
      >
        <div
          className={cn(
            'grid gap-4',
            hasImage && layout === 'split-right' && 'lg:order-1',
            hasImage && layout === 'split-left' && 'lg:order-2',
          )}
        >
          {asText(props.eyebrow) ? (
            <p className={text.eyebrow}>{asText(props.eyebrow)}</p>
          ) : null}
          <h2 className="max-w-[12ch] font-serif text-[clamp(2rem,4vw,3.2rem)] font-bold leading-[0.96] text-[var(--site-foreground)]">
            {asText(props.headline)}
          </h2>
          {asText(props.subheadline) ? (
            <p className="text-[color-mix(in_oklch,var(--site-foreground)_82%,var(--site-background))]">
              {asText(props.subheadline)}
            </p>
          ) : null}
          {primary ? (
            <div className={preview.actionRow}>
              <Button asChild variant="plain" className={preview.button}>
                <a href={resolveHref(asText(primary.href) || '#')}>
                  {asText(primary.label) ?? 'Continue'}
                </a>
              </Button>
            </div>
          ) : null}
        </div>
        {image ? (
          <AssetImage
            image={image}
            linkMode={linkMode}
            siteSlug={siteSlug}
            altFallback={asText(props.headline) || 'Hero image'}
            className={cn(
              'min-h-[240px] w-full rounded-[var(--site-radius-inner)] border border-[var(--site-image-border)] bg-[var(--site-image-background)] object-cover shadow-[var(--site-image-shadow)]',
              hasImage && layout === 'split-right' && 'lg:order-2',
              hasImage && layout === 'split-left' && 'lg:order-1',
            )}
          />
        ) : null}
      </div>
    </section>
  )
}

function TextSectionBlock({ props }: { props: Record<string, unknown> }) {
  return (
    <section className={preview.panel}>
      <h3 className="font-serif text-[1.6rem] font-bold leading-[0.96] text-[var(--site-foreground)]">
        {asText(props.heading)}
      </h3>
      <p className="text-[color-mix(in_oklch,var(--site-foreground)_82%,var(--site-background))]">
        {asText(props.body)}
      </p>
    </section>
  )
}

function FeaturesGridBlock({ props }: { props: Record<string, unknown> }) {
  return (
    <section className={preview.panel}>
      <div className={preview.sectionHeading}>
        <h3 className="font-serif text-[1.6rem] font-bold leading-[0.96] text-[var(--site-foreground)]">
          {asText(props.heading)}
        </h3>
        {asText(props.intro) ? (
          <p className="text-[color-mix(in_oklch,var(--site-foreground)_82%,var(--site-background))]">
            {asText(props.intro)}
          </p>
        ) : null}
      </div>
      <div className={preview.features}>
        {asArray(props.items).map((item, index) => {
          const value = asObject(item)
          return (
            <article key={index} className={preview.feature}>
              <h4 className="mb-2.5 font-serif text-[1.15rem] font-bold leading-[0.96] text-[var(--site-foreground)]">
                {asText(value?.title)}
              </h4>
              <p className="text-[color-mix(in_oklch,var(--site-foreground)_82%,var(--site-background))]">
                {asText(value?.body)}
              </p>
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
        <h3 className="font-serif text-[1.6rem] font-bold leading-[0.96] text-[var(--site-foreground)]">
          {asText(props.heading)}
        </h3>
        <p className="text-[color-mix(in_oklch,var(--site-foreground)_82%,var(--site-background))]">
          {asText(props.body)}
        </p>
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

function ImageTextBlock({
  props,
  linkMode,
  siteSlug,
}: {
  props: Record<string, unknown>
  linkMode: 'anchors' | 'published'
  siteSlug?: string
}) {
  const cta = asObject(props.cta)
  const image = asImageRef(props.image)
  const imagePosition = asText(props.imagePosition) || 'right'
  return (
    <section className={cn(preview.panel, preview.split)}>
      <div className={cn(imagePosition === 'left' && 'lg:order-2')}>
        <h3 className="font-serif text-[1.6rem] font-bold leading-[0.96] text-[var(--site-foreground)]">
          {asText(props.heading)}
        </h3>
        <p className="text-[color-mix(in_oklch,var(--site-foreground)_82%,var(--site-background))]">
          {asText(props.body)}
        </p>
        {cta ? (
          <div className="mt-4">
            <span className={preview.chip}>
              {asText(cta.label) || 'Open link'}
            </span>
          </div>
        ) : null}
      </div>
      {image ? (
        <AssetImage
          image={image}
          linkMode={linkMode}
          siteSlug={siteSlug}
          altFallback={asText(props.heading) || 'Supporting image'}
          className={cn(
            'min-h-[220px] w-full rounded-[var(--site-radius-inner)] border border-[var(--site-image-border)] bg-[var(--site-image-background)] object-cover shadow-[var(--site-image-shadow)]',
            imagePosition === 'left' && 'lg:order-1',
          )}
        />
      ) : (
        <div
          className={cn(
            preview.imagePlaceholder,
            imagePosition === 'left' && 'lg:order-1',
          )}
        >
          <span>Image slot</span>
        </div>
      )}
    </section>
  )
}

function GalleryBlock({
  props,
  linkMode,
  siteSlug,
}: {
  props: Record<string, unknown>
  linkMode: 'anchors' | 'published'
  siteSlug?: string
}) {
  const layout = asText(props.layout) || 'grid'
  const images = asArray(props.images)

  return (
    <section className={preview.panel}>
      <div className={preview.sectionHeading}>
        <h3 className="font-serif text-[1.6rem] font-bold leading-[0.96] text-[var(--site-foreground)]">
          {asText(props.heading)}
        </h3>
        {asText(props.intro) ? (
          <p className="text-[color-mix(in_oklch,var(--site-foreground)_82%,var(--site-background))]">
            {asText(props.intro)}
          </p>
        ) : null}
      </div>
      <div className={galleryGridClassName(layout)}>
        {images.map((item, index) => {
          const value = asObject(item)
          const title = asText(value?.title) || `Image ${index + 1}`
          const caption = asText(value?.caption)
          const image = asImageRef(value?.image)
          return (
            <article
              key={index}
              className={cn(
                preview.imagePlaceholderTall,
                'relative overflow-hidden',
                layout === 'spotlight' &&
                  index === 0 &&
                  'md:col-span-2 xl:col-span-3',
                layout === 'masonry' && index % 3 === 0 && 'md:min-h-[340px]',
              )}
            >
              {image ? (
                <AssetImage
                  image={image}
                  linkMode={linkMode}
                  siteSlug={siteSlug}
                  altFallback={title}
                  className="absolute inset-0 h-full w-full rounded-[var(--site-radius-inner)] object-cover"
                />
              ) : null}
              <div className="absolute inset-0 rounded-[var(--site-radius-inner)] bg-[linear-gradient(180deg,transparent_0%,color-mix(in_oklch,var(--site-background)_22%,transparent)_54%,color-mix(in_oklch,var(--site-background)_82%,transparent)_100%)]" />
              <div className="relative grid gap-1 rounded-[calc(var(--site-radius-inner)-8px)] border border-[color-mix(in_oklch,var(--site-border)_80%,var(--site-background))] bg-[var(--site-image-caption-background)] p-4 backdrop-blur-sm">
                <strong className="font-serif text-[1.08rem] leading-tight text-[var(--site-foreground)]">
                  {title}
                </strong>
                {caption ? (
                  <p className="m-0 text-sm text-[color-mix(in_oklch,var(--site-foreground)_84%,var(--site-background))]">
                    {caption}
                  </p>
                ) : null}
              </div>
            </article>
          )
        })}
      </div>
    </section>
  )
}

function TestimonialsBlock({
  props,
  linkMode,
  siteSlug,
}: {
  props: Record<string, unknown>
  linkMode: 'anchors' | 'published'
  siteSlug?: string
}) {
  return (
    <section className={preview.panel}>
      <div className={preview.sectionHeading}>
        <h3 className="font-serif text-[1.6rem] font-bold leading-[0.96] text-[var(--site-foreground)]">
          {asText(props.heading)}
        </h3>
        {asText(props.intro) ? (
          <p className="text-[color-mix(in_oklch,var(--site-foreground)_82%,var(--site-background))]">
            {asText(props.intro)}
          </p>
        ) : null}
      </div>
      <div className={preview.cardGrid}>
        {asArray(props.items).map((item, index) => {
          const value = asObject(item)
          const avatar = asImageRef(value?.avatar)
          return (
            <article key={index} className={preview.quoteCard}>
              {avatar ? (
                <div className="mb-1 flex items-center gap-3">
                  <AssetImage
                    image={avatar}
                    linkMode={linkMode}
                    siteSlug={siteSlug}
                    altFallback={asText(value?.name) || 'Client portrait'}
                    className="size-12 rounded-full border border-[var(--site-image-border)] bg-[var(--site-image-background)] object-cover shadow-[var(--site-image-shadow)]"
                  />
                </div>
              ) : null}
              <p className="m-0 font-serif text-[1.15rem] leading-[1.45] text-[var(--site-foreground)]">
                "{asText(value?.quote)}"
              </p>
              <div>
                <strong className="block text-[var(--site-foreground)]">
                  {asText(value?.name)}
                </strong>
                {asText(value?.role) ? (
                  <small className="text-[color-mix(in_oklch,var(--site-foreground)_76%,var(--site-background))]">
                    {asText(value?.role)}
                  </small>
                ) : null}
              </div>
            </article>
          )
        })}
      </div>
    </section>
  )
}

function PricingPackagesBlock({
  props,
  resolveHref,
}: {
  props: Record<string, unknown>
  resolveHref: (href: string) => string
}) {
  return (
    <section className={preview.panel}>
      <div className={preview.sectionHeading}>
        <h3 className="font-serif text-[1.6rem] font-bold leading-[0.96] text-[var(--site-foreground)]">
          {asText(props.heading)}
        </h3>
        {asText(props.intro) ? (
          <p className="text-[color-mix(in_oklch,var(--site-foreground)_82%,var(--site-background))]">
            {asText(props.intro)}
          </p>
        ) : null}
      </div>
      <div className={preview.pricingGrid}>
        {asArray(props.plans).map((item, index) => {
          const value = asObject(item)
          const cta = asObject(value?.cta)
          return (
            <article key={index} className={preview.pricingCard}>
              <div className="flex items-start justify-between gap-3">
                <div>
                  <h4 className="mb-1 font-serif text-[1.2rem] font-bold leading-[0.96] text-[var(--site-foreground)]">
                    {asText(value?.name)}
                  </h4>
                  <p className="m-0 text-sm text-[color-mix(in_oklch,var(--site-foreground)_78%,var(--site-background))]">
                    {asText(value?.description)}
                  </p>
                </div>
                <strong className="rounded-full border border-[var(--site-border)] bg-[var(--site-surface-muted)] px-3 py-2 text-[var(--site-foreground)]">
                  {asText(value?.price)}
                </strong>
              </div>
              <div className={preview.chipList}>
                {asArray(value?.features).map((feature, featureIndex) => {
                  const featureValue = asObject(feature)
                  return (
                    <span key={featureIndex} className={preview.chip}>
                      {asText(featureValue?.text)}
                    </span>
                  )
                })}
              </div>
              {cta ? (
                <div>
                  <Button asChild variant="plain" className={preview.button}>
                    <a href={resolveHref(asText(cta.href) || '#')}>
                      {asText(cta.label) || 'Get in touch'}
                    </a>
                  </Button>
                </div>
              ) : null}
            </article>
          )
        })}
      </div>
    </section>
  )
}

function FAQBlock({ props }: { props: Record<string, unknown> }) {
  return (
    <section className={preview.panel}>
      <div className={preview.sectionHeading}>
        <h3 className="font-serif text-[1.6rem] font-bold leading-[0.96] text-[var(--site-foreground)]">
          {asText(props.heading)}
        </h3>
        {asText(props.intro) ? (
          <p className="text-[color-mix(in_oklch,var(--site-foreground)_82%,var(--site-background))]">
            {asText(props.intro)}
          </p>
        ) : null}
      </div>
      <div className={preview.faqList}>
        {asArray(props.items).map((item, index) => {
          const value = asObject(item)
          return (
            <article key={index} className={preview.faqItem}>
              <h4 className="mb-2 font-serif text-[1.08rem] font-bold leading-[1.05] text-[var(--site-foreground)]">
                {asText(value?.question)}
              </h4>
              <p className="m-0 text-[color-mix(in_oklch,var(--site-foreground)_82%,var(--site-background))]">
                {asText(value?.answer)}
              </p>
            </article>
          )
        })}
      </div>
    </section>
  )
}

function TeamProfileCardsBlock({
  props,
  resolveHref,
  linkMode,
  siteSlug,
}: {
  props: Record<string, unknown>
  resolveHref: (href: string) => string
  linkMode: 'anchors' | 'published'
  siteSlug?: string
}) {
  return (
    <section className={preview.panel}>
      <div className={preview.sectionHeading}>
        <h3 className="font-serif text-[1.6rem] font-bold leading-[0.96] text-[var(--site-foreground)]">
          {asText(props.heading)}
        </h3>
        {asText(props.intro) ? (
          <p className="text-[color-mix(in_oklch,var(--site-foreground)_82%,var(--site-background))]">
            {asText(props.intro)}
          </p>
        ) : null}
      </div>
      <div className={preview.cardGrid}>
        {asArray(props.people).map((item, index) => {
          const value = asObject(item)
          const photo = asImageRef(value?.photo)
          return (
            <article key={index} className={preview.pricingCard}>
              <div className="grid gap-4">
                {photo ? (
                  <AssetImage
                    image={photo}
                    linkMode={linkMode}
                    siteSlug={siteSlug}
                    altFallback={asText(value?.name) || 'Team profile'}
                    className="min-h-[220px] w-full rounded-[var(--site-radius-inner)] border border-[var(--site-image-border)] bg-[var(--site-image-background)] object-cover shadow-[var(--site-image-shadow)]"
                  />
                ) : (
                  <div
                    className={cn(preview.imagePlaceholder, 'min-h-[160px]')}
                  >
                    <span>{asText(value?.name) || 'Profile image slot'}</span>
                  </div>
                )}
                <div>
                  <h4 className="mb-1 font-serif text-[1.2rem] font-bold leading-[0.96] text-[var(--site-foreground)]">
                    {asText(value?.name)}
                  </h4>
                  <p className="m-0 text-sm font-semibold text-[var(--site-primary)]">
                    {asText(value?.role)}
                  </p>
                </div>
                <p className="m-0 text-[color-mix(in_oklch,var(--site-foreground)_82%,var(--site-background))]">
                  {asText(value?.bio)}
                </p>
                <div className={preview.footerLinks}>
                  {asArray(value?.links).map((link, linkIndex) => {
                    const linkValue = asObject(link)
                    return (
                      <a
                        key={linkIndex}
                        className={preview.chip}
                        href={resolveHref(asText(linkValue?.href) || '#')}
                      >
                        {asText(linkValue?.label) || 'Open'}
                      </a>
                    )
                  })}
                </div>
              </div>
            </article>
          )
        })}
      </div>
    </section>
  )
}

function FooterBlock({
  props,
  resolveHref,
}: {
  props: Record<string, unknown>
  resolveHref: (href: string) => string
}) {
  return (
    <footer className={preview.footerShell}>
      <div className="grid gap-3">
        <div>
          <p className={text.eyebrow}>Footer</p>
          <h3 className="font-serif text-[1.8rem] font-bold leading-[0.96] text-[var(--site-foreground)]">
            {asText(props.siteName)}
          </h3>
        </div>
        {asText(props.tagline) ? (
          <p className="m-0 max-w-[44ch] text-[color-mix(in_oklch,var(--site-foreground)_82%,var(--site-background))]">
            {asText(props.tagline)}
          </p>
        ) : null}
        {asText(props.contactLine) ? (
          <small className="text-[color-mix(in_oklch,var(--site-foreground)_78%,var(--site-background))]">
            {asText(props.contactLine)}
          </small>
        ) : null}
      </div>
      <div className="grid gap-4">
        <div className={preview.footerLinks}>
          {asArray(props.navigationLinks).map((item, index) => {
            const value = asObject(item)
            return (
              <a
                key={index}
                className={preview.chip}
                href={resolveHref(asText(value?.href) || '#')}
              >
                {asText(value?.label)}
              </a>
            )
          })}
        </div>
        {asArray(props.socialLinks).length > 0 ? (
          <div className={preview.footerLinks}>
            {asArray(props.socialLinks).map((item, index) => {
              const value = asObject(item)
              return (
                <a
                  key={index}
                  className={preview.chip}
                  href={resolveHref(asText(value?.href) || '#')}
                >
                  {asText(value?.label)}
                </a>
              )
            })}
          </div>
        ) : null}
        {asText(props.copyright) ? (
          <small className="text-[color-mix(in_oklch,var(--site-foreground)_76%,var(--site-background))]">
            {asText(props.copyright)}
          </small>
        ) : null}
      </div>
    </footer>
  )
}

function asText(value: unknown) {
  return typeof value === 'string' ? value : ''
}

function asImageRef(value: unknown) {
  const object = asObject(value)
  if (!object) {
    return null
  }

  const assetId = asText(object.assetId)
  if (!assetId) {
    return null
  }

  return {
    assetId,
    alt: asText(object.alt),
  }
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

function AssetImage({
  image,
  linkMode,
  siteSlug,
  altFallback,
  className,
}: {
  image: { assetId: string; alt: string }
  linkMode: 'anchors' | 'published'
  siteSlug?: string
  altFallback: string
  className: string
}) {
  const src =
    linkMode === 'published' && siteSlug
      ? buildPublishedAssetURL(siteSlug, image.assetId)
      : buildDraftAssetURL(image.assetId)

  return (
    <img
      src={src}
      alt={image.alt || altFallback}
      className={className}
      loading="lazy"
    />
  )
}

function galleryGridClassName(layout: string) {
  switch (layout) {
    case 'masonry':
      return 'grid gap-3.5 md:grid-cols-2 xl:grid-cols-3'
    case 'spotlight':
      return 'grid gap-3.5 md:grid-cols-2 xl:grid-cols-3'
    default:
      return preview.cardGrid
  }
}

function resolveNavigationHref(
  item: { pageId?: string; href?: string },
  pageAnchors: Map<string, string>,
  pageById: Map<string, RoutablePage>,
  slugToPage: Map<string, RoutablePage>,
  linkMode: 'anchors' | 'published',
  siteSlug?: string,
  publishedBasePath?: string,
) {
  if (item.pageId && pageAnchors.has(item.pageId)) {
    if (linkMode === 'published') {
      const page = pageById.get(item.pageId)
      if (page) {
        return buildPublishedPageHref(page.slug, siteSlug, publishedBasePath)
      }
    }
    return `#${pageAnchors.get(item.pageId)}`
  }
  return resolvePageHref(
    item.href ?? '#',
    slugToPage,
    linkMode,
    siteSlug,
    publishedBasePath,
  )
}

function resolvePageHref(
  href: string,
  slugToPage: Map<string, RoutablePage>,
  linkMode: 'anchors' | 'published',
  siteSlug?: string,
  publishedBasePath?: string,
) {
  if (!href.startsWith('/')) {
    return href
  }
  const page = slugToPage.get(href)
  if (!page) {
    return href
  }
  if (linkMode === 'published') {
    return buildPublishedPageHref(page.slug, siteSlug, publishedBasePath)
  }
  return `#${pageAnchor(page.slug, page.id)}`
}

function buildPublishedPageHref(
  pageSlug: string,
  siteSlug?: string,
  publishedBasePath?: string,
) {
  const basePath = resolvePublishedBasePath(siteSlug, publishedBasePath)
  if (pageSlug === '/') {
    return basePath || '/'
  }
  return `${basePath}${pageSlug}`
}

function resolvePublishedBasePath(
  siteSlug?: string,
  publishedBasePath?: string,
) {
  if (typeof publishedBasePath === 'string') {
    if (publishedBasePath === '/') {
      return ''
    }
    return publishedBasePath.replace(/\/+$/, '')
  }
  if (!siteSlug) {
    return ''
  }
  return `/public/${siteSlug}`
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
