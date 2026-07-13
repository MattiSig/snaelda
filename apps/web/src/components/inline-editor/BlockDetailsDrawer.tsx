import { useEffect, useState } from 'react';
import { createPortal } from 'react-dom';
import { X } from 'lucide-react';
import { BlockEditor } from '@/components/BlockEditor';
import { useInlineEditor } from './context';
import { setAtPath } from './path';
import { BlockBindingsEditor } from './BlockBindingsEditor';
import type { BlockDefinition, SiteDraft } from '@/lib/api';
import { text } from '@/lib/styles';
import { cn } from '@/lib/utils';

type DraftBlock = SiteDraft['pages'][number]['blocks'][number];

export function BlockDetailsDrawer({
  block,
  definition,
  onClose,
}: {
  block: DraftBlock;
  definition: BlockDefinition | undefined;
  onClose: () => void;
}) {
  const ctx = useInlineEditor();
  const [isSaving, setIsSaving] = useState(false);
  const [errorMessage, setErrorMessage] = useState('');
  const [statusMessage, setStatusMessage] = useState('');

  useEffect(() => {
    function key(event: KeyboardEvent) {
      if (event.key === 'Escape') onClose();
    }
    document.addEventListener('keydown', key);
    return () => document.removeEventListener('keydown', key);
  }, [onClose]);

  if (!ctx) return null;

  const ownerPage = ctx.pages.find((page) =>
    page.blocks.some((entry) => entry.id === block.id),
  );
  const isCollectionDetail = ownerPage?.type === 'collection_detail';
  const ownerCollection = isCollectionDetail
    ? ctx.collections.find(
        (collection) => collection.id === ownerPage?.collectionId,
      )
    : undefined;

  async function handleSave(
    props: Record<string, unknown>,
    hidden: boolean,
  ) {
    setIsSaving(true);
    setErrorMessage('');
    setStatusMessage('');
    try {
      // Diff against the current block.props and dispatch field edits via
      // the inline editor's editField. We update every changed top-level key
      // and the hidden setting.
      const currentProps = block.props as Record<string, unknown>;
      const allKeys = new Set([
        ...Object.keys(currentProps),
        ...Object.keys(props),
      ]);
      for (const key of allKeys) {
        const next = (props as Record<string, unknown>)[key];
        if (!shallowEqual(currentProps[key], next)) {
          ctx?.editField(block.id, [key], next);
        }
      }
      const isHidden = Boolean(block.settings?.hidden);
      if (isHidden !== hidden) {
        await ctx?.onToggleHidden(block.id, hidden);
      }
      setStatusMessage('Saved.');
    } catch (error) {
      setErrorMessage(
        error instanceof Error ? error.message : 'Could not save block',
      );
    } finally {
      setIsSaving(false);
    }
  }

  // Portal to <body>: the drawer is a viewport overlay, but it mounts inside
  // the builder's block subtree, where the canvas transform re-anchors
  // position:fixed and the block wrapper's `isolate` traps its z-index under
  // later sibling blocks — leaving the drawer half-buried and click-blocked.
  return createPortal(
    <div
      role="dialog"
      aria-modal="true"
      aria-label={`Edit ${definition?.displayName ?? block.type} details`}
      className="fixed inset-0 z-50 flex justify-end bg-[oklch(8%_0.02_336_/_0.45)] backdrop-blur-sm"
      onClick={(event) => {
        if (event.target === event.currentTarget) onClose();
      }}
    >
      <div className={cn('flex h-full w-full max-w-[460px] flex-col overflow-hidden border-l border-border bg-[var(--surface-1)] shadow-[-30px_0_70px_oklch(8%_0.02_336_/_0.55)]')}>
        <header className="flex items-start justify-between gap-3 border-b border-border px-5 py-4">
          <div>
            <p className={text.eyebrow}>Block details</p>
            <h2 className="mt-0.5 text-[1rem] font-extrabold leading-tight text-[var(--paper)]">
              {definition?.displayName ?? block.type}
            </h2>
            <p className="mt-1 text-xs text-[var(--paper-muted)]">
              Edit fields that can&rsquo;t be reached inline — CTAs, links,
              repeater items.
            </p>
          </div>
          <button
            type="button"
            onClick={onClose}
            aria-label="Close details"
            className="inline-flex size-8 items-center justify-center rounded-full text-[var(--paper-muted)] transition-colors hover:bg-[var(--surface-2)] hover:text-[var(--paper)]"
          >
            <X className="size-4" aria-hidden />
          </button>
        </header>

        <div className="grow overflow-y-auto p-4">
          <BlockEditor
            block={block}
            definition={definition}
            isSaving={isSaving}
            errorMessage={errorMessage}
            statusMessage={statusMessage}
            assetLibrary={ctx.assetLibrary}
            onSave={handleSave}
            onSuggest={ctx.onSuggestBlock}
            isSuggesting={ctx.isSuggestingBlock}
            siteId={ctx.siteId}
            onImageApplied={ctx.onImageApplied}
          />
          {isCollectionDetail && ownerCollection ? (
            <div className="mt-4">
              <BlockBindingsEditor
                block={block}
                definition={definition}
                collection={ownerCollection}
                onSave={(bindings) =>
                  ctx.onUpdateBindings(block.id, bindings)
                }
              />
            </div>
          ) : isCollectionDetail ? (
            <div className="mt-4 rounded-[10px] border border-border bg-[color-mix(in_oklch,var(--surface-2)_45%,transparent)] p-3 text-xs text-[var(--paper-muted)]">
              This template is not bound to a collection yet. Pick a
              collection in Pages → Bound collection to enable field
              bindings.
            </div>
          ) : null}
        </div>
      </div>
    </div>,
    document.body,
  );
}

function shallowEqual(a: unknown, b: unknown): boolean {
  if (a === b) return true;
  if (typeof a !== typeof b) return false;
  if (a === null || b === null) return false;
  if (typeof a !== 'object') return false;
  if (Array.isArray(a) || Array.isArray(b)) {
    return JSON.stringify(a) === JSON.stringify(b);
  }
  return JSON.stringify(a) === JSON.stringify(b);
}

// Keep import to silence unused warning while we may use it later.
export const __unused = setAtPath;
