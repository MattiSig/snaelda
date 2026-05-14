import type { PublishedSiteResponse } from "@/lib/api";
import { layout, ribbonPanel, text } from "@/lib/styles";

export function PublishedSitePage({
  site,
  errorMessage = "",
}: {
  site: PublishedSiteResponse | null;
  errorMessage?: string;
}) {
  if (errorMessage) {
    return (
      <main className={layout.publicShell}>
        <section className={ribbonPanel}>
          <p className={text.error}>{errorMessage}</p>
        </section>
      </main>
    );
  }

  if (!site) {
    return (
      <main className={layout.publicShell}>
        <section className={ribbonPanel}>
          <p className={text.p}>Loading published page...</p>
        </section>
      </main>
    );
  }

  return (
    <main
      className={layout.publicShell}
      dangerouslySetInnerHTML={{ __html: site.page.html }}
    />
  );
}
