import type {
  BlockEditorField,
  CollectionFieldType,
  FieldDefinition,
} from '@/lib/api';

// Mirrors internal/siteconfig/collections.go propKeyIsBindable. Block props
// that may be bound to a collection entry field in collection_detail
// templates. Keeping this list explicit avoids exposing every nested object
// field; bindings replace whole top-level prop values, not sub-paths.
const BINDABLE_PROP_KEYS = new Set<string>([
  'headline',
  'subheadline',
  'eyebrow',
  'heading',
  'body',
  'title',
  'summary',
  'description',
  'image',
  'cover',
  'gallery',
  'items',
  'href',
  'url',
  'phone',
  'email',
]);

const TEXT_FIELD_TYPES = new Set<CollectionFieldType>([
  'text',
  'long_text',
  'rich_text',
  'url',
  'email',
  'phone',
]);

export function isPropKeyBindable(propName: string): boolean {
  return BINDABLE_PROP_KEYS.has(propName);
}

// Mirrors internal/siteconfig/collections.go bindingTypeMatches. The backend
// is authoritative; the client filters incompatible options up front so
// users only pick combinations that will validate.
export function bindingFieldMatchesProp(
  propName: string,
  fieldType: CollectionFieldType,
): boolean {
  switch (propName) {
    case 'image':
    case 'cover':
      return fieldType === 'asset';
    case 'gallery':
      return fieldType === 'asset_list';
    case 'items':
      return fieldType === 'asset_list' || fieldType === 'enum_multi';
    case 'href':
    case 'url':
      return fieldType === 'url';
    case 'phone':
      return fieldType === 'phone';
    case 'email':
      return fieldType === 'email';
    default:
      return TEXT_FIELD_TYPES.has(fieldType);
  }
}

// Returns the top-level bindable fields a block exposes to the bindings
// editor. We surface only top-level editor fields whose name is in the
// bindable set, so the UI never offers bindings the backend will reject.
export function listBindablePropFields(
  editorSchema: BlockEditorField[] | undefined,
): BlockEditorField[] {
  if (!editorSchema) return [];
  return editorSchema.filter((field) => isPropKeyBindable(field.name));
}

export function compatibleEntryFields(
  propName: string,
  schema: FieldDefinition[],
): FieldDefinition[] {
  return schema.filter((field) =>
    bindingFieldMatchesProp(propName, field.type),
  );
}
