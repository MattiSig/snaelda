import { useEffect, useState, type ReactNode } from 'react';
import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import {
  applyBlockImage,
  suggestBlockImage,
  type ImageApplyResponse,
  type ImageSuggestCandidate,
} from '@/lib/api';
import { emptyState, text } from '@/lib/styles';
import { cn } from '@/lib/utils';

export type ImagePickerContext = {
  siteId: string;
  blockId: string;
  path: ReadonlyArray<string | number>;
  currentAlt: string;
};

export function ImagePickerModal({
  context,
  onClose,
  onApplied,
  title = 'Find a better image',
  intro,
}: {
  context: ImagePickerContext;
  onClose: () => void;
  onApplied: (response: ImageApplyResponse) => void;
  title?: string;
  intro?: ReactNode;
}) {
  const [instruction, setInstruction] = useState('');
  const [query, setQuery] = useState('');
  const [candidates, setCandidates] = useState<ImageSuggestCandidate[]>([]);
  const [searching, setSearching] = useState(true);
  const [applyingId, setApplyingId] = useState<string | null>(null);
  const [errorMessage, setErrorMessage] = useState('');

  useEffect(() => {
    function handleKey(event: KeyboardEvent) {
      if (event.key === 'Escape') onClose();
    }
    document.addEventListener('keydown', handleKey);
    return () => document.removeEventListener('keydown', handleKey);
  }, [onClose]);

  useEffect(() => {
    let cancelled = false;
    suggestBlockImage(context.siteId, context.blockId, {
      path: context.path as string[],
      instruction: '',
    })
      .then((response) => {
        if (cancelled) return;
        setQuery(response.query);
        setCandidates(response.candidates);
        if (response.candidates.length === 0) {
          setErrorMessage(
            'No images matched that query. Try a different instruction.',
          );
        }
      })
      .catch((error) => {
        if (cancelled) return;
        setErrorMessage(
          error instanceof Error
            ? error.message
            : 'Could not load image suggestions.',
        );
      })
      .finally(() => {
        if (!cancelled) setSearching(false);
      });
    return () => {
      cancelled = true;
    };
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []);

  function runSuggest() {
    setSearching(true);
    setErrorMessage('');
    suggestBlockImage(context.siteId, context.blockId, {
      path: context.path as string[],
      instruction,
    })
      .then((response) => {
        setQuery(response.query);
        setCandidates(response.candidates);
        if (response.candidates.length === 0) {
          setErrorMessage(
            'No images matched that query. Try a different instruction.',
          );
        }
      })
      .catch((error) => {
        setErrorMessage(
          error instanceof Error
            ? error.message
            : 'Could not load image suggestions.',
        );
      })
      .finally(() => setSearching(false));
  }

  async function applyCandidate(candidate: ImageSuggestCandidate) {
    setApplyingId(candidate.providerId || candidate.downloadUrl);
    setErrorMessage('');
    try {
      const response = await applyBlockImage(context.siteId, context.blockId, {
        path: context.path as string[],
        photo: candidate,
        alt: context.currentAlt || candidate.description || '',
        query,
        instruction,
      });
      onApplied(response);
    } catch (error) {
      setErrorMessage(
        error instanceof Error
          ? error.message
          : 'Could not apply the chosen image.',
      );
    } finally {
      setApplyingId(null);
    }
  }

  return (
    <div
      role="dialog"
      aria-modal="true"
      aria-label={title}
      className="fixed inset-0 z-[60] flex items-center justify-center bg-[oklch(8%_0.02_336_/_0.7)] p-4 backdrop-blur-sm"
      onClick={(event) => {
        if (event.target === event.currentTarget) onClose();
      }}
    >
      <div className="grid max-h-[88vh] w-full max-w-[820px] gap-4 overflow-y-auto rounded-[20px] border border-border bg-[var(--surface-1)] p-6 shadow-[0_36px_72px_oklch(8%_0.02_336_/_0.5)]">
        <header className="flex items-start justify-between gap-4">
          <div>
            <p className={text.eyebrow}>AI image picker</p>
            <h3 className={text.h2}>{title}</h3>
            {intro ? (
              <p className="mt-1 text-sm text-[var(--paper-muted)]">{intro}</p>
            ) : query ? (
              <p className="mt-1 text-sm text-[var(--paper-muted)]">
                Searching Pexels for{' '}
                <span className="font-semibold text-[var(--paper)]">
                  {query}
                </span>
              </p>
            ) : null}
          </div>
          <Button
            type="button"
            variant="plain"
            size="sm"
            onClick={onClose}
            aria-label="Close image picker"
          >
            Close
          </Button>
        </header>

        <div className="grid gap-2 sm:grid-cols-[1fr_auto]">
          <Input
            type="text"
            placeholder="Optional: describe what you want (e.g. wedding bouquet close-up)"
            value={instruction}
            onChange={(event) => setInstruction(event.target.value)}
            onKeyDown={(event) => {
              if (event.key === 'Enter') {
                event.preventDefault();
                runSuggest();
              }
            }}
            disabled={searching}
          />
          <Button
            type="button"
            size="sm"
            disabled={searching}
            onClick={runSuggest}
          >
            {searching ? 'Searching…' : 'Search again'}
          </Button>
        </div>

        {errorMessage ? <p className={text.error}>{errorMessage}</p> : null}

        {searching && candidates.length === 0 ? (
          <div className={emptyState}>
            <p className={text.p}>Looking for fresh images…</p>
          </div>
        ) : null}

        {candidates.length > 0 ? (
          <div className="grid grid-cols-2 gap-3 sm:grid-cols-3">
            {candidates.map((candidate) => {
              const id = candidate.providerId || candidate.downloadUrl;
              const isApplying = applyingId === id;
              return (
                <button
                  type="button"
                  key={id}
                  onClick={() => void applyCandidate(candidate)}
                  disabled={isApplying || applyingId !== null}
                  className={cn(
                    'group relative grid gap-2 rounded-[14px] border border-border bg-[var(--surface-2)] p-2 text-left transition-colors',
                    'hover:border-[var(--thread-violet)] disabled:cursor-not-allowed disabled:opacity-60',
                  )}
                  aria-label={
                    candidate.description ||
                    `Photo by ${candidate.author || 'a Pexels contributor'}`
                  }
                >
                  <img
                    src={candidate.downloadUrl}
                    alt={
                      candidate.description ||
                      `Photo by ${candidate.author || 'a Pexels contributor'}`
                    }
                    loading="lazy"
                    className="aspect-[4/3] w-full rounded-[10px] object-cover"
                  />
                  <div className="grid gap-0.5 px-1 pb-1">
                    {candidate.description ? (
                      <span className="line-clamp-2 text-xs text-[var(--paper)]">
                        {candidate.description}
                      </span>
                    ) : null}
                    {candidate.author ? (
                      <span className="text-xs text-[var(--paper-muted)]">
                        Photo by {candidate.author}
                      </span>
                    ) : null}
                  </div>
                  {isApplying ? (
                    <span className="absolute inset-0 flex items-center justify-center rounded-[14px] bg-[oklch(8%_0.02_336_/_0.55)] text-sm font-semibold text-[var(--paper)]">
                      Applying…
                    </span>
                  ) : null}
                </button>
              );
            })}
          </div>
        ) : null}

        <p className="text-xs text-[var(--paper-muted)]">
          Click an image to import it as a site asset and replace this slot. We
          credit photographers automatically via Pexels.
        </p>
      </div>
    </div>
  );
}
