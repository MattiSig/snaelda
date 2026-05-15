import type { FormEvent, ReactNode } from 'react'
import { useState } from 'react'
import {
  APIError,
  submitPublicForm,
  type ImageCredit,
  type PublishedSnapshot,
  type SiteDraft,
} from '@/lib/api'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Select } from '@/components/ui/select'
import { Textarea } from '@/components/ui/textarea'
import { buildDraftAssetURL, buildPublishedAssetURL } from '@/lib/assets'
import { buildSiteThemeStyle } from '@/lib/site-theme'
import { preview, text } from '@/lib/styles'
import { cn } from '@/lib/utils'

type RenderableSite = Pick<SiteDraft, 'theme' | 'navigation' | 'pages'> & {
  site: {
    id?: string
    name: string
    seo?: {
      description?: string
    }
  }
  imageCredits?: ImageCredit[]
}

type RoutablePage = {
  id: string
  slug: string
}

type RenderedBlock = SiteDraft['pages'][number]['blocks'][number]
type RenderedPage = SiteDraft['pages'][number]

export type SiteDraftRendererBlockSlot = {
  block: RenderedBlock
  page: RenderedPage
  blockIndex: number
  children: ReactNode
}

export function SiteDraftRenderer({
  site,
  eyebrow = 'Site render',
  showPageMeta = true,
  selectedPageId,
  linkMode = 'anchors',
  siteSlug,
  publishedBasePath,
  mode: _mode = 'default',
  renderBlock,
}: {
  site: SiteDraft | PublishedSnapshot | RenderableSite
  eyebrow?: string
  showPageMeta?: boolean
  selectedPageId?: string
  linkMode?: 'anchors' | 'published'
  siteSlug?: string
  publishedBasePath?: string
  mode?: 'default' | 'builder'
  renderBlock?: (slot: SiteDraftRendererBlockSlot) => React.ReactNode
}) {
  const renderedPages = selectedPageId
    ? site.pages.filter((page) => page.id === selectedPageId)
    : site.pages
  const pageAnchors = new Map(
    site.pages.map((page) => [page.id, pageAnchor(page.slug, page.id)]),
  )
  const pageById = new Map(site.pages.map((page) => [page.id, page]))
  const slugToPage = new Map(site.pages.map((page) => [page.slug, page]))
  const homePage =
    site.pages.find((page) => page.slug === '/') ?? site.pages[0]
  const homeHref = homePage
    ? resolveNavigationHref(
        { pageId: homePage.id },
        pageAnchors,
        pageById,
        slugToPage,
        linkMode,
        siteSlug,
        publishedBasePath,
      )
    : '#'

  return (
    <div className={preview.shell} style={buildSiteThemeStyle(site.theme)}>
      <header className={preview.header}>
        <div className={preview.headerInner}>
          <a className={preview.headerBrand} href={homeHref}>
            {site.site.name}
          </a>
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
        </div>
      </header>

      <main className={preview.page}>
        {renderedPages.map((page) => (
          <article
            key={page.id}
            id={pageAnchor(page.slug, page.id)}
            className="w-full"
          >
            {showPageMeta ? (
              <div className={preview.pageMeta}>
                <span>{eyebrow ? `${eyebrow} · ` : ''}{page.title}</span>
                <small className="text-[color-mix(in_oklch,var(--site-foreground)_55%,var(--site-background))]">
                  {page.slug}
                </small>
              </div>
            ) : null}
            <div className={preview.pageStack}>
              {page.blocks
                .filter((block) => !block.settings?.hidden)
                .map((block, blockIndex) => {
                  const renderedBlock = renderSiteBlock({
                    block,
                    page,
                    blockIndex,
                    siteID: site.site.id,
                    slugToPage,
                    linkMode,
                    siteSlug,
                    publishedBasePath,
                  })

                  if (!renderBlock) {
                    return renderedBlock
                  }

                  return renderBlock({
                    block,
                    page,
                    blockIndex,
                    children: renderedBlock,
                  })
                })}
            </div>
          </article>
        ))}
      </main>
      <ImageCreditsBand credits={'imageCredits' in site ? site.imageCredits : undefined} />
    </div>
  )
}

function ImageCreditsBand({ credits }: { credits?: ImageCredit[] }) {
  if (!credits || credits.length === 0) {
    return null
  }
  const pexels = credits.filter((credit) => credit.provider === 'pexels')
  if (pexels.length === 0) {
    return null
  }
  return (
    <aside
      className="border-t border-[color-mix(in_oklch,var(--site-border)_45%,transparent)] bg-[color-mix(in_oklch,var(--site-background)_92%,var(--site-foreground))]"
      aria-label="Image credits"
    >
      <div className="mx-auto flex w-full max-w-[1180px] flex-wrap items-center gap-x-3 gap-y-1 px-6 py-4 text-xs text-[color-mix(in_oklch,var(--site-foreground)_72%,var(--site-background))]">
        <span>Imagery from</span>
        <a
          href="https://www.pexels.com"
          target="_blank"
          rel="noopener noreferrer"
          className="font-medium text-[var(--site-foreground)] hover:underline"
        >
          Pexels
        </a>
        <span aria-hidden="true">·</span>
        <span>Photos by</span>
        <span className="inline-flex flex-wrap items-center gap-x-2">
          {pexels.map((credit, index) => {
            const name = credit.author?.trim() || 'Pexels contributor'
            const isLast = index === pexels.length - 1
            const Element = credit.authorUrl ? 'a' : 'span'
            return (
              <span key={`${credit.author ?? 'pexels'}-${credit.sourceUrl ?? index}`}>
                <Element
                  {...(credit.authorUrl
                    ? {
                        href: credit.authorUrl,
                        target: '_blank',
                        rel: 'noopener noreferrer',
                        className: 'font-medium text-[var(--site-foreground)] hover:underline',
                      }
                    : { className: 'font-medium text-[var(--site-foreground)]' })}
                >
                  {name}
                </Element>
                {!isLast ? <span aria-hidden="true">, </span> : null}
              </span>
            )
          })}
        </span>
      </div>
    </aside>
  )
}

function renderSiteBlock({
  block,
  page,
  blockIndex,
  siteID,
  slugToPage,
  linkMode,
  siteSlug,
  publishedBasePath,
}: {
  block: RenderedBlock
  page: RenderedPage
  blockIndex: number
  siteID?: string
  slugToPage: Map<string, RoutablePage>
  linkMode: 'anchors' | 'published'
  siteSlug?: string
  publishedBasePath?: string
}) {
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
      return <TextSectionBlock key={block.id} props={block.props} />
    case 'features_grid':
      return <FeaturesGridBlock key={block.id} props={block.props} />
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
    case 'contact_form':
      return (
        <ContactFormBlock
          key={block.id}
          siteId={siteID}
          pageId={page.id}
          blockId={block.id}
          props={block.props}
        />
      )
    case 'image_text':
      return (
        <ImageTextBlock
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
        <section
          key={`${block.id}-${blockIndex}`}
          className={preview.panel}
        >
          <div className={preview.panelInner}>
            <p className={text.eyebrow}>Unsupported block</p>
            <strong>{block.type}</strong>
          </div>
        </section>
      )
  }
}

const headingClass =
  'font-serif text-[clamp(1.65rem,2.8vw,2.4rem)] font-bold leading-[1.08] tracking-tight text-[var(--site-foreground)]'

const bodyClass =
  'text-[1.05rem] leading-[1.65] text-[color-mix(in_oklch,var(--site-foreground)_82%,var(--site-background))]'

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
  const secondary = asObject(props.secondaryCta)
  const image = asImageRef(props.image)
  const layout = asText(props.layout) || 'centered'
  const hasImage = image !== null
  const isSplit = hasImage && layout !== 'centered'

  const content = (
    <div
      className={cn(
        'grid gap-6',
        isSplit && layout === 'split-right' && 'lg:order-1',
        isSplit && layout === 'split-left' && 'lg:order-2',
        !isSplit && 'max-w-[22ch]',
      )}
    >
      {asText(props.eyebrow) ? (
        <p className={text.eyebrow}>{asText(props.eyebrow)}</p>
      ) : null}
      <h2 className="font-serif text-[clamp(2.6rem,6.2vw,5.4rem)] font-bold leading-[0.96] tracking-[-0.02em] text-[var(--site-foreground)]">
        {asText(props.headline)}
      </h2>
      {asText(props.subheadline) ? (
        <p className="max-w-[44ch] text-[1.15rem] leading-[1.6] text-[color-mix(in_oklch,var(--site-foreground)_78%,var(--site-background))]">
          {asText(props.subheadline)}
        </p>
      ) : null}
      {primary || secondary ? (
        <div className={cn(preview.actionRow, 'mt-2')}>
          {primary ? (
            <Button asChild variant="plain" className={preview.button}>
              <a href={resolveHref(asText(primary.href) || '#')}>
                {asText(primary.label) ?? 'Continue'}
              </a>
            </Button>
          ) : null}
          {secondary ? (
            <Button
              asChild
              variant="plain"
              className={cn(preview.button, preview.ghostButton)}
            >
              <a href={resolveHref(asText(secondary.href) || '#')}>
                {asText(secondary.label) ?? 'Learn more'}
              </a>
            </Button>
          ) : null}
        </div>
      ) : null}
    </div>
  )

  return (
    <section className={cn(preview.panel, preview.hero)}>
      <div className={preview.panelInner}>
        <div
          className={cn(
            isSplit
              ? 'grid gap-12 lg:grid-cols-[minmax(0,1.1fr)_minmax(280px,0.9fr)] lg:items-center'
              : 'grid gap-10',
          )}
        >
          {content}
          {image ? (
            <AssetImage
              image={image}
              linkMode={linkMode}
              siteSlug={siteSlug}
              altFallback={asText(props.headline) || 'Hero image'}
              className={cn(
                'w-full rounded-[var(--site-radius-inner)] object-cover',
                isSplit
                  ? 'aspect-[4/5] lg:aspect-auto lg:h-full lg:min-h-[460px]'
                  : 'aspect-[16/9] max-h-[520px]',
                isSplit && layout === 'split-right' && 'lg:order-2',
                isSplit && layout === 'split-left' && 'lg:order-1',
              )}
            />
          ) : null}
        </div>
      </div>
    </section>
  )
}

function TextSectionBlock({ props }: { props: Record<string, unknown> }) {
  const alignment = asText(props.alignment) || 'left'
  const width = asText(props.width) || 'default'
  const widthClass =
    width === 'narrow'
      ? 'max-w-[56ch]'
      : width === 'wide'
        ? 'max-w-[78ch]'
        : 'max-w-[68ch]'
  const alignClass =
    alignment === 'center'
      ? 'text-center'
      : alignment === 'right'
        ? 'text-right'
        : 'text-left'
  const positionClass =
    alignment === 'center'
      ? 'mx-auto'
      : alignment === 'right'
        ? 'ml-auto'
        : ''

  return (
    <section className={preview.panel}>
      <div className={preview.panelInner}>
        <div
          className={cn('grid gap-5', widthClass, positionClass, alignClass)}
        >
          <h3 className={headingClass}>{asText(props.heading)}</h3>
          <p className={bodyClass}>{asText(props.body)}</p>
        </div>
      </div>
    </section>
  )
}

function FeaturesGridBlock({ props }: { props: Record<string, unknown> }) {
  const columns = asInt(props.columns) ?? 3
  const colsClass =
    columns === 2
      ? 'md:grid-cols-2'
      : columns === 4
        ? 'md:grid-cols-2 xl:grid-cols-4'
        : 'md:grid-cols-2 xl:grid-cols-3'
  return (
    <section className={preview.panel}>
      <div className={preview.panelInner}>
        <div className={preview.sectionHeading}>
          <h3 className={headingClass}>{asText(props.heading)}</h3>
          {asText(props.intro) ? (
            <p className={bodyClass}>{asText(props.intro)}</p>
          ) : null}
        </div>
        <div className={cn('grid gap-x-10 gap-y-12', colsClass)}>
          {asArray(props.items).map((item, index) => {
            const value = asObject(item)
            const icon = asText(value?.icon)
            return (
              <div key={index} className={preview.feature}>
                {icon ? <p className={text.eyebrow}>{icon}</p> : null}
                <h4 className="mt-1 font-serif text-[1.2rem] font-bold leading-[1.15] text-[var(--site-foreground)]">
                  {asText(value?.title)}
                </h4>
                <p className="leading-[1.6] text-[color-mix(in_oklch,var(--site-foreground)_82%,var(--site-background))]">
                  {asText(value?.body)}
                </p>
              </div>
            )
          })}
        </div>
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
  const variant = asText(props.variant) || 'primary'
  const surfaceClass =
    variant === 'accent'
      ? 'bg-[var(--site-primary)] text-[var(--site-background)] [--site-button-background:var(--site-background)] [--site-button-foreground:var(--site-primary)] [--site-button-border:var(--site-background)] [--site-button-ghost-foreground:var(--site-background)] [--site-button-ghost-border:var(--site-background)]'
      : variant === 'secondary'
        ? 'bg-[var(--site-surface)] text-[var(--site-foreground)]'
        : preview.ctaSurface
  return (
    <section className={cn(preview.panel, surfaceClass)}>
      <div
        className={cn(
          preview.panelInner,
          'flex flex-wrap items-center justify-between gap-x-12 gap-y-6',
        )}
      >
        <div className="grid max-w-[44ch] gap-3">
          <h3 className="font-serif text-[clamp(1.75rem,3.2vw,2.8rem)] font-bold leading-[1.05] tracking-tight">
            {asText(props.heading)}
          </h3>
          {asText(props.body) ? (
            <p className="text-[1.05rem] leading-[1.6] opacity-85">
              {asText(props.body)}
            </p>
          ) : null}
        </div>
        {cta ? (
          <Button asChild variant="plain" className={preview.button}>
            <a href={resolveHref(asText(cta.href) || '#')}>
              {asText(cta.label) ?? 'Open'}
            </a>
          </Button>
        ) : null}
      </div>
    </section>
  )
}

function ContactFormBlock({
  siteId,
  pageId,
  blockId,
  props,
}: {
  siteId?: string
  pageId: string
  blockId: string
  props: Record<string, unknown>
}) {
  const [values, setValues] = useState<Record<string, string>>({})
  const [isSubmitting, setIsSubmitting] = useState(false)
  const [errorMessage, setErrorMessage] = useState('')
  const [successMessage, setSuccessMessage] = useState('')
  const fields = asFormFields(props.fields)
  const submitLabel = asText(props.submitLabel) || 'Send message'

  async function handleSubmit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault()
    if (!siteId) {
      setErrorMessage('This form is not connected to a site yet.')
      setSuccessMessage('')
      return
    }

    setIsSubmitting(true)
    setErrorMessage('')
    setSuccessMessage('')

    try {
      const payload = fields.reduce<Record<string, unknown>>((result, field) => {
        result[field.name] = values[field.name] ?? ''
        return result
      }, {})
      const response = await submitPublicForm(siteId, blockId, payload)
      setValues({})
      setSuccessMessage(response.message)
    } catch (error) {
      setErrorMessage(
        error instanceof APIError ? error.message : 'Could not send message',
      )
    } finally {
      setIsSubmitting(false)
    }
  }

  return (
    <section className={preview.panel}>
      <div className={cn(preview.panelNarrow)}>
        <div className="mb-8 grid gap-3">
          <h3 className={headingClass}>{asText(props.heading)}</h3>
          {asText(props.intro) ? (
            <p className={bodyClass}>{asText(props.intro)}</p>
          ) : null}
        </div>

        <form className="grid gap-5" onSubmit={handleSubmit}>
          {fields.map((field) => (
            <label key={field.name} className="grid gap-2">
              <span className="text-sm font-medium text-[color-mix(in_oklch,var(--site-foreground)_82%,var(--site-background))]">
                {field.label}
                {field.required ? ' *' : ''}
              </span>
              {field.type === 'message' ? (
                <Textarea
                  name={field.name}
                  rows={5}
                  required={field.required}
                  value={values[field.name] ?? ''}
                  placeholder={formPlaceholder(field)}
                  onChange={(event) =>
                    setValues((current) => ({
                      ...current,
                      [field.name]: event.target.value,
                    }))
                  }
                />
              ) : field.type === 'select' ? (
                <Select
                  name={field.name}
                  required={field.required}
                  value={values[field.name] ?? ''}
                  onChange={(event) =>
                    setValues((current) => ({
                      ...current,
                      [field.name]: event.target.value,
                    }))
                  }
                >
                  <option value="">Select an option</option>
                  {field.options.map((option) => (
                    <option key={option} value={option}>
                      {option}
                    </option>
                  ))}
                </Select>
              ) : (
                <Input
                  name={field.name}
                  type={field.type === 'email' ? 'email' : 'text'}
                  required={field.required}
                  value={values[field.name] ?? ''}
                  placeholder={formPlaceholder(field)}
                  onChange={(event) =>
                    setValues((current) => ({
                      ...current,
                      [field.name]: event.target.value,
                    }))
                  }
                />
              )}
            </label>
          ))}

          <input type="hidden" name="pageId" value={pageId} />

          {errorMessage ? <p className={text.error}>{errorMessage}</p> : null}
          {successMessage ? (
            <p className={text.success} aria-live="polite">
              {successMessage}
            </p>
          ) : null}

          <div>
            <Button
              type="submit"
              disabled={isSubmitting}
              className={preview.button}
            >
              {isSubmitting ? 'Sending...' : submitLabel}
            </Button>
          </div>
        </form>
      </div>
    </section>
  )
}

function ImageTextBlock({
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
  const cta = asObject(props.cta)
  const image = asImageRef(props.image)
  const imagePosition = asText(props.imagePosition) || 'right'
  return (
    <section className={preview.panel}>
      <div className={cn(preview.panelInner, preview.split)}>
        <div
          className={cn(
            'grid gap-5',
            imagePosition === 'left' && 'lg:order-2',
          )}
        >
          <h3 className={headingClass}>{asText(props.heading)}</h3>
          <p className={bodyClass}>{asText(props.body)}</p>
          {cta ? (
            <div className="mt-2">
              <Button asChild variant="plain" className={cn(preview.button, preview.ghostButton)}>
                <a href={resolveHref(asText(cta.href) || '#')}>
                  {asText(cta.label) || 'Open link'}
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
            altFallback={asText(props.heading) || 'Supporting image'}
            className={cn(
              'aspect-[4/5] w-full rounded-[var(--site-radius-inner)] object-cover lg:aspect-auto lg:h-full lg:min-h-[380px]',
              imagePosition === 'left' && 'lg:order-1',
            )}
          />
        ) : (
          <div
            className={cn(
              preview.imagePlaceholder,
              'min-h-[300px]',
              imagePosition === 'left' && 'lg:order-1',
            )}
          >
            <span>Image slot</span>
          </div>
        )}
      </div>
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
      <div className={preview.panelInner}>
        <div className={preview.sectionHeading}>
          <h3 className={headingClass}>{asText(props.heading)}</h3>
          {asText(props.intro) ? (
            <p className={bodyClass}>{asText(props.intro)}</p>
          ) : null}
        </div>
        <div className={galleryGridClassName(layout)}>
          {images.map((item, index) => {
            const value = asObject(item)
            const title = asText(value?.title) || `Image ${index + 1}`
            const caption = asText(value?.caption)
            const image = asImageRef(value?.image)
            const isSpotlight = layout === 'spotlight' && index === 0
            return (
              <figure
                key={index}
                className={cn(
                  'grid gap-3',
                  isSpotlight && 'md:col-span-2 xl:col-span-3',
                )}
              >
                {image ? (
                  <AssetImage
                    image={image}
                    linkMode={linkMode}
                    siteSlug={siteSlug}
                    altFallback={title}
                    className={cn(
                      'w-full rounded-[var(--site-radius-inner)] object-cover',
                      isSpotlight
                        ? 'aspect-[21/9]'
                        : layout === 'masonry' && index % 3 === 0
                          ? 'aspect-[3/4]'
                          : 'aspect-[4/3]',
                    )}
                  />
                ) : (
                  <div
                    className={cn(
                      preview.imagePlaceholderTall,
                      isSpotlight && 'min-h-[440px]',
                    )}
                  >
                    <span className="text-sm">{title}</span>
                  </div>
                )}
                <figcaption className="grid gap-1">
                  <span className="text-sm font-medium text-[var(--site-foreground)]">
                    {title}
                  </span>
                  {caption ? (
                    <span className="text-sm leading-[1.55] text-[color-mix(in_oklch,var(--site-foreground)_72%,var(--site-background))]">
                      {caption}
                    </span>
                  ) : null}
                </figcaption>
              </figure>
            )
          })}
        </div>
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
      <div className={preview.panelInner}>
        <div className={preview.sectionHeading}>
          <h3 className={headingClass}>{asText(props.heading)}</h3>
          {asText(props.intro) ? (
            <p className={bodyClass}>{asText(props.intro)}</p>
          ) : null}
        </div>
        <div className="grid gap-x-12 gap-y-10 md:grid-cols-2">
          {asArray(props.items).map((item, index) => {
            const value = asObject(item)
            const avatar = asImageRef(value?.avatar)
            return (
              <figure key={index} className={preview.quoteCard}>
                <blockquote className="m-0 font-serif text-[1.35rem] leading-[1.45] text-[var(--site-foreground)]">
                  &ldquo;{asText(value?.quote)}&rdquo;
                </blockquote>
                <figcaption className="flex items-center gap-3">
                  {avatar ? (
                    <AssetImage
                      image={avatar}
                      linkMode={linkMode}
                      siteSlug={siteSlug}
                      altFallback={asText(value?.name) || 'Client portrait'}
                      className="size-10 rounded-full object-cover"
                    />
                  ) : null}
                  <div>
                    <span className="block text-sm font-semibold text-[var(--site-foreground)]">
                      {asText(value?.name)}
                    </span>
                    {asText(value?.role) ? (
                      <span className="block text-sm text-[color-mix(in_oklch,var(--site-foreground)_68%,var(--site-background))]">
                        {asText(value?.role)}
                      </span>
                    ) : null}
                  </div>
                </figcaption>
              </figure>
            )
          })}
        </div>
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
      <div className={preview.panelInner}>
        <div className={preview.sectionHeading}>
          <h3 className={headingClass}>{asText(props.heading)}</h3>
          {asText(props.intro) ? (
            <p className={bodyClass}>{asText(props.intro)}</p>
          ) : null}
        </div>
        <div className={preview.pricingGrid}>
          {asArray(props.plans).map((item, index) => {
            const value = asObject(item)
            const cta = asObject(value?.cta)
            return (
              <article key={index} className={preview.pricingCard}>
                <div className="grid gap-2">
                  <h4 className="font-serif text-[1.3rem] font-bold leading-[1.1] text-[var(--site-foreground)]">
                    {asText(value?.name)}
                  </h4>
                  <p className="m-0 font-serif text-[1.8rem] font-bold leading-none text-[var(--site-foreground)]">
                    {asText(value?.price)}
                  </p>
                  <p className="m-0 text-sm leading-[1.55] text-[color-mix(in_oklch,var(--site-foreground)_78%,var(--site-background))]">
                    {asText(value?.description)}
                  </p>
                </div>
                <ul className={preview.chipList}>
                  {asArray(value?.features).map((feature, featureIndex) => {
                    const featureValue = asObject(feature)
                    return (
                      <li key={featureIndex} className={preview.chip}>
                        <span
                          aria-hidden
                          className="text-[var(--site-primary)]"
                        >
                          &#x2014;
                        </span>
                        <span>{asText(featureValue?.text)}</span>
                      </li>
                    )
                  })}
                </ul>
                {cta ? (
                  <div className="mt-auto">
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
      </div>
    </section>
  )
}

function FAQBlock({ props }: { props: Record<string, unknown> }) {
  return (
    <section className={preview.panel}>
      <div className={cn(preview.panelNarrow)}>
        <div className={preview.sectionHeading}>
          <h3 className={headingClass}>{asText(props.heading)}</h3>
          {asText(props.intro) ? (
            <p className={bodyClass}>{asText(props.intro)}</p>
          ) : null}
        </div>
        <ul className={preview.faqList}>
          {asArray(props.items).map((item, index) => {
            const value = asObject(item)
            return (
              <li key={index} className={preview.faqItem}>
                <h4 className="font-serif text-[1.15rem] font-bold leading-[1.25] text-[var(--site-foreground)]">
                  {asText(value?.question)}
                </h4>
                <p className="m-0 leading-[1.65] text-[color-mix(in_oklch,var(--site-foreground)_82%,var(--site-background))]">
                  {asText(value?.answer)}
                </p>
              </li>
            )
          })}
        </ul>
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
      <div className={preview.panelInner}>
        <div className={preview.sectionHeading}>
          <h3 className={headingClass}>{asText(props.heading)}</h3>
          {asText(props.intro) ? (
            <p className={bodyClass}>{asText(props.intro)}</p>
          ) : null}
        </div>
        <div className={preview.cardGrid}>
          {asArray(props.people).map((item, index) => {
            const value = asObject(item)
            const photo = asImageRef(value?.photo)
            return (
              <div key={index} className="grid gap-4">
                {photo ? (
                  <AssetImage
                    image={photo}
                    linkMode={linkMode}
                    siteSlug={siteSlug}
                    altFallback={asText(value?.name) || 'Team profile'}
                    className="aspect-[4/5] w-full rounded-[var(--site-radius-inner)] object-cover"
                  />
                ) : (
                  <div
                    className={cn(
                      preview.imagePlaceholder,
                      'aspect-[4/5] min-h-0',
                    )}
                  >
                    <span>{asText(value?.name) || 'Profile image slot'}</span>
                  </div>
                )}
                <div className="grid gap-1">
                  <h4 className="font-serif text-[1.2rem] font-bold leading-[1.15] text-[var(--site-foreground)]">
                    {asText(value?.name)}
                  </h4>
                  <p className="m-0 text-sm font-medium uppercase tracking-[0.08em] text-[color-mix(in_oklch,var(--site-foreground)_62%,var(--site-background))]">
                    {asText(value?.role)}
                  </p>
                </div>
                <p className="m-0 leading-[1.6] text-[color-mix(in_oklch,var(--site-foreground)_82%,var(--site-background))]">
                  {asText(value?.bio)}
                </p>
                {asArray(value?.links).length > 0 ? (
                  <div className="flex flex-wrap gap-x-5 gap-y-2">
                    {asArray(value?.links).map((link, linkIndex) => {
                      const linkValue = asObject(link)
                      return (
                        <a
                          key={linkIndex}
                          className={preview.footerLink}
                          href={resolveHref(asText(linkValue?.href) || '#')}
                        >
                          {asText(linkValue?.label) || 'Open'}
                        </a>
                      )
                    })}
                  </div>
                ) : null}
              </div>
            )
          })}
        </div>
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
      <div className={preview.footerInner}>
        <div className="grid gap-3">
          <h3 className="m-0 font-serif text-[1.4rem] font-bold leading-tight text-[var(--site-foreground)]">
            {asText(props.siteName)}
          </h3>
          {asText(props.tagline) ? (
            <p className="m-0 max-w-[44ch] text-[color-mix(in_oklch,var(--site-foreground)_78%,var(--site-background))]">
              {asText(props.tagline)}
            </p>
          ) : null}
          {asText(props.contactLine) ? (
            <p className="m-0 text-sm text-[color-mix(in_oklch,var(--site-foreground)_72%,var(--site-background))]">
              {asText(props.contactLine)}
            </p>
          ) : null}
        </div>
        <div className="grid gap-4 md:justify-self-end md:text-right">
          {asArray(props.navigationLinks).length > 0 ? (
            <div className={cn(preview.footerLinks, 'md:justify-end')}>
              {asArray(props.navigationLinks).map((item, index) => {
                const value = asObject(item)
                return (
                  <a
                    key={index}
                    className={preview.footerLink}
                    href={resolveHref(asText(value?.href) || '#')}
                  >
                    {asText(value?.label)}
                  </a>
                )
              })}
            </div>
          ) : null}
          {asArray(props.socialLinks).length > 0 ? (
            <div className={cn(preview.footerLinks, 'md:justify-end')}>
              {asArray(props.socialLinks).map((item, index) => {
                const value = asObject(item)
                return (
                  <a
                    key={index}
                    className={preview.footerLink}
                    href={resolveHref(asText(value?.href) || '#')}
                  >
                    {asText(value?.label)}
                  </a>
                )
              })}
            </div>
          ) : null}
        </div>
      </div>
      {asText(props.copyright) ? (
        <div className="mx-auto mt-10 w-full max-w-[1180px] border-t border-[color-mix(in_oklch,var(--site-border)_45%,transparent)] pt-6">
          <small className="text-xs text-[color-mix(in_oklch,var(--site-foreground)_62%,var(--site-background))]">
            {asText(props.copyright)}
          </small>
        </div>
      ) : null}
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

function asInt(value: unknown): number | null {
  if (typeof value === 'number' && Number.isFinite(value)) {
    return Math.trunc(value)
  }
  if (typeof value === 'string') {
    const parsed = Number.parseInt(value, 10)
    return Number.isFinite(parsed) ? parsed : null
  }
  return null
}

function asFormFields(value: unknown) {
  return asArray(value)
    .map((entry) => asObject(entry))
    .filter(
      (
        entry,
      ): entry is {
        name?: unknown
        label?: unknown
        type?: unknown
        required?: unknown
        options?: unknown
      } => entry !== null,
    )
    .map((field) => ({
      name: asText(field.name),
      label: asText(field.label) || asText(field.name),
      type: asText(field.type),
      required: Boolean(field.required),
      options: asStringArray(field.options),
    }))
    .filter((field) => field.name && field.type)
}

function asStringArray(value: unknown) {
  if (!Array.isArray(value)) {
    return []
  }
  return value.filter((entry): entry is string => typeof entry === 'string')
}

function formPlaceholder(field: { name: string; type: string }) {
  switch (field.type) {
    case 'email':
      return 'name@example.com'
    case 'phone':
      return '+46 70 000 00 00'
    case 'message':
      return 'Tell me a little about the project.'
    default:
      return field.name
  }
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
      return 'grid gap-6 md:grid-cols-2 xl:grid-cols-3'
    case 'spotlight':
      return 'grid gap-6 md:grid-cols-2 xl:grid-cols-3'
    default:
      return 'grid gap-6 md:grid-cols-2 xl:grid-cols-3'
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
