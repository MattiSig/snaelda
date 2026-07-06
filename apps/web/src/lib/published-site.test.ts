import { describe, expect, it } from "vitest";
import type { PublishedSiteResponse } from "@/lib/api";
import {
  buildAppRobotsTXT,
  buildAppSitemapXML,
  buildPublishedPageHead,
  buildPublishedPageURL,
  publishedOGLocale,
  publishedSiteHtmlLang,
} from "./published-site";

describe("published site helpers", () => {
  it("builds canonical and social metadata from page seo", () => {
    const result = buildPublishedPageHead(buildPathPublishedSite());

    expect(result.links).toEqual([
      {
        rel: "canonical",
        href: "https://loom-light.snaelda.test/contact",
      },
    ]);
    expect(result.meta).toContainEqual({
      property: "og:title",
      content: "Contact | Loom & Light",
    });
    expect(result.meta).toContainEqual({
      name: "twitter:description",
      content: "Drop by the studio this weekend.",
    });
  });

  it("emits og:locale en_US when the site has no locale", () => {
    const result = buildPublishedPageHead(buildPathPublishedSite());
    expect(result.meta).toContainEqual({
      property: "og:locale",
      content: "en_US",
    });
  });

  it("emits the Icelandic og:locale for an Icelandic site", () => {
    const result = buildPublishedPageHead({
      ...buildPathPublishedSite(),
      defaultLocale: "is-IS",
    });
    expect(result.meta).toContainEqual({
      property: "og:locale",
      content: "is_IS",
    });
  });

  it("reduces published locales to supported html lang and og tags", () => {
    expect(publishedSiteHtmlLang("is-IS")).toBe("is");
    expect(publishedSiteHtmlLang("IS")).toBe("is");
    expect(publishedSiteHtmlLang("en-US")).toBe("en");
    expect(publishedSiteHtmlLang("")).toBe("en");
    expect(publishedSiteHtmlLang(undefined)).toBe("en");
    expect(publishedSiteHtmlLang("de")).toBe("en");
    expect(publishedOGLocale("is")).toBe("is_IS");
    expect(publishedOGLocale("en")).toBe("en_US");
  });

  it("builds page urls for hosted domains without the local public prefix", () => {
    expect(buildPublishedPageURL(buildHostedPublishedSite(), "/contact")).toBe(
      "https://loom-light.snaelda.test/contact",
    );
  });

  it("builds app-level crawl files for the builder domain", () => {
    expect(buildAppSitemapXML("http://localhost:3000")).toContain(
      "<loc>http://localhost:3000/login</loc>",
    );
    expect(buildAppRobotsTXT("http://localhost:3000")).toContain(
      "Disallow: /app",
    );
  });
});

function buildPathPublishedSite(): PublishedSiteResponse {
  return {
    siteSlug: "loom-light",
    hostname: "loom-light.snaelda.test",
    publicUrl: "https://loom-light.snaelda.test/contact",
    pagePath: "/contact",
    version: {
      id: "version-3",
      siteId: "site-1",
      versionNumber: 3,
      createdAt: "2026-05-10T09:00:00Z",
      isCurrent: true,
    },
    page: {
      pagePath: "/contact",
      title: "Contact | Loom & Light",
      description: "Drop by the studio this weekend.",
      canonicalUrl: "https://loom-light.snaelda.test/contact",
      html: "<div>Contact</div>",
    },
  };
}

function buildHostedPublishedSite(): PublishedSiteResponse {
  return {
    ...buildPathPublishedSite(),
    publicUrl: "https://loom-light.snaelda.test/contact",
  };
}
