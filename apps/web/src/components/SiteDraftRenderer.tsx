import type { CSSProperties, FormEvent, MouseEvent, ReactNode } from 'react';
import { createContext, useContext, useState } from 'react';
import {
  APIError,
  submitPublicForm,
  type BrandConfig,
  type Collection,
  type CollectionEntry,
  type FooterContact,
  type FooterHours,
  type ImageCredit,
  type PublishedSnapshot,
  type SiteDraft,
} from '@/lib/api';
import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import { Select } from '@/components/ui/select';
import { Textarea } from '@/components/ui/textarea';
import {
  buildDraftAssetURL,
  buildPreviewAssetURL,
  buildPublishedAssetURL,
} from '@/lib/assets';
import { buildSiteThemeStyle } from '@/lib/site-theme';
import { preview, text } from '@/lib/styles';
import { cn } from '@/lib/utils';
import {
  InlineEditableImage,
  InlineEditableText,
} from '@/components/inline-editor';

type RenderableSite = Pick<
  SiteDraft,
  'brand' | 'theme' | 'navigation' | 'pages'
> & {
  site: {
    id?: string;
    name: string;
    seo?: {
      description?: string;
    };
  };
  collections?: Collection[];
  imageCredits?: ImageCredit[];
};

type RoutablePage = {
  id: string;
  slug: string;
};

type RenderedBlock = SiteDraft['pages'][number]['blocks'][number];
type RenderedPage = SiteDraft['pages'][number];

export type SiteDraftRendererBlockSlot = {
  block: RenderedBlock;
  page: RenderedPage;
  blockIndex: number;
  children: ReactNode;
};

type CollectionContext = {
  collectionsById: Map<string, Collection>;
  activeCollection?: Collection;
  activeEntry?: CollectionEntry;
  exposesDetailUrlsById: Map<string, boolean>;
};

// Lets anonymous viewers (shared previews, the public re-spin demo) load
// draft assets through the token-scoped public endpoint instead of the
// session-gated /api/assets/{id}/content route.
const PreviewTokenContext = createContext<string | undefined>(undefined);

export function SiteDraftRenderer({
  site,
  eyebrow = 'Site render',
  showPageMeta = true,
  selectedPageId,
  linkMode = 'anchors',
  siteSlug,
  publishedBasePath,
  previewToken,
  mode = 'default',
  renderBlock,
  activeEntry,
  activeCollection,
  onNavigatePage,
}: {
  site: SiteDraft | PublishedSnapshot | RenderableSite;
  eyebrow?: string;
  showPageMeta?: boolean;
  selectedPageId?: string;
  linkMode?: 'anchors' | 'published';
  siteSlug?: string;
  publishedBasePath?: string;
  previewToken?: string;
  mode?: 'default' | 'builder';
  renderBlock?: (slot: SiteDraftRendererBlockSlot) => React.ReactNode;
  activeEntry?: CollectionEntry;
  activeCollection?: Collection;
  onNavigatePage?: (pageId: string) => void;
}) {
  const renderedPages = selectedPageId
    ? site.pages.filter((page) => page.id === selectedPageId)
    : site.pages;
  const pageAnchors = new Map(
    site.pages.map((page) => [page.id, pageAnchor(page.slug, page.id)]),
  );
  const pageById = new Map(site.pages.map((page) => [page.id, page]));
  const slugToPage = new Map(site.pages.map((page) => [page.slug, page]));
  const homePage =
    site.pages.find((page) => page.slug === '/') ?? site.pages[0];
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
    : '#';

  const siteCollections =
    'collections' in site && Array.isArray(site.collections)
      ? (site.collections as Collection[])
      : [];
  const collectionsById = new Map<string, Collection>(
    siteCollections.map((collection) => [collection.id, collection] as const),
  );
  const exposesDetailUrlsById = computeCollectionDetailExposure(
    siteCollections,
    site.pages,
  );

  const firstRenderedPage = renderedPages[0];
  const firstVisibleBlock = firstRenderedPage?.blocks.find(
    (block) => !block.settings?.hidden,
  );
  const headerOverlapsHero =
    firstVisibleBlock?.type === 'hero' &&
    asText(firstVisibleBlock.props.variant) === 'full-page';
  const pageIdByAnchor = new Map(
    site.pages.map((page) => [pageAnchor(page.slug, page.id), page.id]),
  );

  function handleShellClick(event: MouseEvent<HTMLDivElement>) {
    if (!onNavigatePage) {
      return;
    }
    const target = event.target;
    if (!(target instanceof Element)) {
      return;
    }
    const anchor = target.closest('a');
    if (!anchor) {
      return;
    }
    const href = anchor.getAttribute('href');
    if (!href?.startsWith('#')) {
      return;
    }
    const pageId = pageIdByAnchor.get(href.slice(1));
    if (!pageId) {
      return;
    }
    event.preventDefault();
    onNavigatePage(pageId);
  }

  return (
    <PreviewTokenContext.Provider value={previewToken}>
    <div
      className={preview.shell}
      style={
        {
          ...buildSiteThemeStyle(site.theme),
          // The full-page hero pulls itself up under the header by this
          // amount; it must track the header's actual height, which grows
          // with the brand logo size (48px padding + logo height + border).
          '--preview-header-height':
            headerHeightByLogoSize[site.brand?.logo?.size ?? 'small'] ??
            '88px',
        } as CSSProperties
      }
      onClick={handleShellClick}
    >
      <header
        className={cn(
          preview.header,
          'relative z-10',
          headerOverlapsHero && 'border-transparent',
        )}
      >
        <div className={preview.headerInner}>
          <a
            className={cn(
              preview.headerBrand,
              headerOverlapsHero && 'text-[#F9F7F2]',
            )}
            href={homeHref}
          >
            <HeaderBrand
              brand={site.brand}
              siteName={site.site.name}
              linkMode={linkMode}
              siteSlug={siteSlug}
            />
          </a>
          <nav className={preview.nav} aria-label="Site navigation">
            {site.navigation.primary.map((item) => (
              <a
                key={`${item.label}-${item.pageId ?? item.href ?? ''}`}
                className={cn(
                  preview.navLink,
                  headerOverlapsHero &&
                    'text-[color-mix(in_oklch,#F9F7F2_85%,transparent)] hover:text-[#F9F7F2]',
                )}
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
        {renderedPages.map((page) => {
          const pageCollection =
            page.collectionId !== undefined
              ? collectionsById.get(page.collectionId)
              : undefined;
          const pageActiveCollection =
            activeCollection ??
            (page.type === 'collection_detail' ||
            page.type === 'collection_index'
              ? pageCollection
              : undefined);
          const collectionCtx: CollectionContext = {
            collectionsById,
            activeCollection: pageActiveCollection,
            activeEntry:
              page.type === 'collection_detail' ? activeEntry : undefined,
            exposesDetailUrlsById,
          };

          return (
            <article
              key={page.id}
              id={pageAnchor(page.slug, page.id)}
              className="w-full"
            >
              {showPageMeta ? (
                <div className={preview.pageMeta}>
                  <span>
                    {eyebrow ? `${eyebrow} · ` : ''}
                    {page.title}
                  </span>
                  <small className="text-[color-mix(in_oklch,var(--color-text)_55%,var(--color-background))]">
                    {page.slug}
                  </small>
                </div>
              ) : null}
              <div
                className={cn(
                  preview.pageStack,
                  mode === 'builder' && 'pt-12',
                )}
              >
                {page.blocks
                  .filter(
                    (block) => mode === 'builder' || !block.settings?.hidden,
                  )
                  .map((block, blockIndex) => {
                    const renderedBlock = renderSiteBlock({
                      block,
                      page,
                      blockIndex,
                      siteID: site.site.id,
                      brand: site.brand,
                      navigation: site.navigation,
                      locale:
                        'defaultLocale' in site.site
                          ? site.site.defaultLocale
                          : undefined,
                      pageAnchors,
                      pageById,
                      slugToPage,
                      linkMode,
                      siteSlug,
                      publishedBasePath,
                      collectionCtx,
                    });

                    if (!renderBlock) {
                      return renderedBlock;
                    }

                    return renderBlock({
                      block,
                      page,
                      blockIndex,
                      children: renderedBlock,
                    });
                  })}
              </div>
            </article>
          );
        })}
      </main>
      <ImageCreditsBand
        credits={'imageCredits' in site ? site.imageCredits : undefined}
      />
    </div>
    </PreviewTokenContext.Provider>
  );
}

function ImageCreditsBand({ credits }: { credits?: ImageCredit[] }) {
  if (!credits || credits.length === 0) {
    return null;
  }
  const pexels = credits.filter((credit) => credit.provider === 'pexels');
  if (pexels.length === 0) {
    return null;
  }
  return (
    <aside
      className="border-t border-[color-mix(in_oklch,var(--color-border)_45%,transparent)] bg-[color-mix(in_oklch,var(--color-background)_92%,var(--color-text))]"
      aria-label="Image credits"
    >
      <div className="mx-auto flex w-full max-w-[1180px] flex-wrap items-center gap-x-3 gap-y-1 px-6 py-4 text-xs text-[color-mix(in_oklch,var(--color-text)_72%,var(--color-background))]">
        <span>Imagery from</span>
        <a
          href="https://www.pexels.com"
          target="_blank"
          rel="noopener noreferrer"
          className="font-medium text-[var(--color-text)] hover:underline"
        >
          Pexels
        </a>
        <span aria-hidden="true">·</span>
        <span>Photos by</span>
        <span className="inline-flex flex-wrap items-center gap-x-2">
          {pexels.map((credit, index) => {
            const name = credit.author?.trim() || 'Pexels contributor';
            const isLast = index === pexels.length - 1;
            const Element = credit.authorUrl ? 'a' : 'span';
            return (
              <span
                key={`${credit.author ?? 'pexels'}-${credit.sourceUrl ?? index}`}
              >
                <Element
                  {...(credit.authorUrl
                    ? {
                        href: credit.authorUrl,
                        target: '_blank',
                        rel: 'noopener noreferrer',
                        className:
                          'font-medium text-[var(--color-text)] hover:underline',
                      }
                    : {
                        className: 'font-medium text-[var(--color-text)]',
                      })}
                >
                  {name}
                </Element>
                {!isLast ? <span aria-hidden="true">, </span> : null}
              </span>
            );
          })}
        </span>
      </div>
    </aside>
  );
}

function renderSiteBlock({
  block,
  page,
  blockIndex,
  siteID,
  brand,
  navigation,
  locale,
  pageAnchors,
  pageById,
  slugToPage,
  linkMode,
  siteSlug,
  publishedBasePath,
  collectionCtx,
}: {
  block: RenderedBlock;
  page: RenderedPage;
  blockIndex: number;
  siteID?: string;
  brand: BrandConfig;
  navigation: SiteDraft['navigation'];
  locale?: string;
  pageAnchors: Map<string, string>;
  pageById: Map<string, RoutablePage>;
  slugToPage: Map<string, RoutablePage>;
  linkMode: 'anchors' | 'published';
  siteSlug?: string;
  publishedBasePath?: string;
  collectionCtx: CollectionContext;
}) {
  switch (block.type) {
    case 'hero':
      return (
        <HeroBlock
          key={block.id}
          blockId={block.id}
          props={block.props}
          isFirst={blockIndex === 0}
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
      );
    case 'text_section':
      return (
        <TextSectionBlock
          key={block.id}
          blockId={block.id}
          props={block.props}
        />
      );
    case 'features_grid':
      return (
        <FeaturesGridBlock
          key={block.id}
          blockId={block.id}
          props={block.props}
        />
      );
    case 'cta_band':
      return (
        <CTABandBlock
          key={block.id}
          blockId={block.id}
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
      );
    case 'contact_form':
      return (
        <ContactFormBlock
          key={block.id}
          siteId={siteID}
          pageId={page.id}
          blockId={block.id}
          props={block.props}
        />
      );
    case 'image_text':
      return (
        <ImageTextBlock
          key={block.id}
          blockId={block.id}
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
      );
    case 'gallery':
      return (
        <GalleryBlock
          key={block.id}
          blockId={block.id}
          props={block.props}
          linkMode={linkMode}
          siteSlug={siteSlug}
        />
      );
    case 'testimonials':
      return (
        <TestimonialsBlock
          key={block.id}
          blockId={block.id}
          props={block.props}
          linkMode={linkMode}
          siteSlug={siteSlug}
        />
      );
    case 'pricing_packages':
      return (
        <PricingPackagesBlock
          key={block.id}
          blockId={block.id}
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
      );
    case 'faq':
      return (
        <FAQBlock key={block.id} blockId={block.id} props={block.props} />
      );
    case 'stats':
      return (
        <StatsBlock key={block.id} blockId={block.id} props={block.props} />
      );
    case 'team_profile_cards':
      return (
        <TeamProfileCardsBlock
          key={block.id}
          blockId={block.id}
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
      );
    case 'collection_list': {
      const collectionId = asText(block.props.collection);
      const resolved = collectionId
        ? collectionCtx.collectionsById.get(collectionId)
        : undefined;
      return (
        <CollectionListBlock
          key={block.id}
          props={block.props}
          collection={resolved}
          linkMode={linkMode}
          siteSlug={siteSlug}
          publishedBasePath={publishedBasePath}
        />
      );
    }
    case 'collection_index':
      return (
        <CollectionIndexBlock
          key={block.id}
          props={block.props}
          collection={collectionCtx.activeCollection}
          exposesDetailUrls={
            collectionCtx.activeCollection
              ? (collectionCtx.exposesDetailUrlsById.get(
                  collectionCtx.activeCollection.id,
                ) ?? false)
              : false
          }
          linkMode={linkMode}
          siteSlug={siteSlug}
          publishedBasePath={publishedBasePath}
        />
      );
    case 'collection_detail':
      return (
        <CollectionDetailBlock
          key={block.id}
          props={block.props}
          collection={collectionCtx.activeCollection}
          entry={collectionCtx.activeEntry}
          linkMode={linkMode}
          siteSlug={siteSlug}
        />
      );
    case 'footer':
      return (
        <FooterBlock
          key={block.id}
          props={block.props}
          brand={brand}
          navigation={navigation}
          locale={locale}
          linkMode={linkMode}
          siteSlug={siteSlug}
          resolveNavigationItemHref={(item) =>
            resolveNavigationHref(
              item,
              pageAnchors,
              pageById,
              slugToPage,
              linkMode,
              siteSlug,
              publishedBasePath,
            )
          }
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
      );
    default:
      return (
        <section key={`${block.id}-${blockIndex}`} className={preview.panel}>
          <div className={preview.panelInner}>
            <p className={text.eyebrow}>Unsupported block</p>
            <strong>{block.type}</strong>
          </div>
        </section>
      );
  }
}

const headingClass =
  '[font-family:var(--font-heading)] text-[length:var(--size-sectionHeading)] [font-weight:var(--font-headingWeight,700)] leading-[1.08] tracking-tight text-[var(--color-text)]';

const bodyClass =
  'text-[1.05rem] leading-[1.65] text-[color-mix(in_oklch,var(--color-text)_82%,var(--color-background))]';

function HeroBlock({
  blockId,
  props,
  isFirst,
  resolveHref,
  linkMode,
  siteSlug,
}: {
  blockId: string;
  props: Record<string, unknown>;
  isFirst: boolean;
  resolveHref: (href: string) => string;
  linkMode: 'anchors' | 'published';
  siteSlug?: string;
}) {
  const primary = asObject(props.primaryCta);
  const secondary = asObject(props.secondaryCta);
  const image = asImageRef(props.image);
  const variantRaw = asText(props.variant);
  const variant =
    variantRaw === 'full-page' || variantRaw === 'statement'
      ? variantRaw
      : 'standard';

  if (variant === 'statement') {
    return (
      <StatementHeroBlock
        blockId={blockId}
        eyebrow={asText(props.eyebrow)}
        headline={asText(props.headline)}
        subheadline={asText(props.subheadline)}
        primary={primary}
        secondary={secondary}
        resolveHref={resolveHref}
      />
    );
  }

  if (variant === 'full-page') {
    return (
      <FullPageHeroBlock
        blockId={blockId}
        eyebrow={asText(props.eyebrow)}
        headline={asText(props.headline)}
        subheadline={asText(props.subheadline)}
        primary={primary}
        secondary={secondary}
        image={image}
        isFirst={isFirst}
        resolveHref={resolveHref}
        linkMode={linkMode}
        siteSlug={siteSlug}
      />
    );
  }

  const layout = asText(props.layout) || 'centered';
  const hasImage = image !== null;
  const isSplit = hasImage && layout !== 'centered';

  const content = (
    <div
      className={cn(
        'grid gap-6',
        isSplit && layout === 'split-right' && 'lg:order-1',
        isSplit && layout === 'split-left' && 'lg:order-2',
      )}
    >
      <InlineEditableText
        blockId={blockId}
        path={['eyebrow']}
        value={asText(props.eyebrow)}
        placeholder="Eyebrow (optional)"
        as="p"
        className={text.eyebrow}
      />
      <InlineEditableText
        blockId={blockId}
        path={['headline']}
        value={asText(props.headline)}
        placeholder="Add a headline that says what you do"
        as="h2"
        className="max-w-[16ch] [font-family:var(--font-heading)] text-[length:var(--size-heroHeading)] [font-weight:var(--font-headingWeight,700)] leading-[0.96] tracking-[-0.02em] text-[var(--color-text)] [text-wrap:balance]"
      />
      <InlineEditableText
        blockId={blockId}
        path={['subheadline']}
        value={asText(props.subheadline)}
        placeholder="A short subheadline expands on the promise"
        multiline
        as="p"
        className="max-w-[44ch] text-[1.15rem] leading-[1.6] text-[color-mix(in_oklch,var(--color-text)_78%,var(--color-background))]"
      />
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
  );

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
          <InlineEditableImage
            blockId={blockId}
            path={['image']}
            image={image}
            emptyLabel="Add hero image"
            className={cn(
              isSplit && layout === 'split-right' && 'lg:order-2',
              isSplit && layout === 'split-left' && 'lg:order-1',
            )}
          >
            {image ? (
              <AssetImage
                image={image}
                linkMode={linkMode}
                siteSlug={siteSlug}
                className={cn(
                  'w-full rounded-[var(--radius-inner)] object-cover',
                  isSplit
                    ? 'aspect-[4/5] lg:aspect-auto lg:h-full lg:min-h-[460px]'
                    : 'aspect-[16/9] max-h-[520px]',
                )}
              />
            ) : (
              <div
                className={cn(
                  preview.imagePlaceholder,
                  'w-full rounded-[var(--radius-inner)]',
                  isSplit ? 'aspect-[4/5] min-h-[360px]' : 'aspect-[16/9] min-h-[280px]',
                )}
              >
                <span>Click to add a hero image</span>
              </div>
            )}
          </InlineEditableImage>
        </div>
      </div>
    </section>
  );
}

// StatementHeroBlock is the type-led hero (Spec 04 "statement"): a poster, not a
// panel. The section is drenched in the brand primary edge to edge, every line
// is set in the theme background color, and the drama comes entirely from the
// oversized statement type scale — no image, no card framing, no centering. The
// image and layout props are ignored by contract.
function StatementHeroBlock({
  blockId,
  eyebrow,
  headline,
  subheadline,
  primary,
  secondary,
  resolveHref,
}: {
  blockId: string;
  eyebrow: string | null;
  headline: string | null;
  subheadline: string | null;
  primary: Record<string, unknown> | null;
  secondary: Record<string, unknown> | null;
  resolveHref: (href: string) => string;
}) {
  return (
    <section
      data-statement-hero="true"
      className="w-full bg-[var(--color-primary)] text-[var(--color-background)] [--color-buttonBackground:var(--color-background)] [--color-buttonForeground:var(--color-primary)] [--color-buttonBorder:var(--color-background)] [--color-buttonGhostForeground:var(--color-background)] [--color-buttonGhostBorder:color-mix(in_oklch,var(--color-background)_55%,transparent)]"
    >
      <div className="mx-auto flex min-h-[clamp(460px,72vh,820px)] w-full max-w-[1180px] flex-col justify-between gap-14 px-[max(1.25rem,4vw)] pb-[clamp(44px,7vw,84px)] pt-[clamp(36px,5vw,64px)]">
        <InlineEditableText
          blockId={blockId}
          path={['eyebrow']}
          value={eyebrow ?? ''}
          placeholder="Eyebrow (optional)"
          as="p"
          className="text-xs font-bold uppercase tracking-[0.2em] text-[color-mix(in_oklch,var(--color-background)_72%,transparent)]"
        />
        <div className="grid gap-7">
          <InlineEditableText
            blockId={blockId}
            path={['headline']}
            value={headline ?? ''}
            placeholder="Say the one thing you stand for"
            as="h1"
            className="max-w-[14ch] [font-family:var(--font-heading)] text-[length:var(--size-statementHeading)] [font-weight:var(--font-headingWeight,700)] leading-[0.92] tracking-[-0.03em] [text-wrap:balance]"
          />
          <InlineEditableText
            blockId={blockId}
            path={['subheadline']}
            value={subheadline ?? ''}
            placeholder="One supporting line, plain and confident"
            multiline
            as="p"
            className="max-w-[44ch] text-[1.2rem] leading-[1.55] text-[color-mix(in_oklch,var(--color-background)_84%,transparent)]"
          />
          {primary || secondary ? (
            <div className={cn(preview.actionRow, 'mt-1')}>
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
                  className={cn(
                    preview.button,
                    preview.ghostButton,
                    'hover:bg-[color-mix(in_oklch,var(--color-background)_12%,transparent)]',
                  )}
                >
                  <a href={resolveHref(asText(secondary.href) || '#')}>
                    {asText(secondary.label) ?? 'Learn more'}
                  </a>
                </Button>
              ) : null}
            </div>
          ) : null}
        </div>
      </div>
    </section>
  );
}

function FullPageHeroBlock({
  blockId,
  eyebrow,
  headline,
  subheadline,
  primary,
  secondary,
  image,
  isFirst,
  resolveHref,
  linkMode,
  siteSlug,
}: {
  blockId: string;
  eyebrow: string | null;
  headline: string | null;
  subheadline: string | null;
  primary: Record<string, unknown> | null;
  secondary: Record<string, unknown> | null;
  image: { assetId: string; alt: string } | null;
  isFirst: boolean;
  resolveHref: (href: string) => string;
  linkMode: 'anchors' | 'published';
  siteSlug?: string;
}) {
  return (
    <section
      data-fullpage-hero="true"
      className={cn(
        'relative isolate flex min-h-screen w-full overflow-hidden',
        // When this hero is the first block on the page, pull it up so it
        // covers the page header area and reads as a true full viewport on load.
        isFirst && '-mt-[var(--preview-header-height,88px)] pt-[var(--preview-header-height,88px)]',
      )}
    >
      {/* The backdrop image stays at the bottom of the paint order but must
          remain hover-reachable in the editor: the layers above it are
          pointer-transparent, and `background` lets its hover controls escape
          into this stacking context instead of being trapped beneath them. */}
      <InlineEditableImage
        blockId={blockId}
        path={['image']}
        image={image}
        emptyLabel="Add full-page hero image"
        className="absolute inset-0"
        rounded={false}
        background
        overlayPosition="center"
      >
        {image ? (
          <AssetImage
            image={image}
            linkMode={linkMode}
            siteSlug={siteSlug}
            className="absolute inset-0 h-full w-full object-cover"
          />
        ) : (
          <div
            aria-hidden="true"
            className="absolute inset-0 h-full w-full"
            style={{
              background:
                'radial-gradient(circle at 30% 20%, color-mix(in oklch, var(--color-primary) 60%, var(--color-background)) 0%, var(--color-background) 70%)',
            }}
          />
        )}
      </InlineEditableImage>
      <div
        aria-hidden="true"
        className="pointer-events-none absolute inset-0 z-10 bg-[linear-gradient(to_bottom,rgba(0,0,0,0.55)_0%,rgba(0,0,0,0.15)_35%,rgba(0,0,0,0.25)_60%,rgba(0,0,0,0.75)_100%)]"
      />
      <div className="pointer-events-none relative z-20 mx-auto flex w-full max-w-[1180px] flex-col justify-end px-[max(1.25rem,4vw)] pb-[clamp(48px,8vw,96px)] pt-[clamp(72px,12vw,160px)]">
        <div className="pointer-events-auto grid max-w-[40ch] gap-5 text-[#F9F7F2] [text-wrap:balance]">
          <InlineEditableText
            blockId={blockId}
            path={['eyebrow']}
            value={eyebrow ?? ''}
            placeholder="Eyebrow (optional)"
            as="p"
            className="text-xs font-bold uppercase tracking-[0.18em] text-[color-mix(in_oklch,#F9F7F2_82%,transparent)]"
          />
          <InlineEditableText
            blockId={blockId}
            path={['headline']}
            value={headline ?? ''}
            placeholder="A bold full-page promise"
            as="h1"
            className="[font-family:var(--font-heading)] text-[length:var(--size-fullPageHeading)] [font-weight:var(--font-headingWeight,700)] leading-[0.95] tracking-[-0.02em] text-[#F9F7F2] drop-shadow-[0_2px_24px_rgba(0,0,0,0.35)]"
          />
          <InlineEditableText
            blockId={blockId}
            path={['subheadline']}
            value={subheadline ?? ''}
            placeholder="Add a supporting line"
            multiline
            as="p"
            className="max-w-[46ch] text-[1.15rem] leading-[1.55] text-[color-mix(in_oklch,#F9F7F2_88%,transparent)] drop-shadow-[0_1px_12px_rgba(0,0,0,0.3)]"
          />
          {primary || secondary ? (
            <div className={cn(preview.actionRow, 'mt-3')}>
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
                  className={cn(
                    preview.button,
                    'border-[color-mix(in_oklch,#F9F7F2_55%,transparent)] bg-transparent text-[#F9F7F2] shadow-none hover:bg-[color-mix(in_oklch,#F9F7F2_12%,transparent)]',
                  )}
                >
                  <a href={resolveHref(asText(secondary.href) || '#')}>
                    {asText(secondary.label) ?? 'Learn more'}
                  </a>
                </Button>
              ) : null}
            </div>
          ) : null}
        </div>
      </div>
    </section>
  );
}

function TextSectionBlock({
  blockId,
  props,
}: {
  blockId: string;
  props: Record<string, unknown>;
}) {
  const alignment = asText(props.alignment) || 'left';
  const width = asText(props.width) || 'default';
  const widthClass =
    width === 'narrow'
      ? 'max-w-[56ch]'
      : width === 'wide'
        ? 'max-w-[78ch]'
        : 'max-w-[68ch]';
  const alignClass =
    alignment === 'center'
      ? 'text-center'
      : alignment === 'right'
        ? 'text-right'
        : 'text-left';
  const positionClass =
    alignment === 'center' ? 'mx-auto' : alignment === 'right' ? 'ml-auto' : '';

  return (
    <section className={preview.panel}>
      <div className={preview.panelInner}>
        <div
          className={cn('grid gap-5', widthClass, positionClass, alignClass)}
        >
          <InlineEditableText
            blockId={blockId}
            path={['heading']}
            value={asText(props.heading)}
            placeholder="Section heading"
            as="h3"
            className={headingClass}
          />
          <InlineEditableText
            blockId={blockId}
            path={['body']}
            value={asText(props.body)}
            placeholder="Add a paragraph of body copy"
            multiline
            as="p"
            className={bodyClass}
          />
        </div>
      </div>
    </section>
  );
}

function FeaturesGridBlock({
  blockId,
  props,
}: {
  blockId: string;
  props: Record<string, unknown>;
}) {
  const columns = asInt(props.columns) ?? 3;
  const colsClass =
    columns === 2
      ? 'md:grid-cols-2'
      : columns === 4
        ? 'md:grid-cols-2 xl:grid-cols-4'
        : 'md:grid-cols-2 xl:grid-cols-3';
  return (
    <section className={preview.panel}>
      <div className={preview.panelInner}>
        <div className={preview.sectionHeading}>
          <InlineEditableText
            blockId={blockId}
            path={['heading']}
            value={asText(props.heading)}
            placeholder="Section heading"
            as="h3"
            className={headingClass}
          />
          <InlineEditableText
            blockId={blockId}
            path={['intro']}
            value={asText(props.intro)}
            placeholder="Add an intro paragraph (optional)"
            multiline
            as="p"
            className={bodyClass}
          />
        </div>
        <div className={cn('grid gap-x-10 gap-y-12', colsClass)}>
          {asArray(props.items).map((item, index) => {
            const value = asObject(item);
            const icon = asText(value?.icon);
            return (
              <div key={index} className={preview.feature}>
                {icon ? <p className={text.eyebrow}>{icon}</p> : null}
                <InlineEditableText
                  blockId={blockId}
                  path={['items', index, 'title']}
                  value={asText(value?.title)}
                  placeholder="Feature title"
                  as="h4"
                  className="mt-1 [font-family:var(--font-heading)] text-[1.2rem] [font-weight:var(--font-headingWeight,700)] leading-[1.15] text-[var(--color-text)]"
                />
                <InlineEditableText
                  blockId={blockId}
                  path={['items', index, 'body']}
                  value={asText(value?.body)}
                  placeholder="Short description"
                  multiline
                  as="p"
                  className="leading-[1.6] text-[color-mix(in_oklch,var(--color-text)_82%,var(--color-background))]"
                />
              </div>
            );
          })}
        </div>
      </div>
    </section>
  );
}

function CTABandBlock({
  blockId,
  props,
  resolveHref,
}: {
  blockId: string;
  props: Record<string, unknown>;
  resolveHref: (href: string) => string;
}) {
  const cta = asObject(props.cta);
  const variant = asText(props.variant) || 'primary';
  const surfaceClass =
    variant === 'accent'
      ? 'bg-[var(--color-primary)] text-[var(--color-background)] [--color-buttonBackground:var(--color-background)] [--color-buttonForeground:var(--color-primary)] [--color-buttonBorder:var(--color-background)] [--color-buttonGhostForeground:var(--color-background)] [--color-buttonGhostBorder:var(--color-background)]'
      : variant === 'secondary'
        ? 'bg-[var(--color-surface)] text-[var(--color-text)]'
        : preview.ctaSurface;
  return (
    <section className={cn(preview.panel, surfaceClass)}>
      <div
        className={cn(
          preview.panelInner,
          'flex flex-wrap items-center justify-between gap-x-12 gap-y-6',
        )}
      >
        <div className="grid max-w-[44ch] gap-3">
          <InlineEditableText
            blockId={blockId}
            path={['heading']}
            value={asText(props.heading)}
            placeholder="Make it easy to take the next step"
            as="h3"
            className="font-serif text-[clamp(1.75rem,3.2vw,2.8rem)] font-bold leading-[1.05] tracking-tight"
          />
          <InlineEditableText
            blockId={blockId}
            path={['body']}
            value={asText(props.body)}
            placeholder="One short line of support copy (optional)"
            multiline
            as="p"
            className="text-[1.05rem] leading-[1.6] opacity-85"
          />
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
  );
}

function ContactFormBlock({
  siteId,
  pageId,
  blockId,
  props,
}: {
  siteId?: string;
  pageId: string;
  blockId: string;
  props: Record<string, unknown>;
}) {
  const [values, setValues] = useState<Record<string, string>>({});
  const [honeypot, setHoneypot] = useState('');
  const [isSubmitting, setIsSubmitting] = useState(false);
  const [errorMessage, setErrorMessage] = useState('');
  const [successMessage, setSuccessMessage] = useState('');
  const fields = asFormFields(props.fields);
  const submitLabel = asText(props.submitLabel) || 'Send message';

  async function handleSubmit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    if (!siteId) {
      setErrorMessage('This form is not connected to a site yet.');
      setSuccessMessage('');
      return;
    }

    setIsSubmitting(true);
    setErrorMessage('');
    setSuccessMessage('');

    try {
      const payload = fields.reduce<Record<string, unknown>>(
        (result, field) => {
          result[field.name] = values[field.name] ?? '';
          return result;
        },
        {},
      );
      payload.hp_url = honeypot;
      const response = await submitPublicForm(siteId, blockId, payload);
      setValues({});
      setHoneypot('');
      setSuccessMessage(response.message);
    } catch (error) {
      setErrorMessage(
        error instanceof APIError ? error.message : 'Could not send message',
      );
    } finally {
      setIsSubmitting(false);
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
              <span className="text-sm font-medium text-[color-mix(in_oklch,var(--color-text)_82%,var(--color-background))]">
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

          <div
            aria-hidden="true"
            style={{
              position: 'absolute',
              left: '-10000px',
              width: '1px',
              height: '1px',
              overflow: 'hidden',
            }}
          >
            <label>
              Leave this field empty
              <input
                type="text"
                name="hp_url"
                tabIndex={-1}
                autoComplete="off"
                value={honeypot}
                onChange={(event) => setHoneypot(event.target.value)}
              />
            </label>
          </div>

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
  );
}

function ImageTextBlock({
  blockId,
  props,
  resolveHref,
  linkMode,
  siteSlug,
}: {
  blockId: string;
  props: Record<string, unknown>;
  resolveHref: (href: string) => string;
  linkMode: 'anchors' | 'published';
  siteSlug?: string;
}) {
  const cta = asObject(props.cta);
  const image = asImageRef(props.image);
  const imagePosition = asText(props.imagePosition) || 'right';
  return (
    <section className={preview.panel}>
      <div className={cn(preview.panelInner, preview.split)}>
        <div
          className={cn('grid gap-5', imagePosition === 'left' && 'lg:order-2')}
        >
          <InlineEditableText
            blockId={blockId}
            path={['heading']}
            value={asText(props.heading)}
            placeholder="Section heading"
            as="h3"
            className={headingClass}
          />
          <InlineEditableText
            blockId={blockId}
            path={['body']}
            value={asText(props.body)}
            placeholder="Add a paragraph of body copy"
            multiline
            as="p"
            className={bodyClass}
          />
          {cta ? (
            <div className="mt-2">
              <Button
                asChild
                variant="plain"
                className={cn(preview.button, preview.ghostButton)}
              >
                <a href={resolveHref(asText(cta.href) || '#')}>
                  {asText(cta.label) || 'Open link'}
                </a>
              </Button>
            </div>
          ) : null}
        </div>
        <InlineEditableImage
          blockId={blockId}
          path={['image']}
          image={image}
          emptyLabel="Add image"
          className={cn(imagePosition === 'left' && 'lg:order-1')}
        >
          {image ? (
            <AssetImage
              image={image}
              linkMode={linkMode}
              siteSlug={siteSlug}
              className="aspect-[4/5] w-full rounded-[var(--radius-inner)] object-cover lg:aspect-auto lg:h-full lg:min-h-[380px]"
            />
          ) : (
            <div
              className={cn(
                preview.imagePlaceholder,
                'min-h-[300px] w-full rounded-[var(--radius-inner)]',
              )}
            >
              <span>Click to add an image</span>
            </div>
          )}
        </InlineEditableImage>
      </div>
    </section>
  );
}

function GalleryBlock({
  blockId,
  props,
  linkMode,
  siteSlug,
}: {
  blockId: string;
  props: Record<string, unknown>;
  linkMode: 'anchors' | 'published';
  siteSlug?: string;
}) {
  const layout = asText(props.layout) || 'grid';
  const images = asArray(props.images);

  return (
    <section className={preview.panel}>
      <div className={preview.panelInner}>
        <div className={preview.sectionHeading}>
          <InlineEditableText
            blockId={blockId}
            path={['heading']}
            value={asText(props.heading)}
            placeholder="Gallery heading"
            as="h3"
            className={headingClass}
          />
          <InlineEditableText
            blockId={blockId}
            path={['intro']}
            value={asText(props.intro)}
            placeholder="Add an intro (optional)"
            multiline
            as="p"
            className={bodyClass}
          />
        </div>
        <div className={galleryGridClassName(layout)}>
          {images.map((item, index) => {
            const value = asObject(item);
            const title = asText(value?.title) || `Image ${index + 1}`;
            const caption = asText(value?.caption);
            const image = asImageRef(value?.image);
            const isSpotlight = layout === 'spotlight' && index === 0;
            return (
              <figure
                key={index}
                className={cn(
                  'grid gap-3',
                  isSpotlight && 'md:col-span-2 xl:col-span-3',
                )}
              >
                <InlineEditableImage
                  blockId={blockId}
                  path={['images', index, 'image']}
                  image={image}
                  emptyLabel="Add image"
                >
                  {image ? (
                    <AssetImage
                      image={image}
                      linkMode={linkMode}
                      siteSlug={siteSlug}
                      className={cn(
                        'w-full rounded-[var(--radius-inner)] object-cover',
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
                </InlineEditableImage>
                <figcaption className="grid gap-1">
                  <InlineEditableText
                    blockId={blockId}
                    path={['images', index, 'title']}
                    value={asText(value?.title)}
                    placeholder={`Image ${index + 1}`}
                    as="span"
                    className="text-sm font-medium text-[var(--color-text)]"
                  />
                  <InlineEditableText
                    blockId={blockId}
                    path={['images', index, 'caption']}
                    value={caption}
                    placeholder="Caption (optional)"
                    multiline
                    as="span"
                    className="text-sm leading-[1.55] text-[color-mix(in_oklch,var(--color-text)_72%,var(--color-background))]"
                  />
                </figcaption>
              </figure>
            );
          })}
        </div>
      </div>
    </section>
  );
}

function TestimonialsBlock({
  blockId,
  props,
  linkMode,
  siteSlug,
}: {
  blockId: string;
  props: Record<string, unknown>;
  linkMode: 'anchors' | 'published';
  siteSlug?: string;
}) {
  return (
    <section className={preview.panel}>
      <div className={preview.panelInner}>
        <div className={preview.sectionHeading}>
          <InlineEditableText
            blockId={blockId}
            path={['heading']}
            value={asText(props.heading)}
            placeholder="Testimonials heading"
            as="h3"
            className={headingClass}
          />
          <InlineEditableText
            blockId={blockId}
            path={['intro']}
            value={asText(props.intro)}
            placeholder="Add an intro (optional)"
            multiline
            as="p"
            className={bodyClass}
          />
        </div>
        <div className="grid gap-x-12 gap-y-10 md:grid-cols-2">
          {asArray(props.items).map((item, index) => {
            const value = asObject(item);
            const avatar = asImageRef(value?.avatar);
            return (
              <figure key={index} className={preview.quoteCard}>
                <InlineEditableText
                  blockId={blockId}
                  path={['items', index, 'quote']}
                  value={asText(value?.quote)}
                  placeholder="What did they say?"
                  multiline
                  as="blockquote"
                  className="m-0 [font-family:var(--font-heading)] text-[1.35rem] leading-[1.45] text-[var(--color-text)]"
                />
                <figcaption className="flex items-center gap-3">
                  {avatar ? (
                    <AssetImage
                      image={avatar}
                      linkMode={linkMode}
                      siteSlug={siteSlug}
                      className="size-10 rounded-full object-cover"
                    />
                  ) : null}
                  <div>
                    <InlineEditableText
                      blockId={blockId}
                      path={['items', index, 'name']}
                      value={asText(value?.name)}
                      placeholder="Name"
                      as="span"
                      className="block text-sm font-semibold text-[var(--color-text)]"
                    />
                    <InlineEditableText
                      blockId={blockId}
                      path={['items', index, 'role']}
                      value={asText(value?.role)}
                      placeholder="Role (optional)"
                      as="span"
                      className="block text-sm text-[color-mix(in_oklch,var(--color-text)_68%,var(--color-background))]"
                    />
                  </div>
                </figcaption>
              </figure>
            );
          })}
        </div>
      </div>
    </section>
  );
}

function PricingPackagesBlock({
  blockId,
  props,
  resolveHref,
}: {
  blockId: string;
  props: Record<string, unknown>;
  resolveHref: (href: string) => string;
}) {
  return (
    <section className={preview.panel}>
      <div className={preview.panelInner}>
        <div className={preview.sectionHeading}>
          <InlineEditableText
            blockId={blockId}
            path={['heading']}
            value={asText(props.heading)}
            placeholder="Pricing heading"
            as="h3"
            className={headingClass}
          />
          <InlineEditableText
            blockId={blockId}
            path={['intro']}
            value={asText(props.intro)}
            placeholder="Add an intro (optional)"
            multiline
            as="p"
            className={bodyClass}
          />
        </div>
        <div className={preview.pricingGrid}>
          {asArray(props.plans).map((item, index) => {
            const value = asObject(item);
            const cta = asObject(value?.cta);
            return (
              <article key={index} className={preview.pricingCard}>
                <div className="grid gap-2">
                  <InlineEditableText
                    blockId={blockId}
                    path={['plans', index, 'name']}
                    value={asText(value?.name)}
                    placeholder="Plan name"
                    as="h4"
                    className="[font-family:var(--font-heading)] text-[1.3rem] [font-weight:var(--font-headingWeight,700)] leading-[1.1] text-[var(--color-text)]"
                  />
                  <InlineEditableText
                    blockId={blockId}
                    path={['plans', index, 'price']}
                    value={asText(value?.price)}
                    placeholder="$0/mo"
                    as="p"
                    className="m-0 [font-family:var(--font-heading)] text-[1.8rem] [font-weight:var(--font-headingWeight,700)] leading-none text-[var(--color-text)]"
                  />
                  <InlineEditableText
                    blockId={blockId}
                    path={['plans', index, 'description']}
                    value={asText(value?.description)}
                    placeholder="Short description"
                    multiline
                    as="p"
                    className="m-0 text-sm leading-[1.55] text-[color-mix(in_oklch,var(--color-text)_78%,var(--color-background))]"
                  />
                </div>
                <ul className={preview.chipList}>
                  {asArray(value?.features).map((feature, featureIndex) => {
                    const featureValue = asObject(feature);
                    return (
                      <li key={featureIndex} className={preview.chip}>
                        <span
                          aria-hidden
                          className="text-[var(--color-primary)]"
                        >
                          &#x2014;
                        </span>
                        <span>{asText(featureValue?.text)}</span>
                      </li>
                    );
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
            );
          })}
        </div>
      </div>
    </section>
  );
}

function FAQBlock({
  blockId,
  props,
}: {
  blockId: string;
  props: Record<string, unknown>;
}) {
  return (
    <section className={preview.panel}>
      <div className={cn(preview.panelNarrow)}>
        <div className={preview.sectionHeading}>
          <InlineEditableText
            blockId={blockId}
            path={['heading']}
            value={asText(props.heading)}
            placeholder="FAQ heading"
            as="h3"
            className={headingClass}
          />
          <InlineEditableText
            blockId={blockId}
            path={['intro']}
            value={asText(props.intro)}
            placeholder="Add an intro (optional)"
            multiline
            as="p"
            className={bodyClass}
          />
        </div>
        <ul className={preview.faqList}>
          {asArray(props.items).map((item, index) => {
            const value = asObject(item);
            return (
              <li key={index} className={preview.faqItem}>
                <InlineEditableText
                  blockId={blockId}
                  path={['items', index, 'question']}
                  value={asText(value?.question)}
                  placeholder="Question"
                  as="h4"
                  className="[font-family:var(--font-heading)] text-[1.15rem] [font-weight:var(--font-headingWeight,700)] leading-[1.25] text-[var(--color-text)]"
                />
                <InlineEditableText
                  blockId={blockId}
                  path={['items', index, 'answer']}
                  value={asText(value?.answer)}
                  placeholder="Answer"
                  multiline
                  as="p"
                  className="m-0 leading-[1.65] text-[color-mix(in_oklch,var(--color-text)_82%,var(--color-background))]"
                />
              </li>
            );
          })}
        </ul>
      </div>
    </section>
  );
}

function StatsBlock({
  blockId,
  props,
}: {
  blockId: string;
  props: Record<string, unknown>;
}) {
  const items = asArray(props.items);
  const columnsClass =
    items.length >= 4
      ? 'md:grid-cols-2 xl:grid-cols-4'
      : items.length === 3
        ? 'md:grid-cols-3'
        : 'md:grid-cols-2';
  return (
    <section className={preview.panel}>
      <div className={preview.panelInner}>
        <div className={preview.sectionHeading}>
          <InlineEditableText
            blockId={blockId}
            path={['heading']}
            value={asText(props.heading)}
            placeholder="Stats heading"
            as="h3"
            className={headingClass}
          />
          <InlineEditableText
            blockId={blockId}
            path={['intro']}
            value={asText(props.intro)}
            placeholder="Add an intro (optional)"
            multiline
            as="p"
            className={bodyClass}
          />
        </div>
        <div className={cn('grid gap-x-10 gap-y-12', columnsClass)}>
          {items.map((item, index) => {
            const value = asObject(item);
            return (
              <div key={index} className="grid gap-2">
                <InlineEditableText
                  blockId={blockId}
                  path={['items', index, 'value']}
                  value={asText(value?.value)}
                  placeholder="100+"
                  as="p"
                  className="m-0 [font-family:var(--font-heading)] text-[clamp(2rem,4vw,3rem)] [font-weight:var(--font-headingWeight,700)] leading-[1.05] text-[var(--color-text)]"
                />
                <InlineEditableText
                  blockId={blockId}
                  path={['items', index, 'label']}
                  value={asText(value?.label)}
                  placeholder="Label"
                  as="p"
                  className={cn(text.eyebrow, 'm-0')}
                />
                <InlineEditableText
                  blockId={blockId}
                  path={['items', index, 'description']}
                  value={asText(value?.description)}
                  placeholder="Description (optional)"
                  multiline
                  as="p"
                  className="m-0 text-sm leading-[1.55] text-[color-mix(in_oklch,var(--color-text)_78%,var(--color-background))]"
                />
              </div>
            );
          })}
        </div>
      </div>
    </section>
  );
}

function TeamProfileCardsBlock({
  blockId,
  props,
  resolveHref,
  linkMode,
  siteSlug,
}: {
  blockId: string;
  props: Record<string, unknown>;
  resolveHref: (href: string) => string;
  linkMode: 'anchors' | 'published';
  siteSlug?: string;
}) {
  return (
    <section className={preview.panel}>
      <div className={preview.panelInner}>
        <div className={preview.sectionHeading}>
          <InlineEditableText
            blockId={blockId}
            path={['heading']}
            value={asText(props.heading)}
            placeholder="Team heading"
            as="h3"
            className={headingClass}
          />
          <InlineEditableText
            blockId={blockId}
            path={['intro']}
            value={asText(props.intro)}
            placeholder="Add an intro (optional)"
            multiline
            as="p"
            className={bodyClass}
          />
        </div>
        <div className={preview.cardGrid}>
          {asArray(props.people).map((item, index) => {
            const value = asObject(item);
            const photo = asImageRef(value?.photo);
            return (
              <div key={index} className="grid gap-4">
                <InlineEditableImage
                  blockId={blockId}
                  path={['people', index, 'photo']}
                  image={photo}
                  emptyLabel="Add photo"
                >
                  {photo ? (
                    <AssetImage
                      image={photo}
                      linkMode={linkMode}
                      siteSlug={siteSlug}
                      className="aspect-[4/5] w-full rounded-[var(--radius-inner)] object-cover"
                    />
                  ) : (
                    <div
                      className={cn(
                        preview.imagePlaceholder,
                        'aspect-[4/5] min-h-0 w-full rounded-[var(--radius-inner)]',
                      )}
                    >
                      <span>{asText(value?.name) || 'Profile image slot'}</span>
                    </div>
                  )}
                </InlineEditableImage>
                <div className="grid gap-1">
                  <InlineEditableText
                    blockId={blockId}
                    path={['people', index, 'name']}
                    value={asText(value?.name)}
                    placeholder="Name"
                    as="h4"
                    className="[font-family:var(--font-heading)] text-[1.2rem] [font-weight:var(--font-headingWeight,700)] leading-[1.15] text-[var(--color-text)]"
                  />
                  <InlineEditableText
                    blockId={blockId}
                    path={['people', index, 'role']}
                    value={asText(value?.role)}
                    placeholder="Role"
                    as="p"
                    className="m-0 text-sm font-medium uppercase tracking-[0.08em] text-[color-mix(in_oklch,var(--color-text)_62%,var(--color-background))]"
                  />
                </div>
                <InlineEditableText
                  blockId={blockId}
                  path={['people', index, 'bio']}
                  value={asText(value?.bio)}
                  placeholder="Short bio"
                  multiline
                  as="p"
                  className="m-0 leading-[1.6] text-[color-mix(in_oklch,var(--color-text)_82%,var(--color-background))]"
                />
                {asArray(value?.links).length > 0 ? (
                  <div className="flex flex-wrap gap-x-5 gap-y-2">
                    {asArray(value?.links).map((link, linkIndex) => {
                      const linkValue = asObject(link);
                      return (
                        <a
                          key={linkIndex}
                          className={preview.footerLink}
                          href={resolveHref(asText(linkValue?.href) || '#')}
                        >
                          {asText(linkValue?.label) || 'Open'}
                        </a>
                      );
                    })}
                  </div>
                ) : null}
              </div>
            );
          })}
        </div>
      </div>
    </section>
  );
}

function FooterBlock({
  props,
  brand,
  navigation,
  locale,
  linkMode,
  siteSlug,
  resolveNavigationItemHref,
  resolveHref,
}: {
  props: Record<string, unknown>;
  brand: BrandConfig;
  navigation: SiteDraft['navigation'];
  locale?: string;
  linkMode: 'anchors' | 'published';
  siteSlug?: string;
  resolveNavigationItemHref: (item: {
    pageId?: string;
    href?: string;
  }) => string;
  resolveHref: (href: string) => string;
}) {
  const brandName = resolveBrandName(brand, '');
  const contact = asFooterContact(props.contact);
  const footerNavigation =
    (navigation.footer ?? []).length > 0
      ? (navigation.footer ?? [])
      : asArray(props.navigationLinks);
  const showBrand = props.showBrand !== false;
  const showMadeWith = props.showMadeWith !== false;

  return (
    <footer className={preview.footerShell}>
      <div className={preview.footerInner}>
        <div className="grid gap-3">
          {showBrand ? (
            <div className="flex items-center gap-3">
              {brand?.logo ? (
                <AssetImage
                  image={{
                    assetId: brand.logo.assetId,
                    alt: brand.logo.alt,
                  }}
                  linkMode={linkMode}
                  siteSlug={siteSlug}
                  className="h-10 w-10 rounded-full border border-[color-mix(in_oklch,var(--color-border)_52%,transparent)] object-cover"
                />
              ) : null}
              <h3 className="m-0 [font-family:var(--font-heading)] text-[1.4rem] [font-weight:var(--font-headingWeight,700)] leading-tight text-[var(--color-text)]">
                {brandName}
              </h3>
            </div>
          ) : null}
          {asText(props.tagline) ? (
            <p className="m-0 max-w-[44ch] text-[color-mix(in_oklch,var(--color-text)_78%,var(--color-background))]">
              {asText(props.tagline)}
            </p>
          ) : null}
          <FooterContactDetails
            contact={contact}
            fallbackLine={asText(props.contactLine)}
            locale={locale}
          />
        </div>
        <div className="grid gap-4 md:justify-self-end md:text-right">
          {footerNavigation.length > 0 ? (
            <div className={cn(preview.footerLinks, 'md:justify-end')}>
              {footerNavigation.map((item, index) => {
                const value = asObject(item);
                return (
                  <a
                    key={index}
                    className={preview.footerLink}
                    href={resolveNavigationItemHref({
                      pageId: asText(value?.pageId) || undefined,
                      href: asText(value?.href) || undefined,
                    })}
                  >
                    {asText(value?.label)}
                  </a>
                );
              })}
            </div>
          ) : null}
          {asArray(props.socialLinks).length > 0 ? (
            <div className={cn(preview.footerLinks, 'md:justify-end')}>
              {asArray(props.socialLinks).map((item, index) => {
                const value = asObject(item);
                return (
                  <a
                    key={index}
                    className={preview.footerLink}
                    href={resolveHref(asText(value?.href) || '#')}
                  >
                    {asText(value?.label)}
                  </a>
                );
              })}
            </div>
          ) : null}
        </div>
      </div>
      {asText(props.copyright) || showMadeWith ? (
        <div className="mx-auto mt-10 flex w-full max-w-[1180px] flex-wrap items-center justify-between gap-x-6 gap-y-2 border-t border-[color-mix(in_oklch,var(--color-border)_45%,transparent)] pt-6">
          {asText(props.copyright) ? (
            <small className="text-xs text-[color-mix(in_oklch,var(--color-text)_62%,var(--color-background))]">
              {asText(props.copyright)}
            </small>
          ) : (
            <span />
          )}
          {showMadeWith ? (
            <a
              href={MADE_WITH_URL}
              target="_blank"
              rel="noopener noreferrer"
              className="text-xs text-[color-mix(in_oklch,var(--color-text)_58%,var(--color-background))] hover:text-[var(--color-text)] hover:underline"
            >
              {madeWithLabel(locale)}
            </a>
          ) : null}
        </div>
      ) : null}
    </footer>
  );
}

const MADE_WITH_URL = 'https://snaelda.io';

function madeWithLabel(locale?: string) {
  return normalizeFooterLocale(locale) === 'is'
    ? 'Gert með Snældu'
    : 'Made with Snælda';
}

// headerLogoClass scales the header logo per brand.logo.size. Logos render at
// their natural aspect ratio (no crop, no border) so wide lockups survive.
const headerLogoClass: Record<string, string> = {
  small: 'h-9 max-w-[160px]',
  medium: 'h-12 max-w-[240px]',
  large: 'h-16 max-w-[320px]',
};

// headerHeightByLogoSize keeps --preview-header-height in step with the header
// the logo size actually produces (py-6 padding + logo height + border + the
// baseline row's line-height slack). Values overshoot slightly on purpose: a
// full-page hero tucking a couple of pixels further under the header is
// invisible, a gap above it is not.
const headerHeightByLogoSize: Record<string, string> = {
  small: '88px',
  medium: '104px',
  large: '120px',
};

function HeaderBrand({
  brand,
  siteName,
  linkMode,
  siteSlug,
}: {
  brand: BrandConfig;
  siteName: string;
  linkMode: 'anchors' | 'published';
  siteSlug?: string;
}) {
  const brandName = resolveBrandName(brand, siteName);
  const logo = brand?.logo;

  return (
    <span className="flex items-center gap-3">
      {logo ? (
        <AssetImage
          image={{
            assetId: logo.assetId,
            alt: logo.alt,
          }}
          linkMode={linkMode}
          siteSlug={siteSlug}
          className={cn(
            'w-auto object-contain',
            headerLogoClass[logo.size ?? 'small'] ?? headerLogoClass.small,
          )}
        />
      ) : null}
      {/* A lockup logo already carries the name; keep it in the DOM for
          assistive tech and crawlers, just not painted twice. */}
      {logo?.hideName ? (
        <span className="sr-only">{brandName}</span>
      ) : (
        <span>{brandName}</span>
      )}
    </span>
  );
}

function FooterContactDetails({
  contact,
  fallbackLine,
  locale,
}: {
  contact: FooterContact;
  fallbackLine: string;
  locale?: string;
}) {
  const addressLines = formatFooterAddress(contact.address);
  const hours = contact.hours ?? [];
  const hasContent =
    addressLines.length > 0 ||
    Boolean(contact.phone) ||
    Boolean(contact.email) ||
    hours.length > 0;

  if (!hasContent && !fallbackLine) {
    return null;
  }

  return (
    <div className="grid gap-1 text-sm text-[color-mix(in_oklch,var(--color-text)_72%,var(--color-background))]">
      {addressLines.length > 0 ? (
        <address className="m-0 not-italic whitespace-pre-line">
          {addressLines.join('\n')}
        </address>
      ) : null}
      {contact.phone ? (
        <p className="m-0">
          <a
            className={preview.footerLink}
            href={`tel:${contact.phone.replace(/\s+/g, '')}`}
          >
            {contact.phone}
          </a>
        </p>
      ) : null}
      {contact.email ? (
        <p className="m-0">
          <a className={preview.footerLink} href={`mailto:${contact.email}`}>
            {contact.email}
          </a>
        </p>
      ) : null}
      {hours.length > 0 ? (
        <dl className="m-0 mt-1 grid grid-cols-[auto_1fr] gap-x-3 gap-y-0.5">
          {hours.map((entry, index) => (
            <div key={index} className="contents">
              <dt className="m-0 font-medium">{weekdayLabel(entry.day, locale)}</dt>
              <dd className="m-0">{formatFooterHours(entry, locale)}</dd>
            </div>
          ))}
        </dl>
      ) : null}
      {!hasContent && fallbackLine ? (
        <p className="m-0">{fallbackLine}</p>
      ) : null}
    </div>
  );
}

function normalizeFooterLocale(locale?: string): 'is' | 'en' {
  return (locale ?? '').toLowerCase().startsWith('is') ? 'is' : 'en';
}

const FOOTER_WEEKDAY_LABELS: Record<'is' | 'en', Record<string, string>> = {
  en: {
    monday: 'Monday',
    tuesday: 'Tuesday',
    wednesday: 'Wednesday',
    thursday: 'Thursday',
    friday: 'Friday',
    saturday: 'Saturday',
    sunday: 'Sunday',
  },
  is: {
    monday: 'Mánudagur',
    tuesday: 'Þriðjudagur',
    wednesday: 'Miðvikudagur',
    thursday: 'Fimmtudagur',
    friday: 'Föstudagur',
    saturday: 'Laugardagur',
    sunday: 'Sunnudagur',
  },
};

function weekdayLabel(day: string, locale?: string) {
  return FOOTER_WEEKDAY_LABELS[normalizeFooterLocale(locale)][day] ?? day;
}

function formatFooterHours(entry: FooterHours, locale?: string) {
  if (entry.closed) {
    return normalizeFooterLocale(locale) === 'is' ? 'Lokað' : 'Closed';
  }
  if (entry.opens && entry.closes) {
    return `${entry.opens}–${entry.closes}`;
  }
  return entry.opens || entry.closes || '';
}

function formatFooterAddress(address: FooterContact['address']): string[] {
  if (!address) {
    return [];
  }
  const cityLine = [address.postalCode, address.city]
    .filter(Boolean)
    .join(' ')
    .trim();
  return [address.street, cityLine, address.region, address.country]
    .map((line) => (line ?? '').trim())
    .filter(Boolean);
}

function CollectionListBlock({
  props,
  collection,
  linkMode,
  siteSlug,
  publishedBasePath,
}: {
  props: Record<string, unknown>;
  collection?: Collection;
  linkMode: 'anchors' | 'published';
  siteSlug?: string;
  publishedBasePath?: string;
}) {
  const layout = asText(props.layout) || 'grid';
  const limit = asInt(props.limit) ?? 6;
  const cta = asObject(props.cta);
  const entries = filterPublishedEntries(collection?.entries);
  const visible = entries.slice(0, Math.max(1, limit));
  return (
    <section className={preview.panel}>
      <div className={preview.panelInner}>
        <div className={preview.sectionHeading}>
          {asText(props.heading) ? (
            <h3 className={headingClass}>{asText(props.heading)}</h3>
          ) : null}
          {asText(props.intro) ? (
            <p className={bodyClass}>{asText(props.intro)}</p>
          ) : null}
        </div>
        {visible.length === 0 ? (
          <p className={cn(bodyClass, 'm-0')}>No entries to show yet.</p>
        ) : (
          <div className={collectionGridClassName(layout)}>
            {visible.map((entry) => (
              <CollectionEntryCard
                key={entry.id}
                entry={entry}
                collection={collection!}
                linkMode={linkMode}
                siteSlug={siteSlug}
                publishedBasePath={publishedBasePath}
              />
            ))}
          </div>
        )}
        {cta ? (
          <div className={cn(preview.actionRow, 'mt-10')}>
            <Button asChild variant="plain" className={preview.button}>
              <a
                href={resolveCollectionListCtaHref(
                  asText(cta.href),
                  collection,
                  linkMode,
                  siteSlug,
                  publishedBasePath,
                )}
              >
                {asText(cta.label) ?? 'Browse all'}
              </a>
            </Button>
          </div>
        ) : null}
      </div>
    </section>
  );
}

function CollectionIndexBlock({
  props,
  collection,
  exposesDetailUrls,
  linkMode,
  siteSlug,
  publishedBasePath,
}: {
  props: Record<string, unknown>;
  collection?: Collection;
  exposesDetailUrls: boolean;
  linkMode: 'anchors' | 'published';
  siteSlug?: string;
  publishedBasePath?: string;
}) {
  const layout = asText(props.layout) || 'grid';
  const sort =
    asText(props.sort) || collection?.settings?.defaultSort || 'manual';
  const entries = sortEntries(
    filterPublishedEntries(collection?.entries),
    sort,
  );
  return (
    <section className={preview.panel}>
      <div className={preview.panelInner}>
        <div className={preview.sectionHeading}>
          <h3 className={headingClass}>
            {asText(props.heading) || collection?.pluralLabel || 'All entries'}
          </h3>
          {asText(props.intro) ? (
            <p className={bodyClass}>{asText(props.intro)}</p>
          ) : null}
        </div>
        {entries.length === 0 ? (
          <p className={cn(bodyClass, 'm-0')}>No entries to show yet.</p>
        ) : (
          <div className={collectionGridClassName(layout)}>
            {entries.map((entry) => (
              <CollectionEntryCard
                key={entry.id}
                entry={entry}
                collection={collection!}
                linkMode={linkMode}
                siteSlug={siteSlug}
                publishedBasePath={publishedBasePath}
                linkToDetail={exposesDetailUrls}
              />
            ))}
          </div>
        )}
      </div>
    </section>
  );
}

function CollectionDetailBlock({
  props,
  collection,
  entry,
  linkMode,
  siteSlug,
}: {
  props: Record<string, unknown>;
  collection?: Collection;
  entry?: CollectionEntry;
  linkMode: 'anchors' | 'published';
  siteSlug?: string;
}) {
  if (!entry) {
    return (
      <section className={preview.panel}>
        <div className={preview.panelInner}>
          <div className={preview.sectionHeading}>
            <p className={text.eyebrow}>
              {collection?.singularLabel ?? 'Collection'} template
            </p>
            <h3 className={headingClass}>
              {asText(props.heading) || 'Detail template'}
            </h3>
            <p className={bodyClass}>
              This template renders one page per published entry at publish
              time.
            </p>
          </div>
        </div>
      </section>
    );
  }

  const layout = asText(props.layout) || 'default';
  const cover =
    asImageRef(entry.fields.cover) || asImageRef(entry.fields.image);
  const title = asText(props.heading) || asText(entry.fields.title) || '';
  const summary = asText(entry.fields.summary);
  const details = asText(entry.fields.details);
  const widthClass =
    layout === 'narrow'
      ? 'max-w-[60ch]'
      : layout === 'wide'
        ? 'max-w-[1180px]'
        : 'max-w-[80ch]';

  return (
    <section className={preview.panel}>
      <div className={preview.panelInner}>
        <div className={cn('grid gap-10', widthClass)}>
          <div className="grid gap-4">
            {collection ? (
              <p className={text.eyebrow}>{collection.singularLabel}</p>
            ) : null}
            {title ? (
              <h2 className="[font-family:var(--font-heading)] text-[clamp(2rem,4vw,3.4rem)] [font-weight:var(--font-headingWeight,700)] leading-[1.05] tracking-tight text-[var(--color-text)]">
                {title}
              </h2>
            ) : null}
            {summary ? (
              <p className="text-[1.15rem] leading-[1.6] text-[color-mix(in_oklch,var(--color-text)_82%,var(--color-background))]">
                {summary}
              </p>
            ) : null}
          </div>
          {cover ? (
            <AssetImage
              image={cover}
              linkMode={linkMode}
              siteSlug={siteSlug}
              className="w-full rounded-[var(--radius-inner)] object-cover aspect-[16/9]"
            />
          ) : null}
          {details ? (
            <div className="grid gap-4 text-[1.05rem] leading-[1.7] text-[color-mix(in_oklch,var(--color-text)_85%,var(--color-background))]">
              {details.split(/\n{2,}/).map((paragraph, index) => (
                <p key={index}>{paragraph}</p>
              ))}
            </div>
          ) : null}
        </div>
      </div>
    </section>
  );
}

function CollectionEntryCard({
  entry,
  collection,
  linkMode,
  siteSlug,
  publishedBasePath,
  linkToDetail = true,
}: {
  entry: CollectionEntry;
  collection: Collection;
  linkMode: 'anchors' | 'published';
  siteSlug?: string;
  publishedBasePath?: string;
  linkToDetail?: boolean;
}) {
  const title = asText(entry.fields.title) || entry.slug;
  const summary = asText(entry.fields.summary);
  const cover =
    asImageRef(entry.fields.cover) || asImageRef(entry.fields.image);
  const cardClass =
    'group grid gap-4 rounded-[var(--radius-inner)] border border-[color-mix(in_oklch,var(--color-border)_45%,transparent)] bg-[var(--color-surface)] p-5 transition-transform';

  const inner = (
    <>
      {cover ? (
        <AssetImage
          image={cover}
          linkMode={linkMode}
          siteSlug={siteSlug}
          className="aspect-[4/3] w-full rounded-[var(--radius-inner)] object-cover"
        />
      ) : (
        <div className={cn(preview.imagePlaceholder, 'aspect-[4/3] min-h-0')}>
          <span className="text-sm">{title}</span>
        </div>
      )}
      <div className="grid gap-2">
        <h4 className="m-0 [font-family:var(--font-heading)] text-[1.2rem] [font-weight:var(--font-headingWeight,700)] leading-[1.2] text-[var(--color-text)]">
          {title}
        </h4>
        {summary ? (
          <p className="m-0 text-sm leading-[1.55] text-[color-mix(in_oklch,var(--color-text)_78%,var(--color-background))]">
            {summary}
          </p>
        ) : null}
      </div>
    </>
  );

  if (!linkToDetail) {
    return <div className={cardClass}>{inner}</div>;
  }

  const href = buildCollectionEntryHref(
    collection,
    entry,
    linkMode,
    siteSlug,
    publishedBasePath,
  );
  return (
    <a href={href} className={cn(cardClass, 'hover:-translate-y-px')}>
      {inner}
    </a>
  );
}

function buildCollectionEntryHref(
  collection: Collection,
  entry: CollectionEntry,
  linkMode: 'anchors' | 'published',
  siteSlug?: string,
  publishedBasePath?: string,
) {
  const entryPath = `/${collection.slug}/${entry.slug}`;
  if (linkMode === 'published') {
    const basePath = resolvePublishedBasePath(siteSlug, publishedBasePath);
    return `${basePath}${entryPath}`;
  }
  return entryPath;
}

function resolveCollectionListCtaHref(
  href: string,
  collection: Collection | undefined,
  linkMode: 'anchors' | 'published',
  siteSlug?: string,
  publishedBasePath?: string,
) {
  if (href) {
    const safeHref = sanitizeRenderableHref(href);
    if (!safeHref) {
      return '#';
    }
    if (safeHref.startsWith('/') && linkMode === 'published') {
      const basePath = resolvePublishedBasePath(siteSlug, publishedBasePath);
      return `${basePath}${safeHref}`;
    }
    return safeHref;
  }
  if (collection) {
    if (linkMode === 'published') {
      const basePath = resolvePublishedBasePath(siteSlug, publishedBasePath);
      return `${basePath}/${collection.slug}`;
    }
    return `/${collection.slug}`;
  }
  return '#';
}

function filterPublishedEntries(entries?: CollectionEntry[]) {
  return (entries ?? []).filter(
    (entry) => !entry.status || entry.status === 'published',
  );
}

function sortEntries(entries: CollectionEntry[], sort: string) {
  if (sort === 'title') {
    return [...entries].sort((a, b) =>
      asText(a.fields.title).localeCompare(asText(b.fields.title), undefined, {
        sensitivity: 'base',
      }),
    );
  }
  // Manual / newest / oldest fall back to entry.sortOrder since entries
  // don't carry publishedAt in the snapshot. `newest` returns highest
  // SortOrder first; `oldest` returns lowest first.
  return [...entries].sort((a, b) => {
    const left = a.sortOrder ?? 0;
    const right = b.sortOrder ?? 0;
    if (sort === 'newest') return right - left;
    return left - right;
  });
}

// computeCollectionDetailExposure mirrors the backend rule in publishing.go:
// a collection exposes detail URLs when settings.exposeDetailUrls is true or
// a collection_detail page binds to it.
function computeCollectionDetailExposure(
  collections: Collection[],
  pages: Array<{ type?: string; collectionId?: string }>,
) {
  const hasDetailTemplate = new Set<string>();
  for (const page of pages) {
    if (page.type === 'collection_detail' && page.collectionId) {
      hasDetailTemplate.add(page.collectionId);
    }
  }
  const result = new Map<string, boolean>();
  for (const collection of collections) {
    result.set(
      collection.id,
      Boolean(collection.settings?.exposeDetailUrls) ||
        hasDetailTemplate.has(collection.id),
    );
  }
  return result;
}

function collectionGridClassName(layout: string) {
  if (layout === 'list') {
    return 'grid gap-5';
  }
  return 'grid gap-6 md:grid-cols-2 xl:grid-cols-3';
}

function asText(value: unknown) {
  return typeof value === 'string' ? value : '';
}

function asImageRef(value: unknown) {
  const object = asObject(value);
  if (!object) {
    return null;
  }

  const assetId = asText(object.assetId);
  if (!assetId) {
    return null;
  }

  return {
    assetId,
    alt: asText(object.alt),
  };
}

const FOOTER_WEEKDAYS = [
  'monday',
  'tuesday',
  'wednesday',
  'thursday',
  'friday',
  'saturday',
  'sunday',
];

function asFooterContact(value: unknown): FooterContact {
  const object = asObject(value);
  if (!object) {
    return {};
  }

  return {
    address: asFooterAddress(object.address),
    phone: asText(object.phone) || undefined,
    email: asText(object.email) || undefined,
    hours: asFooterHours(object.hours),
  };
}

function asFooterAddress(value: unknown): FooterContact['address'] {
  // Tolerate legacy free-text addresses by folding them into `street`.
  if (typeof value === 'string') {
    const street = value.trim();
    return street ? { street } : undefined;
  }
  const object = asObject(value);
  if (!object) {
    return undefined;
  }
  const address = {
    street: asText(object.street) || undefined,
    city: asText(object.city) || undefined,
    postalCode: asText(object.postalCode) || undefined,
    region: asText(object.region) || undefined,
    country: asText(object.country) || undefined,
  };
  return Object.values(address).some(Boolean) ? address : undefined;
}

function asFooterHours(value: unknown): FooterContact['hours'] {
  if (!Array.isArray(value)) {
    return undefined;
  }
  const entries: FooterHours[] = [];
  for (const raw of value) {
    const object = asObject(raw);
    if (!object) {
      continue;
    }
    const day = asText(object.day).toLowerCase();
    if (!FOOTER_WEEKDAYS.includes(day)) {
      continue;
    }
    entries.push({
      day,
      opens: asText(object.opens) || undefined,
      closes: asText(object.closes) || undefined,
      closed: object.closed === true,
    });
  }
  return entries.length > 0 ? entries : undefined;
}

function resolveBrandName(brand: BrandConfig | undefined, fallback: string) {
  return asText(brand?.businessName) || fallback || 'Business';
}

function asArray(value: unknown) {
  return Array.isArray(value) ? value : [];
}

function asObject(value: unknown) {
  if (!value || typeof value !== 'object' || Array.isArray(value)) {
    return null;
  }
  return value as Record<string, unknown>;
}

function asInt(value: unknown): number | null {
  if (typeof value === 'number' && Number.isFinite(value)) {
    return Math.trunc(value);
  }
  if (typeof value === 'string') {
    const parsed = Number.parseInt(value, 10);
    return Number.isFinite(parsed) ? parsed : null;
  }
  return null;
}

function asFormFields(value: unknown) {
  return asArray(value)
    .map((entry) => asObject(entry))
    .filter(
      (
        entry,
      ): entry is {
        name?: unknown;
        label?: unknown;
        type?: unknown;
        required?: unknown;
        options?: unknown;
      } => entry !== null,
    )
    .map((field) => ({
      name: asText(field.name),
      label: asText(field.label) || asText(field.name),
      type: asText(field.type),
      required: Boolean(field.required),
      options: asStringArray(field.options),
    }))
    .filter((field) => field.name && field.type);
}

function asStringArray(value: unknown) {
  if (!Array.isArray(value)) {
    return [];
  }
  return value.filter((entry): entry is string => typeof entry === 'string');
}

function formPlaceholder(field: { name: string; type: string }) {
  switch (field.type) {
    case 'email':
      return 'name@example.com';
    case 'phone':
      return '+46 70 000 00 00';
    case 'message':
      return 'Tell me a little about the project.';
    default:
      return field.name;
  }
}

function AssetImage({
  image,
  linkMode,
  siteSlug,
  className,
}: {
  image: { assetId: string; alt: string };
  linkMode: 'anchors' | 'published';
  siteSlug?: string;
  className: string;
}) {
  const previewToken = useContext(PreviewTokenContext);
  const src =
    linkMode === 'published' && siteSlug
      ? buildPublishedAssetURL(siteSlug, image.assetId)
      : previewToken
        ? buildPreviewAssetURL(previewToken, image.assetId)
        : buildDraftAssetURL(image.assetId);

  return <img src={src} alt={image.alt} className={className} loading="lazy" />;
}

function galleryGridClassName(layout: string) {
  switch (layout) {
    case 'masonry':
      return 'grid gap-6 md:grid-cols-2 xl:grid-cols-3';
    case 'spotlight':
      return 'grid gap-6 md:grid-cols-2 xl:grid-cols-3';
    default:
      return 'grid gap-6 md:grid-cols-2 xl:grid-cols-3';
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
      const page = pageById.get(item.pageId);
      if (page) {
        return buildPublishedPageHref(page.slug, siteSlug, publishedBasePath);
      }
    }
    return `#${pageAnchors.get(item.pageId)}`;
  }
  return resolvePageHref(
    item.href ?? '#',
    slugToPage,
    linkMode,
    siteSlug,
    publishedBasePath,
  );
}

function resolvePageHref(
  href: string,
  slugToPage: Map<string, RoutablePage>,
  linkMode: 'anchors' | 'published',
  siteSlug?: string,
  publishedBasePath?: string,
) {
  const safeHref = sanitizeRenderableHref(href);
  if (!safeHref) {
    return '#';
  }
  if (!safeHref.startsWith('/')) {
    return safeHref;
  }
  const page = slugToPage.get(safeHref);
  if (!page) {
    return safeHref;
  }
  if (linkMode === 'published') {
    return buildPublishedPageHref(page.slug, siteSlug, publishedBasePath);
  }
  return `#${pageAnchor(page.slug, page.id)}`;
}

function sanitizeRenderableHref(href: string) {
  const trimmed = href.trim();
  if (!trimmed) {
    return '';
  }
  if (
    trimmed.startsWith('/') ||
    trimmed.startsWith('#') ||
    trimmed.startsWith('mailto:') ||
    trimmed.startsWith('tel:')
  ) {
    return trimmed;
  }
  try {
    const url = new URL(trimmed);
    if (url.protocol === 'http:' || url.protocol === 'https:') {
      return trimmed;
    }
  } catch {
    return '';
  }
  return '';
}

function buildPublishedPageHref(
  pageSlug: string,
  siteSlug?: string,
  publishedBasePath?: string,
) {
  const basePath = resolvePublishedBasePath(siteSlug, publishedBasePath);
  if (pageSlug === '/') {
    return basePath || '/';
  }
  return `${basePath}${pageSlug}`;
}

function resolvePublishedBasePath(
  siteSlug?: string,
  publishedBasePath?: string,
) {
  if (typeof publishedBasePath === 'string') {
    if (publishedBasePath === '/') {
      return '';
    }
    return publishedBasePath.replace(/\/+$/, '');
  }
  if (!siteSlug) {
    return '';
  }
  return `/public/${siteSlug}`;
}

function pageAnchor(slug: string, pageId: string) {
  if (slug === '/') {
    return 'page-home';
  }
  const cleaned = slug
    .replaceAll('/', '-')
    .replace(/[^a-zA-Z0-9_-]/g, '')
    .replace(/^-+/, '');
  if (!cleaned) {
    return `page-${pageId}`;
  }
  return `page-${cleaned}`;
}
