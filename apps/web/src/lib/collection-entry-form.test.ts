import { describe, expect, it } from 'vitest'
import type { CollectionEntry, FieldDefinition } from '@/lib/api'
import {
  buildCreateEntryPayload,
  buildUpdateEntryPayload,
  createEntryEditorValues,
  validateEntryEditorValues,
} from './collection-entry-form'

describe('collection-entry-form', () => {
  const schema: FieldDefinition[] = [
    { key: 'title', label: 'Title', type: 'text', required: true },
    { key: 'price', label: 'Price', type: 'number' },
    { key: 'categories', label: 'Categories', type: 'enum_multi', options: ['design', 'hosting', 'seo'] },
    { key: 'cover', label: 'Cover', type: 'asset' },
    { key: 'location', label: 'Location', type: 'location' },
  ]

  it('seeds editor values from an existing entry', () => {
    const entry: CollectionEntry = {
      id: 'entry-1',
      slug: 'starter-site',
      status: 'published',
      sortOrder: 0,
      seo: {
        title: 'Starter site',
        description: 'A polished first draft.',
      },
      fields: {
        title: 'Starter site',
        price: 1200,
      },
    }

    expect(createEntryEditorValues(schema, entry)).toEqual({
      slug: 'starter-site',
      status: 'published',
      seo: {
        title: 'Starter site',
        description: 'A polished first draft.',
      },
      fields: {
        title: 'Starter site',
        price: 1200,
        categories: undefined,
        cover: undefined,
        location: undefined,
      },
    })
  })

  it('validates required and typed values', () => {
    const values = {
      slug: 'Bad Slug',
      status: 'draft' as const,
      seo: {
        title: 'x'.repeat(71),
        description: '',
      },
      fields: {
        title: '',
        price: 'oops',
        categories: ['design', 'design'],
        cover: {},
        location: { region: 'Stockholm' },
      },
    }

    expect(validateEntryEditorValues(schema, values)).toMatchObject({
      slug: 'Use lowercase words separated by hyphens.',
      'seo.title': 'SEO title must stay under 70 characters.',
      'fields.title': 'Title is required.',
      'fields.location': 'Location needs at least a name.',
    })
  })

  it('builds a creation payload with normalized fields', () => {
    const payload = buildCreateEntryPayload(schema, {
      slug: ' starter-site ',
      status: 'draft',
      seo: {
        title: ' Starter site ',
        description: ' Ready to publish ',
      },
      fields: {
        title: ' Starter site ',
        price: '1200',
        categories: ['design', 'hosting', 'hosting'],
        cover: { assetId: 'asset-1', alt: ' Warm thread ' },
        location: { name: 'Stockholm ', country: ' Sweden ' },
      },
    })

    expect(payload).toEqual({
      slug: 'starter-site',
      status: 'draft',
      seo: {
        title: 'Starter site',
        description: 'Ready to publish',
      },
      fields: {
        title: 'Starter site',
        price: 1200,
        categories: ['design', 'hosting'],
        cover: { assetId: 'asset-1', alt: 'Warm thread' },
        location: { name: 'Stockholm', country: 'Sweden' },
      },
    })
  })

  it('builds update payloads with nulls for cleared values', () => {
    const current: CollectionEntry = {
      id: 'entry-1',
      slug: 'starter-site',
      status: 'published',
      sortOrder: 0,
      seo: {
        title: 'Starter site',
        description: 'Ready to publish',
      },
      fields: {
        title: 'Starter site',
        price: 1200,
        categories: ['design'],
      },
    }

    const payload = buildUpdateEntryPayload(schema, current, {
      slug: 'starter-site-pro',
      status: 'draft',
      seo: {
        title: 'Starter site, refreshed',
        description: '',
      },
      fields: {
        title: 'Starter site, refreshed',
        price: undefined,
        categories: ['hosting'],
      },
    })

    expect(payload).toEqual({
      slug: 'starter-site-pro',
      status: 'draft',
      seo: {
        title: 'Starter site, refreshed',
        description: '',
      },
      fields: {
        title: 'Starter site, refreshed',
        price: null,
        categories: ['hosting'],
      },
    })
  })
})
