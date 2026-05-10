import type { FormEvent } from 'react'
import { useState } from 'react'
import { Button } from '@/components/ui/button'
import { Checkbox } from '@/components/ui/checkbox'
import { Input } from '@/components/ui/input'
import { Select } from '@/components/ui/select'
import { Textarea } from '@/components/ui/textarea'
import type { BlockDefinition, BlockEditorField, SiteDraft } from '@/lib/api'
import { actions, emptyState, form, panel, text } from '@/lib/styles'
import { cn } from '@/lib/utils'

type DraftBlock = SiteDraft['pages'][number]['blocks'][number]

type BlockEditorProps = {
  block: DraftBlock
  definition?: BlockDefinition
  isSaving: boolean
  errorMessage: string
  statusMessage: string
  onSave: (props: Record<string, unknown>, hidden: boolean) => Promise<void>
}

export function BlockEditor({
  block,
  definition,
  isSaving,
  errorMessage,
  statusMessage,
  onSave,
}: BlockEditorProps) {
  const [props, setProps] = useState<Record<string, unknown>>(() =>
    cloneProps(block.props),
  )
  const [hidden, setHidden] = useState(block.settings?.hidden ?? false)

  async function handleSubmit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault()
    await onSave(cleanObject(props), hidden)
  }

  const fields = definition?.editorSchema ?? []

  return (
    <form className={cn(panel, 'grid gap-4 p-6 max-sm:p-4')} onSubmit={handleSubmit}>
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

      {fields.length === 0 ? (
        <div className={emptyState}>
          <p className={text.p}>This block does not expose editable fields yet.</p>
        </div>
      ) : (
        <div className="grid gap-4">
          {fields.map((field) => (
            <FieldRenderer
              key={field.name}
              field={field}
              value={props[field.name]}
              onChange={(value) =>
                setProps((current) => updateFieldValue(current, field.name, value))
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

function FieldRenderer({
  field,
  value,
  onChange,
}: {
  field: BlockEditorField
  value: unknown
  onChange: (value: unknown) => void
}) {
  switch (field.control) {
    case 'textarea':
      return (
        <label className={form.field}>
          <span className={cn(text.label, 'tracking-[0.08em]')}>{field.label}</span>
          {field.description ? <small className="text-sm text-[var(--paper-muted)]">{field.description}</small> : null}
          <Textarea
            rows={5}
            value={asText(value)}
            placeholder={field.placeholder}
            onChange={(event) => onChange(event.target.value)}
          />
        </label>
      )
    case 'select':
      return (
        <label className={form.field}>
          <span className={cn(text.label, 'tracking-[0.08em]')}>{field.label}</span>
          {field.description ? <small className="text-sm text-[var(--paper-muted)]">{field.description}</small> : null}
          <Select
            value={String(value ?? '')}
            onChange={(event) => onChange(coerceValue(field, event.target.value))}
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
          onChange={onChange}
          emptyLabel={`Add ${field.label.toLowerCase()}`}
        />
      )
    case 'asset':
      return (
        <ObjectField
          field={{
            ...field,
            fields: field.fields ?? [
              { name: 'assetId', label: 'Asset id', control: 'text' },
              { name: 'alt', label: 'Alt text', control: 'text' },
            ],
          }}
          value={value}
          onChange={onChange}
          emptyLabel="Add image reference"
        />
      )
    case 'repeater':
      return <RepeaterField field={field} value={value} onChange={onChange} />
    case 'text':
    default:
      return (
        <label className={form.field}>
          <span className={cn(text.label, 'tracking-[0.08em]')}>{field.label}</span>
          {field.description ? <small className="text-sm text-[var(--paper-muted)]">{field.description}</small> : null}
          <Input
            type="text"
            value={asText(value)}
            placeholder={field.placeholder}
            onChange={(event) => onChange(coerceValue(field, event.target.value))}
          />
        </label>
      )
  }
}

function ObjectField({
  field,
  value,
  onChange,
  emptyLabel,
}: {
  field: BlockEditorField
  value: unknown
  onChange: (value: unknown) => void
  emptyLabel: string
}) {
  const objectValue = asObject(value)
  const nestedFields =
    field.fields ??
    [
      { name: 'label', label: 'Label', control: 'text' },
      { name: 'href', label: 'Link', control: 'text' },
    ]

  return (
    <div className={cn(form.field, 'rounded-[16px] border border-border bg-[var(--surface-2)] p-4')}>
      <div className="flex items-start justify-between gap-3 max-sm:flex-col">
        <div>
          <span className={cn(text.label, 'tracking-[0.08em]')}>{field.label}</span>
          {field.description ? <small className="mt-1 block text-sm font-normal normal-case tracking-normal text-[var(--paper-muted)]">{field.description}</small> : null}
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
              onChange={(nextValue) =>
                onChange(updateFieldValue(objectValue, nestedField.name, nextValue))
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
  onChange,
}: {
  field: BlockEditorField
  value: unknown
  onChange: (value: unknown) => void
}) {
  const items = asObjectArray(value)
  const itemFields = field.itemFields ?? []

  return (
    <div className={cn(form.field, 'rounded-[16px] border border-border bg-[var(--surface-2)] p-4')}>
      <div className="flex items-start justify-between gap-3 max-sm:flex-col">
        <div>
          <span className={cn(text.label, 'tracking-[0.08em]')}>{field.label}</span>
          {field.description ? <small className="mt-1 block text-sm font-normal normal-case tracking-normal text-[var(--paper-muted)]">{field.description}</small> : null}
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
          <article key={index} className="rounded-[14px] border border-border bg-[var(--surface-1)] p-4">
            <div className="mb-4 flex items-start justify-between gap-3 max-sm:flex-col">
              <strong className={cn(text.label, 'tracking-[0.08em]')}>Item {index + 1}</strong>
              <Button
                type="button"
                variant="plain"
                className={actions.inlineLink}
                onClick={() =>
                  onChange(items.filter((_, candidateIndex) => candidateIndex !== index))
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
                  onChange={(nextValue) =>
                    onChange(
                      items.map((candidate, candidateIndex) =>
                        candidateIndex === index
                          ? updateFieldValue(candidate, nestedField.name, nextValue)
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
    if (field.control === 'link' || field.control === 'asset') {
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
  return Object.entries(value).reduce<Record<string, unknown>>((result, [key, entry]) => {
    if (entry === undefined) {
      return result
    }
    result[key] = cleanValue(entry)
    return result
  }, {})
}

function cleanValue(value: unknown): unknown {
  if (Array.isArray(value)) {
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
