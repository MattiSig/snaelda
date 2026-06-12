import { Link, createFileRoute } from "@tanstack/react-router";
import { type FormEvent, useEffect, useState } from "react";
import { ArrowLeft, FolderTree, Plus, Sparkles, Trash2 } from "lucide-react";
import { EntryWorkspace } from "@/components/collections/EntryWorkspace";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Textarea } from "@/components/ui/textarea";
import { Select } from "@/components/ui/select";
import {
  APIError,
  type Collection,
  type CollectionFieldType,
  type FieldDefinition,
  createCollection,
  listCollections,
  draftCollectionFromPrompt,
  deleteCollection,
  updateCollection,
} from "@/lib/api";
import { actions, emptyState, form, paddedPanel, text } from "@/lib/styles";
import { cn } from "@/lib/utils";

const FIELD_TYPE_LABELS: Record<CollectionFieldType, string> = {
  text: "Short text",
  long_text: "Long text",
  rich_text: "Rich text",
  number: "Number",
  boolean: "Yes / no",
  date: "Date",
  url: "URL",
  email: "Email",
  phone: "Phone",
  location: "Location",
  enum: "One of",
  enum_multi: "Many of",
  asset: "Image / file",
  asset_list: "Gallery",
  reference: "Link to entry",
};

const FIELD_TYPES: CollectionFieldType[] = [
  "text",
  "long_text",
  "rich_text",
  "number",
  "boolean",
  "date",
  "url",
  "email",
  "phone",
  "location",
  "enum",
  "enum_multi",
  "asset",
  "asset_list",
  "reference",
];

export const Route = createFileRoute("/app/sites/$siteId/collections")({
  component: CollectionsRouteView,
});

function CollectionsRouteView() {
  const { siteId } = Route.useParams();
  return <CollectionsPanel siteId={siteId} showBackLink showTitle />;
}

export function CollectionsPanel({
  siteId,
  showBackLink = false,
  showTitle = false,
}: {
  siteId: string;
  showBackLink?: boolean;
  showTitle?: boolean;
}) {
  const [collections, setCollections] = useState<Collection[]>([]);
  const [status, setStatus] = useState<"loading" | "ready" | "error">(
    "loading",
  );
  const [errorMessage, setErrorMessage] = useState("");
  const [selectedId, setSelectedId] = useState<string | null>(null);
  const [showCreate, setShowCreate] = useState(false);
  const [showPrompt, setShowPrompt] = useState(false);
  const [isDrafting, setIsDrafting] = useState(false);

  async function fetchCollections() {
    return listCollections(siteId);
  }

  function isDraftConflictError(error: unknown) {
    if (!(error instanceof APIError)) {
      return false;
    }
    const code =
      typeof error.payload?.error === "object"
        ? error.payload.error.code
        : error.payload?.code;
    return code === "draft_conflict";
  }

  async function refreshCollections(preferredCollectionId?: string | null) {
    const response = await listCollections(siteId);
    setCollections(response.collections);
    setErrorMessage("");
    if (preferredCollectionId) {
      const stillExists = response.collections.some(
        (collection) => collection.id === preferredCollectionId,
      );
      setSelectedId(
        stillExists
          ? preferredCollectionId
          : (response.collections[0]?.id ?? null),
      );
      return response;
    }
    if (
      !response.collections.some((collection) => collection.id === selectedId)
    ) {
      setSelectedId(response.collections[0]?.id ?? null);
    }
    return response;
  }

  useEffect(() => {
    let mounted = true;
    fetchCollections()
      .then((response) => {
        if (!mounted) return;
        setCollections(response.collections);
        setStatus("ready");
        if (response.collections.length > 0 && !selectedId) {
          setSelectedId(response.collections[0].id);
        }
      })
      .catch((error) => {
        if (!mounted) return;
        setErrorMessage(
          error instanceof APIError
            ? error.message
            : "Could not load collections.",
        );
        setStatus("error");
      });
    return () => {
      mounted = false;
    };
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [siteId]);

  const selected = collections.find(
    (collection) => collection.id === selectedId,
  );

  async function handleCreate(input: {
    slug: string;
    singularLabel: string;
    pluralLabel: string;
  }) {
    try {
      const response = await createCollection(siteId, input);
      setErrorMessage("");
      setCollections((prev) => [...prev, response.collection]);
      setSelectedId(response.collection.id);
      setShowCreate(false);
    } catch (error) {
      if (isDraftConflictError(error)) {
        await refreshCollections();
        setErrorMessage(
          "This draft changed in another tab or request. The latest collections were reloaded; apply your change again.",
        );
        return;
      }
      setErrorMessage(
        error instanceof APIError
          ? error.message
          : "Could not create collection.",
      );
    }
  }

  async function handleDraftFromPrompt(prompt: string) {
    setIsDrafting(true);
    setErrorMessage("");
    try {
      const response = await draftCollectionFromPrompt(siteId, { prompt });
      setCollections((prev) => [...prev, response.collection]);
      setSelectedId(response.collection.id);
      setShowPrompt(false);
    } catch (error) {
      if (isDraftConflictError(error)) {
        await refreshCollections();
        setErrorMessage(
          "This draft changed in another tab or request. The latest collections were reloaded; apply your change again.",
        );
        return;
      }
      setErrorMessage(
        error instanceof APIError
          ? error.message
          : "Could not draft a collection from that prompt.",
      );
    } finally {
      setIsDrafting(false);
    }
  }

  async function handleDelete(collectionId: string) {
    if (!confirm("Delete this collection? Entries will be removed too."))
      return;
    try {
      await deleteCollection(siteId, collectionId);
      setErrorMessage("");
      setCollections((prev) => prev.filter((c) => c.id !== collectionId));
      if (selectedId === collectionId) {
        setSelectedId(null);
      }
    } catch (error) {
      if (isDraftConflictError(error)) {
        await refreshCollections();
        setErrorMessage(
          "This draft changed in another tab or request. The latest collections were reloaded; apply your change again.",
        );
        return;
      }
      setErrorMessage(
        error instanceof APIError
          ? error.message
          : "Could not delete collection.",
      );
    }
  }

  async function handleSchemaSave(
    collectionId: string,
    schema: FieldDefinition[],
  ) {
    try {
      const response = await updateCollection(siteId, collectionId, { schema });
      setErrorMessage("");
      setCollections((prev) =>
        prev.map((collection) =>
          collection.id === collectionId ? response.collection : collection,
        ),
      );
    } catch (error) {
      if (isDraftConflictError(error)) {
        await refreshCollections(collectionId);
        setErrorMessage(
          "This draft changed in another tab or request. The latest collections were reloaded; apply your change again.",
        );
        return;
      }
      setErrorMessage(
        error instanceof APIError
          ? error.message
          : "Could not save collection schema.",
      );
    }
  }

  return (
    <div className="grid gap-5">
      <header
        className={cn(
          "flex flex-wrap items-center justify-between gap-3",
          !showTitle && "justify-end",
        )}
      >
        {showTitle ? (
          <div className="flex items-center gap-3">
            {showBackLink ? (
              <Link
                to="/app/sites/$siteId"
                params={{ siteId }}
                search={{ panel: undefined }}
                className={actions.inlineLink}
              >
                <ArrowLeft className="size-4" />
                Back to builder
              </Link>
            ) : null}
            <div>
              <p className={text.eyebrow}>Site collections</p>
              <h1 className={cn(text.h2, "mt-1")}>Collections</h1>
            </div>
          </div>
        ) : null}
        <div className={actions.row}>
          <Button
            type="button"
            size="sm"
            variant="outline"
            onClick={() => {
              setShowPrompt((value) => !value);
              if (!showPrompt) setShowCreate(false);
            }}
          >
            <Sparkles className="mr-1.5 size-4" />
            Prompt up a collection
          </Button>
          <Button
            type="button"
            size="sm"
            onClick={() => {
              setShowCreate((value) => !value);
              if (!showCreate) setShowPrompt(false);
            }}
          >
            <Plus className="mr-1.5 size-4" />
            New collection
          </Button>
        </div>
      </header>

      {errorMessage ? (
        <section className={paddedPanel}>
          <p className={text.error}>{errorMessage}</p>
        </section>
      ) : null}

      {showPrompt ? (
        <PromptCollectionPanel
          onSubmit={handleDraftFromPrompt}
          isSubmitting={isDrafting}
          onCancel={() => setShowPrompt(false)}
        />
      ) : null}

      {showCreate ? <CreateCollectionPanel onSubmit={handleCreate} /> : null}

      {status === "loading" ? (
        <section className={paddedPanel} aria-live="polite">
          <p className={text.p}>Loading collections...</p>
        </section>
      ) : null}

      {status === "ready" && collections.length === 0 && !showCreate ? (
        <section className={cn(emptyState, "grid gap-3")}>
          <div className="flex items-center gap-2">
            <FolderTree className="size-5 text-[var(--paper-muted)]" />
            <h2 className={text.sectionTitle}>No collections yet</h2>
          </div>
          <p className={text.p}>
            Collections store structured lists like services, projects, or menu
            items. Create one to start adding entries.
          </p>
          <div>
            <Button type="button" onClick={() => setShowCreate(true)}>
              Create your first collection
            </Button>
          </div>
        </section>
      ) : null}

      {status === "ready" && collections.length > 0 ? (
        <div className="grid items-start gap-5 lg:grid-cols-[minmax(0,260px)_minmax(0,1fr)]">
          <aside className={cn(paddedPanel, "grid gap-1.5")}>
            <p className={text.label}>Collections</p>
            <ul className="grid gap-1">
              {collections.map((collection) => (
                <li key={collection.id}>
                  <button
                    type="button"
                    className={cn(
                      "flex w-full items-center justify-between gap-2 rounded-md px-3 py-2 text-left text-sm font-bold transition-colors",
                      collection.id === selectedId
                        ? "bg-[var(--surface-3)] text-[var(--paper)]"
                        : "text-[var(--paper-muted)] hover:bg-[var(--surface-2)] hover:text-[var(--paper)]",
                    )}
                    onClick={() => setSelectedId(collection.id)}
                  >
                    <span>{collection.pluralLabel}</span>
                    <span className="text-xs font-normal text-[var(--paper-muted)]">
                      {collection.entries?.length ?? 0}
                    </span>
                  </button>
                </li>
              ))}
            </ul>
          </aside>

          {selected ? (
            <CollectionDetailPanel
              key={selected.id}
              siteId={siteId}
              collection={selected}
              collections={collections}
              onDelete={() => handleDelete(selected.id)}
              onSchemaSave={(schema) => handleSchemaSave(selected.id, schema)}
              onEntriesChanged={(entries) => {
                setCollections((prev) =>
                  prev.map((collection) =>
                    collection.id === selected.id
                      ? { ...collection, entries }
                      : collection,
                  ),
                );
              }}
            />
          ) : null}
        </div>
      ) : null}
    </div>
  );
}

function CreateCollectionPanel({
  onSubmit,
}: {
  onSubmit: (input: {
    slug: string;
    singularLabel: string;
    pluralLabel: string;
  }) => void;
}) {
  const [singularLabel, setSingular] = useState("");
  const [pluralLabel, setPlural] = useState("");
  const [slug, setSlug] = useState("");

  function handleSubmit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    onSubmit({
      singularLabel: singularLabel.trim(),
      pluralLabel: pluralLabel.trim(),
      slug: slug.trim(),
    });
  }

  return (
    <section className={cn(paddedPanel, "grid gap-3")}>
      <h2 className={text.sectionTitle}>Create a collection</h2>
      <form className={form.grid} onSubmit={handleSubmit}>
        <div className={form.field}>
          <label className={text.label} htmlFor="collection-singular">
            Singular label
          </label>
          <Input
            id="collection-singular"
            value={singularLabel}
            placeholder="Service"
            onChange={(event) => setSingular(event.target.value)}
            required
          />
        </div>
        <div className={form.field}>
          <label className={text.label} htmlFor="collection-plural">
            Plural label
          </label>
          <Input
            id="collection-plural"
            value={pluralLabel}
            placeholder="Services"
            onChange={(event) => setPlural(event.target.value)}
            required
          />
        </div>
        <div className={form.field}>
          <label className={text.label} htmlFor="collection-slug">
            URL slug (optional)
          </label>
          <Input
            id="collection-slug"
            value={slug}
            placeholder="services"
            onChange={(event) => setSlug(event.target.value)}
          />
          <p className={form.hint}>
            Used for entry URLs. Leave blank to derive from the plural label.
          </p>
        </div>
        <div className={actions.row}>
          <Button type="submit">Create collection</Button>
        </div>
      </form>
    </section>
  );
}

function PromptCollectionPanel({
  onSubmit,
  isSubmitting,
  onCancel,
}: {
  onSubmit: (prompt: string) => void;
  isSubmitting: boolean;
  onCancel: () => void;
}) {
  const [prompt, setPrompt] = useState("");

  function handleSubmit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    const trimmed = prompt.trim();
    if (!trimmed) return;
    onSubmit(trimmed);
  }

  return (
    <section className={cn(paddedPanel, "grid gap-3")}>
      <div className="flex items-center gap-2">
        <Sparkles className="size-5 text-[var(--thread-gold)]" />
        <h2 className={text.sectionTitle}>Prompt up a collection</h2>
      </div>
      <p className={text.muted}>
        Describe the list you want — services, projects, menu items, team
        members. Snaelda picks the labels, slug, and field schema for you.
      </p>
      <form className={form.grid} onSubmit={handleSubmit}>
        <div className={form.field}>
          <label className={text.label} htmlFor="collection-prompt">
            Prompt
          </label>
          <Textarea
            id="collection-prompt"
            value={prompt}
            placeholder="A menu collection with name, description, price, and a photo. Mark vegan items."
            rows={3}
            onChange={(event) => setPrompt(event.target.value)}
            disabled={isSubmitting}
            required
          />
        </div>
        <div className={actions.row}>
          <Button type="submit" disabled={isSubmitting || !prompt.trim()}>
            {isSubmitting ? "Drafting…" : "Draft collection"}
          </Button>
          <Button
            type="button"
            variant="outline"
            onClick={onCancel}
            disabled={isSubmitting}
          >
            Cancel
          </Button>
        </div>
      </form>
    </section>
  );
}

function CollectionDetailPanel({
  siteId,
  collection,
  collections,
  onDelete,
  onSchemaSave,
  onEntriesChanged,
}: {
  siteId: string;
  collection: Collection;
  collections: Collection[];
  onDelete: () => void;
  onSchemaSave: (schema: FieldDefinition[]) => void;
  onEntriesChanged: (entries: Collection["entries"]) => void;
}) {
  const [tab, setTab] = useState<"entries" | "schema">("entries");

  return (
    <section className={cn(paddedPanel, "grid gap-4")}>
      <div className="flex flex-wrap items-center justify-between gap-3">
        <div>
          <p className={text.label}>{collection.singularLabel}</p>
          <h2 className={text.sectionTitle}>{collection.pluralLabel}</h2>
          <p className={cn(text.muted, "mt-1 text-sm")}>/{collection.slug}</p>
        </div>
        <Button
          type="button"
          variant="outline"
          size="sm"
          onClick={onDelete}
          title="Delete collection"
        >
          <Trash2 className="size-4" />
          Delete
        </Button>
      </div>

      <div role="tablist" aria-label="Collection tabs" className={actions.row}>
        <Button
          type="button"
          variant={tab === "entries" ? "default" : "outline"}
          size="sm"
          onClick={() => setTab("entries")}
          aria-pressed={tab === "entries"}
        >
          Entries
        </Button>
        <Button
          type="button"
          variant={tab === "schema" ? "default" : "outline"}
          size="sm"
          onClick={() => setTab("schema")}
          aria-pressed={tab === "schema"}
        >
          Schema
        </Button>
      </div>

      {tab === "schema" ? (
        <SchemaEditor schema={collection.schema} onSave={onSchemaSave} />
      ) : (
        <EntryWorkspace
          siteId={siteId}
          collection={collection}
          collections={collections}
          onEntriesChanged={onEntriesChanged}
        />
      )}
    </section>
  );
}

function SchemaEditor({
  schema,
  onSave,
}: {
  schema: FieldDefinition[];
  onSave: (schema: FieldDefinition[]) => void;
}) {
  const [appliedSchema, setAppliedSchema] = useState(schema);
  const [draft, setDraft] = useState<FieldDefinition[]>(schema);
  if (appliedSchema !== schema) {
    setAppliedSchema(schema);
    setDraft(schema);
  }

  function updateField(index: number, patch: Partial<FieldDefinition>) {
    setDraft((prev) =>
      prev.map((field, i) => (i === index ? { ...field, ...patch } : field)),
    );
  }

  function addField() {
    setDraft((prev) => [
      ...prev,
      {
        key: `field_${prev.length + 1}`,
        label: "New field",
        type: "text",
        required: false,
      },
    ]);
  }

  function removeField(index: number) {
    setDraft((prev) => prev.filter((_, i) => i !== index));
  }

  function handleSave() {
    onSave(draft);
  }

  return (
    <div className="grid gap-3">
      {draft.length === 0 ? (
        <p className={text.p}>No fields yet. Add one to describe an entry.</p>
      ) : null}
      <ul className="grid gap-3">
        {draft.map((field, index) => (
          <li
            key={`field-${index}`}
            className="rounded-[12px] border border-border bg-[var(--surface-2)] p-3"
          >
            <div className="grid gap-2 sm:grid-cols-[1fr_1fr_140px_auto]">
              <Input
                value={field.key}
                aria-label="Field key"
                onChange={(event) =>
                  updateField(index, { key: event.target.value })
                }
              />
              <Input
                value={field.label}
                aria-label="Field label"
                onChange={(event) =>
                  updateField(index, { label: event.target.value })
                }
              />
              <Select
                value={field.type}
                aria-label="Field type"
                onChange={(event) =>
                  updateField(index, {
                    type: event.target.value as CollectionFieldType,
                  })
                }
              >
                {FIELD_TYPES.map((type) => (
                  <option key={type} value={type}>
                    {FIELD_TYPE_LABELS[type]}
                  </option>
                ))}
              </Select>
              <Button
                type="button"
                variant="outline"
                size="sm"
                onClick={() => removeField(index)}
                aria-label={`Remove field ${field.label}`}
              >
                <Trash2 className="size-4" />
              </Button>
            </div>
            {field.type === "enum" || field.type === "enum_multi" ? (
              <div className="mt-2">
                <label className={text.label}>Options (comma-separated)</label>
                <Input
                  className="mt-1"
                  value={(field.options ?? []).join(", ")}
                  onChange={(event) =>
                    updateField(index, {
                      options: event.target.value
                        .split(",")
                        .map((value) => value.trim())
                        .filter(Boolean),
                    })
                  }
                />
              </div>
            ) : null}
            <label className={cn(text.label, "mt-2 flex items-center gap-2")}>
              <input
                type="checkbox"
                checked={Boolean(field.required)}
                onChange={(event) =>
                  updateField(index, { required: event.target.checked })
                }
              />
              Required
            </label>
          </li>
        ))}
      </ul>
      <div className={actions.row}>
        <Button type="button" variant="outline" onClick={addField}>
          <Plus className="size-4" />
          Add field
        </Button>
        <Button type="button" onClick={handleSave}>
          Save schema
        </Button>
      </div>
    </div>
  );
}
