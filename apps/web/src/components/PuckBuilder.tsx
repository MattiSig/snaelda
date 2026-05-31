import {
  Fragment,
  type ReactNode,
  useMemo,
  useRef,
  useState,
} from "react";
import {
  Compass,
  FileText,
  FolderTree,
  Image as ImageIcon,
  Inbox,
  Layers,
  Palette,
  Rocket,
  Search,
  Settings as SettingsIcon,
} from "lucide-react";
import { SiteDraftRenderer } from "@/components/SiteDraftRenderer";
import {
  AddBlockInserter,
  EditableBlockFrame,
  InlineEditorProvider,
  type InlineEditorContextValue,
} from "@/components/inline-editor";
import { Button } from "@/components/ui/button";
import { Select } from "@/components/ui/select";
import {
  type AssetRecord,
  type BlockDefinition,
  type BlockSuggestInput,
  type ImageApplyResponse,
  type SiteDraft,
} from "@/lib/api";
import {
  buildCanonicalBlockOrder,
  draftToEditorCanvasPage,
  reorderEditorCanvasBlocks,
} from "@/lib/builder-adapter";
import { buildSiteThemeStyle } from "@/lib/site-theme";
import { emptyState, text } from "@/lib/styles";
import { cn } from "@/lib/utils";

type DraftPage = SiteDraft["pages"][number];
type DraftBlock = SiteDraft["pages"][number]["blocks"][number];

export type BuilderSection =
  | "content"
  | "pages"
  | "collections"
  | "theme"
  | "seo"
  | "navigation"
  | "assets"
  | "inquiries"
  | "publish"
  | "settings";

const sectionMeta: Record<
  BuilderSection,
  { label: string; icon: typeof Layers; eyebrow: string; title: string; description: string }
> = {
  content: {
    label: "Content",
    icon: Layers,
    eyebrow: "Content",
    title: "Edit page blocks",
    description:
      "Click any text or image to edit it. Use the + between blocks to insert a new one.",
  },
  pages: {
    label: "Pages",
    icon: FileText,
    eyebrow: "Pages",
    title: "Manage pages in this site",
    description:
      "Rename pages, change routes, toggle navigation visibility, or add and remove pages from the draft.",
  },
  collections: {
    label: "Collections",
    icon: FolderTree,
    eyebrow: "Collections",
    title: "Manage reusable content",
    description:
      "Create structured lists like services, projects, or menu items, then publish entries from one place.",
  },
  theme: {
    label: "Theme",
    icon: Palette,
    eyebrow: "Theme",
    title: "Tune the visual system",
    description:
      "Palette, type, spacing, radius, buttons, and image treatment. Every change reflects in the preview beside.",
  },
  seo: {
    label: "SEO",
    icon: Search,
    eyebrow: "SEO",
    title: "Help people find this site",
    description:
      "Per-page search title, description, and URL preview. Pick a page on the left to edit its snippet.",
  },
  navigation: {
    label: "Navigation",
    icon: Compass,
    eyebrow: "Navigation",
    title: "Shape the menu and footer links",
    description:
      "Label menu items, reorder them, add internal pages or external links, and adjust footer links separately.",
  },
  assets: {
    label: "Assets",
    icon: ImageIcon,
    eyebrow: "Assets",
    title: "Image library for this site",
    description:
      "Upload images once, then reuse them in any block field. Blocks reference assets by id, so renaming is safe.",
  },
  inquiries: {
    label: "Inquiries",
    icon: Inbox,
    eyebrow: "Inquiries",
    title: "Contact form submissions",
    description:
      "Stored from contact form blocks, on the draft preview and the published site alike. Mark statuses as you triage.",
  },
  publish: {
    label: "Publish",
    icon: Rocket,
    eyebrow: "Publish",
    title: "Send the draft live",
    description:
      "Publish the draft as a new live version, attach a custom domain, or roll back to an earlier release.",
  },
  settings: {
    label: "Settings",
    icon: SettingsIcon,
    eyebrow: "Settings",
    title: "Site identity, brand, and rebuild",
    description:
      "Rename, reslug, set brand identity, or rebuild the whole site from a fresh prompt. Danger lives at the bottom.",
  },
};

const orderedSections: BuilderSection[] = [
  "content",
  "pages",
  "collections",
  "theme",
  "seo",
  "navigation",
  "assets",
  "inquiries",
  "publish",
  "settings",
];

const sectionsWithPagePicker: ReadonlySet<BuilderSection> = new Set([
  "content",
  "theme",
  "seo",
]);

const sectionsWithPreview: ReadonlySet<BuilderSection> = new Set([
  "pages",
  "theme",
  "seo",
  "navigation",
  "publish",
]);

function buildBuilderPreviewStyle(theme: SiteDraft["theme"]) {
  const base = buildSiteThemeStyle(theme);
  const surface = theme.tokens.colors?.surface ?? "#241a24";

  return {
    ...base,
    "--color-surface-muted": surface,
    "--radius-panel": "8px",
    "--radius-inner": "6px",
    "--shadow-button": "none",
    "--shadow-image": "none",
  } as React.CSSProperties;
}

type PuckBuilderProps = {
  draft: SiteDraft;
  blockRegistry: BlockDefinition[];
  selectedPage: DraftPage | null;
  selectedBlock: DraftBlock | null;
  selectedBlockIndex: number;
  blockDefinitions: Map<string, BlockDefinition>;
  uploadedSiteAssets: AssetRecord[];
  isMutatingBlocks: boolean;
  isCreatingBlock: boolean;
  blockErrorMessage: string;
  blockStatusMessage: string;
  isPublishing: boolean;
  pages: DraftPage[];

  onSelectPage: (pageId: string) => void;
  onSelectBlock: (blockId: string) => void;
  onEditField: (
    blockId: string,
    path: ReadonlyArray<string | number>,
    value: unknown,
  ) => void;
  onToggleHidden: (blockId: string, hidden: boolean) => Promise<void>;
  onSuggestBlock?: (input: BlockSuggestInput) => Promise<void>;
  isSuggestingBlock?: boolean;
  suggestErrorMessage?: string;
  suggestStatusMessage?: string;
  siteId: string;
  onImageApplied?: (response: ImageApplyResponse) => void;
  onAddBlock: (input: {
    blockType: string;
    targetIndex: number;
    initialProps?: Record<string, unknown>;
  }) => Promise<void>;
  onDuplicateBlock: () => Promise<void>;
  onDeleteBlock: () => Promise<void>;
  onMoveBlock: (direction: -1 | 1) => Promise<void>;
  onReorderBlocks: (blockIds: string[]) => Promise<void>;

  pagesPanelContent: ReactNode;
  collectionsPanelContent: ReactNode;
  themePanelContent: ReactNode;
  seoPanelContent: ReactNode;
  navigationPanelContent: ReactNode;
  assetsPanelContent: ReactNode;
  inquiriesPanelContent: ReactNode;
  publishPanelContent: ReactNode;
  settingsPanelContent: ReactNode;

  section?: BuilderSection;
  onSectionChange?: (section: BuilderSection) => void;
};

export function PuckBuilder({
  draft,
  blockRegistry,
  selectedPage,
  selectedBlock,
  selectedBlockIndex,
  blockDefinitions,
  uploadedSiteAssets,
  isMutatingBlocks,
  isCreatingBlock,
  blockErrorMessage,
  blockStatusMessage,
  isPublishing,
  pages: draftPages,
  onSelectPage,
  onSelectBlock,
  onEditField,
  onToggleHidden,
  onSuggestBlock,
  isSuggestingBlock,
  suggestErrorMessage,
  suggestStatusMessage,
  siteId,
  onImageApplied,
  onAddBlock,
  onDuplicateBlock,
  onDeleteBlock,
  onMoveBlock,
  onReorderBlocks,
  pagesPanelContent,
  collectionsPanelContent,
  themePanelContent,
  seoPanelContent,
  navigationPanelContent,
  assetsPanelContent,
  inquiriesPanelContent,
  publishPanelContent,
  settingsPanelContent,
  section: controlledSection,
  onSectionChange,
}: PuckBuilderProps) {
  const [uncontrolledSection, setUncontrolledSection] =
    useState<BuilderSection>("content");
  const section = controlledSection ?? uncontrolledSection;

  function handleSelectSection(next: BuilderSection) {
    if (controlledSection === undefined) {
      setUncontrolledSection(next);
    }
    onSectionChange?.(next);
  }

  const meta = sectionMeta[section];
  const showPagePicker =
    sectionsWithPagePicker.has(section) && draftPages.length > 0;
  const showPreviewPane = sectionsWithPreview.has(section);

  return (
    <div className="grid min-h-0 gap-4 lg:grid-cols-[224px_minmax(0,1fr)]">
      <LeftRail
        active={section}
        onSelect={handleSelectSection}
        siteName={draft.site.name}
        isPublishing={isPublishing}
      />

      <div className="grid min-h-0 overflow-hidden rounded-[14px] border border-border bg-[var(--surface-1)] grid-rows-[auto_minmax(0,1fr)]">
        <SectionHeader
          meta={meta}
          activeSection={section}
          onSelectSection={handleSelectSection}
          showPagePicker={showPagePicker}
          pages={draftPages}
          selectedPageId={selectedPage?.id ?? null}
          onSelectPage={onSelectPage}
          isPublishing={isPublishing && section !== "publish"}
          onJumpToPublish={
            section === "publish"
              ? undefined
              : () => handleSelectSection("publish")
          }
        />

        <div className="min-h-0 overflow-hidden">
          {section === "content" ? (
            <ContentWorkspace
              draft={draft}
              blockRegistry={blockRegistry}
              selectedPage={selectedPage}
              selectedBlock={selectedBlock}
              selectedBlockIndex={selectedBlockIndex}
              blockDefinitions={blockDefinitions}
              uploadedSiteAssets={uploadedSiteAssets}
              isMutatingBlocks={isMutatingBlocks}
              isCreatingBlock={isCreatingBlock}
              blockErrorMessage={blockErrorMessage}
              blockStatusMessage={blockStatusMessage}
              onSelectPage={onSelectPage}
              onSelectBlock={onSelectBlock}
              onEditField={onEditField}
              onToggleHidden={onToggleHidden}
              onSuggestBlock={onSuggestBlock}
              isSuggestingBlock={isSuggestingBlock}
              suggestErrorMessage={suggestErrorMessage}
              suggestStatusMessage={suggestStatusMessage}
              siteId={siteId}
              onImageApplied={onImageApplied}
              onAddBlock={onAddBlock}
              onDuplicateBlock={onDuplicateBlock}
              onDeleteBlock={onDeleteBlock}
              onMoveBlock={onMoveBlock}
              onReorderBlocks={onReorderBlocks}
            />
          ) : (
            <AdminWorkspace
              controls={
                section === "pages"
                  ? pagesPanelContent
                  : section === "collections"
                    ? collectionsPanelContent
                    : section === "theme"
                      ? themePanelContent
                      : section === "seo"
                        ? seoPanelContent
                        : section === "navigation"
                          ? navigationPanelContent
                          : section === "assets"
                            ? assetsPanelContent
                            : section === "inquiries"
                              ? inquiriesPanelContent
                              : section === "publish"
                                ? publishPanelContent
                                : settingsPanelContent
              }
              preview={
                showPreviewPane ? (
                  <LivePreview
                    draft={draft}
                    selectedPageId={selectedPage?.id ?? draftPages[0]?.id ?? null}
                    pages={draftPages}
                    onSelectPage={onSelectPage}
                  />
                ) : null
              }
            />
          )}
        </div>
      </div>
    </div>
  );
}

function LeftRail({
  active,
  onSelect,
  siteName,
  isPublishing,
}: {
  active: BuilderSection;
  onSelect: (next: BuilderSection) => void;
  siteName: string;
  isPublishing: boolean;
}) {
  return (
    <aside className="rounded-[14px] border border-border bg-[var(--surface-1)] max-lg:hidden">
      <div className="border-b border-border px-4 py-4">
        <p className={text.eyebrow}>Editing</p>
        <p
          className="mt-1 truncate text-[0.95rem] font-extrabold text-[var(--paper)]"
          title={siteName}
        >
          {siteName}
        </p>
      </div>
      <nav className="grid gap-1 p-2" aria-label="Site builder sections">
        {orderedSections.map((id) => {
          const item = sectionMeta[id];
          const Icon = item.icon;
          const isActive = id === active;
          return (
            <button
              key={id}
              type="button"
              onClick={() => onSelect(id)}
              aria-current={isActive ? "page" : undefined}
              className={cn(
                "group flex w-full items-center gap-3 rounded-[10px] px-3 py-2.5 text-left text-sm transition-[background,color] duration-200 ease-out",
                isActive
                  ? "bg-[color-mix(in_oklch,var(--thread-teal)_18%,var(--surface-2))] font-bold text-[var(--paper)]"
                  : "text-[var(--paper-muted)] hover:bg-[var(--surface-2)] hover:text-[var(--paper)]",
              )}
            >
              <Icon
                className={cn(
                  "size-4 shrink-0 transition-colors",
                  isActive
                    ? "text-[var(--thread-teal)]"
                    : "text-[var(--paper-muted)] group-hover:text-[var(--paper)]",
                )}
                aria-hidden="true"
              />
              <span className="flex-1 truncate">{item.label}</span>
              {id === "publish" && isPublishing ? (
                <span className="size-1.5 animate-pulse rounded-full bg-[var(--thread-gold)]" />
              ) : null}
            </button>
          );
        })}
      </nav>
    </aside>
  );
}

function SectionHeader({
  meta,
  activeSection,
  onSelectSection,
  showPagePicker,
  pages,
  selectedPageId,
  onSelectPage,
  isPublishing,
  onJumpToPublish,
}: {
  meta: (typeof sectionMeta)[BuilderSection];
  activeSection: BuilderSection;
  onSelectSection: (next: BuilderSection) => void;
  showPagePicker: boolean;
  pages: DraftPage[];
  selectedPageId: string | null;
  onSelectPage: (pageId: string) => void;
  isPublishing: boolean;
  onJumpToPublish?: () => void;
}) {
  return (
    <header className="grid gap-3 border-b border-border bg-[var(--surface-1)] px-5 py-4 max-sm:px-4">
      <label className="grid gap-1.5 lg:hidden">
        <span className={text.label}>Section</span>
        <Select
          value={activeSection}
          onChange={(event) =>
            onSelectSection(event.target.value as BuilderSection)
          }
          aria-label="Select builder section"
        >
          {orderedSections.map((id) => (
            <option key={id} value={id}>
              {sectionMeta[id].label}
            </option>
          ))}
        </Select>
      </label>
      <div className="flex flex-wrap items-start justify-between gap-4">
        <div className="grid min-w-0 gap-1">
          <p className={text.eyebrow}>{meta.eyebrow}</p>
          <h2 className="m-0 text-[1.25rem] font-black leading-[1.05] text-[var(--paper)]">
            {meta.title}
          </h2>
          <p className={cn(text.p, "max-w-[68ch] text-sm")}>
            {meta.description}
          </p>
        </div>
        <div className="flex flex-wrap items-center gap-2">
          {showPagePicker ? (
            <label className="flex items-center gap-2">
              <span className={cn(text.label, "whitespace-nowrap")}>Page</span>
              <Select
                value={selectedPageId ?? ""}
                onChange={(event) => onSelectPage(event.target.value)}
                className="min-w-[180px]"
                aria-label="Select page"
              >
                {pages.length === 0 ? (
                  <option value="">No pages</option>
                ) : (
                  pages.map((page) => (
                    <option key={page.id} value={page.id}>
                      {page.title}
                    </option>
                  ))
                )}
              </Select>
            </label>
          ) : null}
          {onJumpToPublish ? (
            <Button
              type="button"
              variant="outline"
              size="sm"
              onClick={onJumpToPublish}
              disabled={isPublishing}
            >
              {isPublishing ? "Publishing..." : "Publish"}
            </Button>
          ) : null}
        </div>
      </div>
    </header>
  );
}

function AdminWorkspace({
  controls,
  preview,
}: {
  controls: ReactNode;
  preview: ReactNode | null;
}) {
  if (preview) {
    return (
      <div className="grid min-h-0 overflow-hidden xl:grid-cols-[minmax(0,1fr)_minmax(380px,560px)] 2xl:grid-cols-[minmax(0,1fr)_minmax(440px,640px)]">
        <div className="min-h-0 overflow-auto border-b border-border bg-[var(--surface-1)] p-5 max-sm:p-4 xl:border-b-0 xl:border-r">
          {controls}
        </div>
        <div className="min-h-0 overflow-auto bg-[var(--surface-2)] p-5 max-sm:p-4">
          {preview}
        </div>
      </div>
    );
  }
  return (
    <div className="min-h-0 overflow-auto bg-[var(--surface-1)] p-5 max-sm:p-4">
      {controls}
    </div>
  );
}

function LivePreview({
  draft,
  selectedPageId,
  pages,
  onSelectPage,
}: {
  draft: SiteDraft;
  selectedPageId: string | null;
  pages: DraftPage[];
  onSelectPage: (pageId: string) => void;
}) {
  const effectivePageId = selectedPageId ?? pages[0]?.id ?? null;

  return (
    <div className="grid gap-3">
      <div className="flex items-center justify-between gap-3">
        <p className={text.eyebrow}>Live preview</p>
        {pages.length > 1 ? (
          <Select
            value={effectivePageId ?? ""}
            onChange={(event) => onSelectPage(event.target.value)}
            aria-label="Preview page"
            className="max-w-[200px]"
          >
            {pages.map((page) => (
              <option key={page.id} value={page.id}>
                {page.title}
              </option>
            ))}
          </Select>
        ) : null}
      </div>
      <div
        className="overflow-hidden rounded-[12px] border border-border bg-[var(--surface-1)]"
        style={buildBuilderPreviewStyle(draft.theme)}
      >
        {effectivePageId ? (
          <SiteDraftRenderer
            site={draft}
            eyebrow=""
            showPageMeta={false}
            selectedPageId={effectivePageId}
            mode="builder"
          />
        ) : (
          <div className="p-6">
            <p className={text.p}>This site has no pages yet.</p>
          </div>
        )}
      </div>
    </div>
  );
}

function ContentWorkspace({
  draft,
  blockRegistry,
  selectedPage,
  selectedBlock,
  selectedBlockIndex,
  blockDefinitions,
  uploadedSiteAssets,
  isMutatingBlocks,
  isCreatingBlock,
  blockErrorMessage,
  blockStatusMessage,
  onSelectPage,
  onSelectBlock,
  onEditField,
  onToggleHidden,
  onSuggestBlock,
  isSuggestingBlock,
  suggestErrorMessage,
  suggestStatusMessage,
  siteId,
  onImageApplied,
  onAddBlock,
  onDuplicateBlock,
  onDeleteBlock,
  onMoveBlock,
  onReorderBlocks,
}: {
  draft: SiteDraft;
  blockRegistry: BlockDefinition[];
  selectedPage: DraftPage | null;
  selectedBlock: DraftBlock | null;
  selectedBlockIndex: number;
  blockDefinitions: Map<string, BlockDefinition>;
  uploadedSiteAssets: AssetRecord[];
  isMutatingBlocks: boolean;
  isCreatingBlock: boolean;
  blockErrorMessage: string;
  blockStatusMessage: string;
  onSelectPage: (pageId: string) => void;
  onSelectBlock: (blockId: string) => void;
  onEditField: (
    blockId: string,
    path: ReadonlyArray<string | number>,
    value: unknown,
  ) => void;
  onToggleHidden: (blockId: string, hidden: boolean) => Promise<void>;
  onSuggestBlock?: (input: BlockSuggestInput) => Promise<void>;
  isSuggestingBlock?: boolean;
  suggestErrorMessage?: string;
  suggestStatusMessage?: string;
  siteId: string;
  onImageApplied?: (response: ImageApplyResponse) => void;
  onAddBlock: (input: {
    blockType: string;
    targetIndex: number;
    initialProps?: Record<string, unknown>;
  }) => Promise<void>;
  onDuplicateBlock: () => Promise<void>;
  onDeleteBlock: () => Promise<void>;
  onMoveBlock: (direction: -1 | 1) => Promise<void>;
  onReorderBlocks: (blockIds: string[]) => Promise<void>;
}) {
  const editorPage = draftToEditorCanvasPage(draft, selectedPage?.id ?? null);
  const visibleCount = editorPage?.visibleBlocks.length ?? 0;
  const canMoveUp = selectedBlockIndex > 0;
  const canMoveDown =
    selectedPage !== null &&
    selectedBlockIndex >= 0 &&
    selectedBlockIndex < selectedPage.blocks.length - 1;

  const dragSourceRef = useRef<string | null>(null);
  const [dropIndicator, setDropIndicator] = useState<number | null>(null);

  function handleBlockDragStart(event: React.DragEvent, blockId: string) {
    event.dataTransfer.setData('text/plain', `reorder:${blockId}`);
    event.dataTransfer.effectAllowed = 'move';
    dragSourceRef.current = blockId;
  }

  function handleDropZoneDragOver(
    event: React.DragEvent,
    index: number,
  ) {
    if (!dragSourceRef.current) return;
    event.preventDefault();
    event.dataTransfer.dropEffect = 'move';
    setDropIndicator(index);
  }

  function handleDropZoneDrop(event: React.DragEvent, index: number) {
    event.preventDefault();
    setDropIndicator(null);
    const sourceId = dragSourceRef.current;
    dragSourceRef.current = null;
    if (!sourceId || !editorPage) return;
    const order = reorderEditorCanvasBlocks(
      editorPage.visibleBlocks,
      sourceId,
      index,
    );
    if (!order) return;
    void onReorderBlocks(buildCanonicalBlockOrder(editorPage, order));
  }

  function handleDragEnd() {
    dragSourceRef.current = null;
    setDropIndicator(null);
  }

  const inlineContext = useMemo<InlineEditorContextValue>(
    () => ({
      enabled: true,
      siteId,
      selectedBlockId: selectedBlock?.id ?? null,
      selectBlock: (blockId) => {
        onSelectBlock(blockId);
      },
      editField: onEditField,
      assetLibrary: uploadedSiteAssets,
      blockDefinitions,
      onSuggestBlock,
      isSuggestingBlock,
      onImageApplied,
      onMoveBlock,
      onDuplicateBlock,
      onDeleteBlock,
      onToggleHidden,
      canMoveUp,
      canMoveDown,
    }),
    [
      siteId,
      selectedBlock?.id,
      onSelectBlock,
      onEditField,
      uploadedSiteAssets,
      blockDefinitions,
      onSuggestBlock,
      isSuggestingBlock,
      onImageApplied,
      onMoveBlock,
      onDuplicateBlock,
      onDeleteBlock,
      onToggleHidden,
      canMoveUp,
      canMoveDown,
    ],
  );

  return (
    <InlineEditorProvider value={inlineContext}>
      <section
        className="flex min-h-0 flex-col overflow-auto bg-[var(--surface-1)]"
        onClick={(event) => {
          // Click outside selected block deselects. Clicks inside any block
          // wrapper are owned by that block's EditableBlockFrame.
          if (!selectedBlock?.id) return;
          if ((event.target as HTMLElement).closest('[data-canvas-block-id]'))
            return;
          onSelectBlock('');
        }}
        onDragEnd={handleDragEnd}
      >
        <div className="flex grow flex-col p-4 max-sm:p-2">
          {selectedPage ? (
            <div className="grid gap-4">
              <div
                className="relative grow overflow-hidden rounded-[12px] border border-border bg-[var(--surface-1)]"
                style={buildBuilderPreviewStyle(draft.theme)}
              >
                <div
                  className="absolute inset-x-0 top-0 h-2 z-30 transition-opacity"
                  style={{
                    background:
                      'linear-gradient(180deg, color-mix(in oklch, var(--thread-violet) 20%, transparent), transparent)',
                    opacity: dropIndicator !== null ? 1 : 0,
                  }}
                  aria-hidden="true"
                />
                {(isCreatingBlock || isMutatingBlocks) ? (
                  <div className="pointer-events-none absolute right-4 top-4 z-30 rounded-full border border-[oklch(98%_0.005_336_/_0.18)] bg-[oklch(12%_0.018_336_/_0.92)] px-3 py-1.5 text-[10px] font-bold uppercase tracking-[0.12em] text-[#F9F7F2] backdrop-blur">
                    {isCreatingBlock ? 'Adding block…' : 'Saving…'}
                  </div>
                ) : null}

                {!editorPage || visibleCount === 0 ? (
                  <div className="grid min-h-[60vh] place-items-center p-10 text-center">
                    <div className="grid max-w-[44ch] gap-5">
                      <p className={cn(text.p, 'mx-auto')}>
                        This page is empty. Pick a block to begin.
                      </p>
                      <AddBlockInserter
                        index={0}
                        blockRegistry={blockRegistry}
                        onAdd={onAddBlock}
                        variant="standalone"
                      />
                    </div>
                  </div>
                ) : (
                  <SiteDraftRenderer
                    site={draft}
                    eyebrow=""
                    showPageMeta={false}
                    selectedPageId={selectedPage.id}
                    mode="builder"
                    renderBlock={({ block, blockIndex, children }) => {
                      const definition = blockDefinitions.get(
                        `${block.type}@${block.version}`,
                      );
                      return (
                        <Fragment key={block.id}>
                          <DropZone
                            index={blockIndex}
                            active={dropIndicator === blockIndex}
                            onDragOver={handleDropZoneDragOver}
                            onDrop={handleDropZoneDrop}
                          >
                            <AddBlockInserter
                              index={blockIndex}
                              blockRegistry={blockRegistry}
                              onAdd={onAddBlock}
                            />
                          </DropZone>
                          <EditableBlockFrame
                            block={block}
                            definition={definition}
                            onDragStart={(event) =>
                              handleBlockDragStart(event, block.id)
                            }
                          >
                            {children}
                          </EditableBlockFrame>
                          {blockIndex === visibleCount - 1 ? (
                            <DropZone
                              index={visibleCount}
                              active={dropIndicator === visibleCount}
                              onDragOver={handleDropZoneDragOver}
                              onDrop={handleDropZoneDrop}
                            >
                              <AddBlockInserter
                                index={visibleCount}
                                blockRegistry={blockRegistry}
                                onAdd={onAddBlock}
                                variant="standalone"
                              />
                            </DropZone>
                          ) : null}
                        </Fragment>
                      );
                    }}
                  />
                )}
              </div>

              {blockErrorMessage ? (
                <p className={text.error}>{blockErrorMessage}</p>
              ) : null}
              {blockStatusMessage ? (
                <p className={text.success}>{blockStatusMessage}</p>
              ) : null}
              {suggestErrorMessage ? (
                <p className={text.error}>{suggestErrorMessage}</p>
              ) : null}
              {suggestStatusMessage ? (
                <p className={text.success}>{suggestStatusMessage}</p>
              ) : null}

              {editorPage?.hiddenBlocks.length ? (
                <section className="border-t border-border pt-4">
                  <div className="mb-3">
                    <p className={text.eyebrow}>Hidden blocks</p>
                    <p className={cn(text.p, 'mt-1 text-sm')}>
                      These blocks stay out of the rendered page until you
                      unhide them.
                    </p>
                  </div>
                  <div className="grid gap-2">
                    {editorPage.hiddenBlocks.map(({ block }) => (
                      <button
                        key={block.id}
                        type="button"
                        className={cn(
                          'flex items-center justify-between rounded-[10px] border px-3 py-3 text-left transition-[border-color,transform]',
                          block.id === selectedBlock?.id
                            ? 'border-[var(--thread-violet)] bg-[color-mix(in_oklch,var(--surface-1)_88%,var(--thread-violet))]'
                            : 'border-border bg-[var(--surface-1)] hover:-translate-y-px hover:border-[var(--thread-violet)]',
                        )}
                        onClick={(event) => {
                          event.stopPropagation();
                          onSelectPage(selectedPage.id);
                          onSelectBlock(block.id);
                          void onToggleHidden(block.id, false);
                        }}
                      >
                        <span>
                          <strong className="block text-sm text-[var(--paper)]">
                            {blockDefinitions.get(
                              `${block.type}@${block.version}`,
                            )?.displayName ?? block.type}
                          </strong>
                          <small className="text-[var(--paper-muted)]">
                            Click to unhide
                          </small>
                        </span>
                        <span className="rounded-[999px] border border-border bg-[var(--surface-2)] px-2.5 py-1 text-[10px] font-bold uppercase tracking-[0.08em] text-[var(--paper-muted)]">
                          Hidden
                        </span>
                      </button>
                    ))}
                  </div>
                </section>
              ) : null}
            </div>
          ) : (
            <div className={emptyState}>
              <p className={text.p}>
                Select a page from the header above to start building.
              </p>
            </div>
          )}
        </div>
      </section>
    </InlineEditorProvider>
  );
}

function DropZone({
  index,
  active,
  onDragOver,
  onDrop,
  children,
}: {
  index: number;
  active: boolean;
  onDragOver: (event: React.DragEvent, index: number) => void;
  onDrop: (event: React.DragEvent, index: number) => void;
  children: ReactNode;
}) {
  return (
    <div
      data-drop-index={index}
      onDragOver={(event) => onDragOver(event, index)}
      onDrop={(event) => onDrop(event, index)}
      className={cn(
        'relative transition-colors',
        active && 'bg-[color-mix(in_oklch,var(--thread-violet)_18%,transparent)]',
      )}
    >
      {children}
    </div>
  );
}
