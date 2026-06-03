import { Link, createFileRoute } from "@tanstack/react-router";
import type { FormEvent } from "react";
import { useEffect, useState } from "react";
import {
  GenerationProgressCard,
  type GenerationProgressItem,
} from "@/components/GenerationProgressCard";
import { SiteDraftRenderer } from "@/components/SiteDraftRenderer";
import { Button } from "@/components/ui/button";
import { Textarea } from "@/components/ui/textarea";
import {
  APIError,
  createPreviewToken,
  getSiteDraft,
  revokePreviewToken,
  streamRepromptSite,
  type SiteDraft,
} from "@/lib/api";
import { actions, layout, preview, ribbonPanel, text } from "@/lib/styles";

const previewRefinementSteps: GenerationProgressItem[] = [
  { step: "prompt.normalize", label: "Reading your prompt" },
  { step: "plan.pages", label: "Planning pages and structure" },
  { step: "plan.theme", label: "Picking colors and typography" },
  { step: "plan.blocks", label: "Choosing blocks for each page" },
  { step: "assets.fetch", label: "Finding starter imagery" },
  { step: "copy.write", label: "Writing copy" },
  { step: "validate.repair", label: "Checking and repairing" },
  { step: "persist", label: "Saving your draft" },
];

const previewRefinementChips = [
  "Make it warmer",
  "Simplify the copy",
  "Stronger call to action",
  "Make the layout bolder",
];

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
  const [refinementPrompt, setRefinementPrompt] = useState("");
  const [refinementError, setRefinementError] = useState("");
  const [refinementStatus, setRefinementStatus] = useState("");
  const [isRefining, setIsRefining] = useState(false);
  const [refinementStep, setRefinementStep] = useState("");
  const [refinementStepTotal, setRefinementStepTotal] = useState(0);

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

  async function handlePreviewRefinement(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    const prompt = refinementPrompt.trim();
    if (!prompt) {
      return;
    }
    setIsRefining(true);
    setRefinementError("");
    setRefinementStatus("");
    setRefinementStep("");
    setRefinementStepTotal(0);

    try {
      await streamRepromptSite(
        siteId,
        { prompt },
        {
          onProgress: (step) => {
            setRefinementStep(step.step);
            setRefinementStepTotal(step.total);
          },
        },
      );
      const response = await getSiteDraft(siteId);
      setDraft(response.draft);
      setRefinementPrompt("");
      setRefinementStatus(
        "Draft refined. Open the builder to compare or restore the checkpoint.",
      );
    } catch (error) {
      setRefinementError(
        error instanceof APIError
          ? error.message
          : "Could not refine this draft.",
      );
    } finally {
      setIsRefining(false);
      setRefinementStep("");
      setRefinementStepTotal(0);
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
      <form
        onSubmit={handlePreviewRefinement}
        className="grid gap-3 rounded-[14px] border border-[color-mix(in_oklch,var(--border)_70%,transparent)] bg-[var(--surface-1)] p-4"
      >
        <div className="flex flex-wrap items-start justify-between gap-3">
          <div>
            <p className={text.eyebrow}>Keep shaping it</p>
            <h2 className="m-0 mt-1 text-[1.1rem] font-black leading-[1.05] text-[var(--paper)]">
              Want to change the direction?
            </h2>
            <p className="m-0 mt-1 max-w-[68ch] text-sm text-[var(--paper-muted)]">
              This refines the whole draft from the current preview and saves a
              checkpoint for review in the builder.
            </p>
          </div>
          <Button asChild variant="outline" size="sm">
            <Link
              to="/app/sites/$siteId"
              params={{ siteId }}
              search={{ panel: "prompt" }}
            >
              Open AI refine
            </Link>
          </Button>
        </div>

        <Textarea
          rows={3}
          value={refinementPrompt}
          placeholder="Make it warmer, less corporate, and add a clearer booking path."
          onChange={(event) => setRefinementPrompt(event.target.value)}
          aria-label="Refinement prompt"
        />
        <div className="flex flex-wrap items-center justify-between gap-3">
          <div className="flex flex-wrap gap-2">
            {previewRefinementChips.map((chip) => (
              <Button
                key={chip}
                type="button"
                size="sm"
                variant="outline"
                onClick={() =>
                  setRefinementPrompt((current) =>
                    current.trim() ? `${current.trim()} ${chip}.` : `${chip}.`,
                  )
                }
              >
                {chip}
              </Button>
            ))}
          </div>
          <Button
            type="submit"
            size="sm"
            disabled={isRefining || refinementPrompt.trim() === ""}
          >
            {isRefining ? "Refining..." : "Refine draft"}
          </Button>
        </div>
        {isRefining ? (
          <GenerationProgressCard
            eyebrow="AI refinement"
            title="Refining the draft..."
            description="Snaelda is reshaping the whole site from the prompt below."
            prompt={refinementPrompt}
            steps={previewRefinementSteps}
            activeStep={refinementStep}
            activeTotal={refinementStepTotal}
            showSkeleton={
              refinementStep === "plan.blocks" ||
              refinementStep === "copy.write" ||
              refinementStep === "validate.repair" ||
              refinementStep === "persist"
            }
          />
        ) : null}
        {refinementError ? <p className={text.error}>{refinementError}</p> : null}
        {refinementStatus ? (
          <p className={text.success}>{refinementStatus}</p>
        ) : null}
      </form>
      <SiteDraftRenderer site={draft} eyebrow="Draft preview" />
    </div>
  );
}
