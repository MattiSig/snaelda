import type { FormEvent } from 'react'
import { useEffect, useState, useRef } from 'react'
import { Sparkles, ImagePlus } from 'lucide-react'
import { Button } from '@/components/ui/button'
import { Checkbox } from '@/components/ui/checkbox'
import { Input } from '@/components/ui/input'
import { Select } from '@/components/ui/select'
import { Textarea } from '@/components/ui/textarea'
import { buildDraftAssetURL } from '@/lib/assets'
import type {
  AssetRecord,
  BlockDefinition,
  BlockEditorField,
  BlockSuggestAction,
  BlockSuggestInput,
  BlockSuggestTone,
  ImageApplyResponse,
  ImageSuggestCandidate,
  SiteDraft,
} from '@/lib/api'
import { applyBlockImage, suggestBlockImage } from '@/lib/api'
import { actions, emptyState, form, panel, text } from '@/lib/styles'
import { cn } from '@/lib/utils'

type DraftBlock = SiteDraft['pages'][number]['blocks'][number]

type BlockEditorProps = {
  block: DraftBlock
  definition?: BlockDefinition
  isSaving: boolean
  errorMessage: string
  statusMessage: string
  assetLibrary: AssetRecord[]
  onSave: (props: Record<string, unknown>, hidden: boolean) => Promise<void>
  onSuggest?: (input: BlockSuggestInput) => Promise<void>
  isSuggesting?: boolean
  suggestErrorMessage?: string
  suggestStatusMessage?: string
  siteId?: string
  onImageApplied?: (response: ImageApplyResponse) => void
}

export function BlockEditor({
  block,
  definition,
  isSaving,
  errorMessage,
  statusMessage,
  assetLibrary,
  onSave,
  onSuggest,
  isSuggesting = false,
  suggestErrorMessage = '',
  suggestStatusMessage = '',
  siteId,
  onImageApplied,
}: BlockEditorProps) {
  const imageSuggestContext: ImageSuggestContext | null =
    siteId && block.id
      ? {
          siteId,
          blockId: block.id,
          onApplied: onImageApplied,
        }
      : null

  const [props, setProps] = useState<Record<string, unknown>>(() =>
    cloneProps(block.props),
  )
  const [hidden, setHidden] = useState(block.settings?.hidden ?? false)

  // Reset local prop state when the parent hands us a fresh block reference
  // (e.g. after an AI suggest replaces the block's props in-place). Using
  // render-time state derivation is React 19's recommended pattern for
  // "reset state when a prop changes" — it avoids the cascading-renders cost
  // of doing the same work inside useEffect.
  const [trackedProps, setTrackedProps] = useState(block.props)
  if (trackedProps !== block.props) {
    setTrackedProps(block.props)
    setProps(cloneProps(block.props))
    setHidden(block.settings?.hidden ?? false)
  }

  async function handleSubmit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault()
    await onSave(cleanObject(props), hidden)
  }

  const fields = definition?.editorSchema ?? []
  const textBearing = hasTextBearingFields(fields)

  return (
    <form
      className={cn(panel, 'grid gap-4 p-6 max-sm:p-4')}
      onSubmit={handleSubmit}
    >
      <div className="mb-3 flex items-start justify-between gap-3 max-sm:flex-col">
        <div>
          <p className={text.eyebrow}>Block editor</p>
          <h2 className={text.h2}>{definition?.displayName ?? block.type}</h2>
        </div>
        <div className="flex flex-wrap gap-2">
          <span className="rounded-full border border-border bg-[var(--surface-2)] px-3 py-2 text-xs font-bold uppercase tracking-[0.08em] text-[var(--paper-muted)]">
            {block.version}
          </span>
          {hidden ? (
            <span className="rounded-full border border-border bg-[var(--surface-2)] px-3 py-2 text-xs font-bold uppercase tracking-[0.08em] text-[var(--paper-muted)]">
              Hidden
            </span>
          ) : null}
        </div>
      </div>

      {onSuggest && textBearing ? (
        <AISuggestPanel
          onSuggest={onSuggest}
          isSuggesting={isSuggesting}
          errorMessage={suggestErrorMessage}
          statusMessage={suggestStatusMessage}
        />
      ) : null}

      {fields.length === 0 ? (
        <div className={emptyState}>
          <p className={text.p}>
            This block does not expose editable fields yet.
          </p>
        </div>
      ) : (
        <div className="grid gap-4">
          {fields.map((field) => (
            <FieldRenderer
              key={field.name}
              field={field}
              value={props[field.name]}
              assetLibrary={assetLibrary}
              imageSuggest={imageSuggestContext}
              path={[field.name]}
              onChange={(value) =>
                setProps((current) =>
                  updateFieldValue(current, field.name, value),
                )
              }
            />
          ))}
        </div>
      )}

      <label className={form.toggle}>
        <Checkbox
          checked={hidden}
          onChange={(event) => setHidden(event.target.checked)}
        />
        Hide this block in preview and publish output
      </label>

      {errorMessage ? <p className={text.error}>{errorMessage}</p> : null}
      {statusMessage ? <p className={text.success}>{statusMessage}</p> : null}

      <Button type="submit" disabled={isSaving}>
        {isSaving ? 'Saving block...' : 'Save block'}
      </Button>
    </form>
  )
}

type ImageSuggestContext = {
  siteId: string
  blockId: string
  onApplied?: (response: ImageApplyResponse) => void
}

function FieldRenderer({
  field,
  value,
  assetLibrary,
  imageSuggest,
  path,
  onChange,
}: {
  field: BlockEditorField
  value: unknown
  assetLibrary: AssetRecord[]
  imageSuggest: ImageSuggestContext | null
  path: string[]
  onChange: (value: unknown) => void
}) {
  switch (field.control) {
    case 'textarea':
      return (
        <label className={form.field}>
          <span className={cn(text.label, 'tracking-[0.08em]')}>
            {field.label}
          </span>
          {field.description ? (
            <small className="text-sm text-[var(--paper-muted)]">
              {field.description}
            </small>
          ) : null}
          <Textarea
            rows={5}
            value={asText(value)}
            placeholder={field.placeholder}
            onChange={(event) => onChange(event.target.value)}
          />
        </label>
      )
    case 'checkbox':
      return (
        <label className={form.toggle}>
          <Checkbox
            checked={Boolean(value)}
            onChange={(event) => onChange(event.target.checked)}
          />
          <span className={cn(text.label, 'tracking-[0.08em]')}>
            {field.label}
          </span>
        </label>
      )
    case 'select':
      return (
        <label className={form.field}>
          <span className={cn(text.label, 'tracking-[0.08em]')}>
            {field.label}
          </span>
          {field.description ? (
            <small className="text-sm text-[var(--paper-muted)]">
              {field.description}
            </small>
          ) : null}
          <Select
            value={String(value ?? '')}
            onChange={(event) =>
              onChange(coerceValue(field, event.target.value))
            }
          >
            <option value="">Select an option</option>
            {(field.options ?? []).map((option) => (
              <option key={option} value={option}>
                {option}
              </option>
            ))}
          </Select>
        </label>
      )
    case 'link':
      return (
        <ObjectField
          field={field}
          value={value}
          assetLibrary={assetLibrary}
          imageSuggest={imageSuggest}
          path={path}
          onChange={onChange}
          emptyLabel={`Add ${field.label.toLowerCase()}`}
        />
      )
    case 'object':
      return (
        <ObjectField
          field={field}
          value={value}
          assetLibrary={assetLibrary}
          imageSuggest={imageSuggest}
          path={path}
          onChange={onChange}
          emptyLabel={`Add ${field.label.toLowerCase()}`}
        />
      )
    case 'asset':
      return (
        <AssetField
          field={field}
          value={value}
          assetLibrary={assetLibrary}
          imageSuggest={imageSuggest}
          path={path}
          onChange={onChange}
        />
      )
    case 'repeater':
      return (
        <RepeaterField
          field={field}
          value={value}
          assetLibrary={assetLibrary}
          imageSuggest={imageSuggest}
          path={path}
          onChange={onChange}
        />
      )
    case 'string_list':
      return (
        <label className={form.field}>
          <span className={cn(text.label, 'tracking-[0.08em]')}>
            {field.label}
          </span>
          {field.description ? (
            <small className="text-sm text-[var(--paper-muted)]">
              {field.description}
            </small>
          ) : null}
          <Textarea
            rows={4}
            value={asStringList(value).join('\n')}
            placeholder={field.placeholder}
            onChange={(event) => onChange(parseStringList(event.target.value))}
          />
        </label>
      )
    case 'text':
    default:
      return (
        <label className={form.field}>
          <span className={cn(text.label, 'tracking-[0.08em]')}>
            {field.label}
          </span>
          {field.description ? (
            <small className="text-sm text-[var(--paper-muted)]">
              {field.description}
            </small>
          ) : null}
          <Input
            type="text"
            value={asText(value)}
            placeholder={field.placeholder}
            onChange={(event) =>
              onChange(coerceValue(field, event.target.value))
            }
          />
        </label>
      )
  }
}

function ObjectField({
  field,
  value,
  assetLibrary,
  imageSuggest,
  path,
  onChange,
  emptyLabel,
}: {
  field: BlockEditorField
  value: unknown
  assetLibrary: AssetRecord[]
  imageSuggest: ImageSuggestContext | null
  path: string[]
  onChange: (value: unknown) => void
  emptyLabel: string
}) {
  const objectValue = asObject(value)
  const nestedFields = field.fields ?? [
    { name: 'label', label: 'Label', control: 'text' },
    { name: 'href', label: 'Link', control: 'text' },
  ]

  return (
    <div
      className={cn(
        form.field,
        'rounded-[16px] border border-border bg-[var(--surface-2)] p-4',
      )}
    >
      <div className="flex items-start justify-between gap-3 max-sm:flex-col">
        <div>
          <span className={cn(text.label, 'tracking-[0.08em]')}>
            {field.label}
          </span>
          {field.description ? (
            <small className="mt-1 block text-sm font-normal normal-case tracking-normal text-[var(--paper-muted)]">
              {field.description}
            </small>
          ) : null}
        </div>
        {objectValue ? (
          <Button
            type="button"
            variant="plain"
            className={actions.inlineLink}
            onClick={() => onChange(undefined)}
          >
            Remove
          </Button>
        ) : (
          <Button
            type="button"
            variant="plain"
            className={actions.inlineLink}
            onClick={() => onChange(buildObjectDefaults(nestedFields))}
          >
            {emptyLabel}
          </Button>
        )}
      </div>

      {objectValue ? (
        <div className="grid gap-4">
          {nestedFields.map((nestedField) => (
            <FieldRenderer
              key={nestedField.name}
              field={nestedField}
              value={objectValue[nestedField.name]}
              assetLibrary={assetLibrary}
              imageSuggest={imageSuggest}
              path={[...path, nestedField.name]}
              onChange={(nextValue) =>
                onChange(
                  updateFieldValue(objectValue, nestedField.name, nextValue),
                )
              }
            />
          ))}
        </div>
      ) : null}
    </div>
  )
}

function RepeaterField({
  field,
  value,
  assetLibrary,
  imageSuggest,
  path,
  onChange,
}: {
  field: BlockEditorField
  value: unknown
  assetLibrary: AssetRecord[]
  imageSuggest: ImageSuggestContext | null
  path: string[]
  onChange: (value: unknown) => void
}) {
  const items = asObjectArray(value)
  const itemFields = field.itemFields ?? []

  return (
    <div
      className={cn(
        form.field,
        'rounded-[16px] border border-border bg-[var(--surface-2)] p-4',
      )}
    >
      <div className="flex items-start justify-between gap-3 max-sm:flex-col">
        <div>
          <span className={cn(text.label, 'tracking-[0.08em]')}>
            {field.label}
          </span>
          {field.description ? (
            <small className="mt-1 block text-sm font-normal normal-case tracking-normal text-[var(--paper-muted)]">
              {field.description}
            </small>
          ) : null}
        </div>
        <Button
          type="button"
          variant="plain"
          className={actions.inlineLink}
          onClick={() => onChange([...items, buildObjectDefaults(itemFields)])}
        >
          Add item
        </Button>
      </div>

      <div className="grid gap-4">
        {items.map((item, index) => (
          <article
            key={index}
            className="rounded-[14px] border border-border bg-[var(--surface-1)] p-4"
          >
            <div className="mb-4 flex items-start justify-between gap-3 max-sm:flex-col">
              <strong className={cn(text.label, 'tracking-[0.08em]')}>
                Item {index + 1}
              </strong>
              <Button
                type="button"
                variant="plain"
                className={actions.inlineLink}
                onClick={() =>
                  onChange(
                    items.filter(
                      (_, candidateIndex) => candidateIndex !== index,
                    ),
                  )
                }
              >
                Remove
              </Button>
            </div>
            <div className="grid gap-4">
              {itemFields.map((nestedField) => (
                <FieldRenderer
                  key={nestedField.name}
                  field={nestedField}
                  value={item[nestedField.name]}
                  assetLibrary={assetLibrary}
                  imageSuggest={imageSuggest}
                  path={[...path, String(index), nestedField.name]}
                  onChange={(nextValue) =>
                    onChange(
                      items.map((candidate, candidateIndex) =>
                        candidateIndex === index
                          ? updateFieldValue(
                              candidate,
                              nestedField.name,
                              nextValue,
                            )
                          : candidate,
                      ),
                    )
                  }
                />
              ))}
            </div>
          </article>
        ))}
      </div>
    </div>
  )
}

function AssetField({
  field,
  value,
  assetLibrary,
  imageSuggest,
  path,
  onChange,
}: {
  field: BlockEditorField
  value: unknown
  assetLibrary: AssetRecord[]
  imageSuggest: ImageSuggestContext | null
  path: string[]
  onChange: (value: unknown) => void
}) {
  const objectValue = asObject(value)
  const selectedAssetId = asText(objectValue?.assetId)
  const selectedAsset =
    assetLibrary.find((asset) => asset.id === selectedAssetId) ?? null
  const altText = asText(objectValue?.alt)

  return (
    <div
      className={cn(
        form.field,
        'rounded-[16px] border border-border bg-[var(--surface-2)] p-4',
      )}
    >
      <div className="flex items-start justify-between gap-3 max-sm:flex-col">
        <div>
          <span className={cn(text.label, 'tracking-[0.08em]')}>
            {field.label}
          </span>
          {field.description ? (
            <small className="mt-1 block text-sm font-normal normal-case tracking-normal text-[var(--paper-muted)]">
              {field.description}
            </small>
          ) : null}
        </div>
        <div className="flex items-center gap-3 max-sm:w-full max-sm:justify-between">
          {imageSuggest ? (
            <FindBetterImageButton
              context={imageSuggest}
              path={path}
              currentAlt={altText}
              onApplied={(response) => {
                if (response.image) {
                  onChange(
                    cleanObject({
                      assetId: response.image.assetId,
                      alt: response.image.alt,
                    }),
                  )
                }
                imageSuggest.onApplied?.(response)
              }}
            />
          ) : null}
          {objectValue ? (
            <Button
              type="button"
              variant="plain"
              className={actions.inlineLink}
              onClick={() => onChange(undefined)}
            >
              Clear image
            </Button>
          ) : null}
        </div>
      </div>

      <label className={form.field}>
        <span className={cn(text.label, 'tracking-[0.08em]')}>
          Uploaded image
        </span>
        <Select
          value={selectedAssetId}
          onChange={(event) => {
            const nextAssetId = event.target.value
            if (!nextAssetId) {
              onChange(undefined)
              return
            }

            const nextAsset =
              assetLibrary.find((asset) => asset.id === nextAssetId) ?? null

            onChange(
              cleanObject({
                assetId: nextAssetId,
                alt: altText || nextAsset?.altText || '',
              }),
            )
          }}
        >
          <option value="">Select a site asset</option>
          {assetLibrary.map((asset) => (
            <option key={asset.id} value={asset.id}>
              {[
                asset.metadata.fileName || asset.id,
                asset.metadata.provenance
                  ? `(${asset.metadata.provenance.provider} starter)`
                  : null,
              ]
                .filter(Boolean)
                .join(' ')}
            </option>
          ))}
        </Select>
      </label>

      {selectedAsset ? (
        <div className="grid gap-3 rounded-[14px] border border-border bg-[var(--surface-1)] p-3">
          <img
            src={buildDraftAssetURL(selectedAsset.id)}
            alt={
              altText ||
              selectedAsset.altText ||
              selectedAsset.metadata.fileName ||
              'Selected image'
            }
            className="max-h-[220px] w-full rounded-[12px] border border-border bg-[var(--surface-2)] object-cover"
            loading="lazy"
          />
          <div className="grid gap-1">
            <strong className="text-sm text-[var(--paper)]">
              {selectedAsset.metadata.fileName || selectedAsset.id}
            </strong>
            <small className="text-[var(--paper-muted)]">
              {selectedAsset.metadata.contentType || 'Image'}
            </small>
            {selectedAsset.metadata.provenance ? (
              <small className="text-[var(--paper-muted)]">
                Starter from{' '}
                <span className="font-medium text-[var(--paper)] capitalize">
                  {selectedAsset.metadata.provenance.provider}
                </span>
                {selectedAsset.metadata.provenance.author
                  ? ` · Photo by ${selectedAsset.metadata.provenance.author}`
                  : null}
              </small>
            ) : null}
          </div>
        </div>
      ) : (
        <div className={emptyState}>
          <p className={text.p}>
            {assetLibrary.length > 0
              ? 'Upload assets in the site library, then choose one here.'
              : 'Upload the first image in the site asset library to use it here.'}
          </p>
        </div>
      )}

      <label className={form.field}>
        <span className={cn(text.label, 'tracking-[0.08em]')}>Alt text</span>
        <Input
          type="text"
          value={altText}
          placeholder="Describe the image for screen readers"
          disabled={!objectValue}
          onChange={(event) =>
            onChange(
              cleanObject({
                assetId: selectedAssetId,
                alt: event.target.value,
              }),
            )
          }
        />
      </label>
    </div>
  )
}

const TONE_OPTIONS: { value: BlockSuggestTone; label: string }[] = [
  { value: 'friendlier', label: 'Friendlier' },
  { value: 'professional', label: 'More professional' },
  { value: 'playful', label: 'More playful' },
  { value: 'direct', label: 'More direct' },
]

function AISuggestPanel({
  onSuggest,
  isSuggesting,
  errorMessage,
  statusMessage,
}: {
  onSuggest: (input: BlockSuggestInput) => Promise<void>
  isSuggesting: boolean
  errorMessage: string
  statusMessage: string
}) {
  const [open, setOpen] = useState(false)
  const [mode, setMode] =
    useState<'menu' | 'tone' | 'rewrite'>('menu')
  const [instruction, setInstruction] = useState('')
  const rootRef = useRef<HTMLDivElement>(null)

  useEffect(() => {
    if (!open) return
    function handleDocumentClick(event: MouseEvent) {
      if (!rootRef.current) return
      if (!rootRef.current.contains(event.target as Node)) {
        setOpen(false)
        setMode('menu')
      }
    }
    function handleKey(event: KeyboardEvent) {
      if (event.key === 'Escape') {
        setOpen(false)
        setMode('menu')
      }
    }
    document.addEventListener('mousedown', handleDocumentClick)
    document.addEventListener('keydown', handleKey)
    return () => {
      document.removeEventListener('mousedown', handleDocumentClick)
      document.removeEventListener('keydown', handleKey)
    }
  }, [open])

  async function runAction(action: BlockSuggestAction, tone?: BlockSuggestTone) {
    await onSuggest({ action, tone })
    setOpen(false)
    setMode('menu')
  }

  async function runRewrite(event: FormEvent<HTMLFormElement>) {
    event.preventDefault()
    const trimmed = instruction.trim()
    if (!trimmed) return
    await onSuggest({ action: 'rewrite', instruction: trimmed })
    setInstruction('')
    setOpen(false)
    setMode('menu')
  }

  return (
    <div
      ref={rootRef}
      className="relative -mb-1 flex flex-wrap items-center gap-2"
    >
      <Button
        type="button"
        variant="outline"
        size="sm"
        disabled={isSuggesting}
        onClick={() => {
          setOpen((current) => !current)
          setMode('menu')
        }}
        aria-expanded={open}
        aria-haspopup="menu"
      >
        <Sparkles className="size-4" aria-hidden />
        {isSuggesting ? 'Improving…' : 'Improve with AI'}
      </Button>
      {statusMessage ? (
        <span className="text-xs text-[var(--paper-muted)]">{statusMessage}</span>
      ) : null}
      {errorMessage ? (
        <span className="text-xs text-[var(--destructive)]">{errorMessage}</span>
      ) : null}
      {open ? (
        <div
          role="menu"
          className="absolute left-0 top-[calc(100%+8px)] z-30 w-[260px] rounded-[12px] border border-border bg-[var(--surface-1)] p-2 shadow-[0_18px_36px_oklch(8%_0.02_336_/_0.32)]"
        >
          {mode === 'menu' ? (
            <div className="grid gap-1">
              <SuggestRow
                label="Tighten"
                description="Shorter, sharper version"
                disabled={isSuggesting}
                onClick={() => runAction('tighten')}
              />
              <SuggestRow
                label="Expand"
                description="Add useful detail without padding"
                disabled={isSuggesting}
                onClick={() => runAction('expand')}
              />
              <SuggestRow
                label="Change tone…"
                description="Friendlier, more professional, etc."
                disabled={isSuggesting}
                onClick={() => setMode('tone')}
              />
              <SuggestRow
                label="Rewrite from prompt…"
                description="Describe the change you want"
                disabled={isSuggesting}
                onClick={() => setMode('rewrite')}
              />
            </div>
          ) : null}
          {mode === 'tone' ? (
            <div className="grid gap-1">
              <button
                type="button"
                className="px-2 pt-1 pb-2 text-left text-xs uppercase tracking-[0.08em] text-[var(--paper-muted)] hover:text-[var(--paper)]"
                onClick={() => setMode('menu')}
              >
                ← Back
              </button>
              {TONE_OPTIONS.map((tone) => (
                <SuggestRow
                  key={tone.value}
                  label={tone.label}
                  disabled={isSuggesting}
                  onClick={() => runAction('tone', tone.value)}
                />
              ))}
            </div>
          ) : null}
          {mode === 'rewrite' ? (
            <form className="grid gap-2 p-1" onSubmit={runRewrite}>
              <button
                type="button"
                className="text-left text-xs uppercase tracking-[0.08em] text-[var(--paper-muted)] hover:text-[var(--paper)]"
                onClick={() => setMode('menu')}
              >
                ← Back
              </button>
              <Textarea
                rows={3}
                placeholder="Make it more about families with kids"
                value={instruction}
                onChange={(event) => setInstruction(event.target.value)}
                disabled={isSuggesting}
                autoFocus
              />
              <Button
                type="submit"
                size="sm"
                disabled={isSuggesting || instruction.trim().length === 0}
              >
                {isSuggesting ? 'Rewriting…' : 'Rewrite block'}
              </Button>
            </form>
          ) : null}
        </div>
      ) : null}
    </div>
  )
}

function SuggestRow({
  label,
  description,
  onClick,
  disabled,
}: {
  label: string
  description?: string
  onClick: () => void
  disabled: boolean
}) {
  return (
    <button
      type="button"
      role="menuitem"
      disabled={disabled}
      onClick={onClick}
      className="grid w-full gap-0.5 rounded-[8px] px-2 py-2 text-left transition-colors hover:bg-[var(--surface-2)] disabled:cursor-not-allowed disabled:opacity-60"
    >
      <span className="text-sm font-semibold text-[var(--paper)]">{label}</span>
      {description ? (
        <span className="text-xs text-[var(--paper-muted)]">{description}</span>
      ) : null}
    </button>
  )
}

function FindBetterImageButton({
  context,
  path,
  currentAlt,
  onApplied,
}: {
  context: ImageSuggestContext
  path: string[]
  currentAlt: string
  onApplied: (response: ImageApplyResponse) => void
}) {
  const [open, setOpen] = useState(false)
  return (
    <>
      <Button
        type="button"
        variant="outline"
        size="sm"
        onClick={() => setOpen(true)}
      >
        <ImagePlus className="size-4" aria-hidden />
        Find a better image
      </Button>
      {open ? (
        <ImagePickerModal
          context={context}
          path={path}
          currentAlt={currentAlt}
          onClose={() => setOpen(false)}
          onApplied={(response) => {
            onApplied(response)
            setOpen(false)
          }}
        />
      ) : null}
    </>
  )
}

function ImagePickerModal({
  context,
  path,
  currentAlt,
  onClose,
  onApplied,
}: {
  context: ImageSuggestContext
  path: string[]
  currentAlt: string
  onClose: () => void
  onApplied: (response: ImageApplyResponse) => void
}) {
  const [instruction, setInstruction] = useState('')
  const [query, setQuery] = useState('')
  const [candidates, setCandidates] = useState<ImageSuggestCandidate[]>([])
  const [searching, setSearching] = useState(true)
  const [applyingId, setApplyingId] = useState<string | null>(null)
  const [errorMessage, setErrorMessage] = useState('')

  useEffect(() => {
    function handleKey(event: KeyboardEvent) {
      if (event.key === 'Escape') {
        onClose()
      }
    }
    document.addEventListener('keydown', handleKey)
    return () => document.removeEventListener('keydown', handleKey)
  }, [onClose])

  // The initial fetch runs once when the picker opens. We start with
  // searching=true so no synchronous setState happens inside the effect; the
  // result lands via the promise's then/catch microtask.
  useEffect(() => {
    let cancelled = false
    suggestBlockImage(context.siteId, context.blockId, { path, instruction: '' })
      .then((response) => {
        if (cancelled) return
        setQuery(response.query)
        setCandidates(response.candidates)
        if (response.candidates.length === 0) {
          setErrorMessage(
            'No images matched that query. Try a different instruction.',
          )
        }
      })
      .catch((error) => {
        if (cancelled) return
        setErrorMessage(
          error instanceof Error
            ? error.message
            : 'Could not load image suggestions.',
        )
      })
      .finally(() => {
        if (!cancelled) {
          setSearching(false)
        }
      })
    return () => {
      cancelled = true
    }
    // The initial fetch should only fire when the picker first mounts.
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [])

  function runSuggest() {
    setSearching(true)
    setErrorMessage('')
    suggestBlockImage(context.siteId, context.blockId, {
      path,
      instruction,
    })
      .then((response) => {
        setQuery(response.query)
        setCandidates(response.candidates)
        if (response.candidates.length === 0) {
          setErrorMessage(
            'No images matched that query. Try a different instruction.',
          )
        }
      })
      .catch((error) => {
        setErrorMessage(
          error instanceof Error
            ? error.message
            : 'Could not load image suggestions.',
        )
      })
      .finally(() => {
        setSearching(false)
      })
  }

  async function applyCandidate(candidate: ImageSuggestCandidate) {
    setApplyingId(candidate.providerId || candidate.downloadUrl)
    setErrorMessage('')
    try {
      const response = await applyBlockImage(context.siteId, context.blockId, {
        path,
        photo: candidate,
        alt: currentAlt || candidate.description || '',
        query,
        instruction,
      })
      onApplied(response)
    } catch (error) {
      setErrorMessage(
        error instanceof Error
          ? error.message
          : 'Could not apply the chosen image.',
      )
    } finally {
      setApplyingId(null)
    }
  }

  return (
    <div
      role="dialog"
      aria-modal="true"
      aria-label="Find a better image"
      className="fixed inset-0 z-50 flex items-center justify-center bg-[oklch(8%_0.02_336_/_0.65)] p-4"
      onClick={(event) => {
        if (event.target === event.currentTarget) {
          onClose()
        }
      }}
    >
      <div className="grid max-h-[88vh] w-full max-w-[760px] gap-4 overflow-y-auto rounded-[20px] border border-border bg-[var(--surface-1)] p-6 shadow-[0_36px_72px_oklch(8%_0.02_336_/_0.42)]">
        <header className="flex items-start justify-between gap-4">
          <div>
            <p className={text.eyebrow}>AI image picker</p>
            <h3 className={text.h2}>Find a better image</h3>
            {query ? (
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
                event.preventDefault()
                runSuggest()
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

        {errorMessage ? (
          <p className={text.error}>{errorMessage}</p>
        ) : null}

        {searching && candidates.length === 0 ? (
          <div className={emptyState}>
            <p className={text.p}>Looking for fresh images…</p>
          </div>
        ) : null}

        {candidates.length > 0 ? (
          <div className="grid grid-cols-2 gap-3 sm:grid-cols-3">
            {candidates.map((candidate) => {
              const id =
                candidate.providerId || candidate.downloadUrl
              const isApplying = applyingId === id
              return (
                <button
                  type="button"
                  key={id}
                  onClick={() => void applyCandidate(candidate)}
                  disabled={isApplying || applyingId !== null}
                  className="group relative grid gap-2 rounded-[14px] border border-border bg-[var(--surface-2)] p-2 text-left transition-colors hover:border-[var(--paper)] disabled:cursor-not-allowed disabled:opacity-60"
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
              )
            })}
          </div>
        ) : null}

        <p className="text-xs text-[var(--paper-muted)]">
          Click an image to import it as a site asset and replace this slot.
          We credit photographers automatically via Pexels.
        </p>
      </div>
    </div>
  )
}

function hasTextBearingFields(fields: BlockEditorField[]): boolean {
  return fields.some((field) => {
    if (field.control === 'text' || field.control === 'textarea') {
      return true
    }
    if (field.control === 'repeater') {
      return hasTextBearingFields(field.itemFields ?? [])
    }
    if (field.control === 'object' || field.control === 'link') {
      return hasTextBearingFields(field.fields ?? [])
    }
    return false
  })
}

function coerceValue(field: BlockEditorField, value: string) {
  if (field.valueType === 'integer') {
    const parsed = Number.parseInt(value, 10)
    return Number.isNaN(parsed) ? value : parsed
  }
  return value
}

function buildObjectDefaults(fields: BlockEditorField[]) {
  return fields.reduce<Record<string, unknown>>((result, field) => {
    if (field.control === 'repeater') {
      result[field.name] = []
      return result
    }
    if (field.control === 'string_list') {
      result[field.name] = []
      return result
    }
    if (field.control === 'checkbox') {
      result[field.name] = false
      return result
    }
    if (
      field.control === 'link' ||
      field.control === 'asset' ||
      field.control === 'object'
    ) {
      result[field.name] = buildObjectDefaults(field.fields ?? [])
      return result
    }
    result[field.name] = field.valueType === 'integer' ? 0 : ''
    return result
  }, {})
}

function updateFieldValue(
  source: Record<string, unknown>,
  key: string,
  value: unknown,
) {
  const nextValue = cloneProps(source)
  if (value === undefined) {
    delete nextValue[key]
    return nextValue
  }
  nextValue[key] = value
  return nextValue
}

function cleanObject(value: Record<string, unknown>) {
  return Object.entries(value).reduce<Record<string, unknown>>(
    (result, [key, entry]) => {
      if (entry === undefined) {
        return result
      }
      result[key] = cleanValue(entry)
      return result
    },
    {},
  )
}

function cleanValue(value: unknown): unknown {
  if (Array.isArray(value)) {
    if (value.every((entry) => typeof entry === 'string')) {
      return value
        .map((entry) => String(entry).trim())
        .filter((entry) => entry.length > 0)
    }
    return value.map((entry) => cleanValue(entry))
  }
  if (value && typeof value === 'object') {
    return cleanObject(value as Record<string, unknown>)
  }
  return value
}

function cloneProps(value: Record<string, unknown>) {
  return JSON.parse(JSON.stringify(value ?? {})) as Record<string, unknown>
}

function asText(value: unknown) {
  return typeof value === 'string' ? value : ''
}

function asObject(value: unknown) {
  if (!value || typeof value !== 'object' || Array.isArray(value)) {
    return null
  }
  return value as Record<string, unknown>
}

function asObjectArray(value: unknown) {
  if (!Array.isArray(value)) {
    return []
  }
  return value
    .map((entry) => asObject(entry))
    .filter((entry): entry is Record<string, unknown> => entry !== null)
}

function asStringList(value: unknown) {
  if (!Array.isArray(value)) {
    return []
  }
  return value.filter((entry): entry is string => typeof entry === 'string')
}

function parseStringList(value: string) {
  return value
    .split('\n')
    .map((entry) => entry.trim())
    .filter((entry) => entry.length > 0)
}
