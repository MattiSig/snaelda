import { renderToStaticMarkup } from "react-dom/server";
import type {
  BlockBinding,
  Collection,
  CollectionEntry,
  PublishedSnapshot,
  SiteVersion,
} from "@/lib/api";
import { SiteDraftRenderer } from "@/components/SiteDraftRenderer";
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
  version: SiteVersion;
  pages: Array<{
    pageId: string;
    pagePath: string;
    filePath: string;
    title: string;
    description: string;
    canonicalUrl: string;
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
  html: string;
};

export function buildPublishedArtifactBundle(
  input: PublishedArtifactRenderInput,
): PublishedArtifactBundle {
  const collectionsById = new Map<string, Collection>();
  for (const collection of input.snapshot.collections ?? []) {
    collectionsById.set(collection.id, collection);
  }

  const renderedRoutes: RenderedRoute[] = [];

  for (const page of input.snapshot.pages) {
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
        renderedRoutes.push(
          renderCollectionEntry(input, page, collection, entry),
        );
      }
      continue;
    }
    renderedRoutes.push(renderStaticOrIndexPage(input, page));
  }

  const files: PublishedArtifactFile[] = renderedRoutes.map((route) => ({
    path: route.filePath,
    contentType: "text/html; charset=utf-8",
    body: route.html,
  }));

  files.push({
    path: "assets/theme.css",
    contentType: "text/css; charset=utf-8",
    body: buildPublishedThemeCSS(input.snapshot),
  });

  const manifest: PublishedArtifactManifest = {
    schemaVersion: "published_artifacts.v1",
    siteSlug: input.siteSlug,
    hostname: input.hostname || undefined,
    version: input.version,
    pages: renderedRoutes.map(
      ({ pageId, pagePath, filePath, title, description, canonicalUrl }) => ({
        pageId,
        pagePath,
        filePath,
        title,
        description,
        canonicalUrl,
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

function renderStaticOrIndexPage(
  input: PublishedArtifactRenderInput,
  page: SnapshotPage,
): RenderedRoute {
  const filePath = buildPageArtifactPath(page.slug);
  const title = page.seo?.title || input.snapshot.site.seo?.title || page.title;
  const description =
    page.seo?.description ||
    input.snapshot.site.seo?.description ||
    `Visit ${input.snapshot.site.name}.`;
  const canonicalUrl = buildCanonicalURL(input, page.slug);

  return {
    pageId: page.id,
    pagePath: normalizePagePath(page.slug),
    filePath,
    title,
    description,
    canonicalUrl,
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
  const description = resolveEntryDescription(input, entry);
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

  return {
    pageId: templatePage.id,
    pagePath: normalizePagePath(entryPath),
    filePath,
    title,
    description,
    canonicalUrl,
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
    '<urlset xmlns="https://www.sitemaps.org/schemas/sitemap/0.9">',
    ...manifest.pages.map(
      (page) => `  <url><loc>${escapeXML(page.canonicalUrl)}</loc></url>`,
    ),
    "</urlset>",
    "",
  ].join("\n");
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
) {
  const seo = entry.seo?.description?.trim();
  if (seo) return seo;

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
    `Discover more from ${input.snapshot.site.name}.`
  );
}

function escapeXML(value: string) {
  return value
    .replaceAll("&", "&amp;")
    .replaceAll("<", "&lt;")
    .replaceAll(">", "&gt;")
    .replaceAll('"', "&quot;")
    .replaceAll("'", "&apos;");
}
