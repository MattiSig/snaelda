import { describe, expect, it } from "vitest";
import { buildPublishedArtifactBundle } from "./published-artifacts";
import type { PublishedSnapshot } from "./api";

describe("published artifact bundle", () => {
  it("renders per-page html and crawl artifacts for a published snapshot", () => {
    const bundle = buildPublishedArtifactBundle({
      publicBaseURL: "https://sites.snaelda.test",
      siteSlug: "loom-light",
      hostname: "loom-light.snaelda.test",
      version: {
        id: "version-3",
        siteId: "site-1",
        versionNumber: 3,
        createdAt: "2026-05-11T08:00:00Z",
        isCurrent: true,
      },
      snapshot: buildSnapshot(),
    });

    expect(bundle.schemaVersion).toBe("published_artifacts.v1");

    const pageArtifact = bundle.files.find(
      (file) => file.path === "pages/contact/index.html",
    );
    expect(pageArtifact?.contentType).toBe("text/html; charset=utf-8");
    expect(pageArtifact?.body).toContain("Say hello");
    expect(pageArtifact?.body).toContain('href="/"');
    expect(pageArtifact?.body).not.toContain("/public/loom-light/contact");

    const themeArtifact = bundle.files.find(
      (file) => file.path === "assets/theme.css",
    );
    expect(themeArtifact?.body).toContain("--color-background: #151215;");
    expect(themeArtifact?.body).toContain("--color-primary: #8fc6ff;");

    const sitemap = bundle.files.find((file) => file.path === "sitemap.xml");
    expect(sitemap?.body).toContain(
      "<loc>https://loom-light.snaelda.test/</loc>",
    );
    expect(sitemap?.body).toContain(
      "<loc>https://loom-light.snaelda.test/contact</loc>",
    );

    const robots = bundle.files.find((file) => file.path === "robots.txt");
    expect(robots?.body).toContain(
      "Sitemap: https://loom-light.snaelda.test/sitemap.xml",
    );

    const manifest = bundle.files.find((file) => file.path === "manifest.json");
    expect(manifest?.body).toContain('"pagePath": "/contact"');
    expect(manifest?.body).toContain(
      '"canonicalUrl": "https://loom-light.snaelda.test/contact"',
    );
    expect(manifest?.body).toContain('"pageId": "page-contact"');
  });

  it("emits structured LocalBusiness JSON-LD from the footer contact contract", () => {
    const snapshot = buildSnapshot();
    snapshot.pages[0].blocks.push({
      id: "block-footer",
      type: "footer",
      version: "block.v1",
      props: {
        showBrand: true,
        showMadeWith: true,
        copyright: "Copyright 2026 Loom & Light",
        contact: {
          address: {
            street: "Laugavegur 12",
            city: "Reykjavík",
            postalCode: "101",
            country: "Iceland",
          },
          phone: "+354 555 1234",
          email: "hello@loomlight.is",
          hours: [
            { day: "monday", opens: "09:00", closes: "17:00", closed: false },
            { day: "sunday", closed: true },
          ],
        },
      },
    });

    const bundle = buildPublishedArtifactBundle({
      publicBaseURL: "https://sites.snaelda.test",
      siteSlug: "loom-light",
      hostname: "loom-light.snaelda.test",
      version: {
        id: "version-4",
        siteId: "site-1",
        versionNumber: 4,
        createdAt: "2026-05-11T08:00:00Z",
        isCurrent: true,
      },
      snapshot,
    });

    const manifest = bundle.files.find((file) => file.path === "manifest.json");
    const parsed = JSON.parse(manifest?.body ?? "{}") as {
      pages: Array<{ pageId: string; localBusinessJsonLd?: Record<string, unknown> }>;
    };
    const home = parsed.pages.find((page) => page.pageId === "page-home");
    const jsonLd = home?.localBusinessJsonLd;

    expect(jsonLd?.["@type"]).toBe("LocalBusiness");
    expect(jsonLd?.telephone).toBe("+354 555 1234");
    expect(jsonLd?.email).toBe("hello@loomlight.is");
    expect(jsonLd?.address).toEqual({
      "@type": "PostalAddress",
      streetAddress: "Laugavegur 12",
      addressLocality: "Reykjavík",
      postalCode: "101",
      addressCountry: "Iceland",
    });
    // Only the open Monday range produces a spec; the closed Sunday is omitted.
    expect(jsonLd?.openingHoursSpecification).toEqual([
      {
        "@type": "OpeningHoursSpecification",
        dayOfWeek: "https://schema.org/Monday",
        opens: "09:00",
        closes: "17:00",
      },
    ]);
  });
});

function buildSnapshot(): PublishedSnapshot {
  return {
    schemaVersion: "site_snapshot.v1",
    site: {
      id: "site-1",
      name: "Loom & Light",
      defaultLocale: "en",
      seo: {
        title: "Loom & Light",
        description: "Warm sites spun from a prompt.",
      },
    },
    theme: {
      version: "theme.v1",
      tokens: {
        colors: {
          background: "#151215",
          foreground: "#f6f2ec",
          primary: "#8fc6ff",
          border: "#5a3e57",
        },
        typography: {},
        layout: {},
        shape: {},
      },
    },
    navigation: {
      primary: [
        { label: "Home", pageId: "page-home" },
        { label: "Contact", pageId: "page-contact" },
      ],
    },
    pages: [
      {
        id: "page-home",
        title: "Home",
        slug: "/",
        blocks: [
          {
            id: "block-home",
            type: "hero",
            version: "block.v1",
            props: {
              headline: "Spin a simple site",
              primaryCta: {
                label: "Contact",
                href: "/contact",
              },
              layout: "centered",
            },
          },
        ],
      },
      {
        id: "page-contact",
        title: "Contact",
        slug: "/contact",
        seo: {
          title: "Contact | Loom & Light",
          description: "Drop by the studio this weekend.",
        },
        blocks: [
          {
            id: "block-contact",
            type: "text_section",
            version: "block.v1",
            props: {
              heading: "Say hello",
              body: "Send a note with your launch timeline.",
            },
          },
        ],
      },
    ],
  };
}
