import {
  Fragment,
  type FormEvent,
  type ReactNode,
  useRef,
  useState,
} from "react";
import {
  Compass,
  FileText,
  Image as ImageIcon,
  Inbox,
  Layers,
  Palette,
  Rocket,
  Search,
  Settings as SettingsIcon,
} from "lucide-react";
import { BlockEditor } from "@/components/BlockEditor";
import { SiteDraftRenderer } from "@/components/SiteDraftRenderer";
import { Button } from "@/components/ui/button";
import { Select } from "@/components/ui/select";
import {
  type AssetRecord,
  type BlockDefinition,
  type SiteDraft,
} from "@/lib/api";
import {
  buildCanonicalBlockOrder,
  draftToEditorCanvasPage,
  reorderEditorCanvasBlocks,
} from "@/lib/builder-adapter";
import { buildSiteThemeStyle } from "@/lib/site-theme";
import { actions, emptyState, form, text } from "@/lib/styles";
import { cn } from "@/lib/utils";

type DraftPage = SiteDraft["pages"][number];
type DraftBlock = SiteDraft["pages"][number]["blocks"][number];

export type BuilderSection =
  | "content"
  | "pages"
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
      "Drag a block onto the canvas, click any block to edit it, or reorder by dragging the handle.",
  },
  pages: {
    label: "Pages",
    icon: FileText,
    eyebrow: "Pages",
    title: "Manage pages in this site",
    description:
      "Rename pages, change routes, toggle navigation visibility, or add and remove pages from the draft.",
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
  selectedDefinition: BlockDefinition | undefined;
  selectedBlockIndex: number;
  blockDefinitions: Map<string, BlockDefinition>;
  uploadedSiteAssets: AssetRecord[];
  newBlockType: string;
  isSavingBlock: boolean;
  isMutatingBlocks: boolean;
  isCreatingBlock: boolean;
  blockErrorMessage: string;
  blockStatusMessage: string;
  pageErrorMessage: string;
  pageStatusMessage: string;
  isPublishing: boolean;
  pages: DraftPage[];

  onSelectPage: (pageId: string) => void;
  onSelectBlock: (blockId: string) => void;
  onSaveBlock: (
    props: Record<string, unknown>,
    hidden: boolean,
  ) => Promise<void>;
  onCreateBlock: (event: FormEvent<HTMLFormElement>) => Promise<void>;
  onDuplicateBlock: () => Promise<void>;
  onDeleteBlock: () => Promise<void>;
  onMoveBlock: (direction: -1 | 1) => Promise<void>;
  onChangeNewBlockType: (type: string) => void;
  onReorderBlocks: (blockIds: string[]) => Promise<void>;
  onDropPaletteBlock: (blockType: string, targetIndex: number) => Promise<void>;

  pagesPanelContent: ReactNode;
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
  selectedDefinition,
  selectedBlockIndex,
  blockDefinitions,
  uploadedSiteAssets,
  newBlockType,
  isSavingBlock,
  isMutatingBlocks,
  isCreatingBlock,
  blockErrorMessage,
  blockStatusMessage,
  pageErrorMessage,
  pageStatusMessage,
  isPublishing,
  pages: draftPages,
  onSelectPage,
  onSelectBlock,
  onSaveBlock,
  onCreateBlock,
  onDuplicateBlock,
  onDeleteBlock,
  onMoveBlock,
  onChangeNewBlockType,
  onReorderBlocks,
  onDropPaletteBlock,
  pagesPanelContent,
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
              selectedDefinition={selectedDefinition}
              selectedBlockIndex={selectedBlockIndex}
              blockDefinitions={blockDefinitions}
              uploadedSiteAssets={uploadedSiteAssets}
              newBlockType={newBlockType}
              isSavingBlock={isSavingBlock}
              isMutatingBlocks={isMutatingBlocks}
              isCreatingBlock={isCreatingBlock}
              blockErrorMessage={blockErrorMessage}
              blockStatusMessage={blockStatusMessage}
              pageErrorMessage={pageErrorMessage}
              pageStatusMessage={pageStatusMessage}
              onSelectPage={onSelectPage}
              onSelectBlock={onSelectBlock}
              onSaveBlock={onSaveBlock}
              onCreateBlock={onCreateBlock}
              onDuplicateBlock={onDuplicateBlock}
              onDeleteBlock={onDeleteBlock}
              onMoveBlock={onMoveBlock}
              onChangeNewBlockType={onChangeNewBlockType}
              onReorderBlocks={onReorderBlocks}
              onDropPaletteBlock={onDropPaletteBlock}
            />
          ) : (
            <AdminWorkspace
              controls={
                section === "pages"
                  ? pagesPanelContent
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
  selectedDefinition,
  selectedBlockIndex,
  blockDefinitions,
  uploadedSiteAssets,
  newBlockType,
  isSavingBlock,
  isMutatingBlocks,
  isCreatingBlock,
  blockErrorMessage,
  blockStatusMessage,
  pageErrorMessage,
  pageStatusMessage,
  onSelectPage,
  onSelectBlock,
  onSaveBlock,
  onCreateBlock,
  onDuplicateBlock,
  onDeleteBlock,
  onMoveBlock,
  onChangeNewBlockType,
  onReorderBlocks,
  onDropPaletteBlock,
}: {
  draft: SiteDraft;
  blockRegistry: BlockDefinition[];
  selectedPage: DraftPage | null;
  selectedBlock: DraftBlock | null;
  selectedDefinition: BlockDefinition | undefined;
  selectedBlockIndex: number;
  blockDefinitions: Map<string, BlockDefinition>;
  uploadedSiteAssets: AssetRecord[];
  newBlockType: string;
  isSavingBlock: boolean;
  isMutatingBlocks: boolean;
  isCreatingBlock: boolean;
  blockErrorMessage: string;
  blockStatusMessage: string;
  pageErrorMessage: string;
  pageStatusMessage: string;
  onSelectPage: (pageId: string) => void;
  onSelectBlock: (blockId: string) => void;
  onSaveBlock: (
    props: Record<string, unknown>,
    hidden: boolean,
  ) => Promise<void>;
  onCreateBlock: (event: FormEvent<HTMLFormElement>) => Promise<void>;
  onDuplicateBlock: () => Promise<void>;
  onDeleteBlock: () => Promise<void>;
  onMoveBlock: (direction: -1 | 1) => Promise<void>;
  onChangeNewBlockType: (type: string) => void;
  onReorderBlocks: (blockIds: string[]) => Promise<void>;
  onDropPaletteBlock: (blockType: string, targetIndex: number) => Promise<void>;
}) {
  const [dragState, setDragState] = useState<DragState>({ kind: "idle" });
  const dropIndicatorRef = useRef<DropIndicator | null>(null);
  const [dropIndicator, setDropIndicator] = useState<DropIndicator | null>(
    null,
  );
  const canvasRef = useRef<HTMLDivElement>(null);
  const dragTypeRef = useRef<"move" | "copy">("move");
  const editorPage = draftToEditorCanvasPage(draft, selectedPage?.id ?? null);

  function handlePaletteDragStart(event: React.DragEvent, blockType: string) {
    event.dataTransfer.setData("text/plain", `palette:${blockType}`);
    event.dataTransfer.effectAllowed = "copy";
    dragTypeRef.current = "copy";
    setDragState({ kind: "dragging-palette", blockType });
  }

  function handleBlockDragStart(event: React.DragEvent, blockId: string) {
    event.dataTransfer.setData("text/plain", `reorder:${blockId}`);
    event.dataTransfer.effectAllowed = "move";
    dragTypeRef.current = "move";
    setDragState({ kind: "dragging-block", blockId });
  }

  function handleDragOver(event: React.DragEvent) {
    event.preventDefault();
    if (!canvasRef.current || !editorPage) return;

    const target = event.target as HTMLElement | null;
    const dropZone = target?.closest<HTMLElement>("[data-drop-index]");
    const dropIndexValue = dropZone?.dataset.dropIndex;
    const nextIndicator =
      typeof dropIndexValue === "string"
        ? {
            index: Number.parseInt(dropIndexValue, 10),
          }
        : editorPage.visibleBlocks.length === 0
          ? { index: 0 }
          : null;

    if (!nextIndicator || Number.isNaN(nextIndicator.index)) {
      return;
    }

    dropIndicatorRef.current = nextIndicator;
    setDropIndicator(nextIndicator);
    event.dataTransfer.dropEffect = dragTypeRef.current;
  }

  function handleDragLeave(event: React.DragEvent) {
    if (
      canvasRef.current &&
      !canvasRef.current.contains(event.relatedTarget as Node)
    ) {
      dropIndicatorRef.current = null;
      setDropIndicator(null);
    }
  }

  async function handleDrop(event: React.DragEvent) {
    event.preventDefault();
    const target = dropIndicatorRef.current;
    dropIndicatorRef.current = null;
    setDropIndicator(null);
    const data = event.dataTransfer.getData("text/plain");

    if (data.startsWith("reorder:")) {
      const sourceBlockId = data.slice(8);
      reorderBlock(sourceBlockId, target);
    } else if (data.startsWith("palette:")) {
      const paletteBlockType = data.slice(8);
      if (paletteBlockType) {
        await onDropPaletteBlock(paletteBlockType, target?.index ?? 0);
      }
    }

    setDragState({ kind: "idle" });
  }

  function reorderBlock(sourceBlockId: string, target: DropIndicator | null) {
    if (!editorPage || !target) return;
    const visibleBlockIDs = reorderEditorCanvasBlocks(
      editorPage.visibleBlocks,
      sourceBlockId,
      target.index,
    );
    if (!visibleBlockIDs) {
      return;
    }
    onReorderBlocks(buildCanonicalBlockOrder(editorPage, visibleBlockIDs));
  }

  function handleDragEnd() {
    dropIndicatorRef.current = null;
    setDragState({ kind: "idle" });
    setDropIndicator(null);
  }

  return (
    <div className="grid min-h-0 overflow-hidden xl:grid-cols-[200px_minmax(0,1fr)_minmax(380px,500px)] 2xl:grid-cols-[220px_minmax(0,1fr)_minmax(420px,560px)]">
      <BlockPalette
        blockRegistry={blockRegistry}
        onDragStart={handlePaletteDragStart}
      />

      <section className="flex min-h-0 flex-col overflow-auto border-x border-border bg-[var(--surface-1)] max-xl:border-x-0 max-xl:border-y">
        <div className="p-4">
          {selectedPage ? (
            <div className="grid gap-4">
              <div
                ref={canvasRef}
                className="relative min-h-[400px] overflow-auto bg-[var(--surface-1)] p-4 max-sm:p-3"
                style={buildBuilderPreviewStyle(draft.theme)}
                onDragOver={handleDragOver}
                onDragLeave={handleDragLeave}
                onDrop={handleDrop}
                onDragEnd={handleDragEnd}
              >
                {!editorPage || editorPage.visibleBlocks.length === 0 ? (
                  <div
                    data-drop-index={0}
                    className={cn(
                      emptyState,
                      "grid min-h-[320px] place-items-center text-center",
                      dropIndicator?.index === 0 &&
                        "border-[var(--thread-teal)] bg-[color-mix(in_oklch,var(--surface-1)_74%,var(--thread-teal))]",
                    )}
                  >
                    <div className="grid gap-3">
                      <p className={text.p}>
                        This page has no visible blocks yet. Drag a block type
                        here to start the page, or unhide a block from the
                        inspector.
                      </p>
                      <CanvasDropZone active={dropIndicator?.index === 0} />
                    </div>
                  </div>
                ) : (
                  <SiteDraftRenderer
                    site={draft}
                    eyebrow=""
                    showPageMeta={false}
                    selectedPageId={selectedPage.id}
                    mode="builder"
                    renderBlock={({ block, page, blockIndex, children }) => (
                      <Fragment key={block.id}>
                        <CanvasDropZone
                          index={blockIndex}
                          active={dropIndicator?.index === blockIndex}
                        />
                        <CanvasBlockFrame
                          key={block.id}
                          block={block}
                          definition={blockDefinitions.get(
                            `${block.type}@${block.version}`,
                          )}
                          isSelected={block.id === selectedBlock?.id}
                          isDragging={
                            dragState.kind === "dragging-block" &&
                            dragState.blockId === block.id
                          }
                          onSelect={() => {
                            onSelectPage(page.id);
                            onSelectBlock(block.id);
                          }}
                          onDragStart={(event) =>
                            handleBlockDragStart(event, block.id)
                          }
                        >
                          {children}
                        </CanvasBlockFrame>
                        {blockIndex ===
                        editorPage.visibleBlocks.length - 1 ? (
                          <CanvasDropZone
                            index={editorPage.visibleBlocks.length}
                            active={
                              dropIndicator?.index ===
                              editorPage.visibleBlocks.length
                            }
                          />
                        ) : null}
                      </Fragment>
                    )}
                  />
                )}
              </div>

              {editorPage?.hiddenBlocks.length ? (
                <section className="border-t border-border pt-4">
                  <div className="mb-3">
                    <p className={text.eyebrow}>Hidden blocks</p>
                    <p className={cn(text.p, "mt-1 text-sm")}>
                      These blocks stay out of the rendered page until you
                      unhide them from the inspector.
                    </p>
                  </div>
                  <div className="grid gap-2">
                    {editorPage.hiddenBlocks.map(({ block }) => (
                      <button
                        key={block.id}
                        type="button"
                        className={cn(
                          "flex items-center justify-between rounded-[10px] border px-3 py-3 text-left transition-[border-color,transform]",
                          block.id === selectedBlock?.id
                            ? "border-[var(--thread-teal)] bg-[color-mix(in_oklch,var(--surface-1)_96%,var(--thread-teal))]"
                            : "border-border bg-[var(--surface-1)] hover:-translate-y-px hover:border-[var(--thread-coral)]",
                        )}
                        onClick={() => {
                          onSelectPage(selectedPage.id);
                          onSelectBlock(block.id);
                        }}
                      >
                        <span>
                          <strong className="block text-sm text-[var(--paper)]">
                            {blockDefinitions.get(
                              `${block.type}@${block.version}`,
                            )?.displayName ?? block.type}
                          </strong>
                          <small className="text-[var(--paper-muted)]">
                            Hidden from preview and publish
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

      <InspectorPanel
        selectedPage={selectedPage}
        selectedBlock={selectedBlock}
        selectedDefinition={selectedDefinition}
        selectedBlockIndex={selectedBlockIndex}
        uploadedSiteAssets={uploadedSiteAssets}
        isSavingBlock={isSavingBlock}
        isMutatingBlocks={isMutatingBlocks}
        isCreatingBlock={isCreatingBlock}
        blockErrorMessage={blockErrorMessage}
        blockStatusMessage={blockStatusMessage}
        pageErrorMessage={pageErrorMessage}
        pageStatusMessage={pageStatusMessage}
        blockRegistry={blockRegistry}
        newBlockType={newBlockType}
        onSaveBlock={onSaveBlock}
        onCreateBlock={onCreateBlock}
        onDuplicateBlock={onDuplicateBlock}
        onDeleteBlock={onDeleteBlock}
        onMoveBlock={onMoveBlock}
        onChangeNewBlockType={onChangeNewBlockType}
      />
    </div>
  );
}

function BlockPalette({
  blockRegistry,
  onDragStart,
}: {
  blockRegistry: BlockDefinition[];
  onDragStart: (event: React.DragEvent, blockType: string) => void;
}) {
  const categories = groupByCategory(blockRegistry);

  return (
    <aside className="flex flex-col overflow-auto border-r border-border bg-[var(--surface-1)] max-xl:border-r-0 max-xl:border-b">
      <div className="border-b border-border p-4">
        <p className={text.eyebrow}>Blocks</p>
        <h2 className="mt-1 text-[1rem] font-extrabold leading-tight text-[var(--paper)]">
          Drag onto canvas
        </h2>
      </div>

      <div className="grid gap-4 overflow-auto p-3">
        {Object.entries(categories).map(([category, blocks]) => (
          <div key={category}>
            <h3 className={cn(text.label, "mb-2 uppercase tracking-[0.08em]")}>
              {category}
            </h3>
            <div className="grid gap-2">
              {blocks.map((definition) => (
                <PaletteBlockCard
                  key={`${definition.type}@${definition.version}`}
                  definition={definition}
                  onDragStart={onDragStart}
                />
              ))}
            </div>
          </div>
        ))}
      </div>
    </aside>
  );
}

function PaletteBlockCard({
  definition,
  onDragStart,
}: {
  definition: BlockDefinition;
  onDragStart: (event: React.DragEvent, blockType: string) => void;
}) {
  const icon = blockTypeIcon[definition.type] ?? "⊞";

  return (
    <div
      draggable
      className="cursor-grab rounded-[10px] border border-border bg-[var(--surface-2)] p-3 transition-[border-color,transform] hover:-translate-y-0.5 hover:border-[var(--thread-teal)] active:cursor-grabbing"
      onDragStart={(event) => onDragStart(event, definition.type)}
    >
      <div className="flex items-center gap-3">
        <span className="flex size-10 shrink-0 items-center justify-center rounded-[8px] border border-border bg-[var(--surface-1)] text-lg">
          {icon}
        </span>
        <div>
          <strong className="block text-sm text-[var(--paper)]">
            {definition.displayName}
          </strong>
          <small className="block text-xs text-[var(--paper-muted)]">
            {definition.type}
          </small>
        </div>
      </div>
    </div>
  );
}

const blockTypeIcon: Record<string, string> = {
  hero: "★",
  text_section: "¶",
  image_text: "▣",
  features_grid: "☷",
  gallery: "◫",
  testimonials: "❝",
  pricing_packages: "$",
  cta_band: "➜",
  contact_form: "✉",
  faq: "?",
  team_profile_cards: "👥",
  footer: "⌂",
};

function CanvasBlockFrame({
  block,
  definition,
  isSelected,
  isDragging,
  onSelect,
  onDragStart,
  children,
}: {
  block: DraftBlock;
  definition?: BlockDefinition;
  isSelected: boolean;
  isDragging: boolean;
  onSelect: () => void;
  onDragStart: (event: React.DragEvent) => void;
  children: ReactNode;
}) {
  return (
    <div
      id={`canvas-block-${block.id}`}
      className={cn(
        "group relative transition-[opacity,transform]",
        isDragging && "opacity-40",
      )}
    >
      <div
        className={cn(
          "relative rounded-[var(--radius-panel)]",
          isSelected &&
            "shadow-[0_0_0_3px_color-mix(in_oklch,var(--thread-teal)_38%,transparent)]",
        )}
      >
        <div
          aria-hidden="true"
          className={cn(
            "transition-[transform,filter]",
            isSelected && "scale-[0.997]",
          )}
        >
          {children}
        </div>

        <button
          type="button"
          className={cn(
            "absolute inset-0 rounded-[var(--radius-panel)] border-2 transition-[border-color,box-shadow]",
            isSelected
              ? "border-[var(--thread-teal)]"
              : "border-transparent hover:border-[color-mix(in_oklch,var(--thread-coral)_55%,transparent)]",
          )}
          aria-label={`Edit ${definition?.displayName ?? block.type} block`}
          onClick={onSelect}
        />

        <div className="pointer-events-none absolute left-3 right-3 top-3 flex items-start justify-between gap-3">
          <div className="rounded-full border border-[color-mix(in_oklch,var(--color-border)_78%,transparent)] bg-[color-mix(in_oklch,var(--color-surface)_88%,transparent)] px-3 py-1.5 text-[10px] font-bold uppercase tracking-[0.1em] text-[var(--color-text)] shadow-[var(--shadow-tight)] backdrop-blur">
            {definition?.displayName ?? block.type}
          </div>
          <div className="pointer-events-auto flex items-center gap-2">
            {block.settings?.hidden ? (
              <span className="rounded-full border border-[color-mix(in_oklch,var(--color-border)_78%,transparent)] bg-[color-mix(in_oklch,var(--color-surface)_88%,transparent)] px-2.5 py-1 text-[10px] font-bold uppercase tracking-[0.08em] text-[var(--color-text)] shadow-[var(--shadow-tight)] backdrop-blur">
                Hidden
              </span>
            ) : null}
            <button
              type="button"
              draggable
              className="rounded-full border border-[color-mix(in_oklch,var(--color-border)_78%,transparent)] bg-[color-mix(in_oklch,var(--color-surface)_88%,transparent)] px-3 py-2 text-[var(--color-text)] shadow-[var(--shadow-tight)] backdrop-blur transition-transform hover:-translate-y-px active:cursor-grabbing"
              aria-label={`Drag ${definition?.displayName ?? block.type} block`}
              onClick={(event) => {
                event.preventDefault();
                event.stopPropagation();
              }}
              onDragStart={onDragStart}
            >
              <svg width="14" height="14" viewBox="0 0 14 14" fill="none">
                <circle cx="4" cy="3" r="1.2" fill="currentColor" />
                <circle cx="10" cy="3" r="1.2" fill="currentColor" />
                <circle cx="4" cy="7" r="1.2" fill="currentColor" />
                <circle cx="10" cy="7" r="1.2" fill="currentColor" />
                <circle cx="4" cy="11" r="1.2" fill="currentColor" />
                <circle cx="10" cy="11" r="1.2" fill="currentColor" />
              </svg>
            </button>
          </div>
        </div>
      </div>
    </div>
  );
}

function CanvasDropZone({
  index,
  active,
}: {
  index?: number;
  active: boolean;
}) {
  return (
    <div
      data-drop-index={index}
      aria-hidden="true"
      className="flex h-5 items-center justify-center"
    >
      <div
        className={cn(
          "flex h-1 w-full items-center justify-center rounded-full bg-transparent transition-[background-color,transform]",
          active && "bg-[var(--thread-teal)]",
        )}
      >
        <div
          className={cn(
            "size-2 rounded-full bg-transparent transition-colors",
            active && "bg-[var(--thread-gold)]",
          )}
        />
      </div>
    </div>
  );
}

function InspectorPanel({
  selectedPage,
  selectedBlock,
  selectedDefinition,
  selectedBlockIndex,
  uploadedSiteAssets,
  isSavingBlock,
  isMutatingBlocks,
  isCreatingBlock,
  blockErrorMessage,
  blockStatusMessage,
  pageErrorMessage,
  pageStatusMessage,
  blockRegistry,
  newBlockType,
  onSaveBlock,
  onCreateBlock,
  onDuplicateBlock,
  onDeleteBlock,
  onMoveBlock,
  onChangeNewBlockType,
}: {
  selectedPage: DraftPage | null;
  selectedBlock: DraftBlock | null;
  selectedDefinition: BlockDefinition | undefined;
  selectedBlockIndex: number;
  uploadedSiteAssets: AssetRecord[];
  isSavingBlock: boolean;
  isMutatingBlocks: boolean;
  isCreatingBlock: boolean;
  blockErrorMessage: string;
  blockStatusMessage: string;
  pageErrorMessage: string;
  pageStatusMessage: string;
  blockRegistry: BlockDefinition[];
  newBlockType: string;
  onSaveBlock: (
    props: Record<string, unknown>,
    hidden: boolean,
  ) => Promise<void>;
  onCreateBlock: (event: FormEvent<HTMLFormElement>) => Promise<void>;
  onDuplicateBlock: () => Promise<void>;
  onDeleteBlock: () => Promise<void>;
  onMoveBlock: (direction: -1 | 1) => Promise<void>;
  onChangeNewBlockType: (type: string) => void;
}) {
  return (
    <aside className="flex flex-col overflow-auto bg-[var(--surface-2)]">
      <div className="border-b border-border p-4">
        <p className={text.eyebrow}>Inspector</p>
        <h2 className="mt-1 text-[1rem] font-extrabold leading-tight text-[var(--paper)]">
          {selectedBlock
            ? (selectedDefinition?.displayName ?? "Block")
            : "Pick a block"}
        </h2>
        <p className={cn(text.p, "mt-1 text-sm")}>
          {selectedBlock
            ? "Edit content and visibility for the selected block."
            : "Click a block on the canvas to edit it. Page settings live in the Pages section."}
        </p>
      </div>
      <div className="grid gap-4 overflow-auto p-4">
        {selectedBlock ? (
          <>
            <BlockEditor
              key={selectedBlock.id}
              block={selectedBlock}
              definition={selectedDefinition}
              isSaving={isSavingBlock}
              errorMessage={blockErrorMessage}
              statusMessage={blockStatusMessage}
              assetLibrary={uploadedSiteAssets}
              onSave={onSaveBlock}
            />

            <div className={actions.row}>
              <Button
                type="button"
                variant="outline"
                size="sm"
                disabled={selectedBlockIndex <= 0 || isMutatingBlocks}
                onClick={() => onMoveBlock(-1)}
              >
                Move up
              </Button>
              <Button
                type="button"
                variant="outline"
                size="sm"
                disabled={
                  !selectedPage ||
                  selectedBlockIndex === selectedPage.blocks.length - 1 ||
                  isMutatingBlocks
                }
                onClick={() => onMoveBlock(1)}
              >
                Move down
              </Button>
            </div>

            <div className={actions.row}>
              <Button
                type="button"
                variant="outline"
                size="sm"
                disabled={isMutatingBlocks}
                onClick={onDuplicateBlock}
              >
                Duplicate
              </Button>
              <Button
                type="button"
                variant="outline"
                size="sm"
                disabled={isMutatingBlocks}
                onClick={onDeleteBlock}
              >
                Delete
              </Button>
            </div>
          </>
        ) : (
          <div className={emptyState}>
            <p className={text.p}>
              Nothing selected. Drop a block from the palette, or click any
              block on the canvas to edit it.
            </p>
          </div>
        )}

        <div className="grid gap-3 border-t border-border pt-4">
          <p className={text.eyebrow}>Add block</p>
          <form className={form.grid} onSubmit={onCreateBlock}>
            <label htmlFor="inspector-new-block-type" className={text.label}>
              Block type
            </label>
            <Select
              id="inspector-new-block-type"
              value={newBlockType}
              onChange={(event) => onChangeNewBlockType(event.target.value)}
            >
              {blockRegistry.map((definition) => (
                <option
                  key={`${definition.type}@${definition.version}`}
                  value={definition.type}
                >
                  {definition.displayName}
                </option>
              ))}
            </Select>
            <Button
              type="submit"
              size="sm"
              disabled={isCreatingBlock || !newBlockType || !selectedPage}
            >
              {isCreatingBlock ? "Adding block..." : "Add block to page"}
            </Button>
          </form>

          {pageErrorMessage ? (
            <p className={text.error}>{pageErrorMessage}</p>
          ) : null}
          {pageStatusMessage ? (
            <p className={text.success}>{pageStatusMessage}</p>
          ) : null}
        </div>
      </div>
    </aside>
  );
}

type DragState =
  | { kind: "idle" }
  | { kind: "dragging-palette"; blockType: string }
  | { kind: "dragging-block"; blockId: string };

type DropIndicator = {
  index: number;
};

function groupByCategory(
  registry: BlockDefinition[],
): Record<string, BlockDefinition[]> {
  const groups: Record<string, BlockDefinition[]> = {};
  for (const def of registry) {
    const category = def.category || "Other";
    if (!groups[category]) {
      groups[category] = [];
    }
    groups[category].push(def);
  }
  return groups;
}
