import {
  Fragment,
  type FormEvent,
  type ReactNode,
  useRef,
  useState,
} from "react";
import { BlockEditor } from "@/components/BlockEditor";
import { SiteDraftRenderer } from "@/components/SiteDraftRenderer";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Select } from "@/components/ui/select";
import { Textarea } from "@/components/ui/textarea";
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

function buildBuilderPreviewStyle(theme: SiteDraft["theme"]) {
  const base = buildSiteThemeStyle(theme);
  const surface = theme.tokens.colors?.surface ?? "#241a24";

  return {
    ...base,
    "--site-surface-muted": surface,
    "--site-radius-panel": "8px",
    "--site-radius-inner": "6px",
    "--site-button-shadow": "none",
    "--site-image-shadow": "none",
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
  pageTitle: string;
  pageSlug: string;
  pageStatus: "draft" | "published";
  pageSEOTitle: string;
  pageSEODescription: string;
  pageIncludeInNavigation: boolean;
  isSavingPage: boolean;
  isPublishing: boolean;

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
  onSavePage: (event: FormEvent<HTMLFormElement>) => Promise<void>;
  onSetPageTitle: (title: string) => void;
  onSetPageSlug: (slug: string) => void;
  onSetPageStatus: (status: "draft" | "published") => void;
  onSetPageSEOTitle: (title: string) => void;
  onSetPageSEODescription: (description: string) => void;
  onSetPageIncludeInNavigation: (include: boolean) => void;
  onReorderBlocks: (blockIds: string[]) => Promise<void>;
  onDropPaletteBlock: (blockType: string, targetIndex: number) => Promise<void>;
  onMovePage: (pageId: string, direction: -1 | 1) => Promise<void>;
  onDeletePage: () => Promise<void>;
  isDeletingPage: boolean;
  pages: DraftPage[];
  sitePanelContent?: ReactNode;
  themePanelContent?: ReactNode;
  publishPanelContent?: ReactNode;
  initialWorkspacePanel?: "page" | "site" | "theme" | "publish" | null;
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
  pageTitle,
  pageSlug,
  pageStatus,
  pageSEOTitle,
  pageSEODescription,
  pageIncludeInNavigation,
  isSavingPage,
  onSelectPage,
  onSelectBlock,
  onSaveBlock,
  onCreateBlock,
  onDuplicateBlock,
  onDeleteBlock,
  onMoveBlock,
  onChangeNewBlockType,
  onSavePage,
  onSetPageTitle,
  onSetPageSlug,
  onSetPageStatus,
  onSetPageSEOTitle,
  onSetPageSEODescription,
  onSetPageIncludeInNavigation,
  onReorderBlocks,
  onDropPaletteBlock,
  onMovePage,
  onDeletePage,
  isDeletingPage,
  isPublishing,
  pages: draftPages,
  sitePanelContent,
  themePanelContent,
  publishPanelContent,
  initialWorkspacePanel = null,
}: PuckBuilderProps) {
  const [dragState, setDragState] = useState<DragState>({ kind: "idle" });
  const [workspacePanel, setWorkspacePanel] = useState<
    "page" | "site" | "theme" | "publish" | null
  >(initialWorkspacePanel);
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
    <div className="grid grid-rows-[auto_auto_minmax(0,1fr)]">
      <BuilderToolbar
        draft={draft}
        pages={draftPages}
        selectedPageId={selectedPage?.id ?? null}
        isPublishing={isPublishing}
        activeWorkspacePanel={workspacePanel}
        onOpenPageSetup={() =>
          setWorkspacePanel((current) => (current === "page" ? null : "page"))
        }
        onOpenSiteSettings={() =>
          setWorkspacePanel((current) => (current === "site" ? null : "site"))
        }
        onOpenTheme={() =>
          setWorkspacePanel((current) => (current === "theme" ? null : "theme"))
        }
        onOpenPublish={() =>
          setWorkspacePanel((current) =>
            current === "publish" ? null : "publish",
          )
        }
        onSelectPage={onSelectPage}
      />

      {workspacePanel ? (
        <WorkspacePanel
          tone={workspacePanel === "publish" ? "accent" : "default"}
          eyebrow={
            workspacePanel === "page"
              ? "Page setup"
              : workspacePanel === "site"
                ? "Site settings"
                : workspacePanel === "theme"
                  ? "Theme"
                  : "Publish"
          }
          title={
            workspacePanel === "page"
              ? selectedPage
                ? `Shape ${selectedPage.title}`
                : "Select a page first"
              : workspacePanel === "site"
                ? "Manage the broader site"
                : workspacePanel === "theme"
                  ? "Tune the visual system"
                  : "Review the live release"
          }
          description={
            workspacePanel === "page"
              ? selectedPage
                ? "Keep page-level metadata and route choices close, without burying them in the inspector."
                : "Choose a page from the builder bar, then adjust its route, SEO, and navigation behavior here."
              : workspacePanel === "site"
                ? "These controls affect the broader draft, theme, assets, and site operations. They should not compete with block editing."
                : workspacePanel === "theme"
                  ? "Adjust palette, typography, spacing, and component styling in a dedicated workspace that is easy to find."
                  : "Review what goes live, leave a release note, and keep rollback history visible in one place."
          }
          onClose={() => setWorkspacePanel(null)}
        >
          {workspacePanel === "page" ? (
            <PageSetupPanel
              selectedPage={selectedPage}
              pageErrorMessage={pageErrorMessage}
              pageStatusMessage={pageStatusMessage}
              pageTitle={pageTitle}
              pageSlug={pageSlug}
              pageStatus={pageStatus}
              pageSEOTitle={pageSEOTitle}
              pageSEODescription={pageSEODescription}
              pageIncludeInNavigation={pageIncludeInNavigation}
              isSavingPage={isSavingPage}
              isDeletingPage={isDeletingPage}
              pages={draftPages}
              onSavePage={onSavePage}
              onSetPageTitle={onSetPageTitle}
              onSetPageSlug={onSetPageSlug}
              onSetPageStatus={onSetPageStatus}
              onSetPageSEOTitle={onSetPageSEOTitle}
              onSetPageSEODescription={onSetPageSEODescription}
              onSetPageIncludeInNavigation={onSetPageIncludeInNavigation}
              onMovePage={onMovePage}
              onDeletePage={onDeletePage}
            />
          ) : null}
          {workspacePanel === "site" ? sitePanelContent : null}
          {workspacePanel === "theme" ? themePanelContent : null}
          {workspacePanel === "publish" ? publishPanelContent : null}
        </WorkspacePanel>
      ) : null}

      <div className="grid gap-0 xl:grid-cols-[200px_minmax(0,1fr)_minmax(400px,520px)] 2xl:grid-cols-[220px_minmax(0,1fr)_minmax(440px,580px)]">
        <BlockPalette
          blockRegistry={blockRegistry}
          onDragStart={handlePaletteDragStart}
        />

        <section className="flex min-h-0 flex-col overflow-auto border-x border-border bg-[var(--surface-1)]">
          <div className="p-4">
            <div className="mb-4 flex items-end justify-between gap-3">
              <div>
                <p className={text.eyebrow}>Canvas</p>
                <h2 className={text.h2}>
                  {selectedPage ? selectedPage.title : "Select a page"}
                </h2>
                <p className={cn(text.p, "mt-1 text-sm")}>
                  This viewport should read like the live page, with the editor
                  framing kept quiet.
                </p>
              </div>
            </div>

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
                  Select a page from the toolbar above to start building.
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
    </div>
  );
}

function BuilderToolbar({
  draft,
  pages,
  selectedPageId,
  isPublishing,
  activeWorkspacePanel,
  onOpenPageSetup,
  onOpenSiteSettings,
  onOpenTheme,
  onOpenPublish,
  onSelectPage,
}: {
  draft: SiteDraft;
  pages: DraftPage[];
  selectedPageId: string | null;
  isPublishing: boolean;
  activeWorkspacePanel: "page" | "site" | "theme" | "publish" | null;
  onOpenPageSetup: () => void;
  onOpenSiteSettings: () => void;
  onOpenTheme: () => void;
  onOpenPublish: () => void;
  onSelectPage: (pageId: string) => void;
}) {
  return (
    <div className="flex flex-wrap items-center justify-between gap-3 border-b border-border bg-[var(--surface-1)] px-5 py-3">
      <div className="flex min-w-0 flex-wrap items-center gap-3">
        <div className="text-sm">
          <p className={text.eyebrow}>Editing</p>
          <strong className="text-[var(--paper)]">{draft.site.name}</strong>
        </div>
        <div className="hidden h-7 w-px bg-border sm:block" />
        <label className="flex min-w-0 items-center gap-2">
          <span className={cn(text.label, "whitespace-nowrap")}>Page</span>
          <Select
            value={selectedPageId ?? ""}
            onChange={(event) => onSelectPage(event.target.value)}
            className="min-w-[180px]"
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
      </div>

      <div className="flex flex-wrap items-center gap-2">
        <button
          type="button"
          className={cn(
            "rounded-full border px-4 py-2.5 text-sm font-bold transition-[background,border-color,color,transform] hover:-translate-y-px",
            activeWorkspacePanel === "page"
              ? "border-[var(--thread-teal)] bg-[color-mix(in_oklch,var(--thread-teal)_22%,var(--surface-2))] text-[var(--paper)] shadow-[var(--shadow-tight)]"
              : "border-[color-mix(in_oklch,var(--thread-teal)_30%,var(--border))] bg-[var(--surface-2)] text-[color-mix(in_oklch,var(--paper)_88%,var(--background))] hover:border-[var(--thread-teal)] hover:bg-[color-mix(in_oklch,var(--thread-teal)_12%,var(--surface-2))]",
          )}
          onClick={onOpenPageSetup}
        >
          Page setup
        </button>
        <button
          type="button"
          className={cn(
            "rounded-full border px-4 py-2.5 text-sm font-bold transition-[background,border-color,color,transform] hover:-translate-y-px",
            activeWorkspacePanel === "site"
              ? "border-[var(--thread-coral)] bg-[color-mix(in_oklch,var(--thread-coral)_22%,var(--surface-2))] text-[var(--paper)] shadow-[var(--shadow-tight)]"
              : "border-[color-mix(in_oklch,var(--thread-coral)_30%,var(--border))] bg-[var(--surface-2)] text-[color-mix(in_oklch,var(--paper)_88%,var(--background))] hover:border-[var(--thread-coral)] hover:bg-[color-mix(in_oklch,var(--thread-coral)_12%,var(--surface-2))]",
          )}
          onClick={onOpenSiteSettings}
        >
          Site settings
        </button>
        <button
          type="button"
          className={cn(
            "rounded-full border px-4 py-2.5 text-sm font-bold transition-[background,border-color,color,transform] hover:-translate-y-px",
            activeWorkspacePanel === "theme"
              ? "border-[var(--thread-gold)] bg-[color-mix(in_oklch,var(--thread-gold)_24%,var(--surface-2))] text-[var(--paper)] shadow-[var(--shadow-tight)]"
              : "border-[color-mix(in_oklch,var(--thread-gold)_34%,var(--border))] bg-[var(--surface-2)] text-[color-mix(in_oklch,var(--paper)_92%,var(--background))] hover:border-[var(--thread-gold)] hover:bg-[color-mix(in_oklch,var(--thread-gold)_12%,var(--surface-2))]",
          )}
          onClick={onOpenTheme}
        >
          Theme
        </button>
        <button
          type="button"
          className={cn(
            "rounded-full border px-4 py-2.5 text-sm font-bold transition-[background,border-color,color,transform] hover:-translate-y-px",
            activeWorkspacePanel === "publish"
              ? "border-[color-mix(in_oklch,var(--thread-gold)_64%,var(--border))] bg-[color-mix(in_oklch,var(--thread-gold)_24%,var(--surface-2))] text-[var(--paper)] shadow-[var(--shadow-tight)]"
              : "border-[color-mix(in_oklch,var(--thread-gold)_48%,var(--border))] bg-[color-mix(in_oklch,var(--thread-gold)_12%,var(--surface-2))] text-[color-mix(in_oklch,var(--paper)_92%,var(--background))] hover:border-[var(--thread-gold)]",
          )}
          onClick={onOpenPublish}
        >
          {isPublishing ? "Publishing..." : "Publish"}
        </button>
      </div>
    </div>
  );
}

function WorkspacePanel({
  tone,
  eyebrow,
  title,
  description,
  onClose,
  children,
}: {
  tone: "default" | "accent";
  eyebrow: string;
  title: string;
  description: string;
  onClose: () => void;
  children: ReactNode;
}) {
  return (
    <section
      className={cn(
        "border-b border-border px-5 py-4 max-sm:px-3.5",
        tone === "accent"
          ? "bg-[color-mix(in_oklch,var(--surface-2)_84%,var(--thread-gold))]"
          : "bg-[var(--surface-2)]",
      )}
    >
      <div className="mx-auto grid max-w-[1760px] gap-4">
        <div className="flex items-start justify-between gap-4 max-md:flex-col">
          <div className="grid gap-1">
            <p className={text.eyebrow}>{eyebrow}</p>
            <h2 className="text-[1.55rem] font-black leading-[0.96] text-[var(--paper)]">
              {title}
            </h2>
            <p className={cn(text.p, "max-w-[74ch] text-sm")}>{description}</p>
          </div>
          <Button type="button" variant="outline" size="sm" onClick={onClose}>
            Close
          </Button>
        </div>
        {children}
      </div>
    </section>
  );
}

function PageSetupPanel({
  selectedPage,
  pageErrorMessage,
  pageStatusMessage,
  pageTitle,
  pageSlug,
  pageStatus,
  pageSEOTitle,
  pageSEODescription,
  pageIncludeInNavigation,
  isSavingPage,
  isDeletingPage,
  pages,
  onSavePage,
  onSetPageTitle,
  onSetPageSlug,
  onSetPageStatus,
  onSetPageSEOTitle,
  onSetPageSEODescription,
  onSetPageIncludeInNavigation,
  onMovePage,
  onDeletePage,
}: {
  selectedPage: DraftPage | null;
  pageErrorMessage: string;
  pageStatusMessage: string;
  pageTitle: string;
  pageSlug: string;
  pageStatus: "draft" | "published";
  pageSEOTitle: string;
  pageSEODescription: string;
  pageIncludeInNavigation: boolean;
  isSavingPage: boolean;
  isDeletingPage: boolean;
  pages: DraftPage[];
  onSavePage: (event: FormEvent<HTMLFormElement>) => Promise<void>;
  onSetPageTitle: (title: string) => void;
  onSetPageSlug: (slug: string) => void;
  onSetPageStatus: (status: "draft" | "published") => void;
  onSetPageSEOTitle: (title: string) => void;
  onSetPageSEODescription: (description: string) => void;
  onSetPageIncludeInNavigation: (include: boolean) => void;
  onMovePage: (pageId: string, direction: -1 | 1) => Promise<void>;
  onDeletePage: () => Promise<void>;
}) {
  if (!selectedPage) {
    return (
      <div className={emptyState}>
        <p className={text.p}>
          Select a page from the builder bar to adjust its title, route, search
          metadata, and navigation behavior.
        </p>
      </div>
    );
  }

  const pageIndex = pages.findIndex((page) => page.id === selectedPage.id);

  return (
    <div className="grid gap-4 xl:grid-cols-[minmax(0,1.2fr)_minmax(320px,0.8fr)]">
      <form
        className="grid gap-4 border-t border-border pt-5 first:border-t-0 first:pt-0"
        onSubmit={onSavePage}
      >
        <div className="grid gap-1">
          <p className={text.eyebrow}>Page details</p>
          <p className={cn(text.p, "text-sm")}>
            These choices shape the page URL, search snippet, and whether this
            page belongs in the main menu.
          </p>
        </div>

        <div className="grid gap-4 md:grid-cols-2">
          <div className={form.field}>
            <label htmlFor="page-setup-title" className={text.label}>
              Page title
            </label>
            <Input
              id="page-setup-title"
              value={pageTitle}
              onChange={(event) => onSetPageTitle(event.target.value)}
              required
            />
          </div>
          <div className={form.field}>
            <label htmlFor="page-setup-slug" className={text.label}>
              Page path
            </label>
            <Input
              id="page-setup-slug"
              value={pageSlug}
              onChange={(event) => onSetPageSlug(event.target.value)}
              required
            />
          </div>
          <div className={form.field}>
            <label htmlFor="page-setup-status" className={text.label}>
              Page status
            </label>
            <Select
              id="page-setup-status"
              value={pageStatus}
              onChange={(event) =>
                onSetPageStatus(
                  event.target.value === "published" ? "published" : "draft",
                )
              }
            >
              <option value="draft">Draft</option>
              <option value="published">Published</option>
            </Select>
          </div>
        </div>

        <div className="grid gap-4 md:grid-cols-2">
          <div className={form.field}>
            <label htmlFor="page-setup-seo-title" className={text.label}>
              Search title
            </label>
            <Input
              id="page-setup-seo-title"
              value={pageSEOTitle}
              onChange={(event) => onSetPageSEOTitle(event.target.value)}
              placeholder="Leave blank to reuse the page title"
            />
          </div>
          <div className={form.field}>
            <label htmlFor="page-setup-seo-description" className={text.label}>
              Search description
            </label>
            <Textarea
              id="page-setup-seo-description"
              rows={4}
              value={pageSEODescription}
              onChange={(event) => onSetPageSEODescription(event.target.value)}
              placeholder="Summarize what someone should expect before they click."
            />
          </div>
        </div>

        <label className={form.toggle}>
          <input
            type="checkbox"
            className="size-4 accent-[var(--thread-teal)]"
            checked={pageIncludeInNavigation}
            onChange={(event) =>
              onSetPageIncludeInNavigation(event.target.checked)
            }
          />
          Show this page in the main navigation
        </label>

        {pageErrorMessage ? (
          <p className={text.error}>{pageErrorMessage}</p>
        ) : null}
        {pageStatusMessage ? (
          <p className={text.success}>{pageStatusMessage}</p>
        ) : null}

        <div className={actions.rowLarge}>
          <Button type="submit" disabled={isSavingPage}>
            {isSavingPage ? "Saving page..." : "Save page changes"}
          </Button>
        </div>
      </form>

      <div className="grid gap-5 border-l border-border pl-5 max-xl:border-l-0 max-xl:pl-0">
        <section className="grid gap-4 border-t border-border pt-5 first:border-t-0 first:pt-0">
          <div className="grid gap-1">
            <p className={text.eyebrow}>Position</p>
            <p className={cn(text.p, "text-sm")}>
              Reorder this page in the draft. Menu order stays in the broader
              site controls.
            </p>
          </div>
          <div className={actions.row}>
            <Button
              type="button"
              size="sm"
              variant="outline"
              disabled={pageIndex <= 0 || isSavingPage}
              onClick={() => onMovePage(selectedPage.id, -1)}
            >
              Move earlier
            </Button>
            <Button
              type="button"
              size="sm"
              variant="outline"
              disabled={pageIndex >= pages.length - 1 || isSavingPage}
              onClick={() => onMovePage(selectedPage.id, 1)}
            >
              Move later
            </Button>
          </div>
        </section>

        <section className="grid gap-4 border-t border-border pt-5">
          <div className="grid gap-1">
            <p className={text.eyebrow}>Danger zone</p>
            <p className={cn(text.p, "text-sm")}>
              Delete this page from the draft when it no longer belongs in the
              site structure.
            </p>
          </div>
          <Button
            type="button"
            size="sm"
            variant="outline"
            disabled={isDeletingPage}
            onClick={onDeletePage}
          >
            {isDeletingPage ? "Deleting page..." : "Delete page"}
          </Button>
        </section>
      </div>
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
    <aside className="flex flex-col overflow-auto border-r border-border bg-[var(--surface-1)]">
      <div className="border-b border-border p-4">
        <p className={text.eyebrow}>Blocks</p>
        <h2 className={text.h2}>Block palette</h2>
        <p className={cn(text.p, "mt-1 text-sm")}>
          Drag a block onto the canvas to add it
        </p>
      </div>

      <div className="grid gap-4 overflow-auto p-4">
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
          "relative rounded-[var(--site-radius-panel)]",
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
            "absolute inset-0 rounded-[var(--site-radius-panel)] border-2 transition-[border-color,box-shadow]",
            isSelected
              ? "border-[var(--thread-teal)]"
              : "border-transparent hover:border-[color-mix(in_oklch,var(--thread-coral)_55%,transparent)]",
          )}
          aria-label={`Edit ${definition?.displayName ?? block.type} block`}
          onClick={onSelect}
        />

        <div className="pointer-events-none absolute left-3 right-3 top-3 flex items-start justify-between gap-3">
          <div className="rounded-full border border-[color-mix(in_oklch,var(--site-border)_78%,transparent)] bg-[color-mix(in_oklch,var(--site-surface)_88%,transparent)] px-3 py-1.5 text-[10px] font-bold uppercase tracking-[0.1em] text-[var(--site-foreground)] shadow-[var(--shadow-tight)] backdrop-blur">
            {definition?.displayName ?? block.type}
          </div>
          <div className="pointer-events-auto flex items-center gap-2">
            {block.settings?.hidden ? (
              <span className="rounded-full border border-[color-mix(in_oklch,var(--site-border)_78%,transparent)] bg-[color-mix(in_oklch,var(--site-surface)_88%,transparent)] px-2.5 py-1 text-[10px] font-bold uppercase tracking-[0.08em] text-[var(--site-foreground)] shadow-[var(--shadow-tight)] backdrop-blur">
                Hidden
              </span>
            ) : null}
            <button
              type="button"
              draggable
              className="rounded-full border border-[color-mix(in_oklch,var(--site-border)_78%,transparent)] bg-[color-mix(in_oklch,var(--site-surface)_88%,transparent)] px-3 py-2 text-[var(--site-foreground)] shadow-[var(--shadow-tight)] backdrop-blur transition-transform hover:-translate-y-px active:cursor-grabbing"
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
  const hasSelection = selectedBlock || selectedPage;

  if (!hasSelection) {
    return (
      <aside className="flex flex-col overflow-auto bg-[var(--surface-2)]">
        <div className="border-b border-border p-4">
          <p className={text.eyebrow}>Inspector</p>
          <h2 className={text.h2}>Select an element</h2>
          <p className={cn(text.p, "mt-1 text-sm")}>
            Click a block on the canvas to edit its properties, or select a page
            to adjust its settings.
          </p>
        </div>
      </aside>
    );
  }

  return (
    <aside className="flex flex-col overflow-auto bg-[var(--surface-1)]">
      <div className="border-b border-border p-4">
        <p className={text.eyebrow}>Inspector</p>
        <h2 className="mt-1 text-[1.2rem] font-black leading-[1.02] text-[var(--paper)]">
          {selectedBlock
            ? (selectedDefinition?.displayName ?? "Block")
            : (selectedPage?.title ?? "Page")}
        </h2>
        <p className={cn(text.p, "mt-1 text-sm")}>
          {selectedBlock
            ? "Adjust the selected block here. Broader page and publish decisions now live in the builder workspace."
            : "Pick a block on the canvas to edit its content and visibility. Page-wide decisions live in Page setup."}
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
              Select a block in the canvas to edit its content. Use Page setup
              for route and SEO work, and Site or Publish for broader actions.
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
