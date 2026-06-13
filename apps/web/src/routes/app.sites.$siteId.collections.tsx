import { Link, createFileRoute } from "@tanstack/react-router";
import { type FormEvent, useEffect, useRef, useState } from "react";
import { ArrowLeft, FolderTree, Plus, Sparkles, Trash2 } from "lucide-react";
import { EntryWorkspace } from "@/components/collections/EntryWorkspace";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Textarea } from "@/components/ui/textarea";
import { Select } from "@/components/ui/select";
import {
  APIError,
  type Collection,
  type CollectionDefaultSort,
  type CollectionFieldType,
  type CollectionSettings,
  type FieldDefinition,
  type SchemaChange,
  type SchemaDiff,
  type SchemaFieldMapping,
  type SchemaMigrationPlan,
  applyCollectionSchemaMigration,
  collectionDefaultSortOptions,
  createCollection,
  listCollections,
  draftCollectionFromPrompt,
  deleteCollection,
  previewCollectionSchemaMigration,
  updateCollection,
} from "@/lib/api";
import { actions, emptyState, form, paddedPanel, text } from "@/lib/styles";
import { cn } from "@/lib/utils";

type SchemaSaveResult =
  | { status: "saved" }
  | { status: "conflict" }
  | { status: "error" }
  | {
      status: "migration_required";
      diff: SchemaDiff;
      unmapped: SchemaChange[];
    };

function extractMigrationDetails(
  error: unknown,
): { diff: SchemaDiff; unmapped: SchemaChange[] } | null {
  if (!(error instanceof APIError)) return null;
  const payload = error.payload;
  if (!payload) return null;
  const errorObject =
    typeof payload.error === "object" && payload.error !== null
      ? payload.error
      : null;
  const code = errorObject?.code ?? payload.code;
  if (
    code !== "schema_migration_required" &&
    code !== "schema_migration_incomplete"
  ) {
    return null;
  }
  const diff = payload.diff as SchemaDiff | undefined;
  const unmappedRaw = (payload.unmapped ?? []) as SchemaChange[];
  if (!diff) return null;
  return { diff, unmapped: unmappedRaw };
}

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

  async function handleSettingsSave(
    collectionId: string,
    settings: CollectionSettings,
  ): Promise<"saved" | "conflict" | "error"> {
    try {
      const response = await updateCollection(siteId, collectionId, {
        settings,
      });
      setErrorMessage("");
      setCollections((prev) =>
        prev.map((collection) =>
          collection.id === collectionId ? response.collection : collection,
        ),
      );
      return "saved";
    } catch (error) {
      if (isDraftConflictError(error)) {
        await refreshCollections(collectionId);
        setErrorMessage(
          "This draft changed in another tab or request. The latest collections were reloaded; apply your change again.",
        );
        return "conflict";
      }
      setErrorMessage(
        error instanceof APIError
          ? error.message
          : "Could not save collection settings.",
      );
      return "error";
    }
  }

  async function handleSchemaSave(
    collectionId: string,
    schema: FieldDefinition[],
  ): Promise<SchemaSaveResult> {
    try {
      const response = await updateCollection(siteId, collectionId, { schema });
      setErrorMessage("");
      setCollections((prev) =>
        prev.map((collection) =>
          collection.id === collectionId ? response.collection : collection,
        ),
      );
      return { status: "saved" };
    } catch (error) {
      if (isDraftConflictError(error)) {
        await refreshCollections(collectionId);
        setErrorMessage(
          "This draft changed in another tab or request. The latest collections were reloaded; apply your change again.",
        );
        return { status: "conflict" };
      }
      const migrationRequired = extractMigrationDetails(error);
      if (migrationRequired) {
        return {
          status: "migration_required",
          diff: migrationRequired.diff,
          unmapped: migrationRequired.unmapped,
        };
      }
      setErrorMessage(
        error instanceof APIError
          ? error.message
          : "Could not save collection schema.",
      );
      return { status: "error" };
    }
  }

  async function handleSchemaMigrate(
    collectionId: string,
    schema: FieldDefinition[],
    mappings: SchemaFieldMapping[],
  ): Promise<SchemaSaveResult> {
    try {
      const response = await applyCollectionSchemaMigration(
        siteId,
        collectionId,
        { schema, mappings },
      );
      setErrorMessage("");
      setCollections((prev) =>
        prev.map((collection) =>
          collection.id === collectionId ? response.collection : collection,
        ),
      );
      return { status: "saved" };
    } catch (error) {
      if (isDraftConflictError(error)) {
        await refreshCollections(collectionId);
        setErrorMessage(
          "This draft changed in another tab or request. The latest collections were reloaded; apply your change again.",
        );
        return { status: "conflict" };
      }
      const migrationRequired = extractMigrationDetails(error);
      if (migrationRequired) {
        return {
          status: "migration_required",
          diff: migrationRequired.diff,
          unmapped: migrationRequired.unmapped,
        };
      }
      setErrorMessage(
        error instanceof APIError
          ? error.message
          : "Could not apply schema migration.",
      );
      return { status: "error" };
    }
  }

  async function handleSchemaPreview(
    collectionId: string,
    schema: FieldDefinition[],
    mappings: SchemaFieldMapping[],
  ): Promise<SchemaMigrationPlan | null> {
    try {
      const response = await previewCollectionSchemaMigration(
        siteId,
        collectionId,
        { schema, mappings },
      );
      return response.plan;
    } catch (error) {
      setErrorMessage(
        error instanceof APIError
          ? error.message
          : "Could not preview schema migration.",
      );
      return null;
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
              onSchemaPreview={(schema, mappings) =>
                handleSchemaPreview(selected.id, schema, mappings)
              }
              onSchemaMigrate={(schema, mappings) =>
                handleSchemaMigrate(selected.id, schema, mappings)
              }
              onSettingsSave={(settings) =>
                handleSettingsSave(selected.id, settings)
              }
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
  onSchemaPreview,
  onSchemaMigrate,
  onSettingsSave,
  onEntriesChanged,
}: {
  siteId: string;
  collection: Collection;
  collections: Collection[];
  onDelete: () => void;
  onSchemaSave: (schema: FieldDefinition[]) => Promise<SchemaSaveResult>;
  onSchemaPreview: (
    schema: FieldDefinition[],
    mappings: SchemaFieldMapping[],
  ) => Promise<SchemaMigrationPlan | null>;
  onSchemaMigrate: (
    schema: FieldDefinition[],
    mappings: SchemaFieldMapping[],
  ) => Promise<SchemaSaveResult>;
  onSettingsSave: (
    settings: CollectionSettings,
  ) => Promise<"saved" | "conflict" | "error">;
  onEntriesChanged: (entries: Collection["entries"]) => void;
}) {
  const [tab, setTab] = useState<"entries" | "schema" | "settings">("entries");

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
        <Button
          type="button"
          variant={tab === "settings" ? "default" : "outline"}
          size="sm"
          onClick={() => setTab("settings")}
          aria-pressed={tab === "settings"}
        >
          Settings
        </Button>
      </div>

      {tab === "schema" ? (
        <SchemaEditor
          schema={collection.schema}
          onSave={onSchemaSave}
          onPreview={onSchemaPreview}
          onMigrate={onSchemaMigrate}
        />
      ) : tab === "settings" ? (
        <SettingsEditor collection={collection} onSave={onSettingsSave} />
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

function SettingsEditor({
  collection,
  onSave,
}: {
  collection: Collection;
  onSave: (
    settings: CollectionSettings,
  ) => Promise<"saved" | "conflict" | "error">;
}) {
  const [defaultSort, setDefaultSort] = useState<CollectionDefaultSort>(
    (collection.settings?.defaultSort as CollectionDefaultSort) ?? "manual",
  );
  const [exposeDetailUrls, setExposeDetailUrls] = useState(
    Boolean(collection.settings?.exposeDetailUrls),
  );
  const [seoTitleTemplate, setSeoTitleTemplate] = useState(
    collection.settings?.seoTitleTemplate ?? "",
  );
  const [seoDescriptionTemplate, setSeoDescriptionTemplate] = useState(
    collection.settings?.seoDescriptionTemplate ?? "",
  );
  const [isSaving, setIsSaving] = useState(false);
  const [savedAt, setSavedAt] = useState<number | null>(null);

  async function handleSubmit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    setIsSaving(true);
    try {
      const result = await onSave({
        defaultSort,
        exposeDetailUrls,
        seoTitleTemplate: seoTitleTemplate.trim() || undefined,
        seoDescriptionTemplate: seoDescriptionTemplate.trim() || undefined,
      });
      if (result === "saved") {
        setSavedAt(Date.now());
      }
    } finally {
      setIsSaving(false);
    }
  }

  return (
    <form className={form.grid} onSubmit={handleSubmit}>
      <div className={form.field}>
        <label className={text.label} htmlFor="collection-default-sort">
          Default sort
        </label>
        <Select
          id="collection-default-sort"
          value={defaultSort}
          onChange={(event) =>
            setDefaultSort(event.target.value as CollectionDefaultSort)
          }
        >
          {collectionDefaultSortOptions.map((option) => (
            <option key={option} value={option}>
              {DEFAULT_SORT_LABELS[option]}
            </option>
          ))}
        </Select>
        <p className={form.hint}>
          Index pages use this order when their block does not override it.
        </p>
      </div>

      <div className={form.field}>
        <label className={cn(text.label, "flex items-center gap-2")}>
          <input
            type="checkbox"
            checked={exposeDetailUrls}
            onChange={(event) => setExposeDetailUrls(event.target.checked)}
          />
          Expose public detail URLs
        </label>
        <p className={form.hint}>
          Reserves the <code>/{collection.slug}</code> prefix from other page
          slugs and lets index cards link to per-entry pages even before a
          detail template exists. Detail URLs are also emitted automatically
          when a <code>collection_detail</code> page binds to this collection;
          to stop emitting them, delete that page.
        </p>
      </div>

      <div className={form.field}>
        <label className={text.label} htmlFor="collection-seo-title">
          SEO title template
        </label>
        <Input
          id="collection-seo-title"
          value={seoTitleTemplate}
          placeholder="{{entry.title}} | {{site.name}}"
          onChange={(event) => setSeoTitleTemplate(event.target.value)}
        />
        <p className={form.hint}>
          Use{" "}
          <code>{"{{entry.field}}"}</code>,{" "}
          <code>{"{{collection.singularLabel}}"}</code>, and{" "}
          <code>{"{{site.name}}"}</code>. Per-entry SEO overrides this.
        </p>
      </div>

      <div className={form.field}>
        <label className={text.label} htmlFor="collection-seo-description">
          SEO description template
        </label>
        <Textarea
          id="collection-seo-description"
          value={seoDescriptionTemplate}
          rows={2}
          placeholder="{{entry.summary}}"
          onChange={(event) => setSeoDescriptionTemplate(event.target.value)}
        />
        <p className={form.hint}>
          Falls back to the entry summary or details when blank or unresolved.
        </p>
      </div>

      <div className={actions.row}>
        <Button type="submit" disabled={isSaving}>
          {isSaving ? "Saving…" : "Save settings"}
        </Button>
        {savedAt ? (
          <span className={text.muted}>Saved</span>
        ) : null}
      </div>
    </form>
  );
}

const DEFAULT_SORT_LABELS: Record<CollectionDefaultSort, string> = {
  manual: "Manual (use the order you set in the entries list)",
  title: "Title (A → Z)",
  newest: "Newest first",
  oldest: "Oldest first",
};

function SchemaEditor({
  schema,
  onSave,
  onPreview,
  onMigrate,
}: {
  schema: FieldDefinition[];
  onSave: (schema: FieldDefinition[]) => Promise<SchemaSaveResult>;
  onPreview: (
    schema: FieldDefinition[],
    mappings: SchemaFieldMapping[],
  ) => Promise<SchemaMigrationPlan | null>;
  onMigrate: (
    schema: FieldDefinition[],
    mappings: SchemaFieldMapping[],
  ) => Promise<SchemaSaveResult>;
}) {
  const [appliedSchema, setAppliedSchema] = useState(schema);
  const [draft, setDraft] = useState<FieldDefinition[]>(schema);
  const [migrationState, setMigrationState] = useState<{
    schema: FieldDefinition[];
    diff: SchemaDiff;
  } | null>(null);
  const [isSaving, setIsSaving] = useState(false);
  if (appliedSchema !== schema) {
    setAppliedSchema(schema);
    setDraft(schema);
    setMigrationState(null);
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

  async function handleSave() {
    setIsSaving(true);
    try {
      const result = await onSave(draft);
      if (result.status === "migration_required") {
        setMigrationState({ schema: draft, diff: result.diff });
      }
    } finally {
      setIsSaving(false);
    }
  }

  async function handleMigrationApply(mappings: SchemaFieldMapping[]) {
    if (!migrationState) return;
    setIsSaving(true);
    try {
      const result = await onMigrate(migrationState.schema, mappings);
      if (result.status === "saved") {
        setMigrationState(null);
      } else if (result.status === "migration_required") {
        setMigrationState({
          schema: migrationState.schema,
          diff: result.diff,
        });
      }
    } finally {
      setIsSaving(false);
    }
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
        <Button type="button" onClick={handleSave} disabled={isSaving}>
          {isSaving ? "Saving…" : "Save schema"}
        </Button>
      </div>
      {migrationState ? (
        <SchemaMigrationModal
          diff={migrationState.diff}
          isApplying={isSaving}
          onPreview={(mappings) => onPreview(migrationState.schema, mappings)}
          onCancel={() => setMigrationState(null)}
          onApply={handleMigrationApply}
        />
      ) : null}
    </div>
  );
}

function SchemaMigrationModal({
  diff,
  isApplying,
  onPreview,
  onCancel,
  onApply,
}: {
  diff: SchemaDiff;
  isApplying: boolean;
  onPreview: (
    mappings: SchemaFieldMapping[],
  ) => Promise<SchemaMigrationPlan | null>;
  onCancel: () => void;
  onApply: (mappings: SchemaFieldMapping[]) => Promise<void> | void;
}) {
  const removedFields = diff.changes
    .filter((change) => change.kind === "removed" && change.oldField)
    .map((change) => change.oldField as FieldDefinition);
  const addedFields = diff.changes
    .filter((change) => change.kind === "added" && change.newField)
    .map((change) => change.newField as FieldDefinition);
  const retypedFields = diff.changes
    .filter((change) => change.kind === "retyped")
    .map((change) => ({
      from: change.oldField as FieldDefinition,
      to: change.newField as FieldDefinition,
    }));
  const renamedFields = diff.changes
    .filter((change) => change.kind === "renamed")
    .map((change) => ({
      from: change.oldField as FieldDefinition,
      to: change.newField as FieldDefinition,
    }));
  const modifiedFields = diff.changes
    .filter(
      (change) =>
        change.kind === "modified" && change.oldField && change.newField,
    )
    .map((change) => ({
      from: change.oldField as FieldDefinition,
      to: change.newField as FieldDefinition,
    }));

  // For each removed field the user can either rename it into one of the
  // added fields (carry values forward) or acknowledge the drop.
  type RemovalChoice =
    | { action: "drop" }
    | { action: "rename"; newKey: string };
  const [removalChoices, setRemovalChoices] = useState<
    Record<string, RemovalChoice>
  >(() => {
    const initial: Record<string, RemovalChoice> = {};
    for (const field of removedFields) {
      initial[field.key] = { action: "drop" };
    }
    return initial;
  });
  const [retypeAcks, setRetypeAcks] = useState<Record<string, boolean>>(() => {
    const initial: Record<string, boolean> = {};
    for (const change of retypedFields) {
      initial[change.from.key] = false;
    }
    return initial;
  });
  const [plan, setPlan] = useState<SchemaMigrationPlan | null>(null);
  const [planError, setPlanError] = useState("");

  const mappings: SchemaFieldMapping[] = [];
  for (const field of removedFields) {
    const choice = removalChoices[field.key];
    if (!choice) continue;
    if (choice.action === "drop") {
      mappings.push({ action: "drop", oldKey: field.key });
    } else if (choice.action === "rename") {
      mappings.push({
        action: "rename",
        oldKey: field.key,
        newKey: choice.newKey,
      });
    }
  }
  for (const change of retypedFields) {
    if (retypeAcks[change.from.key]) {
      mappings.push({ action: "retype_clear", oldKey: change.from.key });
    }
  }
  for (const change of renamedFields) {
    mappings.push({
      action: "rename",
      oldKey: change.from.key,
      newKey: change.to.key,
    });
  }

  const claimedAddedKeys = new Set(
    Object.values(removalChoices)
      .filter((choice): choice is { action: "rename"; newKey: string } =>
        choice ? choice.action === "rename" : false,
      )
      .map((choice) => choice.newKey),
  );

  const allRetypesAcknowledged = retypedFields.every(
    (change) => retypeAcks[change.from.key],
  );

  function canApply() {
    if (isApplying) return false;
    if (!allRetypesAcknowledged) return false;
    return Object.values(removalChoices).every(
      (choice) =>
        choice.action === "drop" ||
        (choice.action === "rename" && choice.newKey),
    );
  }

  async function refreshPlan() {
    setPlanError("");
    const result = await onPreview(mappings);
    if (!result) {
      setPlanError("Could not preview migration.");
      return;
    }
    setPlan(result);
  }

  const previewRequested = useRef(false);
  useEffect(() => {
    if (previewRequested.current) return;
    previewRequested.current = true;
    const previewMappings: SchemaFieldMapping[] = [];
    void onPreview(previewMappings).then((result) => {
      if (result) {
        setPlan(result);
      }
    });
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []);

  return (
    <div
      role="dialog"
      aria-modal="true"
      aria-labelledby="schema-migration-title"
      className="fixed inset-0 z-50 flex items-center justify-center bg-black/60 p-4"
    >
      <div className="w-full max-w-2xl overflow-y-auto rounded-[14px] border border-border bg-[var(--surface-1)] p-5 shadow-xl">
        <header className="mb-3 grid gap-1">
          <h3 id="schema-migration-title" className={text.sectionTitle}>
            Confirm schema migration
          </h3>
          <p className={text.muted}>
            This change rewrites how existing entries are stored. Map removed
            fields onto new fields or acknowledge that their data will be
            dropped. Type changes clear the stored value.
          </p>
        </header>

        <div className="grid gap-3">
          {renamedFields.length > 0 ? (
            <section className="grid gap-2 rounded-md border border-border bg-[var(--surface-2)] p-3">
              <p className={text.label}>Renames</p>
              <ul className="grid gap-1 text-sm">
                {renamedFields.map((change) => (
                  <li key={`${change.from.key}-${change.to.key}`}>
                    <span className="font-medium">{change.from.label}</span>{" "}
                    <span className="text-[var(--paper-muted)]">
                      (<code>{change.from.key}</code>) →{" "}
                      <span className="font-medium text-[var(--paper)]">
                        {change.to.label}
                      </span>{" "}
                      (<code>{change.to.key}</code>)
                    </span>
                  </li>
                ))}
              </ul>
            </section>
          ) : null}

          {removedFields.length > 0 ? (
            <section className="grid gap-2 rounded-md border border-border bg-[var(--surface-2)] p-3">
              <p className={text.label}>Removed fields</p>
              <ul className="grid gap-2">
                {removedFields.map((field) => {
                  const choice = removalChoices[field.key];
                  const availableTargets = addedFields.filter(
                    (added) =>
                      !claimedAddedKeys.has(added.key) ||
                      (choice?.action === "rename" &&
                        choice.newKey === added.key),
                  );
                  return (
                    <li
                      key={field.key}
                      className="grid gap-2 rounded-md border border-border bg-[var(--surface-1)] p-3"
                    >
                      <div className="text-sm">
                        <span className="font-medium">{field.label}</span>{" "}
                        <span className="text-[var(--paper-muted)]">
                          (<code>{field.key}</code>,{" "}
                          {FIELD_TYPE_LABELS[field.type]})
                        </span>
                      </div>
                      <div className="flex flex-wrap gap-3 text-sm">
                        <label className="flex items-center gap-2">
                          <input
                            type="radio"
                            checked={choice?.action === "drop"}
                            onChange={() =>
                              setRemovalChoices((prev) => ({
                                ...prev,
                                [field.key]: { action: "drop" },
                              }))
                            }
                          />
                          Drop values for {field.label}
                        </label>
                        {availableTargets.length > 0 ? (
                          <label className="flex items-center gap-2">
                            <input
                              type="radio"
                              checked={choice?.action === "rename"}
                              onChange={() =>
                                setRemovalChoices((prev) => ({
                                  ...prev,
                                  [field.key]: {
                                    action: "rename",
                                    newKey: availableTargets[0].key,
                                  },
                                }))
                              }
                            />
                            Carry values into
                            <Select
                              value={
                                choice?.action === "rename"
                                  ? choice.newKey
                                  : availableTargets[0]?.key
                              }
                              onChange={(event) =>
                                setRemovalChoices((prev) => ({
                                  ...prev,
                                  [field.key]: {
                                    action: "rename",
                                    newKey: event.target.value,
                                  },
                                }))
                              }
                              disabled={choice?.action !== "rename"}
                              className="w-auto"
                            >
                              {availableTargets.map((target) => (
                                <option key={target.key} value={target.key}>
                                  {target.label} ({target.key})
                                </option>
                              ))}
                            </Select>
                          </label>
                        ) : null}
                      </div>
                    </li>
                  );
                })}
              </ul>
            </section>
          ) : null}

          {retypedFields.length > 0 ? (
            <section className="grid gap-2 rounded-md border border-border bg-[var(--surface-2)] p-3">
              <p className={text.label}>Type changes</p>
              <ul className="grid gap-2">
                {retypedFields.map((change) => (
                  <li key={change.from.key} className="grid gap-1 text-sm">
                    <div>
                      <span className="font-medium">{change.from.label}</span>{" "}
                      <span className="text-[var(--paper-muted)]">
                        (<code>{change.from.key}</code>):{" "}
                        {FIELD_TYPE_LABELS[change.from.type]} →{" "}
                        {FIELD_TYPE_LABELS[change.to.type]}
                      </span>
                    </div>
                    <label className="flex items-center gap-2">
                      <input
                        type="checkbox"
                        checked={Boolean(retypeAcks[change.from.key])}
                        onChange={(event) =>
                          setRetypeAcks((prev) => ({
                            ...prev,
                            [change.from.key]: event.target.checked,
                          }))
                        }
                      />
                      Clear existing values on entries (cannot be undone).
                    </label>
                  </li>
                ))}
              </ul>
            </section>
          ) : null}

          {modifiedFields.length > 0 ? (
            <section className="grid gap-2 rounded-md border border-border bg-[var(--surface-2)] p-3">
              <p className={text.label}>Other changes</p>
              <ul className="grid gap-1 text-sm">
                {modifiedFields.map((change) => (
                  <li key={change.from.key}>
                    <span className="font-medium">{change.to.label}</span>{" "}
                    <span className="text-[var(--paper-muted)]">
                      (<code>{change.from.key}</code>) — label, required, or
                      validation updated
                    </span>
                  </li>
                ))}
              </ul>
            </section>
          ) : null}

          {addedFields.length > 0 ? (
            <section className="grid gap-1 rounded-md border border-border bg-[var(--surface-2)] p-3">
              <p className={text.label}>New fields</p>
              <ul className="grid gap-1 text-sm">
                {addedFields.map((field) => (
                  <li key={field.key}>
                    <span className="font-medium">{field.label}</span>{" "}
                    <span className="text-[var(--paper-muted)]">
                      (<code>{field.key}</code>, {FIELD_TYPE_LABELS[field.type]}
                      ) — existing entries start empty
                    </span>
                  </li>
                ))}
              </ul>
            </section>
          ) : null}

          {plan ? (
            <p className={text.muted}>
              {plan.entriesAffected === 0
                ? "No entries currently store the affected fields."
                : plan.entriesAffected === 1
                  ? "1 entry will be updated by this migration."
                  : `${plan.entriesAffected} entries will be updated by this migration.`}{" "}
              Schema version will become v{plan.newSchemaVersion}.
            </p>
          ) : null}
          {planError ? <p className={text.error}>{planError}</p> : null}
        </div>

        <footer className="mt-5 flex flex-wrap justify-end gap-2">
          <Button
            type="button"
            variant="outline"
            onClick={() => void refreshPlan()}
            disabled={isApplying}
          >
            Refresh preview
          </Button>
          <Button
            type="button"
            variant="outline"
            onClick={onCancel}
            disabled={isApplying}
          >
            Cancel
          </Button>
          <Button
            type="button"
            disabled={!canApply()}
            onClick={() => void onApply(mappings)}
          >
            {isApplying ? "Applying…" : "Apply migration"}
          </Button>
        </footer>
      </div>
    </div>
  );
}
