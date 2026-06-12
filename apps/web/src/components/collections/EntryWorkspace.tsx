import {
  type FormEvent,
  type ReactNode,
  useEffect,
  useMemo,
  useState,
} from 'react'
import {
  ArrowDown,
  ArrowUp,
  Copy,
  History,
  ImagePlus,
  Plus,
  Sparkles,
  Trash2,
  WandSparkles,
} from 'lucide-react'
import { Button } from '@/components/ui/button'
import { Checkbox } from '@/components/ui/checkbox'
import { Input } from '@/components/ui/input'
import { Select } from '@/components/ui/select'
import { Textarea } from '@/components/ui/textarea'
import { buildDraftAssetURL } from '@/lib/assets'
import type {
  APIErrorPayload,
  AssetRecord,
  Collection,
  CollectionEntry,
  DraftRevisionRecord,
  FieldDefinition,
  RepromptHistoryRecord,
} from '@/lib/api'
import {
  APIError,
  createCollectionEntry,
  deleteCollectionEntry,
  draftCollectionEntriesFromPrompt,
  duplicateCollectionEntry,
  getDraftRevision,
  listCollectionEntries,
  listRepromptHistory,
  listSiteAssets,
  reorderCollectionEntries,
  repromptCollectionEntry,
  revertReprompt,
  updateCollectionEntry,
} from '@/lib/api'
import {
  buildCreateEntryPayload,
  buildUpdateEntryPayload,
  createEntryEditorValues,
  type EntryEditorValues,
  type EntryValidationErrors,
  normalizeFieldValue,
  validateEntryEditorValues,
} from '@/lib/collection-entry-form'
import { actions, form, paddedPanel, text } from '@/lib/styles'
import { cn } from '@/lib/utils'

const NEW_ENTRY_ID = '__new__'

type EntryWorkspaceProps = {
  siteId: string
  collection: Collection
  collections: Collection[]
  onEntriesChanged: (entries: Collection['entries']) => void
}

export function EntryWorkspace({
  siteId,
  collection,
  collections,
  onEntriesChanged,
}: EntryWorkspaceProps) {
  const entries = collection.entries ?? []
  const [selectedId, setSelectedId] = useState(entries[0]?.id ?? NEW_ENTRY_ID)
  const [trackedCollectionKey, setTrackedCollectionKey] = useState(collection.id)
  if (trackedCollectionKey !== collection.id) {
    setTrackedCollectionKey(collection.id)
    setSelectedId(entries[0]?.id ?? NEW_ENTRY_ID)
  } else if (
    selectedId !== NEW_ENTRY_ID &&
    !entries.some((entry) => entry.id === selectedId)
  ) {
    setSelectedId(entries[0]?.id ?? NEW_ENTRY_ID)
  }

  const selectedEntry =
    entries.find((entry) => entry.id === selectedId) ?? null
  const isCreating = selectedId === NEW_ENTRY_ID || !selectedEntry

  const [trackedEditorKey, setTrackedEditorKey] = useState(
    `${collection.id}:${selectedEntry?.id ?? NEW_ENTRY_ID}`,
  )
  const [editorValues, setEditorValues] = useState<EntryEditorValues>(() =>
    createEntryEditorValues(collection.schema, selectedEntry),
  )
  if (
    trackedEditorKey !== `${collection.id}:${selectedEntry?.id ?? NEW_ENTRY_ID}`
  ) {
    setTrackedEditorKey(`${collection.id}:${selectedEntry?.id ?? NEW_ENTRY_ID}`)
    setEditorValues(createEntryEditorValues(collection.schema, selectedEntry))
  }

  const [assets, setAssets] = useState<AssetRecord[]>([])
  const [history, setHistory] = useState<RepromptHistoryRecord[]>([])
  const [errorMessage, setErrorMessage] = useState('')
  const [statusMessage, setStatusMessage] = useState('')
  const [fieldErrors, setFieldErrors] = useState<EntryValidationErrors>({})
  const [showPromptEntries, setShowPromptEntries] = useState(false)
  const [showRewrite, setShowRewrite] = useState(false)
  const [promptValue, setPromptValue] = useState('')
  const [rewritePrompt, setRewritePrompt] = useState('')
  const [isSaving, setIsSaving] = useState(false)
  const [isPrompting, setIsPrompting] = useState(false)
  const [isRewriting, setIsRewriting] = useState(false)
  const [activeHistory, setActiveHistory] = useState<RepromptHistoryRecord | null>(null)
  const [previousRevision, setPreviousRevision] = useState<DraftRevisionRecord | null>(null)
  const [resultRevision, setResultRevision] = useState<DraftRevisionRecord | null>(null)
  const [isLoadingDiff, setIsLoadingDiff] = useState(false)
  const [diffError, setDiffError] = useState('')
  const [activeRevertId, setActiveRevertId] = useState<string | null>(null)

  useEffect(() => {
    let active = true
    void (async () => {
      try {
        const response = await listSiteAssets(siteId)
        if (active) {
          setAssets(response.assets)
        }
      } catch {
        // Asset loading is supportive, not critical to entry editing.
      }
    })()
    return () => {
      active = false
    }
  }, [siteId])

  useEffect(() => {
    let active = true
    void (async () => {
      try {
        const response = await listRepromptHistory(siteId)
        if (active) {
          setHistory(response.reprompts)
        }
      } catch {
        // History remains optional until the next successful write.
      }
    })()
    return () => {
      active = false
    }
  }, [siteId, collection.id, selectedEntry?.id])

  const entryHistory = useMemo(
    () =>
      history.filter(
        (item) =>
          item.scope === 'entry' &&
          Boolean(selectedEntry?.id) &&
          item.targetId === selectedEntry?.id,
      ),
    [history, selectedEntry?.id],
  )

  async function refreshEntries(preferredEntryId?: string) {
    const response = await listCollectionEntries(siteId, collection.id)
    onEntriesChanged(response.entries)
    if (preferredEntryId) {
      setSelectedId(preferredEntryId)
      return
    }
    if (response.entries.length === 0) {
      setSelectedId(NEW_ENTRY_ID)
    }
  }

  async function refreshHistory() {
    try {
      const response = await listRepromptHistory(siteId)
      setHistory(response.reprompts)
    } catch {
      // History remains optional until the next successful write.
    }
  }

  function setFieldValue(fieldKey: string, value: unknown) {
    setEditorValues((current) => ({
      ...current,
      fields: {
        ...current.fields,
        [fieldKey]: value,
      },
    }))
    setFieldErrors((current) => {
      if (!current[`fields.${fieldKey}`]) return current
      const next = { ...current }
      delete next[`fields.${fieldKey}`]
      return next
    })
  }

  function clearFeedback() {
    setErrorMessage('')
    setStatusMessage('')
  }

  async function handleSave(event: FormEvent<HTMLFormElement>) {
    event.preventDefault()
    clearFeedback()
    const errors = validateEntryEditorValues(collection.schema, editorValues)
    setFieldErrors(errors)
    if (Object.keys(errors).length > 0) {
      setErrorMessage('Fix the highlighted fields and try again.')
      return
    }

    setIsSaving(true)
    try {
      if (selectedEntry) {
        const payload = buildUpdateEntryPayload(
          collection.schema,
          selectedEntry,
          editorValues,
        )
        if (!payload.slug && !payload.status && !payload.seo && !payload.fields) {
          setStatusMessage('No entry changes to save.')
          return
        }
        const response = await updateCollectionEntry(
          siteId,
          collection.id,
          selectedEntry.id,
          payload,
        )
        await refreshEntries(response.entry.id)
        setStatusMessage('Entry updated.')
      } else {
        const response = await createCollectionEntry(
          siteId,
          collection.id,
          buildCreateEntryPayload(collection.schema, editorValues),
        )
        await refreshEntries(response.entry.id)
        setStatusMessage('Entry created.')
      }
    } catch (error) {
      handleAPIError(error, 'Could not save entry.')
    } finally {
      setIsSaving(false)
    }
  }

  async function handleDelete(entryId: string) {
    if (!confirm('Delete this entry?')) return
    clearFeedback()
    try {
      await deleteCollectionEntry(siteId, collection.id, entryId)
      await refreshEntries()
      setStatusMessage('Entry deleted.')
    } catch (error) {
      handleAPIError(error, 'Could not delete entry.')
    }
  }

  async function handleDuplicate(entryId: string) {
    clearFeedback()
    try {
      const response = await duplicateCollectionEntry(siteId, collection.id, entryId)
      await refreshEntries(response.entry.id)
      setStatusMessage('Draft duplicate created.')
    } catch (error) {
      handleAPIError(error, 'Could not duplicate entry.')
    }
  }

  async function moveEntry(entryId: string, direction: -1 | 1) {
    const index = entries.findIndex((entry) => entry.id === entryId)
    if (index === -1) return
    const targetIndex = index + direction
    if (targetIndex < 0 || targetIndex >= entries.length) return
    clearFeedback()
    const next = [...entries]
    const [moved] = next.splice(index, 1)
    next.splice(targetIndex, 0, moved)
    try {
      await reorderCollectionEntries(
        siteId,
        collection.id,
        next.map((entry) => entry.id),
      )
      await refreshEntries(entryId)
      setStatusMessage('Entry order updated.')
    } catch (error) {
      handleAPIError(error, 'Could not reorder entries.')
    }
  }

  async function handlePromptEntries(event: FormEvent<HTMLFormElement>) {
    event.preventDefault()
    const prompt = promptValue.trim()
    if (!prompt) return
    clearFeedback()
    setIsPrompting(true)
    try {
      const response = await draftCollectionEntriesFromPrompt(siteId, collection.id, {
        prompt,
      })
      await refreshEntries()
      await refreshHistory()
      setPromptValue('')
      setShowPromptEntries(false)
      setStatusMessage(
        response.entries.length === 1
          ? 'Drafted 1 entry.'
          : `Drafted ${response.entries.length} entries.`,
      )
    } catch (error) {
      handleAPIError(error, 'Could not draft entries from that prompt.')
    } finally {
      setIsPrompting(false)
    }
  }

  async function handleRewrite(event: FormEvent<HTMLFormElement>) {
    event.preventDefault()
    if (!selectedEntry) return
    const prompt = rewritePrompt.trim()
    if (!prompt) return
    clearFeedback()
    setIsRewriting(true)
    try {
      await repromptCollectionEntry(siteId, collection.id, selectedEntry.id, {
        prompt,
      })
      await Promise.all([
        refreshEntries(selectedEntry.id),
        refreshHistory(),
      ])
      setRewritePrompt('')
      setShowRewrite(false)
      setStatusMessage('Entry rewrite saved as a checkpoint.')
    } catch (error) {
      handleAPIError(error, 'Could not rewrite this entry.')
    } finally {
      setIsRewriting(false)
    }
  }

  async function handleShowDiff(item: RepromptHistoryRecord) {
    setActiveHistory(item)
    setDiffError('')
    setIsLoadingDiff(true)
    try {
      const [previous, result] = await Promise.all([
        getDraftRevision(siteId, item.previousRevisionId),
        getDraftRevision(siteId, item.resultRevisionId),
      ])
      setPreviousRevision(previous.revision)
      setResultRevision(result.revision)
    } catch (error) {
      setDiffError(
        error instanceof APIError ? error.message : 'Could not load revision diff.',
      )
    } finally {
      setIsLoadingDiff(false)
    }
  }

  async function handleRevert(item: RepromptHistoryRecord) {
    setActiveRevertId(item.id)
    clearFeedback()
    try {
      await revertReprompt(siteId, item.id)
      await Promise.all([
        refreshEntries(selectedEntry?.id),
        refreshHistory(),
      ])
      setStatusMessage('Entry reverted to the previous checkpoint.')
    } catch (error) {
      handleAPIError(error, 'Could not revert that checkpoint.')
    } finally {
      setActiveRevertId(null)
    }
  }

  function handleAPIError(error: unknown, fallback: string) {
    if (isDraftConflictError(error)) {
      void Promise.all([refreshEntries(selectedEntry?.id), refreshHistory()])
      setErrorMessage(
        'This draft changed in another tab or request. The latest entry data was reloaded; apply your change again.',
      )
      return
    }
    const payload = error instanceof APIError ? error.payload : null
    const issueErrors = mapIssueErrors(payload)
    setFieldErrors(issueErrors)
    setErrorMessage(error instanceof APIError ? error.message : fallback)
  }

  return (
    <div className="grid gap-4 xl:grid-cols-[minmax(280px,340px)_minmax(0,1fr)]">
      <aside className={cn(paddedPanel, 'grid gap-3 self-start')}>
        <div className="flex flex-wrap items-start justify-between gap-3">
          <div className="grid gap-1">
            <p className={text.label}>Entries</p>
            <h3 className={text.sectionTitle}>{collection.pluralLabel}</h3>
            <p className={cn(text.muted, 'text-sm')}>
              Edit the rows behind /{collection.slug} and any bound detail pages.
            </p>
          </div>
          <div className="rounded-full border border-border bg-[var(--surface-2)] px-3 py-2 text-xs font-bold uppercase tracking-[0.08em] text-[var(--paper-muted)]">
            {entries.length} saved
          </div>
        </div>

        <div className={actions.row}>
          <Button
            type="button"
            size="sm"
            onClick={() => {
              clearFeedback()
              setFieldErrors({})
              setSelectedId(NEW_ENTRY_ID)
              setShowPromptEntries(false)
            }}
          >
            <Plus className="size-4" />
            New entry
          </Button>
          <Button
            type="button"
            size="sm"
            variant="outline"
            disabled={collection.schema.length === 0}
            onClick={() => {
              clearFeedback()
              setShowPromptEntries((current) => !current)
            }}
          >
            <Sparkles className="size-4" />
            Prompt entries
          </Button>
        </div>

        {showPromptEntries ? (
          <form
            className="grid gap-3 rounded-[14px] border border-border bg-[var(--surface-2)] p-4"
            onSubmit={handlePromptEntries}
          >
            <div className="grid gap-1.5">
              <p className={cn(text.h3, 'text-base')}>Draft starter entries</p>
              <p className={cn(text.muted, 'text-sm')}>
                Use the current schema to spin up draft rows, then tighten them by hand.
              </p>
            </div>
            <Textarea
              value={promptValue}
              rows={4}
              placeholder={`Add three ${collection.pluralLabel.toLowerCase()} for a small business.`}
              onChange={(event) => setPromptValue(event.target.value)}
              disabled={isPrompting}
            />
            <div className={actions.row}>
              <Button type="submit" disabled={isPrompting || !promptValue.trim()}>
                {isPrompting ? 'Drafting...' : 'Draft entries'}
              </Button>
              <Button
                type="button"
                variant="outline"
                disabled={isPrompting}
                onClick={() => setShowPromptEntries(false)}
              >
                Cancel
              </Button>
            </div>
          </form>
        ) : null}

        <div className="grid gap-2">
          <button
            type="button"
            onClick={() => setSelectedId(NEW_ENTRY_ID)}
            className={cn(
              'grid gap-1 rounded-[14px] border px-4 py-3 text-left transition-colors',
              isCreating
                ? 'border-[color-mix(in_oklch,var(--thread-teal)_72%,var(--border))] bg-[color-mix(in_oklch,var(--thread-mauve)_15%,var(--surface-1))]'
                : 'border-border bg-[var(--surface-2)] hover:bg-[var(--surface-3)]',
            )}
          >
            <span className={cn(text.h3, 'text-base')}>New draft entry</span>
            <span className="text-sm text-[var(--paper-muted)]">
              Start from the current schema and add SEO before you publish.
            </span>
          </button>

          {entries.length === 0 ? (
            <div className="rounded-[14px] border border-dashed border-border bg-[var(--surface-2)] p-4 text-sm text-[var(--paper-muted)]">
              No entries yet. Create one manually or let AI draft a few starters first.
            </div>
          ) : (
            entries.map((entry, index) => (
              <article
                key={entry.id}
                className={cn(
                  'grid gap-3 rounded-[14px] border px-4 py-3 transition-colors',
                  entry.id === selectedEntry?.id
                    ? 'border-[color-mix(in_oklch,var(--thread-teal)_72%,var(--border))] bg-[color-mix(in_oklch,var(--thread-mauve)_15%,var(--surface-1))]'
                    : 'border-border bg-[var(--surface-2)] hover:bg-[var(--surface-3)]',
                )}
              >
                <button
                  type="button"
                  onClick={() => setSelectedId(entry.id)}
                  className="grid gap-1 text-left"
                >
                  <div className="flex flex-wrap items-center gap-2">
                    <p className={cn(text.h3, 'text-base')}>
                      {resolveEntryTitle(entry, collection.schema)}
                    </p>
                    <span className="rounded-full bg-[color-mix(in_oklch,var(--thread-gold)_18%,var(--surface-1))] px-2.5 py-1 text-[11px] font-bold uppercase tracking-[0.08em] text-[var(--paper-muted)]">
                      {entry.status ?? 'draft'}
                    </span>
                  </div>
                  <p className="text-sm text-[var(--paper-muted)]">
                    /{collection.slug}/{entry.slug}
                  </p>
                </button>
                <div className="flex flex-wrap gap-2">
                  <Button
                    type="button"
                    size="sm"
                    variant="outline"
                    onClick={() => moveEntry(entry.id, -1)}
                    disabled={index === 0}
                    aria-label={`Move ${resolveEntryTitle(entry, collection.schema)} earlier`}
                  >
                    <ArrowUp className="size-4" />
                  </Button>
                  <Button
                    type="button"
                    size="sm"
                    variant="outline"
                    onClick={() => moveEntry(entry.id, 1)}
                    disabled={index === entries.length - 1}
                    aria-label={`Move ${resolveEntryTitle(entry, collection.schema)} later`}
                  >
                    <ArrowDown className="size-4" />
                  </Button>
                  <Button
                    type="button"
                    size="sm"
                    variant="outline"
                    onClick={() => handleDuplicate(entry.id)}
                  >
                    <Copy className="size-4" />
                    Duplicate
                  </Button>
                  <Button
                    type="button"
                    size="sm"
                    variant="outline"
                    onClick={() => handleDelete(entry.id)}
                  >
                    <Trash2 className="size-4" />
                  </Button>
                </div>
              </article>
            ))
          )}
        </div>
      </aside>

      <div className="grid gap-4">
        <section className={cn(paddedPanel, 'grid gap-4')}>
          <div className="flex flex-wrap items-start justify-between gap-3">
            <div className="grid gap-1">
              <p className={text.label}>{isCreating ? 'Draft entry' : 'Entry editor'}</p>
              <h3 className={text.sectionTitle}>
                {isCreating
                  ? `New ${collection.singularLabel.toLowerCase()}`
                  : resolveEntryTitle(selectedEntry, collection.schema)}
              </h3>
              <p className={cn(text.muted, 'text-sm')}>
                Keep structured content tidy here so collection pages, filters, and SEO stay honest.
              </p>
            </div>
            {selectedEntry ? (
              <div className={actions.row}>
                <Button
                  type="button"
                  size="sm"
                  variant="outline"
                  onClick={() => setShowRewrite((current) => !current)}
                >
                  <WandSparkles className="size-4" />
                  Rewrite with AI
                </Button>
                <Button
                  type="button"
                  size="sm"
                  variant="outline"
                  onClick={() => handleDuplicate(selectedEntry.id)}
                >
                  <Copy className="size-4" />
                  Duplicate
                </Button>
              </div>
            ) : null}
          </div>

          {errorMessage ? <p className={text.error}>{errorMessage}</p> : null}
          {statusMessage ? <p className={text.success}>{statusMessage}</p> : null}

          {showRewrite && selectedEntry ? (
            <form
              className="grid gap-3 rounded-[14px] border border-border bg-[var(--surface-2)] p-4"
              onSubmit={handleRewrite}
            >
              <div className="grid gap-1.5">
                <p className={cn(text.h3, 'text-base')}>Rewrite this entry</p>
                <p className={cn(text.muted, 'text-sm')}>
                  Give AI a concrete direction. The result lands as a checkpoint you can diff or revert.
                </p>
              </div>
              <Textarea
                value={rewritePrompt}
                rows={4}
                placeholder={`Rewrite this ${collection.singularLabel.toLowerCase()} with clearer benefits, warmer copy, and tighter SEO.`}
                onChange={(event) => setRewritePrompt(event.target.value)}
                disabled={isRewriting}
              />
              <div className={actions.row}>
                <Button type="submit" disabled={isRewriting || !rewritePrompt.trim()}>
                  {isRewriting ? 'Rewriting...' : 'Rewrite entry'}
                </Button>
                <Button
                  type="button"
                  variant="outline"
                  disabled={isRewriting}
                  onClick={() => setShowRewrite(false)}
                >
                  Cancel
                </Button>
              </div>
            </form>
          ) : null}

          <form className={form.grid} onSubmit={handleSave}>
            <div className="grid gap-4 lg:grid-cols-[minmax(0,1.1fr)_minmax(280px,0.9fr)]">
              <div className="grid gap-4">
                <div className="grid gap-4 rounded-[14px] border border-border bg-[var(--surface-2)] p-4">
                  <div className="grid gap-4 md:grid-cols-2">
                    <LabeledField
                      label="Entry slug"
                      hint="Used for the published detail URL when the collection exposes one."
                      error={fieldErrors.slug}
                    >
                      <Input
                        value={editorValues.slug}
                        placeholder="auto-generated from the title if you leave this blank"
                        onChange={(event) =>
                          setEditorValues((current) => ({
                            ...current,
                            slug: event.target.value,
                          }))
                        }
                      />
                    </LabeledField>

                    <LabeledField label="Status">
                      <Select
                        value={editorValues.status}
                        onChange={(event) =>
                          setEditorValues((current) => ({
                            ...current,
                            status: event.target.value as 'draft' | 'published',
                          }))
                        }
                      >
                        <option value="draft">Draft</option>
                        <option value="published">Published</option>
                      </Select>
                    </LabeledField>
                  </div>

                  <div className="grid gap-1">
                    <p className={text.label}>Entry fields</p>
                    <p className="text-sm text-[var(--paper-muted)]">
                      Typed controls stay in sync with the collection schema so validation happens before save.
                    </p>
                  </div>
                  {collection.schema.map((field) => (
                    <EntryFieldControl
                      key={field.key}
                      field={field}
                      value={editorValues.fields[field.key]}
                      error={fieldErrors[`fields.${field.key}`]}
                      assets={assets}
                      collections={collections}
                      currentCollection={collection}
                      onChange={(value) => setFieldValue(field.key, value)}
                    />
                  ))}
                </div>
              </div>

              <div className="grid gap-4 self-start">
                <div className="grid gap-4 rounded-[14px] border border-border bg-[var(--surface-2)] p-4">
                  <div className="grid gap-1">
                    <p className={text.label}>SEO</p>
                    <p className="text-sm text-[var(--paper-muted)]">
                      Detail pages fall back to the entry title and summary, but this lets you tune the search snippet directly.
                    </p>
                  </div>
                  <LabeledField
                    label="SEO title"
                    error={fieldErrors['seo.title']}
                    hint="Keep it concise, usually under 70 characters."
                  >
                    <Input
                      value={editorValues.seo.title}
                      onChange={(event) =>
                        setEditorValues((current) => ({
                          ...current,
                          seo: { ...current.seo, title: event.target.value },
                        }))
                      }
                    />
                  </LabeledField>
                  <LabeledField
                    label="SEO description"
                    error={fieldErrors['seo.description']}
                    hint="Aim for one honest sentence under 180 characters."
                  >
                    <Textarea
                      rows={4}
                      value={editorValues.seo.description}
                      onChange={(event) =>
                        setEditorValues((current) => ({
                          ...current,
                          seo: {
                            ...current.seo,
                            description: event.target.value,
                          },
                        }))
                      }
                    />
                  </LabeledField>
                </div>

                {selectedEntry ? (
                  <div className="grid gap-3 rounded-[14px] border border-border bg-[var(--surface-2)] p-4">
                    <div className="flex items-center gap-2">
                      <History className="size-4 text-[var(--thread-teal)]" />
                      <p className={cn(text.h3, 'text-base')}>Entry history</p>
                    </div>
                    <p className="text-sm text-[var(--paper-muted)]">
                      AI rewrites land here as checkpoints. Diff them, then roll back if the thread goes the wrong way.
                    </p>
                    {entryHistory.length === 0 ? (
                      <div className="rounded-[12px] border border-dashed border-border bg-[var(--surface-1)] p-3 text-sm text-[var(--paper-muted)]">
                        No AI entry checkpoints yet for this row.
                      </div>
                    ) : (
                      <div className="grid gap-3">
                        {entryHistory.map((item) => (
                          <article
                            key={item.id}
                            className="grid gap-3 rounded-[12px] border border-border bg-[var(--surface-1)] p-3"
                          >
                            <div className="grid gap-1">
                              <div className="flex flex-wrap items-center gap-2">
                                <p className="text-sm font-bold text-[var(--paper)]">
                                  {item.changeSummary || item.prompt}
                                </p>
                                <span className="rounded-full bg-[color-mix(in_oklch,var(--thread-mauve)_18%,var(--surface-1))] px-2.5 py-1 text-[11px] font-bold uppercase tracking-[0.08em] text-[var(--paper-muted)]">
                                  Entry
                                </span>
                              </div>
                              <p className="text-xs uppercase tracking-[0.08em] text-[var(--paper-muted)]">
                                {formatRepromptTime(item.createdAt)}
                              </p>
                            </div>
                            <p className="text-sm text-[var(--paper-muted)]">“{item.prompt}”</p>
                            <div className="flex flex-wrap gap-2">
                              <Button
                                type="button"
                                size="sm"
                                variant="outline"
                                disabled={isLoadingDiff && activeHistory?.id === item.id}
                                onClick={() => handleShowDiff(item)}
                              >
                                {isLoadingDiff && activeHistory?.id === item.id
                                  ? 'Loading diff...'
                                  : 'Show diff'}
                              </Button>
                              <Button
                                type="button"
                                size="sm"
                                variant="outline"
                                disabled={Boolean(item.undoneAt) || activeRevertId === item.id}
                                onClick={() => handleRevert(item)}
                              >
                                {item.undoneAt
                                  ? 'Restored'
                                  : activeRevertId === item.id
                                    ? 'Restoring...'
                                    : 'Revert'}
                              </Button>
                            </div>
                          </article>
                        ))}
                      </div>
                    )}
                  </div>
                ) : null}
              </div>
            </div>

            <div className={actions.rowLarge}>
              <Button type="submit" disabled={isSaving}>
                {isSaving
                  ? isCreating
                    ? 'Creating entry...'
                    : 'Saving entry...'
                  : isCreating
                    ? 'Create entry'
                    : 'Save entry'}
              </Button>
              {selectedEntry ? (
                <Button
                  type="button"
                  variant="outline"
                  onClick={() => handleDelete(selectedEntry.id)}
                >
                  Delete entry
                </Button>
              ) : null}
            </div>
          </form>
        </section>
      </div>

      <EntryHistoryDiffModal
        reprompt={activeHistory}
        previousRevision={previousRevision}
        resultRevision={resultRevision}
        collection={collection}
        errorMessage={diffError}
        isLoading={isLoadingDiff}
        onClose={() => {
          setActiveHistory(null)
          setPreviousRevision(null)
          setResultRevision(null)
          setDiffError('')
          setIsLoadingDiff(false)
        }}
      />
    </div>
  )
}

function EntryFieldControl({
  field,
  value,
  error,
  assets,
  collections,
  currentCollection,
  onChange,
}: {
  field: FieldDefinition
  value: unknown
  error?: string
  assets: AssetRecord[]
  collections: Collection[]
  currentCollection: Collection
  onChange: (value: unknown) => void
}) {
  const id = `entry-field-${field.key}`
  switch (field.type) {
    case 'long_text':
      return (
        <LabeledField
          label={field.label}
          hint={field.description}
          required={field.required}
          error={error}
        >
          <Textarea
            id={id}
            rows={5}
            value={String(value ?? '')}
            onChange={(event) => onChange(event.target.value)}
          />
        </LabeledField>
      )
    case 'rich_text':
      return (
        <LabeledField
          label={field.label}
          hint={field.description || 'Use the quick inserts for headings, lists, or link copy.'}
          required={field.required}
          error={error}
        >
          <div className="grid gap-3">
            <div className="flex flex-wrap gap-2">
              {[
                { label: 'Heading', value: 'Heading\n' },
                { label: 'Bullets', value: '- Point one\n- Point two\n' },
                { label: 'Call to action', value: 'Learn more: https://example.com\n' },
              ].map((preset) => (
                <Button
                  key={preset.label}
                  type="button"
                  size="sm"
                  variant="outline"
                  onClick={() =>
                    onChange(`${String(value ?? '').trim()}\n${preset.value}`.trim())
                  }
                >
                  {preset.label}
                </Button>
              ))}
            </div>
            <Textarea
              id={id}
              rows={8}
              value={String(value ?? '')}
              onChange={(event) => onChange(event.target.value)}
            />
          </div>
        </LabeledField>
      )
    case 'boolean':
      return (
        <label className="grid gap-2 rounded-[14px] border border-border bg-[var(--surface-1)] p-4">
          <span className={text.label}>
            {field.label}
            {field.required ? ' *' : ''}
          </span>
          <span className="flex items-center gap-3 text-sm text-[var(--paper)]">
            <Checkbox
              checked={Boolean(value)}
              onChange={(event) => onChange(event.target.checked)}
            />
            {field.description || 'Enable this flag for the current entry.'}
          </span>
          {error ? <span className={text.error}>{error}</span> : null}
        </label>
      )
    case 'number':
      return (
        <LabeledField
          label={field.label}
          hint={field.description}
          required={field.required}
          error={error}
        >
          <Input
            id={id}
            type="number"
            value={typeof value === 'number' ? value : String(value ?? '')}
            onChange={(event) =>
              onChange(event.target.value === '' ? undefined : Number(event.target.value))
            }
          />
        </LabeledField>
      )
    case 'date':
      return (
        <LabeledField
          label={field.label}
          hint={field.description}
          required={field.required}
          error={error}
        >
          <Input
            id={id}
            type="date"
            value={String(value ?? '')}
            onChange={(event) => onChange(event.target.value)}
          />
        </LabeledField>
      )
    case 'url':
      return (
        <LabeledField
          label={field.label}
          hint={field.description}
          required={field.required}
          error={error}
        >
          <Input
            id={id}
            type="url"
            value={String(value ?? '')}
            onChange={(event) => onChange(event.target.value)}
          />
        </LabeledField>
      )
    case 'email':
      return (
        <LabeledField
          label={field.label}
          hint={field.description}
          required={field.required}
          error={error}
        >
          <Input
            id={id}
            type="email"
            value={String(value ?? '')}
            onChange={(event) => onChange(event.target.value)}
          />
        </LabeledField>
      )
    case 'phone':
      return (
        <LabeledField
          label={field.label}
          hint={field.description}
          required={field.required}
          error={error}
        >
          <Input
            id={id}
            type="tel"
            value={String(value ?? '')}
            onChange={(event) => onChange(event.target.value)}
          />
        </LabeledField>
      )
    case 'enum':
      return (
        <LabeledField
          label={field.label}
          hint={field.description}
          required={field.required}
          error={error}
        >
          <Select
            id={id}
            value={String(value ?? '')}
            onChange={(event) => onChange(event.target.value || undefined)}
          >
            <option value="">Choose an option</option>
            {(field.options ?? []).map((option) => (
              <option key={option} value={option}>
                {option}
              </option>
            ))}
          </Select>
        </LabeledField>
      )
    case 'enum_multi': {
      const selected = Array.isArray(value) ? value.map(String) : []
      return (
        <LabeledField
          label={field.label}
          hint={field.description}
          required={field.required}
          error={error}
        >
          <div className="grid gap-2">
            {(field.options ?? []).map((option) => {
              const checked = selected.includes(option)
              return (
                <label
                  key={option}
                  className="flex items-center gap-3 rounded-[12px] border border-border bg-[var(--surface-1)] px-3 py-2 text-sm text-[var(--paper)]"
                >
                  <Checkbox
                    checked={checked}
                    onChange={(event) => {
                      if (event.target.checked) {
                        onChange([...selected, option])
                        return
                      }
                      onChange(selected.filter((item) => item !== option))
                    }}
                  />
                  {option}
                </label>
              )
            })}
          </div>
        </LabeledField>
      )
    }
    case 'asset':
      return (
        <AssetField
          field={field}
          value={value}
          error={error}
          assets={assets}
          onChange={onChange}
        />
      )
    case 'asset_list':
      return (
        <AssetListField
          field={field}
          value={value}
          error={error}
          assets={assets}
          onChange={onChange}
        />
      )
    case 'location':
      return (
        <LocationField
          field={field}
          value={value}
          error={error}
          onChange={onChange}
        />
      )
    case 'reference':
      return (
        <ReferenceField
          field={field}
          value={value}
          error={error}
          collections={collections}
          currentCollection={currentCollection}
          onChange={onChange}
        />
      )
    default:
      return (
        <LabeledField
          label={field.label}
          hint={field.description}
          required={field.required}
          error={error}
        >
          <Input
            id={id}
            value={String(value ?? '')}
            onChange={(event) => onChange(event.target.value)}
          />
        </LabeledField>
      )
  }
}

function AssetField({
  field,
  value,
  error,
  assets,
  onChange,
}: {
  field: FieldDefinition
  value: unknown
  error?: string
  assets: AssetRecord[]
  onChange: (value: unknown) => void
}) {
  const selected = normalizeFieldValue(field, value) as
    | { assetId: string; alt?: string }
    | undefined
  const selectedAsset = assets.find((asset) => asset.id === selected?.assetId)

  return (
    <LabeledField
      label={field.label}
      hint={field.description || 'Choose from assets already uploaded on this site.'}
      required={field.required}
      error={error}
    >
      <div className="grid gap-3 rounded-[14px] border border-border bg-[var(--surface-1)] p-4">
        <Select
          value={selected?.assetId ?? ''}
          onChange={(event) => {
            const assetId = event.target.value
            if (!assetId) {
              onChange(undefined)
              return
            }
            const asset = assets.find((entry) => entry.id === assetId)
            onChange({
              assetId,
              alt: selected?.alt ?? asset?.altText ?? '',
            })
          }}
        >
          <option value="">Choose an uploaded asset</option>
          {assets.map((asset) => (
            <option key={asset.id} value={asset.id}>
              {asset.metadata.fileName || asset.id}
            </option>
          ))}
        </Select>
        {selectedAsset ? (
          <div className="grid gap-3 sm:grid-cols-[120px_minmax(0,1fr)] sm:items-start">
            <img
              src={buildDraftAssetURL(selectedAsset.id)}
              alt={selected?.alt || selectedAsset.altText || selectedAsset.metadata.fileName || 'Selected asset'}
              className="h-28 w-full rounded-[12px] object-cover"
            />
            <div className="grid gap-3">
              <div className="text-sm text-[var(--paper-muted)]">
                {selectedAsset.metadata.fileName || selectedAsset.id}
              </div>
              <Input
                value={selected?.alt ?? ''}
                placeholder="Describe this image for visitors and SEO"
                onChange={(event) =>
                  onChange({
                    assetId: selectedAsset.id,
                    alt: event.target.value,
                  })
                }
              />
            </div>
          </div>
        ) : (
          <div className="flex items-center gap-2 rounded-[12px] border border-dashed border-border bg-[var(--surface-2)] px-3 py-3 text-sm text-[var(--paper-muted)]">
            <ImagePlus className="size-4" />
            Upload an image in the Assets panel if you need a new option.
          </div>
        )}
      </div>
    </LabeledField>
  )
}

function AssetListField({
  field,
  value,
  error,
  assets,
  onChange,
}: {
  field: FieldDefinition
  value: unknown
  error?: string
  assets: AssetRecord[]
  onChange: (value: unknown) => void
}) {
  const items = Array.isArray(value) ? value : []
  return (
    <LabeledField
      label={field.label}
      hint={field.description || 'Build an ordered gallery from uploaded assets.'}
      required={field.required}
      error={error}
    >
      <div className="grid gap-3 rounded-[14px] border border-border bg-[var(--surface-1)] p-4">
        {items.length === 0 ? (
          <div className="rounded-[12px] border border-dashed border-border bg-[var(--surface-2)] px-3 py-3 text-sm text-[var(--paper-muted)]">
            No gallery items yet.
          </div>
        ) : (
          items.map((item, index) => {
            const normalized = normalizeFieldValue({ ...field, type: 'asset' }, item) as
              | { assetId: string; alt?: string }
              | undefined
            const selectedAsset = assets.find((asset) => asset.id === normalized?.assetId)
            return (
              <div
                key={`${normalized?.assetId ?? 'asset'}-${index}`}
                className="grid gap-3 rounded-[12px] border border-border bg-[var(--surface-2)] p-3"
              >
                <div className="grid gap-3 sm:grid-cols-[minmax(0,1fr)_160px]">
                  <Select
                    value={normalized?.assetId ?? ''}
                    onChange={(event) => {
                      const next = [...items]
                      next[index] = {
                        assetId: event.target.value,
                        alt: normalized?.alt ?? '',
                      }
                      onChange(next)
                    }}
                  >
                    <option value="">Choose an uploaded asset</option>
                    {assets.map((asset) => (
                      <option key={asset.id} value={asset.id}>
                        {asset.metadata.fileName || asset.id}
                      </option>
                    ))}
                  </Select>
                  <Input
                    value={normalized?.alt ?? ''}
                    placeholder="Alt text"
                    onChange={(event) => {
                      const next = [...items]
                      next[index] = {
                        assetId: normalized?.assetId ?? '',
                        alt: event.target.value,
                      }
                      onChange(next)
                    }}
                  />
                </div>
                {selectedAsset ? (
                  <img
                    src={buildDraftAssetURL(selectedAsset.id)}
                    alt={normalized?.alt || selectedAsset.altText || selectedAsset.metadata.fileName || 'Gallery item'}
                    className="h-32 w-full rounded-[12px] object-cover"
                  />
                ) : null}
                <div className="flex flex-wrap gap-2">
                  <Button
                    type="button"
                    size="sm"
                    variant="outline"
                    disabled={index === 0}
                    onClick={() => {
                      const next = [...items]
                      ;[next[index - 1], next[index]] = [next[index], next[index - 1]]
                      onChange(next)
                    }}
                  >
                    <ArrowUp className="size-4" />
                  </Button>
                  <Button
                    type="button"
                    size="sm"
                    variant="outline"
                    disabled={index === items.length - 1}
                    onClick={() => {
                      const next = [...items]
                      ;[next[index + 1], next[index]] = [next[index], next[index + 1]]
                      onChange(next)
                    }}
                  >
                    <ArrowDown className="size-4" />
                  </Button>
                  <Button
                    type="button"
                    size="sm"
                    variant="outline"
                    onClick={() => onChange(items.filter((_, itemIndex) => itemIndex !== index))}
                  >
                    <Trash2 className="size-4" />
                  </Button>
                </div>
              </div>
            )
          })
        )}
        <Button
          type="button"
          variant="outline"
          onClick={() => onChange([...items, { assetId: '', alt: '' }])}
        >
          <Plus className="size-4" />
          Add gallery item
        </Button>
      </div>
    </LabeledField>
  )
}

function LocationField({
  field,
  value,
  error,
  onChange,
}: {
  field: FieldDefinition
  value: unknown
  error?: string
  onChange: (value: unknown) => void
}) {
  const location = (normalizeFieldValue(field, value) as Record<string, string> | undefined) ?? {}
  return (
    <LabeledField
      label={field.label}
      hint={field.description || 'Capture the place name first, then add optional region or country details.'}
      required={field.required}
      error={error}
    >
      <div className="grid gap-3 rounded-[14px] border border-border bg-[var(--surface-1)] p-4 md:grid-cols-2">
        {[
          ['name', 'Place name'],
          ['region', 'Region'],
          ['country', 'Country'],
          ['lat', 'Latitude'],
          ['lng', 'Longitude'],
        ].map(([key, label]) => (
          <Input
            key={key}
            value={location[key] ?? ''}
            placeholder={label}
            onChange={(event) =>
              onChange({
                ...location,
                [key]: event.target.value,
              })
            }
          />
        ))}
      </div>
    </LabeledField>
  )
}

function ReferenceField({
  field,
  value,
  error,
  collections,
  currentCollection,
  onChange,
}: {
  field: FieldDefinition
  value: unknown
  error?: string
  collections: Collection[]
  currentCollection: Collection
  onChange: (value: unknown) => void
}) {
  const reference = (normalizeFieldValue(field, value) as
    | { collectionId: string; entryId: string }
    | undefined) ?? { collectionId: '', entryId: '' }
  const availableCollections = collections.filter(
    (collection) => (collection.entries?.length ?? 0) > 0 || collection.id === currentCollection.id,
  )
  const targetCollection =
    availableCollections.find((collection) => collection.id === reference.collectionId) ??
    availableCollections[0] ??
    null
  const targetEntries = targetCollection?.entries ?? []

  return (
    <LabeledField
      label={field.label}
      hint={field.description || 'Link this row to another entry on the same site.'}
      required={field.required}
      error={error}
    >
      <div className="grid gap-3 rounded-[14px] border border-border bg-[var(--surface-1)] p-4 md:grid-cols-2">
        <Select
          value={reference.collectionId || targetCollection?.id || ''}
          onChange={(event) => {
            const collectionId = event.target.value
            const nextEntries =
              collections.find((collection) => collection.id === collectionId)?.entries ?? []
            onChange({
              collectionId,
              entryId: nextEntries[0]?.id ?? '',
            })
          }}
        >
          <option value="">Choose a collection</option>
          {availableCollections.map((collection) => (
            <option key={collection.id} value={collection.id}>
              {collection.pluralLabel}
            </option>
          ))}
        </Select>
        <Select
          value={reference.entryId}
          onChange={(event) =>
            onChange({
              collectionId: reference.collectionId || targetCollection?.id || '',
              entryId: event.target.value,
            })
          }
        >
          <option value="">Choose an entry</option>
          {targetEntries.map((entry) => (
            <option key={entry.id} value={entry.id}>
              {resolveEntryTitle(entry, targetCollection?.schema ?? currentCollection.schema)}
            </option>
          ))}
        </Select>
      </div>
    </LabeledField>
  )
}

function LabeledField({
  label,
  hint,
  required,
  error,
  children,
}: {
  label: string
  hint?: string
  required?: boolean
  error?: string
  children: ReactNode
}) {
  return (
    <label className={form.field}>
      <span className={text.label}>
        {label}
        {required ? ' *' : ''}
      </span>
      {hint ? <span className="text-sm text-[var(--paper-muted)]">{hint}</span> : null}
      {children}
      {error ? <span className={text.error}>{error}</span> : null}
    </label>
  )
}

function EntryHistoryDiffModal({
  reprompt,
  previousRevision,
  resultRevision,
  collection,
  errorMessage,
  isLoading,
  onClose,
}: {
  reprompt: RepromptHistoryRecord | null
  previousRevision: DraftRevisionRecord | null
  resultRevision: DraftRevisionRecord | null
  collection: Collection
  errorMessage?: string
  isLoading?: boolean
  onClose: () => void
}) {
  useEffect(() => {
    if (!reprompt) return
    function handleKeyDown(event: KeyboardEvent) {
      if (event.key === 'Escape') {
        event.preventDefault()
        onClose()
      }
    }
    window.addEventListener('keydown', handleKeyDown)
    return () => window.removeEventListener('keydown', handleKeyDown)
  }, [onClose, reprompt])

  if (!reprompt) {
    return null
  }

  const previousEntry = previousRevision
    ? findRevisionEntry(previousRevision, collection.id, reprompt.targetId)
    : null
  const resultEntry = resultRevision
    ? findRevisionEntry(resultRevision, collection.id, reprompt.targetId)
    : null
  const rows = buildEntryDiffRows(collection.schema, previousEntry, resultEntry)

  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center bg-[oklch(12%_0.02_330_/_0.7)] px-4 py-6 backdrop-blur-sm">
      <div className="w-full max-w-5xl rounded-[20px] border border-border bg-[var(--surface-1)] shadow-[0_28px_100px_oklch(7%_0.03_340_/_0.5)]">
        <div className="flex flex-wrap items-start justify-between gap-4 border-b border-border px-6 py-5">
          <div className="grid gap-1">
            <p className={text.label}>Entry diff</p>
            <h3 className={text.sectionTitle}>{reprompt.changeSummary || 'Checkpoint changes'}</h3>
            <p className="text-sm text-[var(--paper-muted)]">“{reprompt.prompt}”</p>
          </div>
          <Button type="button" variant="outline" onClick={onClose}>
            Close
          </Button>
        </div>
        <div className="grid gap-4 px-6 py-5">
          {errorMessage ? <p className={text.error}>{errorMessage}</p> : null}
          {isLoading ? (
            <div className="rounded-[14px] border border-border bg-[var(--surface-2)] p-5 text-sm text-[var(--paper-muted)]">
              Loading the before and after revisions...
            </div>
          ) : previousEntry && resultEntry ? (
            <div className="grid gap-4 lg:grid-cols-2">
              <div className="grid gap-3 rounded-[16px] border border-border bg-[var(--surface-2)] p-4">
                <p className={text.label}>Before</p>
                {rows.map((row) => (
                  <DiffRow key={`before-${row.key}`} label={row.label} value={row.before} changed={row.changed} />
                ))}
              </div>
              <div className="grid gap-3 rounded-[16px] border border-border bg-[var(--surface-2)] p-4">
                <p className={text.label}>After</p>
                {rows.map((row) => (
                  <DiffRow key={`after-${row.key}`} label={row.label} value={row.after} changed={row.changed} />
                ))}
              </div>
            </div>
          ) : (
            <div className="rounded-[14px] border border-border bg-[var(--surface-2)] p-5 text-sm text-[var(--paper-muted)]">
              This checkpoint does not have a complete entry diff available.
            </div>
          )}
        </div>
      </div>
    </div>
  )
}

function DiffRow({
  label,
  value,
  changed,
}: {
  label: string
  value: string
  changed: boolean
}) {
  return (
    <div
      className={cn(
        'grid gap-1 rounded-[12px] border px-3 py-2',
        changed
          ? 'border-[color-mix(in_oklch,var(--thread-gold)_72%,var(--border))] bg-[color-mix(in_oklch,var(--thread-gold)_13%,var(--surface-1))]'
          : 'border-border bg-[var(--surface-1)]',
      )}
    >
      <p className="text-[11px] font-bold uppercase tracking-[0.08em] text-[var(--paper-muted)]">
        {label}
      </p>
      <p className="whitespace-pre-wrap break-words text-sm text-[var(--paper)]">{value || 'Not set'}</p>
    </div>
  )
}

function buildEntryDiffRows(
  schema: FieldDefinition[],
  previousEntry: CollectionEntry | null,
  resultEntry: CollectionEntry | null,
) {
  const rows = [
    {
      key: 'slug',
      label: 'Slug',
      before: previousEntry?.slug ?? '',
      after: resultEntry?.slug ?? '',
    },
    {
      key: 'status',
      label: 'Status',
      before: previousEntry?.status ?? 'draft',
      after: resultEntry?.status ?? 'draft',
    },
    {
      key: 'seo.title',
      label: 'SEO title',
      before: previousEntry?.seo?.title ?? '',
      after: resultEntry?.seo?.title ?? '',
    },
    {
      key: 'seo.description',
      label: 'SEO description',
      before: previousEntry?.seo?.description ?? '',
      after: resultEntry?.seo?.description ?? '',
    },
    ...schema.map((field) => ({
      key: field.key,
      label: field.label,
      before: formatDiffValue(previousEntry?.fields?.[field.key]),
      after: formatDiffValue(resultEntry?.fields?.[field.key]),
    })),
  ]

  return rows.map((row) => ({
    ...row,
    changed: row.before !== row.after,
  }))
}

function findRevisionEntry(
  revision: DraftRevisionRecord,
  collectionId: string,
  entryId?: string,
) {
  if (!entryId) return null
  const targetCollection = revision.draft.collections?.find(
    (collection) => collection.id === collectionId,
  )
  return targetCollection?.entries?.find((entry) => entry.id === entryId) ?? null
}

function resolveEntryTitle(entry: CollectionEntry | null, schema: FieldDefinition[]) {
  if (!entry) return 'Untitled entry'
  for (const key of ['title', 'name', 'headline']) {
    const value = entry.fields?.[key]
    if (typeof value === 'string' && value.trim()) {
      return value.trim()
    }
  }
  for (const field of schema) {
    if (field.type !== 'text') continue
    const value = entry.fields?.[field.key]
    if (typeof value === 'string' && value.trim()) {
      return value.trim()
    }
  }
  return entry.slug
}

function formatDiffValue(value: unknown): string {
  if (value === null || value === undefined) return ''
  if (typeof value === 'string') return value
  if (typeof value === 'number' || typeof value === 'boolean') {
    return String(value)
  }
  if (Array.isArray(value)) {
    return value.map((item) => formatDiffValue(item)).filter(Boolean).join('\n')
  }
  try {
    return JSON.stringify(value, null, 2)
  } catch {
    return String(value)
  }
}

function formatRepromptTime(value: string) {
  const timestamp = new Date(value)
  if (Number.isNaN(timestamp.getTime())) {
    return 'Recently'
  }
  return new Intl.DateTimeFormat(undefined, {
    month: 'short',
    day: 'numeric',
    hour: 'numeric',
    minute: '2-digit',
  }).format(timestamp)
}

function isDraftConflictError(error: unknown) {
  if (!(error instanceof APIError)) {
    return false
  }
  const code =
    typeof error.payload?.error === 'object'
      ? error.payload.error.code
      : error.payload?.code
  return code === 'draft_conflict'
}

function mapIssueErrors(payload: APIErrorPayload | null) {
  const errors: EntryValidationErrors = {}
  for (const issue of payload?.issues ?? []) {
    const match = issue.path.match(/fields\.([a-z0-9_]+)/i)
    if (match) {
      errors[`fields.${match[1]}`] = issue.message
      continue
    }
    if (issue.path.includes('.slug')) {
      errors.slug = issue.message
      continue
    }
    if (issue.path.includes('.seo.title')) {
      errors['seo.title'] = issue.message
      continue
    }
    if (issue.path.includes('.seo.description')) {
      errors['seo.description'] = issue.message
    }
  }
  return errors
}
