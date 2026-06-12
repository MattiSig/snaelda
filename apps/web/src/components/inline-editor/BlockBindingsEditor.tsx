import { useMemo, useState } from 'react';
import type {
  BlockBinding,
  BlockDefinition,
  Collection,
  SiteDraft,
} from '@/lib/api';
import { text } from '@/lib/styles';
import {
  compatibleEntryFields,
  isPropKeyBindable,
  listBindablePropFields,
} from './bindings';

type DraftBlock = SiteDraft['pages'][number]['blocks'][number];

type BlockBindingsEditorProps = {
  block: DraftBlock;
  definition: BlockDefinition | undefined;
  collection: Collection;
  onSave: (bindings: Record<string, BlockBinding>) => Promise<void> | void;
};

export function BlockBindingsEditor({
  block,
  definition,
  collection,
  onSave,
}: BlockBindingsEditorProps) {
  const initial = useMemo(
    () => normalizeBindings(block.bindings),
    [block.bindings],
  );
  const [draft, setDraft] = useState<Record<string, string>>(initial);
  const [isSaving, setIsSaving] = useState(false);
  const [errorMessage, setErrorMessage] = useState('');
  const [statusMessage, setStatusMessage] = useState('');

  const bindableFields = useMemo(
    () => listBindablePropFields(definition?.editorSchema),
    [definition],
  );

  if (bindableFields.length === 0) {
    return (
      <div className="grid gap-2 rounded-[10px] border border-border bg-[color-mix(in_oklch,var(--surface-2)_45%,transparent)] p-3">
        <p className={text.eyebrow}>Bindings</p>
        <p className="m-0 text-xs text-[var(--paper-muted)]">
          This block does not expose bindable fields.
        </p>
      </div>
    );
  }

  const isDirty = !shallowEqual(initial, draft);

  function handleSelect(propName: string, value: string) {
    setStatusMessage('');
    setErrorMessage('');
    setDraft((previous) => {
      const next = { ...previous };
      if (value === '') {
        delete next[propName];
      } else {
        next[propName] = value;
      }
      return next;
    });
  }

  async function handleApply() {
    setIsSaving(true);
    setErrorMessage('');
    setStatusMessage('');
    try {
      const bindings: Record<string, BlockBinding> = {};
      for (const [propName, fieldKey] of Object.entries(draft)) {
        bindings[propName] = { source: 'entry', field: fieldKey };
      }
      await onSave(bindings);
      setStatusMessage('Bindings saved.');
    } catch (error) {
      setErrorMessage(
        error instanceof Error ? error.message : 'Could not save bindings',
      );
    } finally {
      setIsSaving(false);
    }
  }

  function handleReset() {
    setDraft(initial);
    setErrorMessage('');
    setStatusMessage('');
  }

  return (
    <div className="grid gap-3 rounded-[10px] border border-border bg-[color-mix(in_oklch,var(--surface-2)_45%,transparent)] p-3">
      <div className="grid gap-1">
        <p className={text.eyebrow}>Bindings</p>
        <p className="m-0 text-xs text-[var(--paper-muted)]">
          Connect block fields to entry fields in{' '}
          <strong className="font-bold text-[var(--paper)]">
            {collection.pluralLabel}
          </strong>
          . The bound value replaces the literal prop when this template
          renders for an entry.
        </p>
      </div>

      <div className="grid gap-3">
        {bindableFields.map((field) => {
          const candidates = compatibleEntryFields(
            field.name,
            collection.schema,
          );
          const value = draft[field.name] ?? '';
          const selectId = `binding-${field.name}`;
          return (
            <div key={field.name} className="grid gap-1">
              <label
                htmlFor={selectId}
                className="text-xs font-bold uppercase tracking-[0.08em] text-[var(--paper-muted)]"
              >
                {field.label} · prop {field.name}
              </label>
              <select
                id={selectId}
                value={value}
                onChange={(event) => handleSelect(field.name, event.target.value)}
                disabled={candidates.length === 0 || isSaving}
                className="rounded-[8px] border border-border bg-[var(--surface-1)] px-3 py-2 text-sm text-[var(--paper)] focus:outline-none focus:ring-2 focus:ring-[var(--thread-violet)]"
              >
                <option value="">Use literal prop value</option>
                {candidates.map((candidate) => (
                  <option key={candidate.key} value={candidate.key}>
                    {candidate.label} ({candidate.type})
                  </option>
                ))}
              </select>
              {candidates.length === 0 ? (
                <small className="text-[var(--paper-muted)]">
                  No compatible fields in this collection.
                </small>
              ) : null}
            </div>
          );
        })}
      </div>

      {errorMessage ? <p className={text.error}>{errorMessage}</p> : null}
      {statusMessage ? <p className={text.success}>{statusMessage}</p> : null}

      <div className="flex items-center gap-2">
        <button
          type="button"
          onClick={handleApply}
          disabled={isSaving || !isDirty}
          className="inline-flex h-8 items-center justify-center rounded-full bg-[var(--thread-violet)] px-3 text-xs font-bold uppercase tracking-[0.08em] text-white disabled:cursor-not-allowed disabled:opacity-50"
        >
          {isSaving ? 'Saving…' : 'Save bindings'}
        </button>
        <button
          type="button"
          onClick={handleReset}
          disabled={isSaving || !isDirty}
          className="inline-flex h-8 items-center justify-center rounded-full border border-border px-3 text-xs font-bold uppercase tracking-[0.08em] text-[var(--paper)] disabled:cursor-not-allowed disabled:opacity-50"
        >
          Reset
        </button>
      </div>
    </div>
  );
}

function normalizeBindings(
  bindings: Record<string, BlockBinding> | undefined,
): Record<string, string> {
  if (!bindings) return {};
  const result: Record<string, string> = {};
  for (const [propName, binding] of Object.entries(bindings)) {
    if (
      binding &&
      binding.source === 'entry' &&
      typeof binding.field === 'string' &&
      binding.field !== '' &&
      isPropKeyBindable(propName)
    ) {
      result[propName] = binding.field;
    }
  }
  return result;
}

function shallowEqual(
  a: Record<string, string>,
  b: Record<string, string>,
): boolean {
  const keysA = Object.keys(a);
  const keysB = Object.keys(b);
  if (keysA.length !== keysB.length) return false;
  for (const key of keysA) {
    if (a[key] !== b[key]) return false;
  }
  return true;
}
