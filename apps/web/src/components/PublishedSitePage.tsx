import type { PublishedSiteResponse } from "@/lib/api";

export function PublishedSitePage({
  site,
  errorMessage = "",
}: {
  site: PublishedSiteResponse | null;
  errorMessage?: string;
}) {
  if (errorMessage) {
    return (
      <div
        role="alert"
        className="grid min-h-screen place-items-center bg-[var(--background)] px-6 py-12 text-[var(--paper)]"
      >
        <div className="grid max-w-[44ch] gap-3 text-center">
          <p className="text-xs font-medium uppercase tracking-[0.14em] text-[var(--paper-muted)]">
            Site unavailable
          </p>
          <p className="m-0 font-serif text-[clamp(1.5rem,2.6vw,2rem)] font-bold leading-[1.15]">
            {errorMessage}
          </p>
        </div>
      </div>
    );
  }

  if (!site) {
    return (
      <div
        aria-busy="true"
        className="grid min-h-screen place-items-center bg-[var(--background)] text-[var(--paper-muted)]"
      >
        <p className="m-0 text-sm">Loading...</p>
      </div>
    );
  }

  return (
    <div
      className="contents"
      dangerouslySetInnerHTML={{ __html: site.page.html }}
    />
  );
}
