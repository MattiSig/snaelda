import { render, screen } from "@testing-library/react";
import { describe, expect, it } from "vitest";
import type { PublishedSiteResponse } from "@/lib/api";
import { PublishedSitePage } from "./PublishedSitePage";

describe("PublishedSitePage", () => {
  it("renders a published artifact page", () => {
    render(<PublishedSitePage site={buildPublishedSiteResponse()} />);

    expect(screen.getByText("Drop by the shop")).toBeTruthy();
  });

  it("shows an error message when published content could not load", () => {
    render(
      <PublishedSitePage site={null} errorMessage="Published page not found" />,
    );

    expect(screen.getByText("Published page not found")).toBeTruthy();
  });
});

function buildPublishedSiteResponse(): PublishedSiteResponse {
  return {
    siteSlug: "loom-light",
    hostname: "loom-light.localhost",
    publicUrl: "http://localhost:3000/public/loom-light/contact",
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
      description: "Drop by the shop this weekend.",
      canonicalUrl: "http://loom-light.localhost:3000/contact",
      html: "<div><h1>Drop by the shop</h1><p>Open weekdays and most Saturdays.</p></div>",
    },
  };
}
