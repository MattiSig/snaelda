import { createFileRoute } from "@tanstack/react-router";
import { useEffect, useState } from "react";
import { SiteDraftRenderer } from "@/components/SiteDraftRenderer";
import { APIError, getPreviewDraft, type SiteDraft } from "@/lib/api";
import { layout, paddedPanel, preview, text } from "@/lib/styles";

export const Route = createFileRoute("/preview/$token")({
  component: TokenPreview,
});

function TokenPreview() {
  const { token } = Route.useParams();
  const [draft, setDraft] = useState<SiteDraft | null>(null);
  const [errorMessage, setErrorMessage] = useState("");

  useEffect(() => {
    let isMounted = true;

    getPreviewDraft(token)
      .then((response) => {
        if (isMounted) {
          setDraft(response.draft);
        }
      })
      .catch((error) => {
        if (!isMounted) {
          return;
        }
        setErrorMessage(
          error instanceof APIError
            ? error.message
            : "Could not load shared preview",
        );
      });

    return () => {
      isMounted = false;
    };
  }, [token]);

  if (errorMessage) {
    return (
      <main className={layout.pageShell}>
        <section className={paddedPanel}>
          <div className={preview.toolbar}>
            <p className={text.eyebrow}>Shared preview</p>
          </div>
          <article className="mt-5 rounded-[16px] border border-border bg-[var(--surface-2)] p-6">
            <h1 className={text.h2}>Preview unavailable</h1>
            <p className={text.p}>{errorMessage}</p>
          </article>
        </section>
      </main>
    );
  }

  if (!draft) {
    return (
      <main className={layout.pageShell}>
        <section className={paddedPanel}>
          <div className={preview.toolbar}>
            <p className={text.eyebrow}>Shared preview</p>
          </div>
          <article className="mt-5 rounded-[16px] border border-border bg-[var(--surface-2)] p-6">
            <p className={text.p}>Loading preview...</p>
          </article>
        </section>
      </main>
    );
  }

  return (
    <main className={layout.pageShell}>
      <section className={paddedPanel}>
        <div className={preview.toolbar}>
          <div>
            <p className={text.eyebrow}>Shared preview</p>
            <strong>{draft.site.name}</strong>
          </div>
        </div>
        <SiteDraftRenderer site={draft} eyebrow="Shared preview" />
      </section>
    </main>
  );
}
