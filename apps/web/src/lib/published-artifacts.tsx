import { renderToStaticMarkup } from "react-dom/server";
import type {
  BrandConfig,
  BlockBinding,
  Collection,
  CollectionEntry,
  FooterContact,
  PublishedSnapshot,
  SiteVersion,
} from "@/lib/api";
import { SiteDraftRenderer } from "@/components/SiteDraftRenderer";
import { buildPublishedAssetURL } from "@/lib/assets";
import { normalizePagePath } from "@/lib/public-site";
import { buildSiteThemeStyle } from "@/lib/site-theme";

export type PublishedArtifactRenderInput = {
  publicBaseURL: string;
  siteSlug: string;
  hostname?: string;
  version: SiteVersion;
  snapshot: PublishedSnapshot;
};

export type PublishedArtifactFile = {
  path: string;
  contentType: string;
  body: string;
};

export type PublishedArtifactBundle = {
  schemaVersion: "published_artifacts.v1";
  files: PublishedArtifactFile[];
};

type SnapshotPage = PublishedSnapshot["pages"][number];
type SnapshotBlock = SnapshotPage["blocks"][number];

type PublishedArtifactManifest = {
  schemaVersion: "published_artifacts.v1";
  siteSlug: string;
  hostname?: string;
  defaultLocale?: string;
  version: SiteVersion;
  pages: Array<{
    pageId: string;
    pagePath: string;
    filePath: string;
    title: string;
    description: string;
    canonicalUrl: string;
    ogImageUrl?: string;
    localBusinessJsonLd?: Record<string, unknown>;
  }>;
  files: string[];
};

type RenderedRoute = {
  pageId: string;
  pagePath: string;
  filePath: string;
  title: string;
  description: string;
  canonicalUrl: string;
  ogImageUrl?: string;
  localBusinessJsonLd?: Record<string, unknown>;
  html: string;
};

// publishedSiteLocale resolves a published snapshot's content locale down to a
// supported primary subtag ("is-IS" -> "is"), defaulting to "en". Published
// artifacts and their metadata must follow the site's own locale, not the
// visitor's (Spec 22).
export function publishedSiteLocale(snapshot: PublishedSnapshot): string {
  const raw = snapshot.site.defaultLocale?.trim().toLowerCase() ?? "";
  const primary = raw.split(/[-_]/)[0];
  return primary === "is" ? "is" : "en";
}

// fallbackPageDescription supplies a locale-appropriate meta description when a
// page and its site both omit one, so an Icelandic site never falls back to
// English boilerplate.
function fallbackPageDescription(siteName: string, locale: string): string {
  return locale === "is" ? `Heimsæktu ${siteName}.` : `Visit ${siteName}.`;
}

function fallbackEntryDescription(siteName: string, locale: string): string {
  return locale === "is"
    ? `Skoðaðu meira frá ${siteName}.`
    : `Discover more from ${siteName}.`;
}

export function buildPublishedArtifactBundle(
  input: PublishedArtifactRenderInput,
): PublishedArtifactBundle {
  const snapshot = filterPublishedSnapshot(input.snapshot);
  const collectionsById = new Map<string, Collection>();
  for (const collection of snapshot.collections ?? []) {
    collectionsById.set(collection.id, collection);
  }

  const renderedRoutes: RenderedRoute[] = [];
  const seenPaths = new Set<string>();

  for (const page of snapshot.pages) {
    if (page.type === "collection_detail") {
      const collection = page.collectionId
        ? collectionsById.get(page.collectionId)
        : undefined;
      if (!collection) {
        // ValidatePublishedSnapshot rejects orphan templates upstream. If we
        // somehow reach here, emit nothing; the manifest validator will then
        // surface the missing pages clearly.
        continue;
      }
      const publishedEntries = (collection.entries ?? []).filter(
        (entry) => !entry.status || entry.status === "published",
      );
      for (const entry of publishedEntries) {
        const path = `/${collection.slug}/${entry.slug}`;
        if (seenPaths.has(path)) {
          continue;
        }
        seenPaths.add(path);
        renderedRoutes.push(
          renderCollectionEntry({ ...input, snapshot }, page, collection, entry),
        );
      }
      continue;
    }
    const path = page.slug === "/" ? "/" : page.slug;
    if (seenPaths.has(path)) {
      continue;
    }
    seenPaths.add(path);
    renderedRoutes.push(renderStaticOrIndexPage({ ...input, snapshot }, page));
  }

  const files: PublishedArtifactFile[] = renderedRoutes.map((route) => ({
    path: route.filePath,
    contentType: "text/html; charset=utf-8",
    body: route.html,
  }));

  files.push({
    path: "assets/theme.css",
    contentType: "text/css; charset=utf-8",
    body: buildPublishedThemeCSS(snapshot),
  });

  const manifest: PublishedArtifactManifest = {
    schemaVersion: "published_artifacts.v1",
    siteSlug: input.siteSlug,
    hostname: input.hostname || undefined,
    defaultLocale: publishedSiteLocale(input.snapshot),
    version: input.version,
    pages: renderedRoutes.map(
      ({
        pageId,
        pagePath,
        filePath,
        title,
        description,
        canonicalUrl,
        ogImageUrl,
        localBusinessJsonLd,
      }) => ({
        pageId,
        pagePath,
        filePath,
        title,
        description,
        canonicalUrl,
        ogImageUrl,
        localBusinessJsonLd,
      }),
    ),
    files: [
      ...files.map((file) => file.path),
      "sitemap.xml",
      "robots.txt",
      "manifest.json",
    ],
  };

  files.push({
    path: "sitemap.xml",
    contentType: "application/xml; charset=utf-8",
    body: buildSitemapXML(manifest),
  });
  files.push({
    path: "robots.txt",
    contentType: "text/plain; charset=utf-8",
    body: buildRobotsTXT(manifest),
  });
  files.push({
    path: "manifest.json",
    contentType: "application/json; charset=utf-8",
    body: JSON.stringify(manifest, null, 2) + "\n",
  });

  return {
    schemaVersion: "published_artifacts.v1",
    files,
  };
}

function filterPublishedSnapshot(snapshot: PublishedSnapshot): PublishedSnapshot {
  const pages = snapshot.pages
    .filter((page) => page.status !== "draft")
    .map((page) => ({
      ...page,
      status: "published" as const,
    }));
  const pageIds = new Set(pages.map((page) => page.id));
  const filterNav = (
    items: PublishedSnapshot["navigation"]["primary"] | PublishedSnapshot["navigation"]["footer"],
  ) => (items ?? []).filter((item) => !item.pageId || pageIds.has(item.pageId));

  return {
    ...snapshot,
    navigation: {
      primary: filterNav(snapshot.navigation.primary),
      footer: filterNav(snapshot.navigation.footer),
    },
    pages,
  };
}

function renderStaticOrIndexPage(
  input: PublishedArtifactRenderInput,
  page: SnapshotPage,
): RenderedRoute {
  const filePath = buildPageArtifactPath(page.slug);
  const title = page.seo?.title || input.snapshot.site.seo?.title || page.title;
  const description =
    page.seo?.description ||
    input.snapshot.site.seo?.description ||
    fallbackPageDescription(
      input.snapshot.site.name,
      publishedSiteLocale(input.snapshot),
    );
  const canonicalUrl = buildCanonicalURL(input, page.slug);
  const ogImageUrl = deriveOGImageURL(input.siteSlug, page);
  const localBusinessJsonLd = buildLocalBusinessJsonLd(
    input.siteSlug,
    input.snapshot.brand,
    input.snapshot.site.name,
    page,
    canonicalUrl,
    ogImageUrl,
  );

  return {
    pageId: page.id,
    pagePath: normalizePagePath(page.slug),
    filePath,
    title,
    description,
    canonicalUrl,
    ogImageUrl,
    localBusinessJsonLd,
    html: renderToStaticMarkup(
      <SiteDraftRenderer
        site={input.snapshot}
        eyebrow=""
        showPageMeta={false}
        selectedPageId={page.id}
        linkMode="published"
        siteSlug={input.siteSlug}
        publishedBasePath=""
      />,
    ),
  };
}

function renderCollectionEntry(
  input: PublishedArtifactRenderInput,
  templatePage: SnapshotPage,
  collection: Collection,
  entry: CollectionEntry,
): RenderedRoute {
  const entryPath = buildEntryPagePath(collection, entry);
  const filePath = buildPageArtifactPath(entryPath);
  const title = resolveEntryTitle(input, collection, entry, templatePage);
  const description = resolveEntryDescription(input, entry, collection);
  const canonicalUrl = buildCanonicalURL(input, entryPath);

  const transformedPage: SnapshotPage = {
    ...templatePage,
    slug: entryPath,
    blocks: templatePage.blocks.map((block) =>
      applyBindings(block, entry),
    ),
  };

  const transformedSnapshot: PublishedSnapshot = {
    ...input.snapshot,
    pages: input.snapshot.pages.map((candidate) =>
      candidate.id === templatePage.id ? transformedPage : candidate,
    ),
  };
  const ogImageUrl = deriveOGImageURL(input.siteSlug, transformedPage);
  const localBusinessJsonLd = buildLocalBusinessJsonLd(
    input.siteSlug,
    transformedSnapshot.brand,
    transformedSnapshot.site.name,
    transformedPage,
    canonicalUrl,
    ogImageUrl,
  );

  return {
    pageId: templatePage.id,
    pagePath: normalizePagePath(entryPath),
    filePath,
    title,
    description,
    canonicalUrl,
    ogImageUrl,
    localBusinessJsonLd,
    html: renderToStaticMarkup(
      <SiteDraftRenderer
        site={transformedSnapshot}
        eyebrow=""
        showPageMeta={false}
        selectedPageId={templatePage.id}
        linkMode="published"
        siteSlug={input.siteSlug}
        publishedBasePath=""
        activeEntry={entry}
        activeCollection={collection}
      />,
    ),
  };
}

function applyBindings(block: SnapshotBlock, entry: CollectionEntry): SnapshotBlock {
  if (!block.bindings || Object.keys(block.bindings).length === 0) {
    return block;
  }
  const nextProps: Record<string, unknown> = { ...block.props };
  for (const [propKey, bindingValue] of Object.entries(block.bindings)) {
    const binding = bindingValue as BlockBinding;
    if (binding.source !== "entry") {
      continue;
    }
    const value = entry.fields[binding.field];
    if (value !== undefined && value !== null) {
      nextProps[propKey] = value;
    }
  }
  return { ...block, props: nextProps };
}

function buildEntryPagePath(collection: Collection, entry: CollectionEntry) {
  return `/${collection.slug}/${entry.slug}`;
}

function buildPageArtifactPath(pagePath: string) {
  const normalizedPath = normalizePagePath(pagePath);
  if (normalizedPath === "/") {
    return "pages/index.html";
  }

  return `pages${normalizedPath}/index.html`;
}

function buildPublishedThemeCSS(snapshot: PublishedSnapshot) {
  const style = buildSiteThemeStyle(snapshot.theme);
  const declarations = Object.entries(style)
    .filter(
      ([, value]) => value !== undefined && value !== null && value !== "",
    )
    .map(([name, value]) => `  ${name}: ${String(value)};`);

  return [":root {", ...declarations, "}", ""].join("\n");
}

function buildSitemapXML(manifest: PublishedArtifactManifest) {
  return [
    '<?xml version="1.0" encoding="UTF-8"?>',
    '<urlset xmlns="http://www.sitemaps.org/schemas/sitemap/0.9">',
    ...manifest.pages.map(
      (page) => `  <url><loc>${escapeXML(page.canonicalUrl)}</loc></url>`,
    ),
    "</urlset>",
    "",
  ].join("\n");
}

function deriveOGImageURL(siteSlug: string, page: SnapshotPage) {
  const hero = page.blocks.find((block) => block.type === "hero");
  const image = hero ? asImageRef(hero.props.image) : null;
  if (!image) {
    return undefined;
  }
  return buildPublishedAssetURL(siteSlug, image.assetId);
}

function buildLocalBusinessJsonLd(
  siteSlug: string,
  brand: BrandConfig,
  siteName: string,
  page: SnapshotPage,
  canonicalUrl: string,
  ogImageUrl?: string,
) {
  const footer = page.blocks.find((block) => block.type === "footer");
  if (!footer) {
    return undefined;
  }

  const contact = asFooterContact(footer.props.contact);
  const address = buildPostalAddress(contact.address);
  const openingHours = buildOpeningHoursSpecification(contact.hours);
  // The structured shape is the LocalBusiness gate: without an address or real
  // opening hours there is no markup worth emitting (Spec 04/09).
  if (!address && openingHours.length === 0) {
    return undefined;
  }

  const brandName = (brand?.businessName ?? "").trim() || siteName;
  const result: Record<string, unknown> = {
    "@context": "https://schema.org",
    "@type": "LocalBusiness",
    name: brandName,
    url: canonicalUrl,
  };

  if (address) {
    result.address = address;
  }
  if (contact.phone) {
    result.telephone = contact.phone;
  }
  if (contact.email) {
    result.email = contact.email;
  }
  if (openingHours.length > 0) {
    result.openingHoursSpecification = openingHours;
  }
  if (ogImageUrl) {
    result.image = ogImageUrl;
  }
  if (brand?.logo?.assetId) {
    result.logo = buildPublishedAssetURL(siteSlug, brand.logo.assetId);
  }

  return result;
}

function buildPostalAddress(
  address: FooterContact["address"],
): Record<string, unknown> | undefined {
  if (!address) {
    return undefined;
  }
  const postal: Record<string, unknown> = { "@type": "PostalAddress" };
  if (address.street) postal.streetAddress = address.street;
  if (address.city) postal.addressLocality = address.city;
  if (address.postalCode) postal.postalCode = address.postalCode;
  if (address.region) postal.addressRegion = address.region;
  if (address.country) postal.addressCountry = address.country;
  return Object.keys(postal).length > 1 ? postal : undefined;
}

const SCHEMA_DAY_OF_WEEK: Record<string, string> = {
  monday: "https://schema.org/Monday",
  tuesday: "https://schema.org/Tuesday",
  wednesday: "https://schema.org/Wednesday",
  thursday: "https://schema.org/Thursday",
  friday: "https://schema.org/Friday",
  saturday: "https://schema.org/Saturday",
  sunday: "https://schema.org/Sunday",
};

function buildOpeningHoursSpecification(
  hours: FooterContact["hours"],
): Array<Record<string, unknown>> {
  if (!hours) {
    return [];
  }
  const specs: Array<Record<string, unknown>> = [];
  for (const entry of hours) {
    const dayOfWeek = SCHEMA_DAY_OF_WEEK[entry.day];
    // Closed days and entries missing a full open/close range don't yield valid
    // OpeningHoursSpecification markup, so they're omitted rather than emitted empty.
    if (!dayOfWeek || entry.closed || !entry.opens || !entry.closes) {
      continue;
    }
    specs.push({
      "@type": "OpeningHoursSpecification",
      dayOfWeek,
      opens: entry.opens,
      closes: entry.closes,
    });
  }
  return specs;
}

function buildRobotsTXT(manifest: PublishedArtifactManifest) {
  const sitemapPath = manifest.hostname
    ? "/sitemap.xml"
    : `/public/${manifest.siteSlug}/sitemap.xml`;
  const sitemapURL = new URL(
    sitemapPath,
    manifest.pages[0]?.canonicalUrl ?? "http://localhost:3000",
  ).toString();

  return ["User-agent: *", "Allow: /", `Sitemap: ${sitemapURL}`, ""].join("\n");
}

function buildCanonicalURL(
  input: PublishedArtifactRenderInput,
  pagePath: string,
) {
  const url = new URL(input.publicBaseURL);
  if (input.hostname) {
    url.hostname = input.hostname;
  }
  url.pathname = normalizePagePath(pagePath);
  url.search = "";
  url.hash = "";
  return url.toString();
}

function resolveEntryTitle(
  input: PublishedArtifactRenderInput,
  collection: Collection,
  entry: CollectionEntry,
  templatePage: SnapshotPage,
) {
  const entrySeoTitle = entry.seo?.title?.trim();
  if (entrySeoTitle) return entrySeoTitle;

  const templateOutput = applySEOTemplate(
    collection.settings?.seoTitleTemplate,
    input.snapshot.site.name,
    collection,
    entry,
  );
  if (templateOutput) return templateOutput;

  const entryTitle = typeof entry.fields.title === "string"
    ? entry.fields.title.trim()
    : "";
  const siteName = input.snapshot.site.name;
  if (entryTitle) {
    return `${entryTitle} | ${siteName}`;
  }

  const fallback =
    templatePage.seo?.title?.trim() ||
    input.snapshot.site.seo?.title?.trim() ||
    `${collection.singularLabel} | ${siteName}`;
  return fallback;
}

function resolveEntryDescription(
  input: PublishedArtifactRenderInput,
  entry: CollectionEntry,
  collection: Collection,
) {
  const seo = entry.seo?.description?.trim();
  if (seo) return seo;

  const templateOutput = applySEOTemplate(
    collection.settings?.seoDescriptionTemplate,
    input.snapshot.site.name,
    collection,
    entry,
  );
  if (templateOutput) return templateOutput;

  return resolveEntryDescriptionFromTemplateOrFields(input, entry);
}

function resolveEntryDescriptionFromTemplateOrFields(
  input: PublishedArtifactRenderInput,
  entry: CollectionEntry,
) {
  const summary = typeof entry.fields.summary === "string"
    ? entry.fields.summary.trim()
    : "";
  if (summary) return summary;

  const details = typeof entry.fields.details === "string"
    ? entry.fields.details.trim()
    : "";
  if (details) {
    return details.length > 180 ? `${details.slice(0, 177)}...` : details;
  }

  return (
    input.snapshot.site.seo?.description ||
    fallbackEntryDescription(
      input.snapshot.site.name,
      publishedSiteLocale(input.snapshot),
    )
  );
}

// applySEOTemplate substitutes {{entry.field}}, {{collection.*}}, and
// {{site.name}} placeholders. Returns an empty string when the template is
// missing or when any required placeholder cannot be resolved — the caller
// then falls back to the legacy automatic derivation.
function applySEOTemplate(
  template: string | undefined,
  siteName: string,
  collection: Collection,
  entry: CollectionEntry,
): string {
  const raw = template?.trim();
  if (!raw) return "";

  const entryTitle = typeof entry.fields.title === "string"
    ? entry.fields.title.trim()
    : "";

  let resolvable = true;
  const replaced = raw.replace(
    /\{\{\s*([a-zA-Z][a-zA-Z0-9_.]*)\s*\}\}/g,
    (_match, ref: string) => {
      const [namespace, ...rest] = ref.split(".");
      const key = rest.join(".");
      if (namespace === "site" && key === "name") {
        return siteName;
      }
      if (namespace === "collection") {
        if (key === "singularLabel") return collection.singularLabel;
        if (key === "pluralLabel") return collection.pluralLabel;
        if (key === "slug") return collection.slug;
      }
      if (namespace === "entry") {
        if (key === "title") return entryTitle;
        if (key === "slug") return entry.slug;
        const value = entry.fields[key];
        if (typeof value === "string") return value.trim();
        if (typeof value === "number" || typeof value === "boolean") {
          return String(value);
        }
      }
      resolvable = false;
      return "";
    },
  );

  if (!resolvable) return "";
  const cleaned = replaced.trim();
  if (!cleaned) return "";
  return cleaned;
}

function escapeXML(value: string) {
  return value
    .replaceAll("&", "&amp;")
    .replaceAll("<", "&lt;")
    .replaceAll(">", "&gt;")
    .replaceAll('"', "&quot;")
    .replaceAll("'", "&apos;");
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
  "monday",
  "tuesday",
  "wednesday",
  "thursday",
  "friday",
  "saturday",
  "sunday",
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

function asFooterAddress(value: unknown): FooterContact["address"] {
  if (typeof value === "string") {
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

function asFooterHours(value: unknown): FooterContact["hours"] {
  if (!Array.isArray(value)) {
    return undefined;
  }
  const entries: NonNullable<FooterContact["hours"]> = [];
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

function asObject(value: unknown) {
  if (!value || typeof value !== "object" || Array.isArray(value)) {
    return null;
  }
  return value as Record<string, unknown>;
}

function asText(value: unknown) {
  return typeof value === "string" ? value : "";
}
