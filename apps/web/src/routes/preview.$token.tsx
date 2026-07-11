import { createFileRoute } from "@tanstack/react-router";
import { useEffect, useState } from "react";
import { SiteDraftRenderer } from "@/components/SiteDraftRenderer";
import { APIError, getPreviewDraft, type SiteDraft } from "@/lib/api";

export const Route = createFileRoute("/preview/$token")({
  component: TokenPreview,
});

function TokenPreview() {
  const { token } = Route.useParams();
  const [draft, setDraft] = useState<SiteDraft | null>(null);
  const [selectedPageId, setSelectedPageId] = useState<string | null>(null);
  const [errorMessage, setErrorMessage] = useState("");

  useEffect(() => {
    let isMounted = true;

    getPreviewDraft(token)
      .then((response) => {
        if (isMounted) {
          setDraft(response.draft);
          setSelectedPageId((current) =>
            resolvePreviewPageId(response.draft, current),
          );
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
      <div
        role="alert"
        className="grid min-h-screen place-items-center bg-[var(--background)] px-6 py-12 text-[var(--paper)]"
      >
        <div className="grid max-w-[44ch] gap-3 text-center">
          <p className="text-xs font-bold uppercase tracking-[0.14em] text-[var(--paper-muted)]">
            Preview unavailable
          </p>
          <p className="m-0 font-serif text-[clamp(1.5rem,2.6vw,2rem)] font-bold leading-[1.15]">
            {errorMessage}
          </p>
        </div>
      </div>
    );
  }

  if (!draft) {
    return (
      <div
        aria-busy="true"
        className="grid min-h-screen place-items-center bg-[var(--background)] text-[var(--paper-muted)]"
      >
        <p className="m-0 text-sm">Loading preview…</p>
      </div>
    );
  }

  return (
    <div className="relative min-h-screen w-full">
      <SiteDraftRenderer
        site={draft}
        eyebrow="Draft preview"
        showPageMeta={false}
        selectedPageId={selectedPageId ?? undefined}
        previewToken={token}
        onNavigatePage={setSelectedPageId}
      />
      <div
        aria-label="This is a draft preview"
        className="pointer-events-none fixed bottom-5 right-5 z-50 rounded-full border border-[color-mix(in_oklch,var(--thread-gold)_42%,transparent)] bg-[color-mix(in_oklch,var(--surface-1)_88%,transparent)] px-3.5 py-1.5 text-[0.7rem] font-bold uppercase tracking-[0.16em] text-[var(--thread-gold)] backdrop-blur-md"
      >
        Draft preview
      </div>
    </div>
  );
}

function resolvePreviewPageId(draft: SiteDraft, preferredPageId: string | null) {
  if (
    preferredPageId &&
    draft.pages.some((page) => page.id === preferredPageId)
  ) {
    return preferredPageId;
  }
  return draft.pages.find((page) => page.slug === "/")?.id ?? draft.pages[0]?.id ?? null;
}
