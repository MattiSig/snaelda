import { Link, createFileRoute, useNavigate } from "@tanstack/react-router";
import type { CSSProperties, FormEvent } from "react";
import { useCallback, useEffect, useRef, useState } from "react";
import {
  GenerationProgressCard,
  type GenerationProgressItem,
} from "@/components/GenerationProgressCard";
import { PuckBuilder, type BuilderSection } from "@/components/PuckBuilder";
import { RepromptHistoryPanel } from "@/components/RepromptHistoryPanel";
import { RevisionDiffModal } from "@/components/RevisionDiffModal";
import { Button } from "@/components/ui/button";
import { CollectionsPanel } from "./app.sites.$siteId.collections";

import { Input } from "@/components/ui/input";
import { Select } from "@/components/ui/select";
import { Textarea } from "@/components/ui/textarea";
import {
  buildDraftAssetURL,
  describeAssetDimensions,
  formatAssetFileSize,
  readImageDimensions,
} from "@/lib/assets";
import { buildSiteThemeFromSelection } from "@/lib/site-theme";
import {
  APIError,
  completeAssetUpload,
  createBlock,
  createAssetUploadURL,
  createBillingCheckout,
  createSiteDomain,
  createPage,
  deleteBlock,
  deletePage,
  deleteSiteDomain,
  deleteSite,
  duplicateBlock,
  getDraftRevision,
  getBillingState,
  getSiteDomains,
  getSiteDraft,
  getSiteTheme,
  listRepromptHistory,
  listSiteFormSubmissions,
  listSiteAssets,
  listSiteVersions,
  publishSite,
  rollbackSiteVersion,
  revertReprompt,
  reorderBlocks,
  streamRepromptPage,
  streamRegenerateSiteTheme,
  streamRepromptSite,
  reorderPages,
  suggestBlock,
  updateSiteNavigation,
  type NavigationItemInput,
  type AssetRecord,
  type BillingState,
  type BlockBinding,
  type BlockDefinition,
  type BlockSuggestInput,
  type FormSubmissionRecord,
  type ImageApplyResponse,
  type FormSubmissionStatus,
  type GenerationMetadata,
  type DraftRevisionRecord,
  type PageType,
  type RepromptHistoryRecord,
  type SiteDraft,
  type SiteDomainsResponse,
  type SiteVersion,
  type ThemeEditorCatalog,
  type ThemeOption,
  type ThemeSelection,
  updateBlock,
  updateFormSubmission,
  updatePage,
  updateSite,
  updateSiteTheme,
  verifySiteDomain,
} from "@/lib/api";
import { actions, emptyState, form, ribbonPanel, text } from "@/lib/styles";
import { cn } from "@/lib/utils";

type DraftPage = SiteDraft["pages"][number];
type RefinementScope = "page" | "site";
type NavigationDraftState = {
  primary: NavigationItemInput[];
  footer: NavigationItemInput[];
};

const siteRepromptSteps: GenerationProgressItem[] = [
  { step: "prompt.normalize", label: "Reading your prompt" },
  { step: "plan.pages", label: "Planning pages and structure" },
  { step: "plan.theme", label: "Picking colors and typography" },
  { step: "plan.blocks", label: "Choosing blocks for each page" },
  { step: "assets.fetch", label: "Finding starter imagery" },
  { step: "copy.write", label: "Writing copy" },
  { step: "validate.repair", label: "Checking and repairing" },
  { step: "persist", label: "Saving your draft" },
];

const pageRepromptSteps: GenerationProgressItem[] = [
  { step: "prompt.normalize", label: "Reading your prompt" },
  { step: "plan.blocks", label: "Choosing blocks for each page" },
  { step: "copy.write", label: "Writing copy" },
  { step: "validate.repair", label: "Checking and repairing" },
  { step: "persist", label: "Saving your draft" },
];

const themeRegenerateSteps: GenerationProgressItem[] = [
  { step: "prompt.normalize", label: "Reading your prompt" },
  { step: "plan.theme", label: "Picking colors and typography" },
  { step: "validate.repair", label: "Checking and repairing" },
  { step: "persist", label: "Saving your draft" },
];

const refinementChips: Array<{ label: string; prompt: string }> = [
  {
    label: "Warmer",
    prompt: "Make this feel warmer, more personal, and less corporate.",
  },
  {
    label: "Bolder layout",
    prompt:
      "Push the layout to feel bolder, more memorable, and less template-like.",
  },
  {
    label: "Simpler copy",
    prompt:
      "Tighten the copy, reduce repetition, and make the main offer easier to scan.",
  },
  {
    label: "Stronger CTA",
    prompt: "Make the primary call to action clearer and easier to find.",
  },
  {
    label: "More premium",
    prompt:
      "Make the design feel more polished and premium while keeping it approachable.",
  },
  {
    label: "Add booking path",
    prompt:
      "Add a clearer booking or inquiry path for visitors who are ready to act.",
  },
];

const validSections: BuilderSection[] = [
  "content",
  "prompt",
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

const legacyPanelToSection: Record<string, BuilderSection> = {
  page: "pages",
  site: "settings",
  theme: "theme",
  publish: "publish",
  reprompt: "prompt",
  rebuild: "prompt",
};

export const Route = createFileRoute("/app/sites/$siteId/")({
  validateSearch: (search: Record<string, unknown>) => {
    const raw = typeof search.panel === "string" ? search.panel : undefined;
    if (!raw) {
      return { panel: undefined };
    }
    if (validSections.includes(raw as BuilderSection)) {
      return { panel: raw as BuilderSection };
    }
    const mapped = legacyPanelToSection[raw];
    return { panel: mapped };
  },
  component: SiteDetail,
});

function SiteDetail() {
  const { siteId } = Route.useParams();
  const search = Route.useSearch();
  const navigate = useNavigate();
  const [draft, setDraft] = useState<SiteDraft | null>(null);
  const [generationMetadata, setGenerationMetadata] =
    useState<GenerationMetadata | null>(null);
  const [blockRegistry, setBlockRegistry] = useState<BlockDefinition[]>([]);
  const [name, setName] = useState("");
  const [slug, setSlug] = useState("");
  const [brandBusinessName, setBrandBusinessName] = useState("");
  const [brandPrimaryColor, setBrandPrimaryColor] = useState("");
  const [brandLogoAssetId, setBrandLogoAssetId] = useState("");
  const [brandLogoAlt, setBrandLogoAlt] = useState("");
  const [selectedPageId, setSelectedPageId] = useState("");
  const [selectedBlockId, setSelectedBlockId] = useState("");
  const [versions, setVersions] = useState<SiteVersion[]>([]);
  const [billingState, setBillingState] = useState<BillingState | null>(null);
  const [domainState, setDomainState] = useState<SiteDomainsResponse | null>(
    null,
  );
  const [newPageTitle, setNewPageTitle] = useState("");
  const [newPageSlug, setNewPageSlug] = useState("");
  const [newPageType, setNewPageType] = useState<PageType>("static");
  const [newPageCollectionId, setNewPageCollectionId] = useState("");
  const [newPageIncludeInNavigation, setNewPageIncludeInNavigation] =
    useState(true);
  const [pageTitle, setPageTitle] = useState("");
  const [pageSlug, setPageSlug] = useState("");
  const [pageStatus, setPageStatus] = useState<"draft" | "published">("draft");
  const [pageSEOTitle, setPageSEOTitle] = useState("");
  const [pageSEODescription, setPageSEODescription] = useState("");
  const [pageCollectionId, setPageCollectionId] = useState("");
  const [pageIncludeInNavigation, setPageIncludeInNavigation] = useState(true);
  const [siteAssets, setSiteAssets] = useState<AssetRecord[]>([]);
  const [formSubmissions, setFormSubmissions] = useState<
    FormSubmissionRecord[]
  >([]);
  const [assetAltText, setAssetAltText] = useState("");
  const [assetFile, setAssetFile] = useState<File | null>(null);
  const [assetInputKey, setAssetInputKey] = useState(0);
  const [isLoading, setIsLoading] = useState(true);
  const [isSavingSite, setIsSavingSite] = useState(false);
  const [isSavingBrand, setIsSavingBrand] = useState(false);
  const [brandErrorMessage, setBrandErrorMessage] = useState("");
  const [brandStatusMessage, setBrandStatusMessage] = useState("");
  const [deleteConfirmSlug, setDeleteConfirmSlug] = useState("");
  const [isCreatingPage, setIsCreatingPage] = useState(false);
  const [isSavingPage, setIsSavingPage] = useState(false);
  const [isDeletingPage, setIsDeletingPage] = useState(false);
  const [isCreatingBlock, setIsCreatingBlock] = useState(false);
  const [isSuggestingBlock, setIsSuggestingBlock] = useState(false);
  const [suggestErrorMessage, setSuggestErrorMessage] = useState("");
  const [suggestStatusMessage, setSuggestStatusMessage] = useState("");
  const [isMutatingBlocks, setIsMutatingBlocks] = useState(false);
  const [isDeleting, setIsDeleting] = useState(false);
  const [isPublishing, setIsPublishing] = useState(false);
  const [activeRollbackVersionId, setActiveRollbackVersionId] = useState("");
  const [publishNote, setPublishNote] = useState("");
  const [newCustomDomainHostname, setNewCustomDomainHostname] = useState("");
  const [refinementPrompt, setRefinementPrompt] = useState("");
  const [refinementScope, setRefinementScope] =
    useState<RefinementScope>("page");
  const [themeSelection, setThemeSelection] = useState<ThemeSelection | null>(
    null,
  );
  const [savedThemeSelection, setSavedThemeSelection] =
    useState<ThemeSelection | null>(null);
  const [savedTheme, setSavedTheme] = useState<SiteDraft["theme"] | null>(null);
  const [themeOptions, setThemeOptions] = useState<ThemeEditorCatalog | null>(
    null,
  );
  const [loadErrorMessage, setLoadErrorMessage] = useState("");
  const [siteErrorMessage, setSiteErrorMessage] = useState("");
  const [siteStatusMessage, setSiteStatusMessage] = useState("");
  const [pageErrorMessage, setPageErrorMessage] = useState("");
  const [pageStatusMessage, setPageStatusMessage] = useState("");
  const [navigationErrorMessage, setNavigationErrorMessage] = useState("");
  const [navigationStatusMessage, setNavigationStatusMessage] = useState("");
  const [navigationDraft, setNavigationDraft] = useState<NavigationDraftState>({
    primary: [],
    footer: [],
  });
  const [primaryExternalLinkLabel, setPrimaryExternalLinkLabel] = useState("");
  const [primaryExternalLinkHref, setPrimaryExternalLinkHref] = useState("");
  const [footerExternalLinkLabel, setFooterExternalLinkLabel] = useState("");
  const [footerExternalLinkHref, setFooterExternalLinkHref] = useState("");
  const [blockErrorMessage, setBlockErrorMessage] = useState("");
  const [blockStatusMessage, setBlockStatusMessage] = useState("");
  const [themeErrorMessage, setThemeErrorMessage] = useState("");
  const [themeStatusMessage, setThemeStatusMessage] = useState("");
  const [isSavingTheme, setIsSavingTheme] = useState(false);
  const [isRegeneratingTheme, setIsRegeneratingTheme] = useState(false);
  const [themeProgressStep, setThemeProgressStep] = useState("");
  const [themeProgressStepTotal, setThemeProgressStepTotal] = useState(0);
  const [isSavingNavigation, setIsSavingNavigation] = useState(false);
  const [isUploadingAsset, setIsUploadingAsset] = useState(false);
  const [isRepromptingSite, setIsRepromptingSite] = useState(false);
  const [isRepromptingPage, setIsRepromptingPage] = useState(false);
  const [repromptHistory, setRepromptHistory] = useState<
    RepromptHistoryRecord[]
  >([]);
  const [repromptProgressStep, setRepromptProgressStep] = useState("");
  const [repromptProgressStepTotal, setRepromptProgressStepTotal] = useState(0);
  const [repromptProgressScope, setRepromptProgressScope] = useState<
    RefinementScope | ""
  >("");
  const [repromptHistoryScope, setRepromptHistoryScope] = useState<
    "site" | "page" | "block"
  >("site");
  const [isUndoingReprompt, setIsUndoingReprompt] = useState(false);
  const [activeRepromptHistoryId, setActiveRepromptHistoryId] = useState("");
  const [activeRepromptDiff, setActiveRepromptDiff] =
    useState<RepromptHistoryRecord | null>(null);
  const [repromptDiffPrevious, setRepromptDiffPrevious] =
    useState<DraftRevisionRecord | null>(null);
  const [repromptDiffResult, setRepromptDiffResult] =
    useState<DraftRevisionRecord | null>(null);
  const [isLoadingRepromptDiff, setIsLoadingRepromptDiff] = useState(false);
  const [repromptDiffErrorMessage, setRepromptDiffErrorMessage] = useState("");
  const [activeSubmissionId, setActiveSubmissionId] = useState("");
  const [publishErrorMessage, setPublishErrorMessage] = useState("");
  const [publishValidationIssues, setPublishValidationIssues] = useState<
    Array<{ path: string; code: string; message: string }>
  >([]);
  const [publishStatusMessage, setPublishStatusMessage] = useState("");
  const [domainErrorMessage, setDomainErrorMessage] = useState("");
  const [domainStatusMessage, setDomainStatusMessage] = useState("");
  const [isMutatingDomain, setIsMutatingDomain] = useState(false);
  const [activeDomainId, setActiveDomainId] = useState("");
  const [assetErrorMessage, setAssetErrorMessage] = useState("");
  const [assetStatusMessage, setAssetStatusMessage] = useState("");
  const [submissionErrorMessage, setSubmissionErrorMessage] = useState("");
  const [submissionStatusMessage, setSubmissionStatusMessage] = useState("");
  const [repromptErrorMessage, setRepromptErrorMessage] = useState("");
  const [repromptStatusMessage, setRepromptStatusMessage] = useState("");
  const [blockedActionMessage, setBlockedActionMessage] = useState("");
  const [blockedActionMode, setBlockedActionMode] = useState<
    "billing" | "claim" | ""
  >("");
  const [isStartingUpgrade, setIsStartingUpgrade] = useState(false);
  const draftRef = useRef<SiteDraft | null>(null);
  const blockSaveChains = useRef(new Map<string, Promise<void>>());
  const repromptDiffOpenerRef = useRef<HTMLElement | null>(null);

  useEffect(() => {
    draftRef.current = draft;
  }, [draft]);

  const replaceDraft = useCallback((nextDraft: SiteDraft) => {
    draftRef.current = nextDraft;
    setDraft(nextDraft);
  }, []);

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

  async function recoverDraftConflict({
    preferredPageID,
    preferredBlockID,
    includeTheme = false,
    includeReprompts = false,
  }: {
    preferredPageID?: string;
    preferredBlockID?: string;
    includeTheme?: boolean;
    includeReprompts?: boolean;
  } = {}) {
    const tasks: Promise<unknown>[] = [
      refreshDraftState(preferredPageID, preferredBlockID),
    ];
    if (includeTheme) {
      tasks.push(
        getSiteTheme(siteId).then((response) => {
          setThemeSelection(response.selection);
          setSavedThemeSelection(response.selection);
          setSavedTheme(response.theme);
          setThemeOptions(response.options);
        }),
      );
    }
    if (includeReprompts) {
      tasks.push(refreshRepromptHistoryState().catch(() => null));
    }
    await Promise.all(tasks);
    return "This draft changed in another tab or request. The latest version was reloaded; apply your change again.";
  }

  function enqueueBlockSave(blockId: string, task: () => Promise<void>) {
    const previous = blockSaveChains.current.get(blockId) ?? Promise.resolve();
    const next = previous.catch(() => undefined).then(task);
    const tracked = next.finally(() => {
      if (blockSaveChains.current.get(blockId) === tracked) {
        blockSaveChains.current.delete(blockId);
      }
    });
    blockSaveChains.current.set(blockId, tracked);
    return tracked;
  }

  function syncSiteFields(nextDraft: SiteDraft) {
    setName(nextDraft.site.name);
    setSlug(nextDraft.site.slug);
    setBrandBusinessName(nextDraft.brand.businessName || nextDraft.site.name);
    setBrandPrimaryColor(
      nextDraft.brand.primaryColor ||
        nextDraft.theme.tokens.colors.primary ||
        "",
    );
    setBrandLogoAssetId(nextDraft.brand.logo?.assetId ?? "");
    setBrandLogoAlt(nextDraft.brand.logo?.alt ?? "");
  }

  function syncSelectedPageFields(
    nextDraft: SiteDraft,
    nextPage: DraftPage | null,
  ) {
    if (!nextPage) {
      setPageTitle("");
      setPageSlug("");
      setPageStatus("draft");
      setPageSEOTitle("");
      setPageSEODescription("");
      setPageCollectionId("");
      setPageIncludeInNavigation(true);
      return;
    }

    setPageTitle(nextPage.title);
    setPageSlug(nextPage.slug);
    setPageStatus(nextPage.status === "published" ? "published" : "draft");
    setPageSEOTitle(nextPage.seo?.title ?? "");
    setPageSEODescription(nextPage.seo?.description ?? "");
    setPageCollectionId(nextPage.collectionId ?? "");
    setPageIncludeInNavigation(
      nextDraft.navigation.primary.some((item) => item.pageId === nextPage.id),
    );
  }

  function applyDraftUpdate(
    nextDraft: SiteDraft,
    preferredPageID?: string,
    preferredBlockID?: string,
    nextGenerationMetadata?: GenerationMetadata | null,
  ) {
    const nextPage =
      nextDraft.pages.find((page) => page.id === preferredPageID) ??
      nextDraft.pages[0] ??
      null;
    const nextBlock =
      nextPage?.blocks.find((block) => block.id === preferredBlockID) ??
      nextPage?.blocks[0] ??
      null;

    setDraft(nextDraft);
    if (nextGenerationMetadata !== undefined) {
      setGenerationMetadata(nextGenerationMetadata);
    }
    setSelectedPageId(nextPage?.id ?? "");
    setSelectedBlockId(nextBlock?.id ?? "");
    syncSiteFields(nextDraft);
    syncSelectedPageFields(nextDraft, nextPage);
    setNavigationDraft(navigationItemsFromDraft(nextDraft));
    setNavigationErrorMessage("");
    setNavigationStatusMessage("");
  }

  useEffect(() => {
    let isMounted = true;

    listSiteFormSubmissions(siteId)
      .then((submissionResponse) => {
        if (isMounted) {
          setFormSubmissions(submissionResponse.submissions);
        }
      })
      .catch(() => {
        // Trial sessions can't read form submissions; leave the list empty
        // so the rest of the builder still loads.
      });

    Promise.all([
      getSiteDraft(siteId),
      listSiteVersions(siteId),
      getSiteDomains(siteId),
      getBillingState(),
      getSiteTheme(siteId),
      listRepromptHistory(siteId),
      listSiteAssets(siteId),
    ])
      .then(
        ([
          draftResponse,
          versionResponse,
          domainResponse,
          billingResponse,
          themeResponse,
          repromptHistoryResponse,
          assetResponse,
        ]) => {
          if (!isMounted) {
            return;
          }
          setBlockRegistry(draftResponse.blockRegistry);
          setGenerationMetadata(draftResponse.generation);
          setVersions(versionResponse.versions);
          setDomainState(domainResponse);
          setBillingState(billingResponse);
          setThemeSelection(themeResponse.selection);
          setSavedThemeSelection(themeResponse.selection);
          setSavedTheme(themeResponse.theme);
          setThemeOptions(themeResponse.options);
          setRepromptHistory(repromptHistoryResponse.reprompts);
          setSiteAssets(assetResponse.assets);
          syncSiteFields(draftResponse.draft);
          const initialPage = draftResponse.draft.pages[0] ?? null;
          replaceDraft(draftResponse.draft);
          setSelectedPageId(initialPage?.id ?? "");
          setSelectedBlockId(initialPage?.blocks[0]?.id ?? "");
          syncSelectedPageFields(draftResponse.draft, initialPage);
          setNavigationDraft(navigationItemsFromDraft(draftResponse.draft));
          setIsLoading(false);
        },
      )
      .catch((error) => {
        if (!isMounted) {
          return;
        }
        setLoadErrorMessage(
          error instanceof APIError ? error.message : "Could not load site",
        );
        setIsLoading(false);
      });

    return () => {
      isMounted = false;
    };
  }, [replaceDraft, siteId]);

  // Keyed by "type@version", with a bare "type" fallback entry: the registry
  // payload carries only each block's latest version, while stored blocks keep
  // the version they were created with (migrations are passthrough, so the
  // latest schema is always valid for older blocks).
  const blockDefinitions = new Map(
    blockRegistry.flatMap((definition): [string, BlockDefinition][] => [
      [`${definition.type}@${definition.version}`, definition],
      [definition.type, definition],
    ]),
  );

  const selectedPage =
    draft?.pages.find((page) => page.id === selectedPageId) ??
    draft?.pages[0] ??
    null;
  const selectedBlock =
    selectedPage?.blocks.find((block) => block.id === selectedBlockId) ??
    selectedPage?.blocks[0] ??
    null;
  const selectedDefinition = selectedBlock
    ? (blockDefinitions.get(
        `${selectedBlock.type}@${selectedBlock.version}`,
      ) ?? blockDefinitions.get(selectedBlock.type))
    : undefined;
  const selectedBlockLabel = selectedDefinition?.displayName
    ? `${selectedDefinition.displayName} block`
    : selectedBlock
      ? `${formatRepromptScopeLabel(selectedBlock.type)} block`
      : "";
  const resolvedRepromptHistoryScope =
    repromptHistoryScope === "block" && !selectedBlock
      ? selectedPage
        ? "page"
        : "site"
      : repromptHistoryScope === "page" && !selectedPage
        ? "site"
        : repromptHistoryScope;

  function clearBlockedAction() {
    setBlockedActionMessage("");
    setBlockedActionMode("");
  }

  function setBlockedActionFromError(error: unknown) {
    if (!(error instanceof APIError)) {
      return;
    }
    const code =
      typeof error.payload?.error === "object"
        ? error.payload.error.code
        : error.payload?.code;
    if (code === "claim_required" || code === "identity_required") {
      // identity_required is the re-spin publish gate (Spec 21): a draft spun
      // from another website can only be published once an email is added.
      setBlockedActionMode("claim");
      setBlockedActionMessage(error.message);
      return;
    }
    if (code === "subscription_required" || code === "plan_limit_exceeded") {
      setBlockedActionMode("billing");
      setBlockedActionMessage(error.message);
    }
  }

  async function refreshDraftState(
    preferredPageID?: string,
    preferredBlockID?: string,
  ) {
    const response = await getSiteDraft(siteId);
    setBlockRegistry(response.blockRegistry);
    applyDraftUpdate(
      response.draft,
      preferredPageID,
      preferredBlockID,
      response.generation,
    );
    return response;
  }

  async function refreshRepromptHistoryState() {
    const response = await listRepromptHistory(siteId);
    setRepromptHistory(response.reprompts);
    return response;
  }

  async function handleStartUpgrade() {
    setIsStartingUpgrade(true);
    try {
      const response = await createBillingCheckout({ plan: "site" });
      window.location.href = response.url;
    } catch (error) {
      setBlockedActionMessage(
        error instanceof APIError ? error.message : "Could not open checkout",
      );
    } finally {
      setIsStartingUpgrade(false);
    }
  }

  async function handleSaveSite(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    setIsSavingSite(true);
    setSiteErrorMessage("");
    setSiteStatusMessage("");
    clearBlockedAction();

    try {
      const response = await updateSite(siteId, {
        name,
        slug,
      });
      setSiteStatusMessage("Site details saved.");
      applyDraftUpdate(response.draft, selectedPage?.id, selectedBlock?.id);
    } catch (error) {
      if (isDraftConflictError(error)) {
        setSiteErrorMessage(
          await recoverDraftConflict({
            preferredPageID: selectedPage?.id,
            preferredBlockID: selectedBlock?.id,
          }),
        );
        return;
      }
      setSiteErrorMessage(
        error instanceof APIError ? error.message : "Could not save site",
      );
    } finally {
      setIsSavingSite(false);
    }
  }

  async function handleSaveBrand(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    setIsSavingBrand(true);
    setBrandErrorMessage("");
    setBrandStatusMessage("");
    clearBlockedAction();

    try {
      const response = await updateSite(siteId, {
        brand: {
          businessName: brandBusinessName.trim(),
          primaryColor: brandPrimaryColor.trim(),
          ...(brandLogoAssetId.trim()
            ? {
                logo: {
                  assetId: brandLogoAssetId.trim(),
                  alt: brandLogoAlt.trim(),
                },
              }
            : {}),
        },
      });
      setBrandStatusMessage("Brand saved.");
      applyDraftUpdate(response.draft, selectedPage?.id, selectedBlock?.id);
    } catch (error) {
      if (isDraftConflictError(error)) {
        setBrandErrorMessage(
          await recoverDraftConflict({
            preferredPageID: selectedPage?.id,
            preferredBlockID: selectedBlock?.id,
          }),
        );
        return;
      }
      setBrandErrorMessage(
        error instanceof APIError ? error.message : "Could not save brand",
      );
    } finally {
      setIsSavingBrand(false);
    }
  }

  async function handleCreatePage(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    setIsCreatingPage(true);
    setPageErrorMessage("");
    setPageStatusMessage("");
    clearBlockedAction();

    if (
      newPageType !== "static" &&
      newPageCollectionId.trim() === ""
    ) {
      setPageErrorMessage(
        "Pick a collection for collection_index and collection_detail pages.",
      );
      setIsCreatingPage(false);
      return;
    }
    try {
      const response = await createPage(siteId, {
        title: newPageTitle,
        slug: newPageSlug || undefined,
        type: newPageType,
        collectionId:
          newPageType === "static" ? undefined : newPageCollectionId,
        includeInNavigation: newPageIncludeInNavigation,
      });
      const createdPage = findNewPage(draft, response.draft);
      applyDraftUpdate(
        response.draft,
        createdPage?.id,
        createdPage?.blocks[0]?.id,
      );
      setNewPageTitle("");
      setNewPageSlug("");
      setNewPageType("static");
      setNewPageCollectionId("");
      setNewPageIncludeInNavigation(true);
      setPageStatusMessage("Page added to the draft.");
    } catch (error) {
      if (isDraftConflictError(error)) {
        setPageErrorMessage(
          await recoverDraftConflict({
            preferredPageID: selectedPage?.id,
            preferredBlockID: selectedBlock?.id,
          }),
        );
        return;
      }
      setPageErrorMessage(
        error instanceof APIError ? error.message : "Could not create page",
      );
    } finally {
      setIsCreatingPage(false);
    }
  }

  async function handleSavePage(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    if (!selectedPage) {
      return;
    }

    setIsSavingPage(true);
    setPageErrorMessage("");
    setPageStatusMessage("");
    clearBlockedAction();

    try {
      const isCollectionPage =
        selectedPage.type === "collection_index" ||
        selectedPage.type === "collection_detail";
      const response = await updatePage(siteId, selectedPage.id, {
        title: pageTitle,
        slug: pageSlug,
        status: pageStatus,
        seo: {
          title: pageSEOTitle,
          description: pageSEODescription,
        },
        includeInNavigation: pageIncludeInNavigation,
        ...(isCollectionPage
          ? { collectionId: pageCollectionId }
          : {}),
      });
      applyDraftUpdate(response.draft, selectedPage.id, selectedBlock?.id);
      setPageStatusMessage("Page details saved.");
    } catch (error) {
      if (isDraftConflictError(error)) {
        setPageErrorMessage(
          await recoverDraftConflict({
            preferredPageID: selectedPage.id,
            preferredBlockID: selectedBlock?.id,
          }),
        );
        return;
      }
      setPageErrorMessage(
        error instanceof APIError ? error.message : "Could not save page",
      );
    } finally {
      setIsSavingPage(false);
    }
  }

  async function handleDeletePage() {
    if (!selectedPage) {
      return;
    }
    const confirmed = window.confirm(
      `Delete the page "${selectedPage.title}" from this draft?`,
    );
    if (!confirmed) {
      return;
    }

    setIsDeletingPage(true);
    setPageErrorMessage("");
    setPageStatusMessage("");
    clearBlockedAction();

    try {
      const response = await deletePage(siteId, selectedPage.id);
      applyDraftUpdate(response.draft);
      setPageStatusMessage("Page removed from the draft.");
    } catch (error) {
      if (isDraftConflictError(error)) {
        setPageErrorMessage(await recoverDraftConflict());
        return;
      }
      setPageErrorMessage(
        error instanceof APIError ? error.message : "Could not delete page",
      );
    } finally {
      setIsDeletingPage(false);
    }
  }

  async function handleMovePage(pageId: string, direction: -1 | 1) {
    if (!draft) {
      return;
    }
    const nextOrder = moveItem(draft.pages, pageId, direction);
    if (!nextOrder) {
      return;
    }

    setIsSavingPage(true);
    setPageErrorMessage("");
    setPageStatusMessage("");

    try {
      const response = await reorderPages(
        siteId,
        nextOrder.map((page) => page.id),
      );
      applyDraftUpdate(response.draft, pageId, selectedBlock?.id);
      setPageStatusMessage("Page order updated.");
    } catch (error) {
      if (isDraftConflictError(error)) {
        setPageErrorMessage(
          await recoverDraftConflict({
            preferredPageID: pageId,
            preferredBlockID: selectedBlock?.id,
          }),
        );
        return;
      }
      setPageErrorMessage(
        error instanceof APIError ? error.message : "Could not reorder pages",
      );
    } finally {
      setIsSavingPage(false);
    }
  }

  function updateNavigationDraftItem(
    section: keyof NavigationDraftState,
    index: number,
    patch: Partial<NavigationItemInput>,
  ) {
    setNavigationDraft((items) => ({
      ...items,
      [section]: items[section].map((item, current) =>
        current === index ? { ...item, ...patch } : item,
      ),
    }));
    setNavigationStatusMessage("");
  }

  function moveNavigationDraftItem(
    section: keyof NavigationDraftState,
    index: number,
    direction: -1 | 1,
  ) {
    const nextIndex = index + direction;
    if (nextIndex < 0 || nextIndex >= navigationDraft[section].length) {
      return;
    }
    setNavigationDraft((items) => {
      const next = [...items[section]];
      [next[index], next[nextIndex]] = [next[nextIndex], next[index]];
      return {
        ...items,
        [section]: next,
      };
    });
    setNavigationStatusMessage("");
  }

  function removeNavigationDraftItem(
    section: keyof NavigationDraftState,
    index: number,
  ) {
    setNavigationDraft((items) => ({
      ...items,
      [section]: items[section].filter((_, current) => current !== index),
    }));
    setNavigationStatusMessage("");
  }

  function addNavigationPageReference(
    section: keyof NavigationDraftState,
    pageId: string,
  ) {
    if (!draft) {
      return;
    }
    const page = draft.pages.find((candidate) => candidate.id === pageId);
    if (!page) {
      return;
    }
    setNavigationDraft((items) => ({
      ...items,
      [section]: [...items[section], { label: page.title, pageId: page.id }],
    }));
    setNavigationStatusMessage("");
  }

  function addNavigationExternalLink(
    section: keyof NavigationDraftState,
    event: FormEvent<HTMLFormElement>,
  ) {
    event.preventDefault();
    const label =
      section === "primary"
        ? primaryExternalLinkLabel.trim()
        : footerExternalLinkLabel.trim();
    const href =
      section === "primary"
        ? primaryExternalLinkHref.trim()
        : footerExternalLinkHref.trim();
    if (!label || !href) {
      setNavigationErrorMessage(
        "External links need both a label and a destination URL.",
      );
      return;
    }
    setNavigationDraft((items) => ({
      ...items,
      [section]: [...items[section], { label, href }],
    }));
    if (section === "primary") {
      setPrimaryExternalLinkLabel("");
      setPrimaryExternalLinkHref("");
    } else {
      setFooterExternalLinkLabel("");
      setFooterExternalLinkHref("");
    }
    setNavigationErrorMessage("");
    setNavigationStatusMessage("");
  }

  async function handleSaveNavigation() {
    setIsSavingNavigation(true);
    setNavigationErrorMessage("");
    setNavigationStatusMessage("");
    clearBlockedAction();

    const sanitized = {
      primary: navigationDraft.primary.map((item) => ({
        label: item.label.trim(),
        pageId: item.pageId,
        href: item.href,
      })),
      footer: navigationDraft.footer.map((item) => ({
        label: item.label.trim(),
        pageId: item.pageId,
        href: item.href,
      })),
    };

    try {
      const response = await updateSiteNavigation(siteId, sanitized);
      applyDraftUpdate(response.draft, selectedPage?.id, selectedBlock?.id);
      setNavigationStatusMessage("Navigation updated.");
    } catch (error) {
      if (isDraftConflictError(error)) {
        setNavigationErrorMessage(
          await recoverDraftConflict({
            preferredPageID: selectedPage?.id,
            preferredBlockID: selectedBlock?.id,
          }),
        );
        return;
      }
      setNavigationErrorMessage(
        error instanceof APIError ? error.message : "Could not save navigation",
      );
    } finally {
      setIsSavingNavigation(false);
    }
  }

  function handleResetNavigation() {
    if (!draft) {
      return;
    }
    setNavigationDraft(navigationItemsFromDraft(draft));
    setNavigationErrorMessage("");
    setNavigationStatusMessage("");
  }

  function handleEditField(
    blockId: string,
    path: ReadonlyArray<string | number>,
    value: unknown,
  ) {
    const currentDraft = draftRef.current;
    if (!currentDraft) return;
    const ownerPage = currentDraft.pages.find((page) =>
      page.blocks.some((block) => block.id === blockId),
    );
    const block = ownerPage?.blocks.find((b) => b.id === blockId);
    if (!ownerPage || !block) return;

    const nextProps = setBlockPropPath(block.props, path, value);
    const hidden = Boolean(block.settings?.hidden);

    // Optimistic local update so the UI commits immediately, then patch on the
    // server. Failed patches reload the draft so we don't drift.
    const optimistic = applyBlockUpdate(currentDraft, ownerPage.id, blockId, {
      props: nextProps,
      hidden,
    });
    replaceDraft(optimistic);
    setBlockErrorMessage("");
    setBlockStatusMessage("");

    void enqueueBlockSave(blockId, async () => {
      try {
        const response = await updateBlock(siteId, ownerPage.id, blockId, {
          props: nextProps,
          hidden,
        });
        applyDraftUpdate(response.draft, ownerPage.id, blockId);
      } catch (error) {
        if (isDraftConflictError(error)) {
          setBlockErrorMessage(
            await recoverDraftConflict({
              preferredPageID: ownerPage.id,
              preferredBlockID: blockId,
            }),
          );
          return;
        }
        setBlockErrorMessage(
          error instanceof APIError ? error.message : "Could not save edit",
        );
        await refreshDraftState(ownerPage.id, blockId).catch(() => null);
      }
    });
  }

  async function handleUpdateBindings(
    blockId: string,
    bindings: Record<string, BlockBinding>,
  ) {
    const currentDraft = draftRef.current;
    if (!currentDraft) return;
    const ownerPage = currentDraft.pages.find((page) =>
      page.blocks.some((block) => block.id === blockId),
    );
    if (!ownerPage) return;

    setBlockErrorMessage("");
    setBlockStatusMessage("");
    try {
      await enqueueBlockSave(blockId, async () => {
        const response = await updateBlock(siteId, ownerPage.id, blockId, {
          bindings,
        });
        applyDraftUpdate(response.draft, ownerPage.id, blockId);
      });
      setBlockStatusMessage("Bindings saved.");
    } catch (error) {
      if (isDraftConflictError(error)) {
        const message = await recoverDraftConflict({
          preferredPageID: ownerPage.id,
          preferredBlockID: blockId,
        });
        setBlockErrorMessage(message);
        throw new Error(message, { cause: error });
      }
      const message =
        error instanceof APIError ? error.message : "Could not save bindings";
      setBlockErrorMessage(message);
      throw error instanceof Error ? error : new Error(message);
    }
  }

  async function handleToggleHidden(blockId: string, hidden: boolean) {
    const currentDraft = draftRef.current;
    if (!currentDraft) return;
    const ownerPage = currentDraft.pages.find((page) =>
      page.blocks.some((block) => block.id === blockId),
    );
    const block = ownerPage?.blocks.find((b) => b.id === blockId);
    if (!ownerPage || !block) return;

    setIsMutatingBlocks(true);
    setBlockErrorMessage("");
    setBlockStatusMessage("");
    try {
      await enqueueBlockSave(blockId, async () => {
        const response = await updateBlock(siteId, ownerPage.id, blockId, {
          props: block.props as Record<string, unknown>,
          hidden,
        });
        applyDraftUpdate(response.draft, ownerPage.id, blockId);
      });
      setBlockStatusMessage(hidden ? "Block hidden." : "Block unhidden.");
    } catch (error) {
      if (isDraftConflictError(error)) {
        setBlockErrorMessage(
          await recoverDraftConflict({
            preferredPageID: ownerPage.id,
            preferredBlockID: blockId,
          }),
        );
        return;
      }
      setBlockErrorMessage(
        error instanceof APIError
          ? error.message
          : "Could not change block visibility",
      );
    } finally {
      setIsMutatingBlocks(false);
    }
  }

  async function handleAddBlock(input: {
    blockType: string;
    targetIndex: number;
    initialProps?: Record<string, unknown>;
  }) {
    if (!selectedPage) return;
    setIsCreatingBlock(true);
    setBlockErrorMessage("");
    setBlockStatusMessage("");
    try {
      const createResponse = await createBlock(siteId, selectedPage.id, {
        type: input.blockType,
      });

      const previousPage =
        draft?.pages.find((p) => p.id === selectedPage.id) ?? null;
      let nextDraft = createResponse.draft;
      let nextPage =
        nextDraft.pages.find((p) => p.id === selectedPage.id) ?? null;
      const createdBlock = findNewBlock(previousPage, nextPage);

      if (!nextPage || !createdBlock) {
        applyDraftUpdate(nextDraft, selectedPage.id);
        setBlockStatusMessage("Block added.");
        return;
      }

      // Patch initial props (e.g. picked variant) before reordering.
      if (input.initialProps) {
        const mergedProps = {
          ...(createdBlock.props as Record<string, unknown>),
          ...input.initialProps,
        };
        const patched = await updateBlock(
          siteId,
          selectedPage.id,
          createdBlock.id,
          {
            props: mergedProps,
            hidden: Boolean(createdBlock.settings?.hidden),
          },
        );
        nextDraft = patched.draft;
        nextPage =
          nextDraft.pages.find((p) => p.id === selectedPage.id) ?? nextPage;
      }

      const visibleBlocks = (nextPage?.blocks ?? []).filter(
        (block) => !block.settings?.hidden,
      );
      const visibleOrder = visibleBlocks.map((block) => block.id);
      const createdVisibleIndex = visibleOrder.findIndex(
        (id) => id === createdBlock.id,
      );

      if (createdVisibleIndex !== -1) {
        const reorderedVisible = [...visibleOrder];
        reorderedVisible.splice(createdVisibleIndex, 1);
        reorderedVisible.splice(
          Math.max(0, Math.min(input.targetIndex, reorderedVisible.length)),
          0,
          createdBlock.id,
        );
        const hiddenIDs = (nextPage?.blocks ?? [])
          .filter((block) => block.settings?.hidden)
          .map((block) => block.id);
        const reorderResponse = await reorderBlocks(siteId, selectedPage.id, [
          ...reorderedVisible,
          ...hiddenIDs,
        ]);
        nextDraft = reorderResponse.draft;
      }

      applyDraftUpdate(nextDraft, selectedPage.id, createdBlock.id);
      setBlockStatusMessage("Block added.");
    } catch (error) {
      if (isDraftConflictError(error)) {
        setBlockErrorMessage(
          await recoverDraftConflict({
            preferredPageID: selectedPage?.id,
            preferredBlockID: selectedBlock?.id,
          }),
        );
        return;
      }
      setBlockErrorMessage(
        error instanceof APIError ? error.message : "Could not add block",
      );
    } finally {
      setIsCreatingBlock(false);
    }
  }

  async function handleSuggestBlock(input: BlockSuggestInput) {
    if (!selectedPage || !selectedBlock) {
      return;
    }
    setIsSuggestingBlock(true);
    setSuggestErrorMessage("");
    setSuggestStatusMessage("");
    clearBlockedAction();
    try {
      const response = await suggestBlock(siteId, selectedBlock.id, input);
      applyDraftUpdate(response.draft, selectedPage.id, selectedBlock.id);
      const historyResponse = await listRepromptHistory(siteId).catch(
        () => null,
      );
      if (historyResponse) {
        setRepromptHistory(historyResponse.reprompts);
      }
      setSuggestStatusMessage(suggestionToastForInput(input));
    } catch (error) {
      if (isDraftConflictError(error)) {
        setSuggestErrorMessage(
          await recoverDraftConflict({
            preferredPageID: selectedPage?.id,
            preferredBlockID: selectedBlock?.id,
            includeReprompts: true,
          }),
        );
        return;
      }
      setBlockedActionFromError(error);
      setSuggestErrorMessage(
        error instanceof APIError ? error.message : "Could not rewrite block",
      );
    } finally {
      setIsSuggestingBlock(false);
    }
  }

  async function handleImageApplied(response: ImageApplyResponse) {
    applyDraftUpdate(response.draft, selectedPage?.id, selectedBlock?.id);
    if (response.asset) {
      setSiteAssets((current) => {
        if (current.some((asset) => asset.id === response.asset?.id)) {
          return current;
        }
        return [response.asset as AssetRecord, ...current];
      });
    } else {
      try {
        const assetResponse = await listSiteAssets(siteId);
        setSiteAssets(assetResponse.assets);
      } catch {
        // best-effort refresh
      }
    }
    const historyResponse = await listRepromptHistory(siteId).catch(() => null);
    if (historyResponse) {
      setRepromptHistory(historyResponse.reprompts);
    }
  }

  async function handleUploadAsset(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    if (!assetFile) {
      setAssetErrorMessage("Choose an image file before uploading.");
      setAssetStatusMessage("");
      return;
    }

    setIsUploadingAsset(true);
    setAssetErrorMessage("");
    setAssetStatusMessage("");
    clearBlockedAction();

    try {
      const ticket = await createAssetUploadURL({
        siteId,
        fileName: assetFile.name,
        contentType: assetFile.type,
        sizeBytes: assetFile.size,
        altText: assetAltText || undefined,
      });

      const uploadHeaders = new Headers(ticket.upload.headers ?? {});
      if (!uploadHeaders.has("Content-Type") && assetFile.type) {
        uploadHeaders.set("Content-Type", assetFile.type);
      }

      const uploadResponse = await fetch(ticket.upload.url, {
        method: ticket.upload.method || "PUT",
        headers: uploadHeaders,
        body: assetFile,
      });
      if (!uploadResponse.ok) {
        throw new Error(
          `Storage upload failed with status ${uploadResponse.status}`,
        );
      }

      const dimensions = await readImageDimensions(assetFile).catch(() => null);
      const completed = await completeAssetUpload(ticket.asset.id, {
        altText: assetAltText || undefined,
        width: dimensions?.width,
        height: dimensions?.height,
      });

      setSiteAssets((current) => [
        completed.asset,
        ...current.filter((asset) => asset.id !== completed.asset.id),
      ]);
      setAssetFile(null);
      setAssetAltText("");
      setAssetInputKey((current) => current + 1);
      setAssetStatusMessage("Asset uploaded and ready for block fields.");
    } catch (error) {
      setBlockedActionFromError(error);
      setAssetErrorMessage(
        error instanceof APIError
          ? error.message
          : error instanceof Error
            ? error.message
            : "Could not upload asset",
      );
    } finally {
      setIsUploadingAsset(false);
    }
  }

  function handleThemeSelectionChange(
    field: keyof ThemeSelection,
    value: string,
  ) {
    if (!themeSelection || !themeOptions) {
      return;
    }
    const nextSelection = {
      ...themeSelection,
      [field]: value,
    };
    setThemeSelection(nextSelection);
    setDraft((current) =>
      current
        ? {
            ...current,
            theme: buildSiteThemeFromSelection(
              current.theme,
              nextSelection,
              themeOptions,
            ),
          }
        : current,
    );
    setThemeErrorMessage("");
    setThemeStatusMessage(
      "Unsaved theme changes are shown in the live preview.",
    );
  }

  function handleResetThemeSelection() {
    if (!savedThemeSelection || !savedTheme) {
      return;
    }
    setThemeSelection(savedThemeSelection);
    setDraft((current) =>
      current
        ? {
            ...current,
            theme: savedTheme,
          }
        : current,
    );
    setThemeErrorMessage("");
    setThemeStatusMessage("Theme changes reset to the saved version.");
  }

  async function handleSaveTheme(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    if (!themeSelection) {
      return;
    }

    setIsSavingTheme(true);
    setThemeErrorMessage("");
    setThemeStatusMessage("");

    try {
      const response = await updateSiteTheme(siteId, themeSelection);
      setThemeSelection(response.selection);
      setSavedThemeSelection(response.selection);
      setSavedTheme(response.theme);
      setThemeOptions(response.options);
      setDraft((current) =>
        current
          ? {
              ...current,
              theme: response.theme,
            }
          : current,
      );
      setThemeStatusMessage("Theme saved for preview and publish.");
    } catch (error) {
      if (isDraftConflictError(error)) {
        setThemeErrorMessage(
          await recoverDraftConflict({
            preferredPageID: selectedPage?.id,
            preferredBlockID: selectedBlock?.id,
            includeTheme: true,
          }),
        );
        return;
      }
      setThemeErrorMessage(
        error instanceof APIError ? error.message : "Could not save theme",
      );
    } finally {
      setIsSavingTheme(false);
    }
  }

  async function handleRegenerateTheme() {
    setIsRegeneratingTheme(true);
    setThemeErrorMessage("");
    setThemeStatusMessage("");
    setThemeProgressStep("");
    setThemeProgressStepTotal(0);
    clearBlockedAction();

    try {
      const response = await streamRegenerateSiteTheme(siteId, {
        onProgress: (step) => {
          setThemeProgressStep(step.step);
          setThemeProgressStepTotal(step.total);
        },
      });
      setThemeSelection(response.selection);
      setSavedThemeSelection(response.selection);
      setSavedTheme(response.theme);
      setThemeOptions(response.options);
      setDraft((current) =>
        current
          ? {
              ...current,
              theme: response.theme,
            }
          : current,
      );
      setThemeStatusMessage("Theme regenerated from the site brief.");
    } catch (error) {
      if (isDraftConflictError(error)) {
        setThemeErrorMessage(
          await recoverDraftConflict({
            preferredPageID: selectedPage?.id,
            preferredBlockID: selectedBlock?.id,
            includeTheme: true,
          }),
        );
        return;
      }
      setThemeErrorMessage(
        error instanceof APIError
          ? error.message
          : "Could not regenerate theme",
      );
      setThemeProgressStep("");
      setThemeProgressStepTotal(0);
    } finally {
      setIsRegeneratingTheme(false);
    }
  }

  async function handleUpdateSubmissionStatus(
    submissionId: string,
    status: FormSubmissionStatus,
  ) {
    setActiveSubmissionId(submissionId);
    setSubmissionErrorMessage("");
    setSubmissionStatusMessage("");

    try {
      const response = await updateFormSubmission(submissionId, { status });
      setFormSubmissions((current) =>
        current.map((submission) =>
          submission.id === submissionId ? response.submission : submission,
        ),
      );
      setSubmissionStatusMessage("Submission status saved.");
    } catch (error) {
      setSubmissionErrorMessage(
        error instanceof APIError
          ? error.message
          : "Could not update submission status",
      );
    } finally {
      setActiveSubmissionId("");
    }
  }

  async function handleDuplicateBlock() {
    if (!selectedPage || !selectedBlock) {
      return;
    }

    setIsMutatingBlocks(true);
    setBlockErrorMessage("");
    setBlockStatusMessage("");

    try {
      const response = await duplicateBlock(
        siteId,
        selectedPage.id,
        selectedBlock.id,
      );
      const duplicatedBlock = findNewBlock(
        draft?.pages.find((page) => page.id === selectedPage.id) ?? null,
        response.draft.pages.find((page) => page.id === selectedPage.id) ??
          null,
      );
      applyDraftUpdate(response.draft, selectedPage.id, duplicatedBlock?.id);
      setBlockStatusMessage("Block duplicated.");
    } catch (error) {
      if (isDraftConflictError(error)) {
        setBlockErrorMessage(
          await recoverDraftConflict({
            preferredPageID: selectedPage.id,
            preferredBlockID: selectedBlock.id,
          }),
        );
        return;
      }
      setBlockErrorMessage(
        error instanceof APIError ? error.message : "Could not duplicate block",
      );
    } finally {
      setIsMutatingBlocks(false);
    }
  }

  async function handleDeleteBlock() {
    if (!selectedPage || !selectedBlock) {
      return;
    }
    const confirmed = window.confirm(
      `Delete the ${selectedDefinition?.displayName ?? selectedBlock.type} block?`,
    );
    if (!confirmed) {
      return;
    }

    setIsMutatingBlocks(true);
    setBlockErrorMessage("");
    setBlockStatusMessage("");

    try {
      const response = await deleteBlock(
        siteId,
        selectedPage.id,
        selectedBlock.id,
      );
      applyDraftUpdate(response.draft, selectedPage.id);
      setBlockStatusMessage("Block removed from the page.");
    } catch (error) {
      if (isDraftConflictError(error)) {
        setBlockErrorMessage(
          await recoverDraftConflict({
            preferredPageID: selectedPage.id,
          }),
        );
        return;
      }
      setBlockErrorMessage(
        error instanceof APIError ? error.message : "Could not delete block",
      );
    } finally {
      setIsMutatingBlocks(false);
    }
  }

  async function handleMoveBlock(direction: -1 | 1) {
    if (!draft || !selectedPage || !selectedBlock) {
      return;
    }
    const page = draft.pages.find(
      (candidate) => candidate.id === selectedPage.id,
    );
    if (!page) {
      return;
    }
    const nextOrder = moveItem(page.blocks, selectedBlock.id, direction);
    if (!nextOrder) {
      return;
    }

    setIsMutatingBlocks(true);
    setBlockErrorMessage("");
    setBlockStatusMessage("");

    try {
      const response = await reorderBlocks(
        siteId,
        selectedPage.id,
        nextOrder.map((block) => block.id),
      );
      applyDraftUpdate(response.draft, selectedPage.id, selectedBlock.id);
      setBlockStatusMessage("Block order updated.");
    } catch (error) {
      if (isDraftConflictError(error)) {
        setBlockErrorMessage(
          await recoverDraftConflict({
            preferredPageID: selectedPage.id,
            preferredBlockID: selectedBlock.id,
          }),
        );
        return;
      }
      setBlockErrorMessage(
        error instanceof APIError ? error.message : "Could not reorder blocks",
      );
    } finally {
      setIsMutatingBlocks(false);
    }
  }

  async function handleReorderBlocks(blockIds: string[]) {
    if (!selectedPage) return;
    setIsMutatingBlocks(true);
    setBlockErrorMessage("");
    setBlockStatusMessage("");

    try {
      const response = await reorderBlocks(siteId, selectedPage.id, blockIds);
      applyDraftUpdate(response.draft, selectedPage.id, selectedBlock?.id);
      setBlockStatusMessage("Blocks reordered.");
    } catch (error) {
      if (isDraftConflictError(error)) {
        setBlockErrorMessage(
          await recoverDraftConflict({
            preferredPageID: selectedPage.id,
            preferredBlockID: selectedBlock?.id,
          }),
        );
        return;
      }
      setBlockErrorMessage(
        error instanceof APIError ? error.message : "Could not reorder blocks",
      );
    } finally {
      setIsMutatingBlocks(false);
    }
  }

  async function handleApplyRefinement(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    const trimmedPrompt = refinementPrompt.trim();
    if (!trimmedPrompt) {
      return;
    }
    if (refinementScope === "page" && !selectedPage) {
      setRepromptErrorMessage("Pick a page before refining the current page.");
      return;
    }

    setRepromptProgressStep("");
    setRepromptProgressStepTotal(0);
    setRepromptProgressScope(refinementScope);
    setRepromptErrorMessage("");
    setRepromptStatusMessage("");
    clearBlockedAction();

    try {
      if (refinementScope === "site") {
        setIsRepromptingSite(true);
        await streamRepromptSite(
          siteId,
          { prompt: trimmedPrompt },
          {
            onProgress: (step) => {
              setRepromptProgressStep(step.step);
              setRepromptProgressStepTotal(step.total);
            },
          },
        );
        await Promise.all([refreshDraftState(), refreshRepromptHistoryState()]);
        setRepromptHistoryScope("site");
        setRepromptStatusMessage(
          "Site refined. Review the diff or restore the earlier checkpoint from history.",
        );
      } else if (refinementScope === "page" && selectedPage) {
        setIsRepromptingPage(true);
        await streamRepromptPage(
          siteId,
          selectedPage.id,
          {
            prompt: trimmedPrompt,
          },
          {
            onProgress: (step) => {
              setRepromptProgressStep(step.step);
              setRepromptProgressStepTotal(step.total);
            },
          },
        );
        await Promise.all([
          refreshDraftState(selectedPage.id, selectedBlock?.id),
          refreshRepromptHistoryState(),
        ]);
        setRepromptHistoryScope("page");
        setRepromptStatusMessage(
          `${selectedPage.title} was refined. Review the diff or restore the earlier checkpoint from history.`,
        );
      }
      setRefinementPrompt("");
    } catch (error) {
      if (isDraftConflictError(error)) {
        setRepromptErrorMessage(
          await recoverDraftConflict({
            preferredPageID: selectedPage?.id,
            preferredBlockID: selectedBlock?.id,
            includeReprompts: true,
          }),
        );
        return;
      }
      setBlockedActionFromError(error);
      setRepromptErrorMessage(
        error instanceof APIError
          ? error.message
          : "Could not apply refinement",
      );
    } finally {
      setIsRepromptingSite(false);
      setIsRepromptingPage(false);
      setRepromptProgressStep("");
      setRepromptProgressStepTotal(0);
      setRepromptProgressScope("");
    }
  }

  async function handleUndoReprompt() {
    setIsUndoingReprompt(true);
    setRepromptErrorMessage("");
    setRepromptStatusMessage("");

    try {
      const latestReprompt = repromptHistory.find(
        (entry) => !entry.undoneAt,
      );
      if (!latestReprompt) {
        setRepromptErrorMessage("There is no AI checkpoint to restore.");
        return;
      }
      setActiveRepromptHistoryId(latestReprompt.id);
      await revertReprompt(siteId, latestReprompt.id);
      await Promise.all([
        refreshDraftState(selectedPage?.id, selectedBlock?.id),
        refreshRepromptHistoryState(),
      ]);
      setRepromptStatusMessage("Previous AI checkpoint restored.");
    } catch (error) {
      if (isDraftConflictError(error)) {
        setRepromptErrorMessage(
          await recoverDraftConflict({
            preferredPageID: selectedPage?.id,
            preferredBlockID: selectedBlock?.id,
            includeReprompts: true,
          }),
        );
        return;
      }
      setRepromptErrorMessage(
        error instanceof APIError
          ? error.message
          : "Could not restore draft revision",
      );
    } finally {
      setActiveRepromptHistoryId("");
      setIsUndoingReprompt(false);
    }
  }

  async function handleShowRepromptDiff(reprompt: RepromptHistoryRecord) {
    repromptDiffOpenerRef.current =
      document.activeElement instanceof HTMLElement
        ? document.activeElement
        : null;
    setActiveRepromptDiff(reprompt);
    setActiveRepromptHistoryId(reprompt.id);
    setRepromptDiffPrevious(null);
    setRepromptDiffResult(null);
    setRepromptDiffErrorMessage("");
    setIsLoadingRepromptDiff(true);

    try {
      const [previousResponse, resultResponse] = await Promise.all([
        getDraftRevision(siteId, reprompt.previousRevisionId),
        getDraftRevision(siteId, reprompt.resultRevisionId),
      ]);
      setRepromptDiffPrevious(previousResponse.revision);
      setRepromptDiffResult(resultResponse.revision);
    } catch (error) {
      setRepromptDiffErrorMessage(
        error instanceof APIError
          ? error.message
          : "Could not load revision diff",
      );
    } finally {
      setIsLoadingRepromptDiff(false);
      setActiveRepromptHistoryId("");
    }
  }

  async function handleRevertReprompt(reprompt: RepromptHistoryRecord) {
    setIsUndoingReprompt(true);
    setActiveRepromptHistoryId(reprompt.id);
    setRepromptErrorMessage("");
    setRepromptStatusMessage("");

    try {
      await revertReprompt(siteId, reprompt.id);
      await Promise.all([
        refreshDraftState(selectedPage?.id, selectedBlock?.id),
        refreshRepromptHistoryState(),
      ]);
      setRepromptStatusMessage("Draft restored from history.");
    } catch (error) {
      if (isDraftConflictError(error)) {
        setRepromptErrorMessage(
          await recoverDraftConflict({
            preferredPageID: selectedPage?.id,
            preferredBlockID: selectedBlock?.id,
            includeReprompts: true,
          }),
        );
        return;
      }
      setRepromptErrorMessage(
        error instanceof APIError
          ? error.message
          : "Could not restore draft revision",
      );
    } finally {
      setActiveRepromptHistoryId("");
      setIsUndoingReprompt(false);
    }
  }

  function handleSelectPage(pageId: string) {
    const page = draft?.pages.find((p) => p.id === pageId) ?? null;
    if (!page || !draft) return;
    setSelectedPageId(pageId);
    setSelectedBlockId(page.blocks[0]?.id ?? "");
    syncSelectedPageFields(draft, page);
    setBlockErrorMessage("");
    setBlockStatusMessage("");
    setPageErrorMessage("");
    setPageStatusMessage("");
    setRepromptErrorMessage("");
  }

  function handleSelectBlock(blockId: string) {
    if (!draft || !selectedPage) return;
    setSelectedBlockId(blockId);
    setBlockErrorMessage("");
    setBlockStatusMessage("");
  }

  async function handleDelete() {
    const confirmed = window.confirm(
      "Delete this site draft? This removes the stored draft and its pages.",
    );
    if (!confirmed) {
      return;
    }

    setIsDeleting(true);
    setSiteErrorMessage("");

    try {
      await deleteSite(siteId);
      await navigate({ to: "/app" });
    } catch (error) {
      setSiteErrorMessage(
        error instanceof APIError ? error.message : "Could not delete site",
      );
      setIsDeleting(false);
    }
  }

  async function handleCreateDomain(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    setIsMutatingDomain(true);
    setActiveDomainId("");
    setDomainErrorMessage("");
    setDomainStatusMessage("");
    clearBlockedAction();

    try {
      const response = await createSiteDomain(siteId, {
        hostname: newCustomDomainHostname,
      });
      setDomainState(response);
      setNewCustomDomainHostname("");
      setDomainStatusMessage(
        "Custom domain added. Publish once if needed, then verify the DNS TXT record.",
      );
    } catch (error) {
      setBlockedActionFromError(error);
      setDomainErrorMessage(
        error instanceof APIError ? error.message : "Could not add domain",
      );
    } finally {
      setIsMutatingDomain(false);
    }
  }

  async function handleVerifyDomain(domainId: string) {
    setIsMutatingDomain(true);
    setActiveDomainId(domainId);
    setDomainErrorMessage("");
    setDomainStatusMessage("");
    clearBlockedAction();

    try {
      const response = await verifySiteDomain(siteId, domainId);
      setDomainState(response);
      setDomainStatusMessage("Domain verified and ready for live traffic.");
    } catch (error) {
      setBlockedActionFromError(error);
      setDomainErrorMessage(
        error instanceof APIError ? error.message : "Could not verify domain",
      );
    } finally {
      setIsMutatingDomain(false);
      setActiveDomainId("");
    }
  }

  async function handleDeleteDomain(domainId: string, hostname: string) {
    const confirmed = window.confirm(
      `Remove ${hostname} from this site? The hosted Snaelda URL will keep working.`,
    );
    if (!confirmed) {
      return;
    }

    setIsMutatingDomain(true);
    setActiveDomainId(domainId);
    setDomainErrorMessage("");
    setDomainStatusMessage("");
    clearBlockedAction();

    try {
      const response = await deleteSiteDomain(siteId, domainId);
      setDomainState(response);
      setDomainStatusMessage("Custom domain removed.");
    } catch (error) {
      setBlockedActionFromError(error);
      setDomainErrorMessage(
        error instanceof APIError ? error.message : "Could not remove domain",
      );
    } finally {
      setIsMutatingDomain(false);
      setActiveDomainId("");
    }
  }

  async function handlePublish() {
    setIsPublishing(true);
    setPublishErrorMessage("");
    setPublishValidationIssues([]);
    setPublishStatusMessage("");
    clearBlockedAction();

    try {
      const response = await publishSite(siteId, { publishNote });
      const versionResponse = await listSiteVersions(siteId);
      const domainResponse = await getSiteDomains(siteId);
      const nextBillingState = await getBillingState();
      setVersions(versionResponse.versions);
      setDomainState(domainResponse);
      setBillingState(nextBillingState);
      setPublishStatusMessage(
        `Published version ${response.version.versionNumber} live at ${response.publicUrl}`,
      );
    } catch (error) {
      setBlockedActionFromError(error);
      setPublishErrorMessage(
        error instanceof APIError ? error.message : "Could not publish site",
      );
      if (error instanceof APIError && error.payload?.issues?.length) {
        setPublishValidationIssues(error.payload.issues);
      }
    } finally {
      setIsPublishing(false);
    }
  }

  async function handleRollback(version: SiteVersion) {
    if (version.isCurrent) {
      return;
    }
    const confirmed = window.confirm(
      `Roll the live site back to version ${version.versionNumber}? The current draft will stay editable.`,
    );
    if (!confirmed) {
      return;
    }

    setActiveRollbackVersionId(version.id);
    setPublishErrorMessage("");
    setPublishStatusMessage("");
    clearBlockedAction();

    try {
      const response = await rollbackSiteVersion(siteId, version.id);
      const versionResponse = await listSiteVersions(siteId);
      const domainResponse = await getSiteDomains(siteId);
      setVersions(versionResponse.versions);
      setDomainState(domainResponse);
      setPublishStatusMessage(
        `Rolled back live site to version ${response.version.versionNumber} at ${response.publicUrl}`,
      );
    } catch (error) {
      setPublishErrorMessage(
        error instanceof APIError ? error.message : "Could not roll back site",
      );
    } finally {
      setActiveRollbackVersionId("");
    }
  }

  if (isLoading) {
    return (
      <div className={ribbonPanel}>
        <p className={text.p}>Loading site...</p>
      </div>
    );
  }

  if (!draft) {
    return (
      <div className={ribbonPanel}>
        <p className={text.error}>{loadErrorMessage || "Site not found"}</p>
      </div>
    );
  }

  const currentVersion = versions.find((version) => version.isCurrent) ?? null;
  const livePublicUrl =
    domainState?.publicUrl ||
    (domainState?.hostedHostname
      ? `https://${domainState.hostedHostname}/`
      : "");
  const liveHostname = domainState?.hostedHostname ?? "";
  const customDomains =
    domainState?.domains.filter((domain) => domain.type === "custom") ?? [];
  const billingPlanLabel =
    billingState?.entitlement.plan === "pro"
      ? "Pro"
      : billingState?.entitlement.plan === "site"
        ? "Site"
        : "Trial";
  const uploadedSiteAssets = siteAssets.filter(
    (asset) => asset.metadata.uploadStatus === "uploaded",
  );
  const selectedBlockIndex =
    selectedPage && selectedBlock
      ? selectedPage.blocks.findIndex((block) => block.id === selectedBlock.id)
      : -1;
  const themeHasUnsavedChanges = Boolean(
    themeSelection &&
    savedThemeSelection &&
    !sameThemeSelection(themeSelection, savedThemeSelection),
  );
  const primaryNavigationPageIds = new Set(
    navigationDraft.primary
      .map((item) => item.pageId)
      .filter((id): id is string => Boolean(id)),
  );
  const navigationAvailablePages = draft.pages.filter(
    (page) => !primaryNavigationPageIds.has(page.id),
  );
  const navigationIsDirty = !sameNavigationDraft(
    navigationDraft,
    navigationItemsFromDraft(draft),
  );
  const hasContactForm = draft.pages.some((page) =>
    page.blocks.some((block) => block.type === "contact_form"),
  );
  const workspaceSection =
    "grid gap-4 border-t border-border pt-5 first:border-t-0 first:pt-0";
  const workspaceWideSection = cn(workspaceSection, "2xl:col-span-2");
  const workspaceRow =
    "flex items-center justify-between gap-3 rounded-[10px] border border-border bg-[color-mix(in_oklch,var(--surface-1)_42%,transparent)] px-4 py-3";
  const workspaceInset =
    "rounded-[10px] border border-border bg-[color-mix(in_oklch,var(--surface-1)_42%,transparent)] p-4";
  const billingPromptNotice = blockedActionMessage ? (
    <div className="rounded-[14px] border border-[color-mix(in_oklch,var(--thread-gold)_65%,var(--border))] bg-[color-mix(in_oklch,var(--thread-gold)_10%,var(--surface-1))] p-4">
      <p className="text-sm font-bold text-[var(--paper)]">Action blocked</p>
      <p className="mt-2 text-sm text-[var(--paper-muted)]">
        {blockedActionMessage}
      </p>
      <div className="mt-4 flex flex-wrap gap-3">
        {blockedActionMode === "billing" ? (
          <>
            <Button
              type="button"
              size="sm"
              onClick={handleStartUpgrade}
              disabled={isStartingUpgrade}
            >
              {isStartingUpgrade ? "Opening checkout..." : "Upgrade in Stripe"}
            </Button>
            <Button asChild type="button" size="sm" variant="outline">
              <Link to="/app/billing">See billing details</Link>
            </Button>
          </>
        ) : (
          <p className="text-sm font-semibold text-[var(--paper)]">
            Save your workspace in the trial banner above, then retry.
          </p>
        )}
      </div>
    </div>
  ) : null;

  const pagesPanelContent = (
    <div className="grid gap-4">
      <section className={workspaceSection}>
        <div className="grid gap-1">
          <p className={text.eyebrow}>Edit page</p>
          <h3 className="m-0 text-[1.1rem] font-extrabold leading-[1.05] text-[var(--paper)]">
            {selectedPage ? selectedPage.title : "No page selected"}
          </h3>
          <p className={cn(text.p, "text-sm")}>
            Change the title, URL slug, publish status, and navigation
            visibility for the selected page. Pick a different page from the
            header.
          </p>
        </div>
        {selectedPage ? (
          <form className={form.grid} onSubmit={handleSavePage}>
            <div className="grid gap-4 md:grid-cols-2">
              <div className={form.field}>
                <label htmlFor="pages-edit-title" className={text.label}>
                  Page title
                </label>
                <Input
                  id="pages-edit-title"
                  value={pageTitle}
                  onChange={(event) => setPageTitle(event.target.value)}
                  required
                />
              </div>
              <div className={form.field}>
                <label htmlFor="pages-edit-slug" className={text.label}>
                  Page path
                </label>
                <Input
                  id="pages-edit-slug"
                  value={pageSlug}
                  onChange={(event) => setPageSlug(event.target.value)}
                  required
                />
              </div>
              <div className={form.field}>
                <label htmlFor="pages-edit-status" className={text.label}>
                  Page status
                </label>
                <Select
                  id="pages-edit-status"
                  value={pageStatus}
                  onChange={(event) =>
                    setPageStatus(
                      event.target.value === "published"
                        ? "published"
                        : "draft",
                    )
                  }
                >
                  <option value="draft">Draft</option>
                  <option value="published">Published</option>
                </Select>
              </div>
              <div className={form.field}>
                <label htmlFor="pages-edit-type" className={text.label}>
                  Page type
                </label>
                <Input
                  id="pages-edit-type"
                  value={pageTypeLabel(selectedPage.type)}
                  readOnly
                  disabled
                />
                <p className={form.hint}>
                  Type is fixed after the page is created.
                </p>
              </div>
              {selectedPage.type === "collection_index" ||
              selectedPage.type === "collection_detail" ? (
                <div className={form.field}>
                  <label
                    htmlFor="pages-edit-collection"
                    className={text.label}
                  >
                    Bound collection
                  </label>
                  <Select
                    id="pages-edit-collection"
                    value={pageCollectionId}
                    onChange={(event) =>
                      setPageCollectionId(event.target.value)
                    }
                  >
                    <option value="">Select a collection</option>
                    {(draft.collections ?? []).map((collection) => (
                      <option key={collection.id} value={collection.id}>
                        {collection.pluralLabel}
                      </option>
                    ))}
                  </Select>
                </div>
              ) : null}
            </div>
            <label className={form.toggle}>
              <input
                type="checkbox"
                className="size-4 accent-[var(--thread-teal)]"
                checked={pageIncludeInNavigation}
                onChange={(event) =>
                  setPageIncludeInNavigation(event.target.checked)
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
              <Button
                type="button"
                size="sm"
                variant="outline"
                disabled={
                  isSavingPage ||
                  draft.pages.findIndex((p) => p.id === selectedPage.id) <= 0
                }
                onClick={() => handleMovePage(selectedPage.id, -1)}
              >
                Move earlier
              </Button>
              <Button
                type="button"
                size="sm"
                variant="outline"
                disabled={
                  isSavingPage ||
                  draft.pages.findIndex((p) => p.id === selectedPage.id) >=
                    draft.pages.length - 1
                }
                onClick={() => handleMovePage(selectedPage.id, 1)}
              >
                Move later
              </Button>
              <Button
                type="button"
                size="sm"
                variant="outline"
                disabled={isDeletingPage}
                onClick={handleDeletePage}
              >
                {isDeletingPage ? "Deleting page..." : "Delete page"}
              </Button>
            </div>
          </form>
        ) : (
          <div className={emptyState}>
            <p className={text.p}>
              No page selected. Pick a page from the header above, or add one
              below.
            </p>
          </div>
        )}
      </section>
      <section className={workspaceSection}>
        <div>
          <p className={text.eyebrow}>Pages</p>
          <h2 className="mt-1 text-[1.2rem] font-black leading-[1.02] text-[var(--paper)]">
            Add another page
          </h2>
        </div>

        <form className={form.grid} onSubmit={handleCreatePage}>
          <label htmlFor="new-page-title" className={text.label}>
            Page title
          </label>
          <Input
            id="new-page-title"
            value={newPageTitle}
            onChange={(event) => setNewPageTitle(event.target.value)}
            placeholder="Pricing"
            required
          />

          <label htmlFor="new-page-slug" className={text.label}>
            Page path
          </label>
          <Input
            id="new-page-slug"
            value={newPageSlug}
            onChange={(event) => setNewPageSlug(event.target.value)}
            placeholder="/pricing"
          />

          <label htmlFor="new-page-type" className={text.label}>
            Page type
          </label>
          <Select
            id="new-page-type"
            value={newPageType}
            onChange={(event) => {
              const value = event.target.value as PageType;
              setNewPageType(value);
              if (value === "static") {
                setNewPageCollectionId("");
              }
            }}
          >
            <option value="static">Static — hand-authored page</option>
            <option
              value="collection_index"
              disabled={(draft.collections ?? []).length === 0}
            >
              Collection index — list a collection
            </option>
            <option
              value="collection_detail"
              disabled={(draft.collections ?? []).length === 0}
            >
              Collection detail — one URL per entry
            </option>
          </Select>
          {(draft.collections ?? []).length === 0 ? (
            <p className={form.hint}>
              Create a collection first to enable collection page types.
            </p>
          ) : null}

          {newPageType !== "static" ? (
            <>
              <label
                htmlFor="new-page-collection"
                className={text.label}
              >
                Bound collection
              </label>
              <Select
                id="new-page-collection"
                value={newPageCollectionId}
                onChange={(event) =>
                  setNewPageCollectionId(event.target.value)
                }
                required
              >
                <option value="">Select a collection</option>
                {(draft.collections ?? []).map((collection) => (
                  <option key={collection.id} value={collection.id}>
                    {collection.pluralLabel}
                  </option>
                ))}
              </Select>
            </>
          ) : null}

          <label className={form.toggle}>
            <input
              type="checkbox"
              className="size-4 accent-[var(--thread-teal)]"
              checked={newPageIncludeInNavigation}
              onChange={(event) =>
                setNewPageIncludeInNavigation(event.target.checked)
              }
            />
            Include this page in the main navigation
          </label>

          <Button
            type="submit"
            size="sm"
            disabled={isCreatingPage || draft.pages.length >= 10}
          >
            {isCreatingPage ? "Adding page..." : "Add page"}
          </Button>

          <p className={form.hint}>
            {draft.pages.length >= 10
              ? "This draft already has the 10-page MVP limit."
              : `${draft.pages.length} of 10 pages currently in this draft.`}
          </p>
        </form>
      </section>
    </div>
  );

  const collectionsPanelContent = <CollectionsPanel siteId={siteId} />;

  const seoPanelContent = (
    <div className="grid gap-4">
      <section className={workspaceSection}>
        <div className="grid gap-1">
          <p className={text.eyebrow}>SEO</p>
          <h3 className="m-0 text-[1.1rem] font-extrabold leading-[1.05] text-[var(--paper)]">
            {selectedPage
              ? `Search snippet for ${selectedPage.title}`
              : "Pick a page"}
          </h3>
          <p className={cn(text.p, "text-sm")}>
            What this page shows up as in Google results and when shared. Empty
            fields fall back to the page title and the first text block.
          </p>
        </div>
        {selectedPage ? (
          <form className={form.grid} onSubmit={handleSavePage}>
            <div className={form.field}>
              <label htmlFor="seo-edit-title" className={text.label}>
                Search title
              </label>
              <Input
                id="seo-edit-title"
                value={pageSEOTitle}
                onChange={(event) => setPageSEOTitle(event.target.value)}
                placeholder="Leave blank to reuse the page title"
              />
            </div>
            <div className={form.field}>
              <label htmlFor="seo-edit-description" className={text.label}>
                Search description
              </label>
              <Textarea
                id="seo-edit-description"
                rows={4}
                value={pageSEODescription}
                onChange={(event) => setPageSEODescription(event.target.value)}
                placeholder="Summarize what someone should expect before they click."
              />
            </div>
            <div className="grid gap-2 rounded-[12px] border border-border bg-[color-mix(in_oklch,var(--surface-1)_42%,transparent)] p-4">
              <p className={text.label}>Snippet preview</p>
              <p className="m-0 break-words text-[var(--paper-muted)]">
                {`${draft.site.slug}.local${selectedPage.slug === "/" ? "" : selectedPage.slug}`}
              </p>
              <p className="m-0 truncate text-[1.05rem] font-extrabold text-[var(--paper)]">
                {pageSEOTitle.trim() || selectedPage.title}
              </p>
              <p className={cn(text.p, "m-0 text-sm")}>
                {pageSEODescription.trim() ||
                  "No description yet. A good description is around 150 characters and tells the reader exactly what they will find on this page."}
              </p>
            </div>
            {pageErrorMessage ? (
              <p className={text.error}>{pageErrorMessage}</p>
            ) : null}
            {pageStatusMessage ? (
              <p className={text.success}>{pageStatusMessage}</p>
            ) : null}
            <div className={actions.rowLarge}>
              <Button type="submit" disabled={isSavingPage}>
                {isSavingPage ? "Saving..." : "Save SEO for this page"}
              </Button>
            </div>
          </form>
        ) : (
          <div className={emptyState}>
            <p className={text.p}>
              Pick a page from the header to edit its search title and
              description.
            </p>
          </div>
        )}
      </section>
    </div>
  );

  const navigationPanelContent = (
    <div className="grid gap-4">
      <section className={workspaceSection}>
        <div>
          <p className={text.eyebrow}>Navigation</p>
          <h2 className="mt-1 text-[1.2rem] font-black leading-[1.02] text-[var(--paper)]">
            Edit the main menu
          </h2>
          <p className={cn(text.p, "mt-2 text-sm")}>
            Labels are independent of page titles, so you can name menu items
            anything you like. Add internal pages or external links, reorder
            them, then save the menu.
          </p>
        </div>

        {navigationDraft.primary.length > 0 ? (
          <div className="grid gap-3">
            {navigationDraft.primary.map((item, index) => {
              const page = item.pageId
                ? draft.pages.find((candidate) => candidate.id === item.pageId)
                : null;
              const subtitle = item.pageId
                ? page
                  ? `Internal page · ${page.slug}`
                  : "Internal page · missing"
                : `External link · ${item.href ?? ""}`;
              return (
                <article
                  key={`${item.pageId ?? "ext"}-${item.href ?? ""}-${index}`}
                  className={cn(workspaceRow, "items-start gap-4")}
                >
                  <div className="grid flex-1 gap-2">
                    <label
                      htmlFor={`nav-label-${index}`}
                      className={text.label}
                    >
                      Menu label
                    </label>
                    <Input
                      id={`nav-label-${index}`}
                      value={item.label}
                      onChange={(event) =>
                        updateNavigationDraftItem("primary", index, {
                          label: event.target.value,
                        })
                      }
                      maxLength={60}
                    />
                    {item.href !== undefined ? (
                      <div className="grid gap-2">
                        <label
                          htmlFor={`nav-href-${index}`}
                          className={text.label}
                        >
                          External URL
                        </label>
                        <Input
                          id={`nav-href-${index}`}
                          value={item.href}
                          onChange={(event) =>
                            updateNavigationDraftItem("primary", index, {
                              href: event.target.value,
                            })
                          }
                          placeholder="https://example.com"
                        />
                      </div>
                    ) : null}
                    <small className="text-[var(--paper-muted)]">
                      {subtitle}
                    </small>
                  </div>
                  <div className={cn(actions.row, "flex-col items-stretch")}>
                    <Button
                      type="button"
                      variant="plain"
                      className={actions.inlineLink}
                      disabled={index === 0 || isSavingNavigation}
                      onClick={() =>
                        moveNavigationDraftItem("primary", index, -1)
                      }
                    >
                      Move up
                    </Button>
                    <Button
                      type="button"
                      variant="plain"
                      className={actions.inlineLink}
                      disabled={
                        index === navigationDraft.primary.length - 1 ||
                        isSavingNavigation
                      }
                      onClick={() =>
                        moveNavigationDraftItem("primary", index, 1)
                      }
                    >
                      Move down
                    </Button>
                    <Button
                      type="button"
                      variant="plain"
                      className={actions.inlineLink}
                      disabled={isSavingNavigation}
                      onClick={() =>
                        removeNavigationDraftItem("primary", index)
                      }
                    >
                      Remove
                    </Button>
                  </div>
                </article>
              );
            })}
          </div>
        ) : (
          <div className={emptyState}>
            <p className={text.p}>
              The menu is empty. Add a page or external link below.
            </p>
          </div>
        )}

        <div className="grid gap-3">
          <p className={text.label}>Add an internal page</p>
          {navigationAvailablePages.length > 0 ? (
            <div className="flex flex-wrap gap-2">
              {navigationAvailablePages.map((page) => (
                <Button
                  key={page.id}
                  type="button"
                  variant="outline"
                  size="sm"
                  disabled={isSavingNavigation}
                  onClick={() => addNavigationPageReference("primary", page.id)}
                >
                  + {page.title}
                </Button>
              ))}
            </div>
          ) : (
            <p className={form.hint}>
              Every page is already in the menu. Create more pages below to add
              them.
            </p>
          )}
        </div>

        <form
          className={form.grid}
          onSubmit={(event) => addNavigationExternalLink("primary", event)}
        >
          <p className={text.label}>Add an external link</p>
          <div className="grid gap-2 md:grid-cols-[1fr_2fr_auto]">
            <Input
              value={primaryExternalLinkLabel}
              onChange={(event) =>
                setPrimaryExternalLinkLabel(event.target.value)
              }
              placeholder="Label"
              maxLength={60}
            />
            <Input
              value={primaryExternalLinkHref}
              onChange={(event) =>
                setPrimaryExternalLinkHref(event.target.value)
              }
              placeholder="https://example.com"
            />
            <Button
              type="submit"
              size="sm"
              variant="outline"
              disabled={
                isSavingNavigation ||
                primaryExternalLinkLabel.trim() === "" ||
                primaryExternalLinkHref.trim() === ""
              }
            >
              Add link
            </Button>
          </div>
        </form>

        <div className={workspaceInset}>
          <div className="grid gap-2">
            <p className={text.label}>Footer links</p>
            <p className={form.hint}>
              These links render in the footer block. They can reference pages
              or external URLs and are independent of the main menu.
            </p>
          </div>
          {navigationDraft.footer.length > 0 ? (
            <div className="mt-4 grid gap-3">
              {navigationDraft.footer.map((item, index) => {
                const page = item.pageId
                  ? draft.pages.find(
                      (candidate) => candidate.id === item.pageId,
                    )
                  : null;
                const subtitle = item.pageId
                  ? page
                    ? `Internal page · ${page.slug}`
                    : "Internal page · missing"
                  : `External link · ${item.href ?? ""}`;
                return (
                  <article
                    key={`footer-${item.pageId ?? "ext"}-${item.href ?? ""}-${index}`}
                    className={cn(workspaceRow, "items-start gap-4")}
                  >
                    <div className="grid flex-1 gap-2">
                      <label
                        htmlFor={`footer-nav-label-${index}`}
                        className={text.label}
                      >
                        Footer label
                      </label>
                      <Input
                        id={`footer-nav-label-${index}`}
                        value={item.label}
                        onChange={(event) =>
                          updateNavigationDraftItem("footer", index, {
                            label: event.target.value,
                          })
                        }
                        maxLength={60}
                      />
                      {item.href !== undefined ? (
                        <div className="grid gap-2">
                          <label
                            htmlFor={`footer-nav-href-${index}`}
                            className={text.label}
                          >
                            External URL
                          </label>
                          <Input
                            id={`footer-nav-href-${index}`}
                            value={item.href}
                            onChange={(event) =>
                              updateNavigationDraftItem("footer", index, {
                                href: event.target.value,
                              })
                            }
                            placeholder="https://example.com"
                          />
                        </div>
                      ) : null}
                      <small className="text-[var(--paper-muted)]">
                        {subtitle}
                      </small>
                    </div>
                    <div className={cn(actions.row, "flex-col items-stretch")}>
                      <Button
                        type="button"
                        variant="plain"
                        className={actions.inlineLink}
                        disabled={index === 0 || isSavingNavigation}
                        onClick={() =>
                          moveNavigationDraftItem("footer", index, -1)
                        }
                      >
                        Move up
                      </Button>
                      <Button
                        type="button"
                        variant="plain"
                        className={actions.inlineLink}
                        disabled={
                          index === navigationDraft.footer.length - 1 ||
                          isSavingNavigation
                        }
                        onClick={() =>
                          moveNavigationDraftItem("footer", index, 1)
                        }
                      >
                        Move down
                      </Button>
                      <Button
                        type="button"
                        variant="plain"
                        className={actions.inlineLink}
                        disabled={isSavingNavigation}
                        onClick={() =>
                          removeNavigationDraftItem("footer", index)
                        }
                      >
                        Remove
                      </Button>
                    </div>
                  </article>
                );
              })}
            </div>
          ) : (
            <div className="mt-4 rounded-[10px] border border-dashed border-border p-4">
              <p className={text.p}>The footer link list is empty.</p>
            </div>
          )}

          <div className="mt-4 grid gap-3">
            <p className={text.label}>Add a footer page link</p>
            <div className="flex flex-wrap gap-2">
              {draft.pages.map((page) => (
                <Button
                  key={`footer-page-${page.id}`}
                  type="button"
                  variant="outline"
                  size="sm"
                  disabled={isSavingNavigation}
                  onClick={() => addNavigationPageReference("footer", page.id)}
                >
                  + {page.title}
                </Button>
              ))}
            </div>
          </div>

          <form
            className={cn(form.grid, "mt-4")}
            onSubmit={(event) => addNavigationExternalLink("footer", event)}
          >
            <p className={text.label}>Add a footer external link</p>
            <div className="grid gap-2 md:grid-cols-[1fr_2fr_auto]">
              <Input
                value={footerExternalLinkLabel}
                onChange={(event) =>
                  setFooterExternalLinkLabel(event.target.value)
                }
                placeholder="Label"
                maxLength={60}
              />
              <Input
                value={footerExternalLinkHref}
                onChange={(event) =>
                  setFooterExternalLinkHref(event.target.value)
                }
                placeholder="https://example.com"
              />
              <Button
                type="submit"
                size="sm"
                variant="outline"
                disabled={
                  isSavingNavigation ||
                  footerExternalLinkLabel.trim() === "" ||
                  footerExternalLinkHref.trim() === ""
                }
              >
                Add link
              </Button>
            </div>
          </form>
        </div>

        <div className={cn(actions.row, "flex-wrap")}>
          <Button
            type="button"
            disabled={isSavingNavigation || !navigationIsDirty}
            onClick={handleSaveNavigation}
          >
            {isSavingNavigation ? "Saving..." : "Save navigation"}
          </Button>
          <Button
            type="button"
            variant="outline"
            size="sm"
            disabled={isSavingNavigation || !navigationIsDirty}
            onClick={handleResetNavigation}
          >
            Discard changes
          </Button>
        </div>

        {navigationErrorMessage ? (
          <p className={text.error}>{navigationErrorMessage}</p>
        ) : null}
        {navigationStatusMessage ? (
          <p className={text.success}>{navigationStatusMessage}</p>
        ) : null}
      </section>
    </div>
  );

  const assetsPanelContent = (
    <div className="grid gap-4">
      <section className={workspaceSection}>
        <div>
          <p className={text.eyebrow}>Assets</p>
          <h2 className="mt-1 text-[1.2rem] font-black leading-[1.02] text-[var(--paper)]">
            Upload the image library
          </h2>
        </div>

        <form className={form.grid} onSubmit={handleUploadAsset}>
          <label htmlFor="asset-file" className={text.label}>
            Image file
          </label>
          <Input
            key={assetInputKey}
            id="asset-file"
            type="file"
            accept="image/avif,image/gif,image/jpeg,image/png,image/webp"
            onChange={(event) => setAssetFile(event.target.files?.[0] ?? null)}
          />

          <label htmlFor="asset-alt-text" className={text.label}>
            Default alt text
          </label>
          <Input
            id="asset-alt-text"
            value={assetAltText}
            onChange={(event) => setAssetAltText(event.target.value)}
            placeholder="Describe what the image shows"
          />

          {assetErrorMessage ? (
            <p className={text.error}>{assetErrorMessage}</p>
          ) : null}
          {assetStatusMessage ? (
            <p className={text.success}>{assetStatusMessage}</p>
          ) : null}

          <Button type="submit" disabled={isUploadingAsset || !assetFile}>
            {isUploadingAsset ? "Uploading image..." : "Upload image"}
          </Button>
        </form>

        <div className="grid gap-3">
          {siteAssets.length > 0 ? (
            siteAssets.map((asset) => (
              <article
                key={asset.id}
                className={cn(workspaceInset, "grid gap-3")}
              >
                <div className="grid gap-3 sm:grid-cols-[120px_minmax(0,1fr)] sm:items-start">
                  {asset.metadata.uploadStatus === "uploaded" ? (
                    <img
                      src={buildDraftAssetURL(asset.id)}
                      alt={
                        asset.altText ||
                        asset.metadata.fileName ||
                        "Uploaded site asset"
                      }
                      className="h-[108px] w-full rounded-[10px] border border-border bg-[var(--surface-1)] object-cover"
                      loading="lazy"
                    />
                  ) : (
                    <div className="grid h-[108px] w-full place-items-center rounded-[10px] border border-dashed border-border bg-[var(--surface-1)] text-sm text-[var(--paper-muted)]">
                      Processing upload
                    </div>
                  )}
                  <div className="grid gap-1">
                    <strong className="text-[var(--paper)]">
                      {asset.metadata.fileName || asset.id}
                    </strong>
                    <small className="text-[var(--paper-muted)]">
                      {asset.metadata.contentType || "Image"} ·{" "}
                      {formatAssetFileSize(
                        asset.metadata.sizeBytes ||
                          asset.metadata.requestedSizeBytes,
                      )}
                    </small>
                    <small className="text-[var(--paper-muted)]">
                      {describeAssetDimensions(asset)} ·{" "}
                      {asset.metadata.uploadStatus || "pending"}
                    </small>
                    {asset.altText ? (
                      <p className="m-0 text-sm text-[var(--paper-muted)]">
                        Alt: {asset.altText}
                      </p>
                    ) : null}
                    {asset.metadata.provenance ? (
                      <p className="m-0 text-sm text-[var(--paper-muted)]">
                        Starter from{" "}
                        <span className="font-medium capitalize text-[var(--paper)]">
                          {asset.metadata.provenance.provider}
                        </span>
                        {asset.metadata.provenance.author ? (
                          <>
                            {" "}
                            · Photo by{" "}
                            {asset.metadata.provenance.authorUrl ? (
                              <a
                                href={asset.metadata.provenance.authorUrl}
                                target="_blank"
                                rel="noopener noreferrer"
                                className="underline"
                              >
                                {asset.metadata.provenance.author}
                              </a>
                            ) : (
                              asset.metadata.provenance.author
                            )}
                          </>
                        ) : null}
                      </p>
                    ) : null}
                  </div>
                </div>
              </article>
            ))
          ) : (
            <div className={emptyState}>
              <p className={text.p}>
                No site assets yet. Upload the first image here, then pick it in
                any asset-enabled block field.
              </p>
            </div>
          )}
        </div>
      </section>
    </div>
  );

  const inquiriesPanelContent = (
    <div className="grid gap-4">
      <section className={workspaceWideSection}>
        <div>
          <p className={text.eyebrow}>Inquiries</p>
          <h2 className="mt-1 text-[1.2rem] font-black leading-[1.02] text-[var(--paper)]">
            Review contact form submissions
          </h2>
        </div>

        {submissionErrorMessage ? (
          <p className={text.error}>{submissionErrorMessage}</p>
        ) : null}
        {submissionStatusMessage ? (
          <p className={text.success}>{submissionStatusMessage}</p>
        ) : null}

        <div className="grid gap-3">
          {formSubmissions.length > 0 ? (
            formSubmissions.map((submission) => (
              <article
                key={submission.id}
                className={cn(workspaceInset, "grid gap-3")}
              >
                <div className="flex items-start justify-between gap-3 max-sm:flex-col">
                  <div>
                    <strong className="block text-[var(--paper)]">
                      {String(
                        submission.payload["name"] ||
                          submission.payload["email"] ||
                          "New inquiry",
                      )}
                    </strong>
                    <small className="text-[var(--paper-muted)]">
                      {submission.pageTitle || "Stored submission"} ·{" "}
                      {formatTimestamp(submission.createdAt)}
                    </small>
                  </div>
                  <Select
                    value={submission.status}
                    disabled={activeSubmissionId === submission.id}
                    onChange={(event) =>
                      handleUpdateSubmissionStatus(
                        submission.id,
                        event.target.value as FormSubmissionStatus,
                      )
                    }
                  >
                    <option value="new">New</option>
                    <option value="reviewed">Reviewed</option>
                    <option value="resolved">Resolved</option>
                    <option value="spam">Spam</option>
                  </Select>
                </div>

                <div className="grid gap-2">
                  {Object.entries(submission.payload).map(([key, value]) => (
                    <div key={key} className="grid gap-1">
                      <strong className="text-sm uppercase tracking-[0.08em] text-[var(--paper-muted)]">
                        {formatSubmissionKey(key)}
                      </strong>
                      <p className="m-0 whitespace-pre-wrap text-[var(--paper)]">
                        {String(value)}
                      </p>
                    </div>
                  ))}
                </div>
              </article>
            ))
          ) : (
            <div className={emptyState}>
              <p className={text.p}>
                {hasContactForm
                  ? "No submissions yet. Published and preview contact forms will start listing messages here."
                  : "Add a contact form block to start collecting inquiries."}
              </p>
            </div>
          )}
        </div>
      </section>
    </div>
  );

  const isApplyingRefinement = isRepromptingSite || isRepromptingPage;
  const refinementScopeLabel =
    refinementScope === "page"
      ? selectedPage
        ? `${selectedPage.title} page`
        : "Current page"
      : "Whole site";
  const refinementScopeHelp =
    refinementScope === "page"
      ? "Refines only the selected page while keeping its route and place in the draft."
      : "Refines every page. Brand, slug, and checkpoint history are kept.";
  const refinementProgressSteps =
    repromptProgressScope === "page" ? pageRepromptSteps : siteRepromptSteps;

  const promptPanelContent = (
    <div className="grid gap-4">
      {billingPromptNotice}
      <section className={workspaceSection}>
        <div>
          <p className={text.eyebrow}>AI refine</p>
          <h2 className="mt-1 text-[1.2rem] font-black leading-[1.02] text-[var(--paper)]">
            Keep shaping this draft
          </h2>
          <p className={cn(text.p, "mt-2 text-sm")}>
            Say what should change next. Pick the scope first, then apply the
            refinement into the draft. Every page or site refinement keeps a
            checkpoint with a diff.
          </p>
        </div>

        <form
          className="grid gap-4 rounded-[14px] border border-[color-mix(in_oklch,var(--border)_78%,transparent)] bg-[color-mix(in_oklch,var(--surface-2)_88%,var(--thread-mauve))] p-4"
          onSubmit={handleApplyRefinement}
        >
          <div className="grid gap-3 lg:grid-cols-[minmax(180px,240px)_minmax(0,1fr)]">
            <label className={form.field}>
              <span className={text.label}>Scope</span>
              <Select
                value={refinementScope}
                onChange={(event) =>
                  setRefinementScope(event.target.value as RefinementScope)
                }
              >
                <option value="page">Current page</option>
                <option value="site">Whole site</option>
              </Select>
            </label>
            <div className="grid gap-1.5 rounded-[10px] border border-[color-mix(in_oklch,var(--border)_68%,transparent)] bg-[var(--surface-1)] px-3 py-2.5">
              <p className="text-sm font-bold text-[var(--paper)]">
                {refinementScopeLabel}
              </p>
              <p className="m-0 text-sm text-[var(--paper-muted)]">
                {refinementScopeHelp}
              </p>
            </div>
          </div>

          <div className={form.field}>
            <label htmlFor="refinement-prompt" className={text.label}>
              What should change next?
            </label>
            <Textarea
              id="refinement-prompt"
              rows={5}
              value={refinementPrompt}
              placeholder="Make this feel warmer and less corporate. Add a clearer booking path and tighten the copy."
              onChange={(event) => setRefinementPrompt(event.target.value)}
            />
          </div>

          <div className="flex flex-wrap gap-2">
            {refinementChips.map((chip) => (
              <Button
                key={chip.label}
                type="button"
                size="sm"
                variant="outline"
                onClick={() =>
                  setRefinementPrompt((current) =>
                    current.trim()
                      ? `${current.trim()} ${chip.prompt}`
                      : chip.prompt,
                  )
                }
              >
                {chip.label}
              </Button>
            ))}
          </div>

          <div className="flex flex-wrap items-center justify-between gap-3">
            <p className={cn(form.hint, "m-0")}>
              This is a refinement rail, not a chat transcript. The next prompt
              is applied to the current draft.
            </p>
            <Button
              type="submit"
              size="sm"
              disabled={
                isApplyingRefinement ||
                refinementPrompt.trim() === "" ||
                (refinementScope === "page" && !selectedPage)
              }
            >
              {isApplyingRefinement ? "Applying..." : "Apply refinement"}
            </Button>
          </div>
        </form>

        {isApplyingRefinement ? (
          <GenerationProgressCard
            eyebrow="AI refinement"
            title={
              repromptProgressScope === "page"
                ? "Refining the selected page..."
                : "Refining the site direction..."
            }
            description={
              repromptProgressScope === "page"
                ? "Snaelda is reshaping the selected page while keeping its route and draft position."
                : "Snaelda is reshaping the broader draft while keeping the current site identity."
            }
            prompt={refinementPrompt}
            steps={refinementProgressSteps}
            activeStep={repromptProgressStep}
            activeTotal={repromptProgressStepTotal}
            showSkeleton={
              repromptProgressStep === "plan.blocks" ||
              repromptProgressStep === "copy.write" ||
              repromptProgressStep === "validate.repair" ||
              repromptProgressStep === "persist"
            }
          />
        ) : null}

        {repromptErrorMessage ? (
          <p className={text.error}>{repromptErrorMessage}</p>
        ) : null}
        {repromptStatusMessage ? (
          <p className={text.success}>{repromptStatusMessage}</p>
        ) : null}
      </section>

      <section className={workspaceSection}>
        <div className="flex flex-wrap items-end justify-between gap-3">
          <div>
            <p className={text.eyebrow}>History</p>
            <h2 className="mt-1 text-[1.2rem] font-black leading-[1.02] text-[var(--paper)]">
              Recent AI changes
            </h2>
            <p className={cn(text.p, "mt-2 text-sm")}>
              Every AI change leaves a checkpoint. Compare the diff or restore
              an earlier draft from here.
            </p>
          </div>
          <Button
            type="button"
            size="sm"
            variant="outline"
            disabled={
              isUndoingReprompt || !repromptHistory.some((entry) => !entry.undoneAt)
            }
            onClick={handleUndoReprompt}
          >
            {isUndoingReprompt ? "Restoring..." : "Restore latest AI change"}
          </Button>
        </div>

        <RepromptHistoryPanel
          reprompts={repromptHistory}
          activeScope={resolvedRepromptHistoryScope}
          selectedPageId={selectedPage?.id}
          selectedPageTitle={selectedPage?.title}
          selectedBlockId={selectedBlock?.id}
          selectedBlockLabel={selectedBlockLabel}
          activeDiffId={
            isLoadingRepromptDiff ? activeRepromptHistoryId : undefined
          }
          activeRevertId={
            isUndoingReprompt ? activeRepromptHistoryId : undefined
          }
          onActiveScopeChange={setRepromptHistoryScope}
          onShowDiff={handleShowRepromptDiff}
          onRevert={handleRevertReprompt}
        />
      </section>

      <section className={workspaceSection}>
        <div>
          <p className={text.eyebrow}>Original brief</p>
          <h2 className="mt-1 text-[1.2rem] font-black leading-[1.02] text-[var(--paper)]">
            The prompt this draft came from
          </h2>
          <p className={cn(text.p, "mt-2 text-sm")}>
            Use it as the reference point before rewriting, or load it into the
            rebuild prompt and edit from there.
          </p>
        </div>

        {generationMetadata ? (
          <div className="grid gap-4 xl:grid-cols-[minmax(0,1.15fr)_minmax(0,0.85fr)]">
            <div className={form.field}>
              <label htmlFor="stored-generation-prompt" className={text.label}>
                Stored site prompt
              </label>
              <Textarea
                id="stored-generation-prompt"
                rows={6}
                value={generationMetadata.prompt}
                readOnly
              />
              <div className={actions.row}>
                <Button
                  type="button"
                  size="sm"
                  variant="outline"
                  disabled={!generationMetadata.prompt}
                  onClick={() => {
                    setRefinementScope("site");
                    setRefinementPrompt(generationMetadata.prompt);
                  }}
                >
                  Edit this prompt
                </Button>
              </div>
            </div>

            <div className="grid gap-4">
              <div className={workspaceInset}>
                <p className={text.label}>Theme preset</p>
                <p className="mt-2 text-lg font-black text-[var(--paper)]">
                  {generationMetadata.themePreset || "Not captured"}
                </p>
                {typeof generationMetadata.validationRetryCount === "number" ? (
                  <p className="mt-2 text-sm text-[var(--paper-muted)]">
                    Validation retries:{" "}
                    {generationMetadata.validationRetryCount}
                  </p>
                ) : null}
              </div>

              <div className={workspaceInset}>
                <p className={text.label}>Assets the prompt expected</p>
                {generationMetadata.assetsNeeded?.length ? (
                  <div className="mt-3 flex flex-wrap gap-2">
                    {generationMetadata.assetsNeeded.map((item) => (
                      <span
                        key={item}
                        className="rounded-full border border-border bg-[var(--surface-1)] px-3 py-1.5 text-sm text-[var(--paper)]"
                      >
                        {item}
                      </span>
                    ))}
                  </div>
                ) : (
                  <p className="mt-2 text-sm text-[var(--paper-muted)]">
                    No starter assets were recorded for this draft.
                  </p>
                )}
              </div>
            </div>
          </div>
        ) : (
          <div className={emptyState}>
            <p className={text.p}>
              No generation metadata was stored for this draft yet.
            </p>
          </div>
        )}

        <div className={workspaceInset}>
          <p className={text.label}>Generation assumptions</p>
          {generationMetadata?.assumptions?.length ? (
            <ul className="mt-3 grid gap-2 text-sm text-[var(--paper-muted)]">
              {generationMetadata.assumptions.map((item) => (
                <li key={item} className="list-disc pl-1 ml-5">
                  {item}
                </li>
              ))}
            </ul>
          ) : (
            <p className="mt-2 text-sm text-[var(--paper-muted)]">
              No assumptions were captured for this draft.
            </p>
          )}
        </div>
      </section>
    </div>
  );

  const settingsPanelContent = (
    <div className="grid gap-4">
      {billingPromptNotice}
      <section className={workspaceSection}>
        <div>
          <p className={text.eyebrow}>Identity</p>
          <h2 className="mt-1 text-[1.2rem] font-black leading-[1.02] text-[var(--paper)]">
            Rename and reslug the draft
          </h2>
          <p className={cn(text.p, "mt-2 text-sm")}>
            The slug is the path of the published URL. Brand business name and
            colors live in Theme &amp; brand.
          </p>
        </div>

        <form className={form.grid} onSubmit={handleSaveSite}>
          <label htmlFor="site-name" className={text.label}>
            Site name
          </label>
          <Input
            id="site-name"
            name="name"
            value={name}
            onChange={(event) => setName(event.target.value)}
            required
          />

          <label htmlFor="site-slug" className={text.label}>
            Site slug
          </label>
          <Input
            id="site-slug"
            name="slug"
            value={slug}
            onChange={(event) => setSlug(event.target.value)}
            required
          />

          {siteErrorMessage ? (
            <p className={text.error}>{siteErrorMessage}</p>
          ) : null}
          {siteStatusMessage ? (
            <p className={text.success}>{siteStatusMessage}</p>
          ) : null}

          <div className={actions.row}>
            <Button type="submit" disabled={isSavingSite}>
              {isSavingSite ? "Saving..." : "Save identity"}
            </Button>
          </div>
        </form>
      </section>

      <section
        className={cn(
          "grid gap-4 rounded-[12px] border border-[color-mix(in_oklch,oklch(58%_0.18_27)_55%,var(--border))] bg-[color-mix(in_oklch,oklch(58%_0.18_27)_8%,var(--surface-1))] p-5",
        )}
      >
        <div>
          <p className="text-[0.7rem] font-bold uppercase tracking-[0.12em] text-[oklch(72%_0.16_27)]">
            Danger zone
          </p>
          <h2 className="mt-1 text-[1.2rem] font-black leading-[1.02] text-[var(--paper)]">
            Delete this draft
          </h2>
          <p className={cn(text.p, "mt-2 text-sm")}>
            Removing the draft also removes its rebuild checkpoints. Published
            versions stay reachable until the live site is taken down
            separately. Type{" "}
            <strong className="font-extrabold text-[var(--paper)]">
              {slug || "the slug"}
            </strong>{" "}
            to confirm.
          </p>
        </div>

        <div className="grid gap-3 md:grid-cols-[minmax(0,1fr)_auto] md:items-end">
          <div className={form.field}>
            <label htmlFor="delete-confirm-slug" className={text.label}>
              Confirm slug
            </label>
            <Input
              id="delete-confirm-slug"
              value={deleteConfirmSlug}
              onChange={(event) => setDeleteConfirmSlug(event.target.value)}
              placeholder={slug}
              autoComplete="off"
            />
          </div>
          <Button
            type="button"
            variant="outline"
            disabled={
              isDeleting || !slug || deleteConfirmSlug.trim() !== slug.trim()
            }
            onClick={handleDelete}
            className="border-[color-mix(in_oklch,oklch(58%_0.18_27)_70%,var(--border))] text-[oklch(78%_0.16_27)] hover:border-[oklch(58%_0.18_27)] hover:bg-[color-mix(in_oklch,oklch(58%_0.18_27)_14%,var(--surface-1))] hover:text-[var(--paper)]"
          >
            {isDeleting ? "Deleting draft..." : "Delete draft"}
          </Button>
        </div>
      </section>
    </div>
  );

  const themePanelContent = (
    <div className="grid gap-4">
      <section className={workspaceSection}>
        <div className="flex flex-wrap items-start justify-between gap-4">
          <div>
            <p className={text.eyebrow}>Brand</p>
            <h2 className="mt-1 text-[1.2rem] font-black leading-[1.02] text-[var(--paper)]">
              Identity that visitors see
            </h2>
            <p className={cn(text.p, "mt-2 text-sm")}>
              Business name, primary color, and logo. The primary color seeds
              the palette below.
            </p>
          </div>
          <div className="flex items-center gap-3 rounded-[10px] border border-border bg-[var(--surface-1)] px-3 py-2">
            {brandLogoAssetId &&
            uploadedSiteAssets.find(
              (asset) => asset.id === brandLogoAssetId,
            ) ? (
              <img
                src={buildDraftAssetURL(brandLogoAssetId)}
                alt={brandLogoAlt || `${brandBusinessName || name} logo`}
                className="size-9 rounded-[8px] object-contain"
              />
            ) : (
              <span
                aria-hidden="true"
                className="grid size-9 place-items-center rounded-[8px] text-sm font-black text-[var(--paper)]"
                style={{
                  background: brandPrimaryColor || "var(--surface-2)",
                  color: brandPrimaryColor ? "#fff" : "var(--paper)",
                }}
              >
                {(brandBusinessName || name || "S")
                  .trim()
                  .charAt(0)
                  .toUpperCase()}
              </span>
            )}
            <div className="grid gap-0.5">
              <span className="text-sm font-bold text-[var(--paper)]">
                {brandBusinessName || name || "Brand preview"}
              </span>
              <span className="text-xs text-[var(--paper-muted)] tabular-nums">
                {brandPrimaryColor || "—"}
              </span>
            </div>
          </div>
        </div>

        <form className={form.grid} onSubmit={handleSaveBrand}>
          <div className="grid gap-4 md:grid-cols-2">
            <div className={form.field}>
              <label htmlFor="site-brand-name" className={text.label}>
                Business name
              </label>
              <Input
                id="site-brand-name"
                value={brandBusinessName}
                onChange={(event) => setBrandBusinessName(event.target.value)}
                placeholder="What customers should see in the header and footer"
                required
              />
              <p className={form.hint}>
                Public-facing. Shows in the header, footer, and meta tags.
              </p>
            </div>
            <div className={form.field}>
              <label htmlFor="site-brand-color" className={text.label}>
                Primary color
              </label>
              <div className="flex items-center gap-3">
                <Input
                  id="site-brand-color"
                  type="color"
                  value={brandPrimaryColor || "#f4a261"}
                  onChange={(event) => setBrandPrimaryColor(event.target.value)}
                  className="h-11 w-16 rounded-[10px] p-1"
                />
                <Input
                  value={brandPrimaryColor}
                  onChange={(event) => setBrandPrimaryColor(event.target.value)}
                  placeholder="#F4A261"
                  required
                />
              </div>
              <p className={form.hint}>
                Seeds the palette below. The buttons and links inherit it.
              </p>
            </div>
          </div>

          <div className={workspaceInset}>
            <div className="grid gap-4 md:grid-cols-[minmax(0,1fr)_minmax(0,1fr)]">
              <label className={form.field}>
                <span className={text.label}>Logo</span>
                <Select
                  value={brandLogoAssetId}
                  onChange={(event) => {
                    const nextAssetId = event.target.value;
                    setBrandLogoAssetId(nextAssetId);
                    if (!nextAssetId) {
                      setBrandLogoAlt("");
                      return;
                    }
                    const nextAsset = uploadedSiteAssets.find(
                      (asset) => asset.id === nextAssetId,
                    );
                    if (!brandLogoAlt.trim()) {
                      setBrandLogoAlt(
                        nextAsset?.altText ||
                          `${brandBusinessName.trim() || name.trim()} logo`,
                      );
                    }
                  }}
                >
                  <option value="">No logo selected</option>
                  {uploadedSiteAssets.map((asset) => (
                    <option key={asset.id} value={asset.id}>
                      {asset.metadata.fileName || asset.id}
                    </option>
                  ))}
                </Select>
              </label>
              <label className={form.field}>
                <span className={text.label}>Logo alt text</span>
                <Input
                  value={brandLogoAlt}
                  onChange={(event) => setBrandLogoAlt(event.target.value)}
                  placeholder="Describe the logo for screen readers"
                  disabled={!brandLogoAssetId}
                  required={Boolean(brandLogoAssetId)}
                />
              </label>
            </div>
          </div>

          {brandErrorMessage ? (
            <p className={text.error}>{brandErrorMessage}</p>
          ) : null}
          {brandStatusMessage ? (
            <p className={text.success}>{brandStatusMessage}</p>
          ) : null}

          <div className={actions.row}>
            <Button type="submit" disabled={isSavingBrand}>
              {isSavingBrand ? "Saving brand..." : "Save brand"}
            </Button>
          </div>
        </form>
      </section>

      <section className={workspaceSection}>
        <div>
          <p className={text.eyebrow}>Theme</p>
          <h2 className="mt-1 text-[1.2rem] font-black leading-[1.02] text-[var(--paper)]">
            Set the site direction
          </h2>
          <p className={cn(text.p, "mt-2 text-sm")}>
            Theme choices change the public site styling, not the builder
            chrome. The brand primary color seeds the palette here.
          </p>
        </div>

        {isRegeneratingTheme ? (
          <GenerationProgressCard
            eyebrow="Theme"
            title="Reweaving the theme..."
            description="Snaelda is regenerating colors and typography from the current site brief."
            steps={themeRegenerateSteps}
            activeStep={themeProgressStep}
            activeTotal={themeProgressStepTotal}
            previewTitle="Theme preview"
            idlePreviewText="We'll refresh the live preview as soon as the new palette is ready."
          />
        ) : themeSelection && themeOptions ? (
          <form className={form.grid} onSubmit={handleSaveTheme}>
            <div className={workspaceInset}>
              <div className="grid grid-cols-2 gap-3 max-lg:grid-cols-1">
                {Object.entries(draft.theme.tokens.colors).map(
                  ([key, value]) => (
                    <div
                      key={key}
                      className="flex items-center gap-3 rounded-[8px] border border-border bg-[color-mix(in_oklch,var(--surface-1)_58%,transparent)] px-3 py-2.5"
                    >
                      <span
                        className="size-[34px] shrink-0 rounded-[999px] border border-border shadow-[inset_0_0_0_1px_oklch(7%_0.022_336_/_0.12)]"
                        style={{ backgroundColor: value }}
                      />
                      <div>
                        <strong className="block">
                          {formatThemeLabel(key)}
                        </strong>
                        <small className="block text-[var(--paper-muted)]">
                          {value}
                        </small>
                      </div>
                    </div>
                  ),
                )}
              </div>
            </div>

            <div className={form.field}>
              <div>
                <label className={text.label}>Theme direction</label>
                <p className={form.hint}>
                  Each direction keeps the brand primary color and changes the
                  surrounding surface, contrast, and styling.
                </p>
              </div>
              <div className="grid gap-3 lg:grid-cols-2 2xl:grid-cols-3">
                {themeOptions.palettes.map((option) => {
                  const selected = themeSelection.palette === option.id;
                  const colors = themePreviewColors(
                    option,
                    draft.theme.tokens.colors,
                  );

                  return (
                    <button
                      key={option.id}
                      type="button"
                      aria-pressed={selected}
                      onClick={() =>
                        handleThemeSelectionChange("palette", option.id)
                      }
                      className={cn(
                        "grid gap-3 rounded-[10px] border p-3 text-left transition duration-200 ease-out hover:border-[color-mix(in_oklch,var(--thread-gold)_78%,var(--border))] focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-[var(--thread-gold)]",
                        selected
                          ? "border-[var(--thread-gold)] bg-[color-mix(in_oklch,var(--surface-2)_82%,var(--thread-gold))]"
                          : "border-border bg-[color-mix(in_oklch,var(--surface-1)_52%,transparent)]",
                      )}
                    >
                      <span
                        className="grid min-h-[118px] gap-3 overflow-hidden rounded-[8px] border p-3"
                        style={themePreviewStyle(colors)}
                      >
                        <span className="flex items-center justify-between gap-3">
                          <span className="h-2.5 w-16 rounded-[999px] bg-current opacity-80" />
                          <span
                            className="size-5 rounded-[999px]"
                            style={{ backgroundColor: colors.primary }}
                          />
                        </span>
                        <span className="grid gap-2">
                          <span className="h-3 w-3/4 rounded-[999px] bg-current" />
                          <span
                            className="h-2 w-full rounded-[999px]"
                            style={{ backgroundColor: colors.muted }}
                          />
                          <span
                            className="h-2 w-2/3 rounded-[999px]"
                            style={{ backgroundColor: colors.muted }}
                          />
                        </span>
                        <span className="flex items-center gap-2">
                          <span
                            className="h-7 w-20 rounded-[999px]"
                            style={{ backgroundColor: colors.primary }}
                          />
                          <span
                            className="h-7 flex-1 rounded-[8px] border"
                            style={{
                              backgroundColor: colors.surfaceMuted,
                              borderColor: colors.border,
                            }}
                          />
                        </span>
                      </span>
                      <span>
                        <strong className="block text-[var(--paper)]">
                          {option.label}
                        </strong>
                        {option.description ? (
                          <small className="mt-1 block leading-snug text-[var(--paper-muted)]">
                            {option.description}
                          </small>
                        ) : null}
                      </span>
                    </button>
                  );
                })}
              </div>
            </div>

            <div className="grid gap-4 xl:grid-cols-3">
              <div className={form.field}>
                <label htmlFor="theme-font-preset" className={text.label}>
                  Font preset
                </label>
                <Select
                  id="theme-font-preset"
                  value={themeSelection.fontPreset}
                  onChange={(event) =>
                    handleThemeSelectionChange("fontPreset", event.target.value)
                  }
                >
                  {themeOptions.fontPresets.map((option) => (
                    <option key={option.id} value={option.id}>
                      {option.label}
                    </option>
                  ))}
                </Select>
                <p className={form.hint}>
                  {describeThemeOption(
                    themeOptions.fontPresets,
                    themeSelection.fontPreset,
                  )}
                </p>
              </div>

              <div className={form.field}>
                <label htmlFor="theme-type-scale" className={text.label}>
                  Type scale
                </label>
                <Select
                  id="theme-type-scale"
                  value={themeSelection.typeScale}
                  onChange={(event) =>
                    handleThemeSelectionChange("typeScale", event.target.value)
                  }
                >
                  {themeOptions.typeScales.map((option) => (
                    <option key={option.id} value={option.id}>
                      {option.label}
                    </option>
                  ))}
                </Select>
                <p className={form.hint}>
                  {describeThemeOption(
                    themeOptions.typeScales,
                    themeSelection.typeScale,
                  )}
                </p>
              </div>

              <div className={form.field}>
                <label htmlFor="theme-section-spacing" className={text.label}>
                  Section spacing
                </label>
                <Select
                  id="theme-section-spacing"
                  value={themeSelection.sectionSpacing}
                  onChange={(event) =>
                    handleThemeSelectionChange(
                      "sectionSpacing",
                      event.target.value,
                    )
                  }
                >
                  {themeOptions.sectionSpacings.map((option) => (
                    <option key={option.id} value={option.id}>
                      {option.label}
                    </option>
                  ))}
                </Select>
                <p className={form.hint}>
                  {describeThemeOption(
                    themeOptions.sectionSpacings,
                    themeSelection.sectionSpacing,
                  )}
                </p>
              </div>

              <div className={form.field}>
                <label htmlFor="theme-content-width" className={text.label}>
                  Content width
                </label>
                <Select
                  id="theme-content-width"
                  value={themeSelection.contentWidth}
                  onChange={(event) =>
                    handleThemeSelectionChange(
                      "contentWidth",
                      event.target.value,
                    )
                  }
                >
                  {themeOptions.contentWidths.map((option) => (
                    <option key={option.id} value={option.id}>
                      {option.label}
                    </option>
                  ))}
                </Select>
                <p className={form.hint}>
                  {describeThemeOption(
                    themeOptions.contentWidths,
                    themeSelection.contentWidth,
                  )}
                </p>
              </div>

              <div className={form.field}>
                <label htmlFor="theme-radius" className={text.label}>
                  Corner radius
                </label>
                <Select
                  id="theme-radius"
                  value={themeSelection.radius}
                  onChange={(event) =>
                    handleThemeSelectionChange("radius", event.target.value)
                  }
                >
                  {themeOptions.radii.map((option) => (
                    <option key={option.id} value={option.id}>
                      {option.label}
                    </option>
                  ))}
                </Select>
                <p className={form.hint}>
                  {describeThemeOption(
                    themeOptions.radii,
                    themeSelection.radius,
                  )}
                </p>
              </div>

              <div className={form.field}>
                <label htmlFor="theme-button-style" className={text.label}>
                  Button style
                </label>
                <Select
                  id="theme-button-style"
                  value={themeSelection.buttonStyle}
                  onChange={(event) =>
                    handleThemeSelectionChange(
                      "buttonStyle",
                      event.target.value,
                    )
                  }
                >
                  {themeOptions.buttonStyles.map((option) => (
                    <option key={option.id} value={option.id}>
                      {option.label}
                    </option>
                  ))}
                </Select>
                <p className={form.hint}>
                  {describeThemeOption(
                    themeOptions.buttonStyles,
                    themeSelection.buttonStyle,
                  )}
                </p>
              </div>

              <div className={form.field}>
                <label htmlFor="theme-image-style" className={text.label}>
                  Image style
                </label>
                <Select
                  id="theme-image-style"
                  value={themeSelection.imageStyle}
                  onChange={(event) =>
                    handleThemeSelectionChange("imageStyle", event.target.value)
                  }
                >
                  {themeOptions.imageStyles.map((option) => (
                    <option key={option.id} value={option.id}>
                      {option.label}
                    </option>
                  ))}
                </Select>
                <p className={form.hint}>
                  {describeThemeOption(
                    themeOptions.imageStyles,
                    themeSelection.imageStyle,
                  )}
                </p>
              </div>
            </div>

            {themeErrorMessage ? (
              <p className={text.error}>{themeErrorMessage}</p>
            ) : null}
            {themeStatusMessage ? (
              <p className={text.success}>{themeStatusMessage}</p>
            ) : null}

            <div className="flex flex-wrap gap-3">
              <Button
                type="button"
                variant="outline"
                onClick={handleRegenerateTheme}
                disabled={isSavingTheme || isRegeneratingTheme}
              >
                {isRegeneratingTheme
                  ? "Regenerating theme..."
                  : "Regenerate theme"}
              </Button>
              {themeHasUnsavedChanges ? (
                <Button
                  type="button"
                  variant="outline"
                  onClick={handleResetThemeSelection}
                  disabled={isSavingTheme || isRegeneratingTheme}
                >
                  Reset live changes
                </Button>
              ) : null}
              <Button
                type="submit"
                disabled={
                  isSavingTheme ||
                  isRegeneratingTheme ||
                  !themeHasUnsavedChanges
                }
              >
                {isSavingTheme ? "Saving theme..." : "Save theme"}
              </Button>
            </div>
          </form>
        ) : (
          <div className={emptyState}>
            <p className={text.p}>Loading theme controls...</p>
          </div>
        )}
      </section>
    </div>
  );

  const publishPanelContent = (
    <div className="grid gap-4 xl:grid-cols-[minmax(0,0.95fr)_minmax(0,1.05fr)]">
      {billingPromptNotice ? (
        <div className="xl:col-span-2">{billingPromptNotice}</div>
      ) : null}
      <section className={workspaceSection}>
        <div>
          <p className={text.eyebrow}>Draft to live</p>
          <h2 className="mt-1 text-[1.2rem] font-black leading-[1.02] text-[var(--paper)]">
            Publish the current draft
          </h2>
          <p className={cn(text.p, "mt-2 text-sm")}>
            Publishing sends the current draft live and creates a rollback
            point. Keep the note concise so future rollbacks make sense.
          </p>
        </div>

        <div className="grid gap-3 md:grid-cols-3">
          <div className={workspaceInset}>
            <p className={text.label}>Draft pages</p>
            <p className="mt-2 text-2xl font-black text-[var(--paper)]">
              {draft.pages.length}
            </p>
          </div>
          <div className="rounded-[12px] border border-border bg-[var(--surface-2)] p-4">
            <p className={text.label}>Current live version</p>
            <p className="mt-2 text-2xl font-black text-[var(--paper)]">
              {currentVersion ? `v${currentVersion.versionNumber}` : "None yet"}
            </p>
          </div>
          <div className="rounded-[12px] border border-border bg-[var(--surface-2)] p-4">
            <p className={text.label}>Workspace plan</p>
            <p className="mt-2 text-2xl font-black text-[var(--paper)]">
              {billingPlanLabel}
            </p>
          </div>
        </div>

        <div className={workspaceInset}>
          <p className={text.label}>
            {currentVersion ? "Live now" : "Will go live at"}
          </p>
          <p className="mt-2 break-all text-sm font-semibold text-[var(--paper)]">
            {currentVersion
              ? livePublicUrl || liveHostname || "No live hostname yet"
              : liveHostname || "Hosted domain will appear after first publish"}
          </p>
          <p className={cn(form.hint, "mt-2")}>
            Hosted sites resolve through the published domain record, not the
            internal preview route.
          </p>
          {!domainState?.customDomainsEnabled ? (
            <p className={cn(form.hint, "mt-2")}>
              Custom domains stay locked until the workspace is on a paid plan.
            </p>
          ) : null}
        </div>

        <div className={workspaceInset}>
          <div>
            <p className={text.label}>Custom domains</p>
            <p className={cn(form.hint, "mt-2")}>
              Attach the domain you already own, add the TXT record exactly as
              shown, then verify it here. Traffic keeps working on the hosted
              Snaelda URL until your custom hostname is active.
            </p>
          </div>

          {domainErrorMessage ? (
            <p className={cn(text.error, "mt-4")}>{domainErrorMessage}</p>
          ) : null}
          {domainStatusMessage ? (
            <p className={cn(text.success, "mt-4")}>{domainStatusMessage}</p>
          ) : null}

          {domainState?.customDomainsEnabled ? (
            <form
              className={cn(form.grid, "mt-4")}
              onSubmit={handleCreateDomain}
            >
              <label htmlFor="custom-domain-hostname" className={text.label}>
                Add a hostname
              </label>
              <div className="grid gap-2 md:grid-cols-[minmax(0,1fr)_auto]">
                <Input
                  id="custom-domain-hostname"
                  value={newCustomDomainHostname}
                  onChange={(event) =>
                    setNewCustomDomainHostname(event.target.value)
                  }
                  placeholder="example.com or www.example.com"
                  autoCapitalize="none"
                  autoCorrect="off"
                  spellCheck={false}
                />
                <Button
                  type="submit"
                  disabled={
                    isMutatingDomain || newCustomDomainHostname.trim() === ""
                  }
                >
                  {isMutatingDomain && activeDomainId === ""
                    ? "Adding..."
                    : "Add domain"}
                </Button>
              </div>
            </form>
          ) : (
            <div className="mt-4 rounded-[12px] border border-[color-mix(in_oklch,var(--thread-gold)_45%,var(--border))] bg-[color-mix(in_oklch,var(--surface-2)_86%,var(--thread-gold))] p-4">
              <p className="text-sm font-semibold text-[var(--paper)]">
                Upgrade to attach a custom domain.
              </p>
              <p className={cn(form.hint, "mt-2")}>
                Paid plans unlock DNS verification and live traffic on your own
                hostname.
              </p>
              <div className="mt-4">
                <Button
                  type="button"
                  size="sm"
                  onClick={handleStartUpgrade}
                  disabled={isStartingUpgrade}
                >
                  {isStartingUpgrade ? "Opening checkout..." : "Upgrade"}
                </Button>
              </div>
            </div>
          )}

          <div className="mt-4 grid gap-3">
            {customDomains.length > 0 ? (
              customDomains.map((domain) => {
                const isActive = domain.status === "active";
                const isPending = !isActive;
                return (
                  <article
                    key={domain.id}
                    className="rounded-[12px] border border-border bg-[color-mix(in_oklch,var(--surface-1)_42%,transparent)] p-4"
                  >
                    <div className="flex items-start justify-between gap-3 max-sm:flex-col">
                      <div>
                        <p className="m-0 break-all text-sm font-semibold text-[var(--paper)]">
                          {domain.hostname}
                        </p>
                        <p className="mt-2 text-sm text-[var(--paper-muted)]">
                          {isActive
                            ? "Active and eligible for live traffic."
                            : "Pending DNS TXT verification."}
                        </p>
                      </div>
                      <span
                        className={cn(
                          "rounded-full px-3 py-1 text-xs font-bold uppercase tracking-[0.12em]",
                          isActive
                            ? "bg-[color-mix(in_oklch,var(--thread-teal)_26%,var(--surface-1))] text-[var(--paper)]"
                            : "bg-[color-mix(in_oklch,var(--thread-gold)_24%,var(--surface-1))] text-[var(--paper)]",
                        )}
                      >
                        {domain.status}
                      </span>
                    </div>

                    {domain.publicUrl ? (
                      <p className="mt-3 text-sm text-[var(--paper-muted)]">
                        Live URL:{" "}
                        <a
                          href={domain.publicUrl}
                          target="_blank"
                          rel="noreferrer"
                          className="font-semibold text-[var(--paper)] underline"
                        >
                          {domain.publicUrl}
                        </a>
                      </p>
                    ) : null}

                    {isPending &&
                    domain.verificationHostname &&
                    domain.verificationValue ? (
                      <div className="mt-4 grid gap-3 md:grid-cols-2">
                        <div className="rounded-[10px] border border-border bg-[var(--surface-1)] p-3">
                          <p className={text.label}>TXT record name</p>
                          <p className="mt-2 break-all text-sm font-semibold text-[var(--paper)]">
                            {domain.verificationHostname}
                          </p>
                        </div>
                        <div className="rounded-[10px] border border-border bg-[var(--surface-1)] p-3">
                          <p className={text.label}>TXT record value</p>
                          <p className="mt-2 break-all text-sm font-semibold text-[var(--paper)]">
                            {domain.verificationValue}
                          </p>
                        </div>
                      </div>
                    ) : null}

                    <div className="mt-4 flex flex-wrap gap-3">
                      {isPending ? (
                        <Button
                          type="button"
                          size="sm"
                          variant="outline"
                          disabled={isMutatingDomain}
                          onClick={() => handleVerifyDomain(domain.id)}
                        >
                          {isMutatingDomain && activeDomainId === domain.id
                            ? "Checking DNS..."
                            : "Verify domain"}
                        </Button>
                      ) : null}
                      <Button
                        type="button"
                        size="sm"
                        variant="plain"
                        className={actions.inlineLink}
                        disabled={isMutatingDomain}
                        onClick={() =>
                          handleDeleteDomain(domain.id, domain.hostname)
                        }
                      >
                        {isMutatingDomain && activeDomainId === domain.id
                          ? "Removing..."
                          : "Remove domain"}
                      </Button>
                    </div>
                  </article>
                );
              })
            ) : (
              <div className="rounded-[12px] border border-dashed border-border p-4">
                <p className={text.p}>
                  No custom domains yet. Add one when you are ready to point
                  your own hostname at this site.
                </p>
              </div>
            )}
          </div>
        </div>

        <div className={form.field}>
          <label htmlFor="publish-note" className={text.label}>
            Release note
          </label>
          <Textarea
            id="publish-note"
            name="publishNote"
            rows={4}
            value={publishNote}
            onChange={(event) => setPublishNote(event.target.value)}
            placeholder="Tightened the hero, updated pricing, and refreshed the gallery."
          />
          <p className={form.hint}>
            Optional, but useful when you need to compare or roll back versions.
          </p>
        </div>

        {publishErrorMessage ? (
          <p className={text.error}>{publishErrorMessage}</p>
        ) : null}
        {publishValidationIssues.length > 0 ? (
          <ul className="space-y-1 text-sm text-[var(--paper-muted)]">
            {publishValidationIssues.map((issue) => (
              <li key={`${issue.path}-${issue.code}`}>
                <span className="font-mono text-xs">{issue.path}</span>{" "}
                {issue.message}
              </li>
            ))}
          </ul>
        ) : null}
        {publishStatusMessage ? (
          <p className={text.success}>{publishStatusMessage}</p>
        ) : null}

        <div className={actions.rowLarge}>
          <Button
            type="button"
            disabled={isPublishing || activeRollbackVersionId !== ""}
            onClick={handlePublish}
          >
            {isPublishing ? "Publishing live..." : "Publish live update"}
          </Button>
          {currentVersion && livePublicUrl ? (
            <Button asChild variant="plain" className={actions.inlineLink}>
              <a href={livePublicUrl} target="_blank" rel="noreferrer">
                Open live site
              </a>
            </Button>
          ) : currentVersion ? (
            <Button asChild variant="plain" className={actions.inlineLink}>
              <Link
                to="/public/$siteSlug"
                params={{ siteSlug: draft.site.slug }}
              >
                View published route
              </Link>
            </Button>
          ) : null}
        </div>
      </section>

      <section className={workspaceSection}>
        <div>
          <p className={text.eyebrow}>Release history</p>
          <h2 className="mt-1 text-[1.2rem] font-black leading-[1.02] text-[var(--paper)]">
            Inspect and roll back versions
          </h2>
        </div>

        <div className="grid gap-3">
          {versions.length === 0 ? (
            <div className={emptyState}>
              <p className={text.p}>No published versions yet.</p>
            </div>
          ) : (
            versions.map((version) => (
              <article
                key={version.id}
                className={cn(
                  "grid gap-3 rounded-[12px] border p-4",
                  version.isCurrent
                    ? "border-[var(--thread-gold)] bg-[color-mix(in_oklch,var(--surface-2)_86%,var(--thread-gold))]"
                    : "border-border bg-[var(--surface-2)]",
                )}
              >
                <div className="flex items-start justify-between gap-3">
                  <div>
                    <strong className="block text-[var(--paper)]">
                      Version {version.versionNumber}
                      {version.isCurrent ? " · live now" : ""}
                    </strong>
                    <small className="text-[var(--paper-muted)]">
                      {formatTimestamp(version.createdAt)}
                    </small>
                  </div>
                  {!version.isCurrent ? (
                    <Button
                      type="button"
                      variant="outline"
                      size="sm"
                      disabled={isPublishing || activeRollbackVersionId !== ""}
                      onClick={() => handleRollback(version)}
                    >
                      {activeRollbackVersionId === version.id
                        ? "Rolling back..."
                        : "Make live again"}
                    </Button>
                  ) : null}
                </div>
                {version.publishNote ? (
                  <p className="m-0 text-sm text-[var(--paper-muted)]">
                    {version.publishNote}
                  </p>
                ) : (
                  <p className="m-0 text-sm text-[var(--paper-muted)]">
                    No release note was saved for this version.
                  </p>
                )}
              </article>
            ))
          )}
        </div>
      </section>
    </div>
  );

  return (
    <>
      <PuckBuilder
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
        isPublishing={isPublishing}
        pages={draft.pages}
        onSelectPage={handleSelectPage}
        onSelectBlock={handleSelectBlock}
        onEditField={handleEditField}
        onToggleHidden={handleToggleHidden}
        onUpdateBindings={handleUpdateBindings}
        onSuggestBlock={handleSuggestBlock}
        isSuggestingBlock={isSuggestingBlock}
        suggestErrorMessage={suggestErrorMessage}
        suggestStatusMessage={suggestStatusMessage}
        siteId={siteId}
        onImageApplied={handleImageApplied}
        onAddBlock={handleAddBlock}
        onDuplicateBlock={handleDuplicateBlock}
        onDeleteBlock={handleDeleteBlock}
        onMoveBlock={handleMoveBlock}
        onReorderBlocks={handleReorderBlocks}
        pagesPanelContent={pagesPanelContent}
        promptPanelContent={promptPanelContent}
        collectionsPanelContent={collectionsPanelContent}
        seoPanelContent={seoPanelContent}
        navigationPanelContent={navigationPanelContent}
        assetsPanelContent={assetsPanelContent}
        inquiriesPanelContent={inquiriesPanelContent}
        settingsPanelContent={settingsPanelContent}
        themePanelContent={themePanelContent}
        publishPanelContent={publishPanelContent}
        section={search.panel ?? "content"}
        onSectionChange={(nextSection) =>
          navigate({
            to: ".",
            search: {
              panel: nextSection === "content" ? undefined : nextSection,
            },
            replace: true,
          })
        }
      />
      <RevisionDiffModal
        reprompt={activeRepromptDiff}
        previousRevision={repromptDiffPrevious}
        resultRevision={repromptDiffResult}
        errorMessage={repromptDiffErrorMessage}
        isLoading={isLoadingRepromptDiff}
        onClose={() => {
          const opener = repromptDiffOpenerRef.current;
          setActiveRepromptDiff(null);
          setRepromptDiffPrevious(null);
          setRepromptDiffResult(null);
          setRepromptDiffErrorMessage("");
          repromptDiffOpenerRef.current = null;
          if (opener) {
            window.requestAnimationFrame(() => {
              opener.focus();
            });
          }
        }}
      />
    </>
  );
}

function pageTypeLabel(type: PageType | undefined): string {
  switch (type) {
    case "collection_index":
      return "Collection index";
    case "collection_detail":
      return "Collection detail";
    default:
      return "Static";
  }
}

function findNewPage(previousDraft: SiteDraft | null, nextDraft: SiteDraft) {
  const previousIDs = new Set(
    previousDraft?.pages.map((page) => page.id) ?? [],
  );
  return (
    nextDraft.pages.find((page) => !previousIDs.has(page.id)) ??
    nextDraft.pages.at(-1)
  );
}

function findNewBlock(
  previousPage: DraftPage | null,
  nextPage: DraftPage | null,
) {
  if (!nextPage) {
    return null;
  }
  const previousIDs = new Set(
    previousPage?.blocks.map((block) => block.id) ?? [],
  );
  return (
    nextPage.blocks.find((block) => !previousIDs.has(block.id)) ??
    nextPage.blocks.at(-1) ??
    null
  );
}

function setBlockPropPath(
  source: Record<string, unknown>,
  path: ReadonlyArray<string | number>,
  value: unknown,
): Record<string, unknown> {
  const cloned = JSON.parse(JSON.stringify(source ?? {})) as Record<
    string,
    unknown
  >;
  if (path.length === 0) {
    return value && typeof value === "object" && !Array.isArray(value)
      ? (value as Record<string, unknown>)
      : cloned;
  }
  let cursor: unknown = cloned;
  for (let i = 0; i < path.length - 1; i++) {
    const segment = path[i];
    const next = path[i + 1];
    if (Array.isArray(cursor)) {
      const index = Number(segment);
      while ((cursor as unknown[]).length <= index) {
        (cursor as unknown[]).push(undefined);
      }
      if (
        (cursor as unknown[])[index] === undefined ||
        (cursor as unknown[])[index] === null
      ) {
        (cursor as unknown[])[index] =
          typeof next === "number" || /^\d+$/.test(String(next)) ? [] : {};
      }
      cursor = (cursor as unknown[])[index];
    } else if (cursor && typeof cursor === "object") {
      const key = String(segment);
      const record = cursor as Record<string, unknown>;
      if (record[key] === undefined || record[key] === null) {
        record[key] =
          typeof next === "number" || /^\d+$/.test(String(next)) ? [] : {};
      }
      cursor = record[key];
    }
  }
  const lastSegment = path[path.length - 1];
  if (Array.isArray(cursor)) {
    const index = Number(lastSegment);
    while ((cursor as unknown[]).length <= index) {
      (cursor as unknown[]).push(undefined);
    }
    if (value === undefined) {
      (cursor as unknown[]).splice(index, 1);
    } else {
      (cursor as unknown[])[index] = value;
    }
  } else if (cursor && typeof cursor === "object") {
    const key = String(lastSegment);
    const record = cursor as Record<string, unknown>;
    if (value === undefined) {
      delete record[key];
    } else {
      record[key] = value;
    }
  }
  return cloned;
}

function applyBlockUpdate(
  draft: SiteDraft,
  pageId: string,
  blockId: string,
  patch: { props: Record<string, unknown>; hidden: boolean },
): SiteDraft {
  return {
    ...draft,
    pages: draft.pages.map((page) => {
      if (page.id !== pageId) return page;
      return {
        ...page,
        blocks: page.blocks.map((block) => {
          if (block.id !== blockId) return block;
          return {
            ...block,
            props: patch.props,
            settings: {
              ...(block.settings ?? {}),
              hidden: patch.hidden,
            },
          };
        }),
      };
    }),
  };
}

function navigationItemsFromDraft(draft: SiteDraft): NavigationDraftState {
  return {
    primary: normalizeNavigationItems(draft, draft.navigation.primary),
    footer: normalizeNavigationItems(draft, draft.navigation.footer ?? []),
  };
}

function normalizeNavigationItems(
  draft: SiteDraft,
  items: Array<{ label: string; pageId?: string; href?: string }>,
) {
  const normalized: NavigationItemInput[] = [];
  for (const item of items) {
    if (item.pageId) {
      const page = draft.pages.find(
        (candidate) => candidate.id === item.pageId,
      );
      if (!page) {
        continue;
      }
      normalized.push({ label: item.label, pageId: item.pageId });
      continue;
    }
    if (item.href) {
      normalized.push({ label: item.label, href: item.href });
    }
  }
  return normalized;
}

function sameNavigationItems(
  left: NavigationItemInput[],
  right: NavigationItemInput[],
): boolean {
  if (left.length !== right.length) {
    return false;
  }
  for (let index = 0; index < left.length; index += 1) {
    const a = left[index];
    const b = right[index];
    if (a.label !== b.label) {
      return false;
    }
    if ((a.pageId ?? "") !== (b.pageId ?? "")) {
      return false;
    }
    if ((a.href ?? "") !== (b.href ?? "")) {
      return false;
    }
  }
  return true;
}

function sameNavigationDraft(
  left: NavigationDraftState,
  right: NavigationDraftState,
) {
  return (
    sameNavigationItems(left.primary, right.primary) &&
    sameNavigationItems(left.footer, right.footer)
  );
}

function moveItem<T extends { id: string }>(
  items: T[],
  itemID: string,
  direction: -1 | 1,
) {
  const index = items.findIndex((item) => item.id === itemID);
  const nextIndex = index + direction;
  if (index === -1 || nextIndex < 0 || nextIndex >= items.length) {
    return null;
  }
  const reordered = [...items];
  [reordered[index], reordered[nextIndex]] = [
    reordered[nextIndex],
    reordered[index],
  ];
  return reordered;
}

function formatSubmissionKey(value: string) {
  return value.replaceAll("_", " ").replace(/^./, (char) => char.toUpperCase());
}

function formatTimestamp(value: string) {
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) {
    return value;
  }
  return new Intl.DateTimeFormat(undefined, {
    dateStyle: "medium",
    timeStyle: "short",
  }).format(date);
}

function describeThemeOption(
  options: Array<{ id: string; description?: string }>,
  selectedID: string,
) {
  return options.find((option) => option.id === selectedID)?.description ?? "";
}

function sameThemeSelection(left: ThemeSelection, right: ThemeSelection) {
  return (
    left.palette === right.palette &&
    left.fontPreset === right.fontPreset &&
    left.typeScale === right.typeScale &&
    left.sectionSpacing === right.sectionSpacing &&
    left.contentWidth === right.contentWidth &&
    left.radius === right.radius &&
    left.buttonStyle === right.buttonStyle &&
    left.imageStyle === right.imageStyle
  );
}

function themePreviewColors(
  option: ThemeOption,
  fallback: Record<string, string>,
) {
  const colors = option.previewColors ?? fallback;
  return {
    background: colors.background ?? "#f7f3ea",
    foreground: colors.foreground ?? colors.text ?? "#2c2721",
    surface: colors.surface ?? "#fffaf1",
    surfaceMuted: colors.surfaceMuted ?? "#ebe3d5",
    primary: colors.primary ?? "#426b5c",
    muted: colors.muted ?? "#8c765c",
    border: colors.border ?? "#d9cebd",
  };
}

function themePreviewStyle(
  colors: ReturnType<typeof themePreviewColors>,
): CSSProperties {
  return {
    backgroundColor: colors.surface,
    borderColor: colors.border,
    color: colors.foreground,
    boxShadow: `inset 0 0 0 999px ${withPreviewAlpha(colors.background, "66")}`,
  };
}

function withPreviewAlpha(color: string, alphaHex: string) {
  const normalized = color.trim();
  if (/^#[0-9a-fA-F]{6}$/.test(normalized)) {
    return `${normalized}${alphaHex}`;
  }
  if (/^#[0-9a-fA-F]{3}$/.test(normalized)) {
    const expanded = normalized
      .slice(1)
      .split("")
      .map((part) => part + part)
      .join("");
    return `#${expanded}${alphaHex}`;
  }
  return normalized;
}

function formatThemeLabel(value: string) {
  return value
    .replace(/([A-Z])/g, " $1")
    .replace(/^./, (char) => char.toUpperCase());
}

function formatRepromptScopeLabel(value: string) {
  return value
    .replaceAll("_", " ")
    .replace(/^./, (char) => char.toUpperCase());
}

function suggestionToastForInput(input: BlockSuggestInput) {
  switch (input.action) {
    case "tighten":
      return "Tightened block copy.";
    case "expand":
      return "Expanded block copy.";
    case "tone":
      return `Shifted block tone to ${input.tone ?? "the chosen voice"}.`;
    case "rewrite":
      return "Rewrote block from your prompt.";
    default:
      return "Block updated with AI.";
  }
}
