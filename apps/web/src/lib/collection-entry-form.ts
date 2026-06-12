import type {
  CollectionEntry,
  FieldDefinition,
} from '@/lib/api'

export type EntryEditorValues = {
  slug: string
  status: 'draft' | 'published'
  seo: {
    title: string
    description: string
  }
  fields: Record<string, unknown>
}

export type EntryValidationErrors = Record<string, string>

const slugPattern = /^[a-z][a-z0-9-]*$/
const isoDatePattern = /^\d{4}-\d{2}-\d{2}$/
const phonePattern = /^[+0-9 ()\-.]{3,32}$/

export function createEntryEditorValues(
  schema: FieldDefinition[],
  entry?: CollectionEntry | null,
): EntryEditorValues {
  const fields = Object.fromEntries(
    schema.map((field) => [
      field.key,
      cloneValue(entry?.fields?.[field.key] ?? field.defaultValue),
    ]),
  )

  return {
    slug: entry?.slug ?? '',
    status: entry?.status ?? 'draft',
    seo: {
      title: entry?.seo?.title ?? '',
      description: entry?.seo?.description ?? '',
    },
    fields,
  }
}

export function validateEntryEditorValues(
  schema: FieldDefinition[],
  values: EntryEditorValues,
): EntryValidationErrors {
  const errors: EntryValidationErrors = {}
  const slug = values.slug.trim()
  if (slug && !slugPattern.test(slug)) {
    errors.slug = 'Use lowercase words separated by hyphens.'
  }

  if (values.seo.title.trim().length > 70) {
    errors['seo.title'] = 'SEO title must stay under 70 characters.'
  }
  if (values.seo.description.trim().length > 180) {
    errors['seo.description'] = 'SEO description must stay under 180 characters.'
  }

  for (const field of schema) {
    const raw = values.fields[field.key]
    const normalized = normalizeFieldValue(field, raw)
    const path = `fields.${field.key}`

    if (field.required && normalized === undefined) {
      errors[path] = `${field.label} is required.`
      continue
    }
    if (normalized === undefined) {
      continue
    }

    switch (field.type) {
      case 'text':
      case 'long_text':
      case 'rich_text':
        if (typeof normalized !== 'string') {
          errors[path] = `${field.label} must be text.`
          continue
        }
        if (field.validation?.minLength && normalized.length < field.validation.minLength) {
          errors[path] = `${field.label} is too short.`
        }
        if (field.validation?.maxLength && normalized.length > field.validation.maxLength) {
          errors[path] = `${field.label} is too long.`
        }
        break
      case 'number':
        if (typeof normalized !== 'number' || Number.isNaN(normalized)) {
          errors[path] = `${field.label} must be a number.`
          continue
        }
        if (field.validation?.min !== undefined && normalized < field.validation.min) {
          errors[path] = `${field.label} is below the minimum.`
        }
        if (field.validation?.max !== undefined && normalized > field.validation.max) {
          errors[path] = `${field.label} is above the maximum.`
        }
        break
      case 'date':
        if (typeof normalized !== 'string' || !isoDatePattern.test(normalized)) {
          errors[path] = 'Use a YYYY-MM-DD date.'
        }
        break
      case 'url':
        if (typeof normalized !== 'string' || !isValidURL(normalized)) {
          errors[path] = 'Enter a valid URL.'
        }
        break
      case 'email':
        if (typeof normalized !== 'string' || !isValidEmail(normalized)) {
          errors[path] = 'Enter a valid email address.'
        }
        break
      case 'phone':
        if (typeof normalized !== 'string' || !phonePattern.test(normalized)) {
          errors[path] = 'Enter a valid phone number.'
        }
        break
      case 'enum':
        if (
          typeof normalized !== 'string' ||
          !field.options?.includes(normalized)
        ) {
          errors[path] = 'Choose one of the provided options.'
        }
        break
      case 'enum_multi':
        if (!Array.isArray(normalized)) {
          errors[path] = 'Choose one or more options.'
          continue
        }
        if (new Set(normalized).size !== normalized.length) {
          errors[path] = 'Each option can only be selected once.'
        }
        if (normalized.some((value) => !field.options?.includes(String(value)))) {
          errors[path] = 'Choose only the provided options.'
        }
        break
      case 'location':
        if (
          typeof normalized !== 'object' ||
          normalized === null ||
          typeof (normalized as Record<string, unknown>).name !== 'string' ||
          !(normalized as Record<string, string>).name.trim()
        ) {
          errors[path] = 'Location needs at least a name.'
        }
        break
      case 'asset':
        if (
          typeof normalized !== 'object' ||
          normalized === null ||
          !String((normalized as Record<string, unknown>).assetId ?? '').trim()
        ) {
          errors[path] = 'Choose an uploaded asset.'
        }
        break
      case 'asset_list':
        if (!Array.isArray(normalized) || normalized.length === 0) {
          errors[path] = 'Choose at least one uploaded asset.'
        }
        break
      case 'reference':
        if (
          typeof normalized !== 'object' ||
          normalized === null ||
          !String((normalized as Record<string, unknown>).collectionId ?? '').trim() ||
          !String((normalized as Record<string, unknown>).entryId ?? '').trim()
        ) {
          errors[path] = 'Choose both a collection and an entry.'
        }
        break
      default:
        break
    }
  }

  return errors
}

export function buildCreateEntryPayload(
  schema: FieldDefinition[],
  values: EntryEditorValues,
) {
  return {
    slug: values.slug.trim() || undefined,
    status: values.status,
    seo: normalizeSEO(values.seo),
    fields: normalizeFields(schema, values.fields),
  }
}

export function buildUpdateEntryPayload(
  schema: FieldDefinition[],
  current: CollectionEntry,
  values: EntryEditorValues,
) {
  const nextFields = normalizeFields(schema, values.fields)
  const currentFields = normalizeFields(schema, current.fields)
  const fieldKeys = new Set([
    ...Object.keys(currentFields),
    ...Object.keys(nextFields),
  ])
  const fieldsPatch: Record<string, unknown> = {}
  for (const key of fieldKeys) {
    const before = currentFields[key]
    const after = nextFields[key]
    if (after === undefined && before !== undefined) {
      fieldsPatch[key] = null
      continue
    }
    if (!isEqual(before, after)) {
      fieldsPatch[key] = cloneValue(after)
    }
  }

  const slug = values.slug.trim()
  const seo = normalizeSEO(values.seo)
  const currentSEO = normalizeSEO(current.seo)

  return {
    slug: slug !== current.slug ? slug || undefined : undefined,
    status: values.status !== (current.status ?? 'draft') ? values.status : undefined,
    seo: !isEqual(seo, currentSEO) ? seo : undefined,
    fields: Object.keys(fieldsPatch).length ? fieldsPatch : undefined,
  }
}

export function normalizeFields(
  schema: FieldDefinition[],
  fields: Record<string, unknown>,
) {
  return Object.fromEntries(
    schema
      .map((field) => [field.key, normalizeFieldValue(field, fields[field.key])] as const)
      .filter(([, value]) => value !== undefined),
  )
}

export function normalizeFieldValue(
  field: FieldDefinition,
  value: unknown,
): unknown {
  switch (field.type) {
    case 'text':
    case 'long_text':
    case 'rich_text':
    case 'url':
    case 'email':
    case 'phone':
    case 'date':
    case 'enum': {
      const text = typeof value === 'string' ? value.trim() : ''
      return text ? text : undefined
    }
    case 'number': {
      if (typeof value === 'number' && Number.isFinite(value)) {
        return value
      }
      if (typeof value === 'string' && value.trim()) {
        const next = Number(value)
        return Number.isFinite(next) ? next : undefined
      }
      return undefined
    }
    case 'boolean':
      if (value === undefined || value === null || value === '') {
        return undefined
      }
      return Boolean(value)
    case 'enum_multi': {
      if (!Array.isArray(value)) return undefined
      const options = value
        .map((item) => String(item).trim())
        .filter(Boolean)
      return options.length ? Array.from(new Set(options)) : undefined
    }
    case 'location': {
      const object = asObject(value)
      const name = String(object?.name ?? '').trim()
      const region = String(object?.region ?? '').trim()
      const country = String(object?.country ?? '').trim()
      const lat = String(object?.lat ?? '').trim()
      const lng = String(object?.lng ?? '').trim()
      if (!name && !region && !country && !lat && !lng) return undefined
      return {
        ...(name ? { name } : {}),
        ...(region ? { region } : {}),
        ...(country ? { country } : {}),
        ...(lat ? { lat } : {}),
        ...(lng ? { lng } : {}),
      }
    }
    case 'asset': {
      const object = asObject(value)
      const assetId = String(object?.assetId ?? '').trim()
      const alt = String(object?.alt ?? '').trim()
      if (!assetId) return undefined
      return {
        assetId,
        ...(alt ? { alt } : {}),
      }
    }
    case 'asset_list': {
      if (!Array.isArray(value)) return undefined
      const items = value
        .map((item) => normalizeFieldValue({ ...field, type: 'asset' }, item))
        .filter((item): item is Record<string, unknown> => item !== undefined)
      return items.length ? items : undefined
    }
    case 'reference': {
      const object = asObject(value)
      const collectionId = String(object?.collectionId ?? '').trim()
      const entryId = String(object?.entryId ?? '').trim()
      if (!collectionId || !entryId) return undefined
      return { collectionId, entryId }
    }
    default:
      return value
  }
}

function normalizeSEO(seo?: { title?: string; description?: string }) {
  return {
    title: seo?.title?.trim() ?? '',
    description: seo?.description?.trim() ?? '',
  }
}

function asObject(value: unknown) {
  if (!value || typeof value !== 'object' || Array.isArray(value)) {
    return null
  }
  return value as Record<string, unknown>
}

function isValidURL(value: string) {
  if (value.startsWith('/')) return !value.startsWith('//')
  if (value.startsWith('#')) return value.length > 1
  try {
    const url = new URL(value)
    return ['http:', 'https:', 'mailto:', 'tel:'].includes(url.protocol)
  } catch {
    return false
  }
}

function isValidEmail(value: string) {
  return /^[^\s@]+@[^\s@]+\.[^\s@]+$/.test(value)
}

function cloneValue<T>(value: T): T {
  if (value === undefined) {
    return value
  }
  try {
    return JSON.parse(JSON.stringify(value)) as T
  } catch {
    return value
  }
}

function isEqual(left: unknown, right: unknown) {
  return JSON.stringify(left) === JSON.stringify(right)
}
