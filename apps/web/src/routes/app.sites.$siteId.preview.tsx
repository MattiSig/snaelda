import { Link, createFileRoute } from "@tanstack/react-router";
import { useEffect, useState } from "react";
import { SiteDraftRenderer } from "@/components/SiteDraftRenderer";
import { Button } from "@/components/ui/button";
import {
  APIError,
  createPreviewToken,
  getSiteDraft,
  revokePreviewToken,
  type SiteDraft,
} from "@/lib/api";
import { actions, layout, preview, ribbonPanel, text } from "@/lib/styles";

export const Route = createFileRoute("/app/sites/$siteId/preview")({
  component: DraftPreview,
});

function DraftPreview() {
  const { siteId } = Route.useParams();
  const [draft, setDraft] = useState<SiteDraft | null>(null);
  const [errorMessage, setErrorMessage] = useState("");
  const [shareURL, setShareURL] = useState("");
  const [shareExpiresAt, setShareExpiresAt] = useState("");
  const [shareStatus, setShareStatus] = useState("");
  const [shareBusy, setShareBusy] = useState(false);

  useEffect(() => {
    let isMounted = true;

    getSiteDraft(siteId)
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
          error instanceof APIError ? error.message : "Could not load preview",
        );
      });

    return () => {
      isMounted = false;
    };
  }, [siteId]);

  async function handleCreateShareLink() {
    setShareBusy(true);
    setShareStatus("");
    try {
      const response = await createPreviewToken(siteId);
      const url = new URL(`/preview/${response.token}`, window.location.origin);
      setShareURL(url.toString());
      setShareExpiresAt(response.expiresAt);
      setShareStatus("Shared preview link ready.");
    } catch (error) {
      setShareStatus(
        error instanceof APIError
          ? error.message
          : "Could not create preview link.",
      );
    } finally {
      setShareBusy(false);
    }
  }

  async function handleCopyShareLink() {
    if (!shareURL) {
      return;
    }
    try {
      await navigator.clipboard.writeText(shareURL);
      setShareStatus("Shared preview link copied.");
    } catch {
      setShareStatus("Could not copy the preview link.");
    }
  }

  async function handleRevokeShareLink() {
    setShareBusy(true);
    setShareStatus("");
    try {
      await revokePreviewToken(siteId);
      setShareURL("");
      setShareExpiresAt("");
      setShareStatus("Shared preview link revoked.");
    } catch (error) {
      setShareStatus(
        error instanceof APIError
          ? error.message
          : "Could not revoke preview link.",
      );
    } finally {
      setShareBusy(false);
    }
  }

  if (errorMessage) {
    return (
      <div className={ribbonPanel}>
        <p className={text.error}>{errorMessage}</p>
      </div>
    );
  }

  if (!draft) {
    return (
      <div className={ribbonPanel}>
        <p className={text.p}>Loading preview...</p>
      </div>
    );
  }

  return (
    <div className={layout.previewShell}>
      <div className={preview.toolbar}>
        <div>
          <p className={text.eyebrow}>Draft preview</p>
          <strong>{draft.site.name}</strong>
          <p className={text.p}>
            Create a temporary share link when someone needs to review the draft
            without signing in.
          </p>
          {shareURL ? (
            <div className="mt-3 space-y-2">
              <p className="break-all text-sm text-[var(--paper-muted)]">
                {shareURL}
              </p>
              <p className="text-xs uppercase tracking-[0.16em] text-[var(--paper-muted)]">
                Expires {new Date(shareExpiresAt).toLocaleString()}
              </p>
            </div>
          ) : null}
          {shareStatus ? (
            <p className="mt-2 text-sm text-[var(--thread-mauve)]">
              {shareStatus}
            </p>
          ) : null}
        </div>
        <div className="flex flex-wrap items-center justify-end gap-3">
          <Button
            type="button"
            variant="outline"
            onClick={handleCreateShareLink}
            disabled={shareBusy}
          >
            {shareURL ? "Refresh share link" : "Create share link"}
          </Button>
          <Button
            type="button"
            variant="outline"
            onClick={handleCopyShareLink}
            disabled={!shareURL || shareBusy}
          >
            Copy link
          </Button>
          <Button
            type="button"
            variant="ghost"
            onClick={handleRevokeShareLink}
            disabled={!shareURL || shareBusy}
          >
            Revoke link
          </Button>
          <Button asChild variant="plain" className={actions.inlineLink}>
            <Link
              to="/app/sites/$siteId"
              params={{ siteId }}
              search={{ panel: undefined }}
            >
              Back to builder
            </Link>
          </Button>
        </div>
      </div>
      <SiteDraftRenderer site={draft} eyebrow="Draft preview" />
    </div>
  );
}
