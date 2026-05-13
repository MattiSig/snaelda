import { Link, createFileRoute, useNavigate } from '@tanstack/react-router'
import type { FormEvent } from 'react'
import { useEffect, useState } from 'react'
import { PuckBuilder } from '@/components/PuckBuilder'
import { Button } from '@/components/ui/button'

import { Input } from '@/components/ui/input'
import { Select } from '@/components/ui/select'
import { Textarea } from '@/components/ui/textarea'
import {
  buildDraftAssetURL,
  describeAssetDimensions,
  formatAssetFileSize,
  readImageDimensions,
} from '@/lib/assets'
import {
  APIError,
  completeAssetUpload,
  createBlock,
  createAssetUploadURL,
  createPage,
  deleteBlock,
  deletePage,
  deleteSite,
  duplicateBlock,
  getSiteDraft,
  getSiteTheme,
  listSiteFormSubmissions,
  listSiteAssets,
  listSiteVersions,
  publishSite,
  repromptPage,
  repromptSite,
  rollbackSiteVersion,
  reorderBlocks,
  reorderPages,
  reorderSiteNavigation,
  type AssetRecord,
  type BlockDefinition,
  type FormSubmissionRecord,
  type FormSubmissionStatus,
  type SiteDraft,
  type SiteVersion,
  type ThemeEditorCatalog,
  type ThemeSelection,
  updateBlock,
  updateFormSubmission,
  updatePage,
  updateSite,
  updateSiteTheme,
  undoSiteReprompt,
} from '@/lib/api'
import {
  actions,
  emptyState,
  form,
  ribbonPanel,
  text,
} from '@/lib/styles'
import { cn } from '@/lib/utils'

type DraftPage = SiteDraft['pages'][number]

export const Route = createFileRoute('/app/sites/$siteId/')({
  component: SiteDetail,
})

function SiteDetail() {
  const { siteId } = Route.useParams()
  const navigate = useNavigate()
  const [draft, setDraft] = useState<SiteDraft | null>(null)
  const [blockRegistry, setBlockRegistry] = useState<BlockDefinition[]>([])
  const [name, setName] = useState('')
  const [slug, setSlug] = useState('')
  const [selectedPageId, setSelectedPageId] = useState('')
  const [selectedBlockId, setSelectedBlockId] = useState('')
  const [versions, setVersions] = useState<SiteVersion[]>([])
  const [newPageTitle, setNewPageTitle] = useState('')
  const [newPageSlug, setNewPageSlug] = useState('')
  const [newPageIncludeInNavigation, setNewPageIncludeInNavigation] =
    useState(true)
  const [pageTitle, setPageTitle] = useState('')
  const [pageSlug, setPageSlug] = useState('')
  const [pageSEOTitle, setPageSEOTitle] = useState('')
  const [pageSEODescription, setPageSEODescription] = useState('')
  const [pageIncludeInNavigation, setPageIncludeInNavigation] = useState(true)
  const [newBlockType, setNewBlockType] = useState('')
  const [siteAssets, setSiteAssets] = useState<AssetRecord[]>([])
  const [formSubmissions, setFormSubmissions] = useState<FormSubmissionRecord[]>(
    [],
  )
  const [assetAltText, setAssetAltText] = useState('')
  const [assetFile, setAssetFile] = useState<File | null>(null)
  const [assetInputKey, setAssetInputKey] = useState(0)
  const [isLoading, setIsLoading] = useState(true)
  const [isSavingSite, setIsSavingSite] = useState(false)
  const [isCreatingPage, setIsCreatingPage] = useState(false)
  const [isSavingPage, setIsSavingPage] = useState(false)
  const [isDeletingPage, setIsDeletingPage] = useState(false)
  const [isSavingBlock, setIsSavingBlock] = useState(false)
  const [isCreatingBlock, setIsCreatingBlock] = useState(false)
  const [isMutatingBlocks, setIsMutatingBlocks] = useState(false)
  const [isDeleting, setIsDeleting] = useState(false)
  const [isPublishing, setIsPublishing] = useState(false)
  const [activeRollbackVersionId, setActiveRollbackVersionId] = useState('')
  const [publishNote, setPublishNote] = useState('')
  const [siteReprompt, setSiteReprompt] = useState('')
  const [pageReprompt, setPageReprompt] = useState('')
  const [themeSelection, setThemeSelection] = useState<ThemeSelection | null>(
    null,
  )
  const [themeOptions, setThemeOptions] = useState<ThemeEditorCatalog | null>(
    null,
  )
  const [loadErrorMessage, setLoadErrorMessage] = useState('')
  const [siteErrorMessage, setSiteErrorMessage] = useState('')
  const [siteStatusMessage, setSiteStatusMessage] = useState('')
  const [pageErrorMessage, setPageErrorMessage] = useState('')
  const [pageStatusMessage, setPageStatusMessage] = useState('')
  const [navigationErrorMessage, setNavigationErrorMessage] = useState('')
  const [navigationStatusMessage, setNavigationStatusMessage] = useState('')
  const [blockErrorMessage, setBlockErrorMessage] = useState('')
  const [blockStatusMessage, setBlockStatusMessage] = useState('')
  const [themeErrorMessage, setThemeErrorMessage] = useState('')
  const [themeStatusMessage, setThemeStatusMessage] = useState('')
  const [isSavingTheme, setIsSavingTheme] = useState(false)
  const [isSavingNavigation, setIsSavingNavigation] = useState(false)
  const [isUploadingAsset, setIsUploadingAsset] = useState(false)
  const [isRepromptingSite, setIsRepromptingSite] = useState(false)
  const [isRepromptingPage, setIsRepromptingPage] = useState(false)
  const [isUndoingReprompt, setIsUndoingReprompt] = useState(false)
  const [activeSubmissionId, setActiveSubmissionId] = useState('')
  const [publishErrorMessage, setPublishErrorMessage] = useState('')
  const [publishStatusMessage, setPublishStatusMessage] = useState('')
  const [assetErrorMessage, setAssetErrorMessage] = useState('')
  const [assetStatusMessage, setAssetStatusMessage] = useState('')
  const [submissionErrorMessage, setSubmissionErrorMessage] = useState('')
  const [submissionStatusMessage, setSubmissionStatusMessage] = useState('')
  const [repromptErrorMessage, setRepromptErrorMessage] = useState('')
  const [repromptStatusMessage, setRepromptStatusMessage] = useState('')

  function syncSelectedPageFields(
    nextDraft: SiteDraft,
    nextPage: DraftPage | null,
  ) {
    if (!nextPage) {
      setPageTitle('')
      setPageSlug('')
      setPageSEOTitle('')
      setPageSEODescription('')
      setPageIncludeInNavigation(true)
      return
    }

    setPageTitle(nextPage.title)
    setPageSlug(nextPage.slug)
    setPageSEOTitle(nextPage.seo?.title ?? '')
    setPageSEODescription(nextPage.seo?.description ?? '')
    setPageIncludeInNavigation(
      nextDraft.navigation.primary.some((item) => item.pageId === nextPage.id),
    )
  }

  function applyDraftUpdate(
    nextDraft: SiteDraft,
    preferredPageID?: string,
    preferredBlockID?: string,
  ) {
    const nextPage =
      nextDraft.pages.find((page) => page.id === preferredPageID) ??
      nextDraft.pages[0] ??
      null
    const nextBlock =
      nextPage?.blocks.find((block) => block.id === preferredBlockID) ??
      nextPage?.blocks[0] ??
      null

    setDraft(nextDraft)
    setSelectedPageId(nextPage?.id ?? '')
    setSelectedBlockId(nextBlock?.id ?? '')
    syncSelectedPageFields(nextDraft, nextPage)
    setNavigationErrorMessage('')
    setNavigationStatusMessage('')
  }

  useEffect(() => {
    let isMounted = true

    Promise.all([
      getSiteDraft(siteId),
      listSiteVersions(siteId),
      getSiteTheme(siteId),
      listSiteAssets(siteId),
      listSiteFormSubmissions(siteId),
    ])
      .then(
        ([
          draftResponse,
          versionResponse,
          themeResponse,
          assetResponse,
          submissionResponse,
        ]) => {
          if (!isMounted) {
            return
          }
          setBlockRegistry(draftResponse.blockRegistry)
          setNewBlockType(
            (current) => current || draftResponse.blockRegistry[0]?.type || '',
          )
          setVersions(versionResponse.versions)
          setThemeSelection(themeResponse.selection)
          setThemeOptions(themeResponse.options)
          setSiteAssets(assetResponse.assets)
          setFormSubmissions(submissionResponse.submissions)
          setName(draftResponse.draft.site.name)
          setSlug(draftResponse.draft.site.slug)
          const initialPage = draftResponse.draft.pages[0] ?? null
          setDraft(draftResponse.draft)
          setSelectedPageId(initialPage?.id ?? '')
          setSelectedBlockId(initialPage?.blocks[0]?.id ?? '')
          syncSelectedPageFields(draftResponse.draft, initialPage)
          setIsLoading(false)
        },
      )
      .catch((error) => {
        if (!isMounted) {
          return
        }
        setLoadErrorMessage(
          error instanceof APIError ? error.message : 'Could not load site',
        )
        setIsLoading(false)
      })

    return () => {
      isMounted = false
    }
  }, [siteId])

  const blockDefinitions = new Map(
    blockRegistry.map((definition) => [
      `${definition.type}@${definition.version}`,
      definition,
    ]),
  )

  const selectedPage =
    draft?.pages.find((page) => page.id === selectedPageId) ??
    draft?.pages[0] ??
    null
  const selectedBlock =
    selectedPage?.blocks.find((block) => block.id === selectedBlockId) ??
    selectedPage?.blocks[0] ??
    null
  const selectedDefinition = selectedBlock
    ? blockDefinitions.get(`${selectedBlock.type}@${selectedBlock.version}`)
    : undefined

  async function handleSaveSite(event: FormEvent<HTMLFormElement>) {
    event.preventDefault()
    setIsSavingSite(true)
    setSiteErrorMessage('')
    setSiteStatusMessage('')

    try {
      const response = await updateSite(siteId, { name, slug })
      setName(response.draft.site.name)
      setSlug(response.draft.site.slug)
      setSiteStatusMessage('Site details saved.')
      applyDraftUpdate(response.draft, selectedPage?.id, selectedBlock?.id)
    } catch (error) {
      setSiteErrorMessage(
        error instanceof APIError ? error.message : 'Could not save site',
      )
    } finally {
      setIsSavingSite(false)
    }
  }

  async function handleCreatePage(event: FormEvent<HTMLFormElement>) {
    event.preventDefault()
    setIsCreatingPage(true)
    setPageErrorMessage('')
    setPageStatusMessage('')

    try {
      const response = await createPage(siteId, {
        title: newPageTitle,
        slug: newPageSlug || undefined,
        includeInNavigation: newPageIncludeInNavigation,
      })
      const createdPage = findNewPage(draft, response.draft)
      applyDraftUpdate(
        response.draft,
        createdPage?.id,
        createdPage?.blocks[0]?.id,
      )
      setNewPageTitle('')
      setNewPageSlug('')
      setNewPageIncludeInNavigation(true)
      setPageStatusMessage('Page added to the draft.')
    } catch (error) {
      setPageErrorMessage(
        error instanceof APIError ? error.message : 'Could not create page',
      )
    } finally {
      setIsCreatingPage(false)
    }
  }

  async function handleSavePage(event: FormEvent<HTMLFormElement>) {
    event.preventDefault()
    if (!selectedPage) {
      return
    }

    setIsSavingPage(true)
    setPageErrorMessage('')
    setPageStatusMessage('')

    try {
      const response = await updatePage(siteId, selectedPage.id, {
        title: pageTitle,
        slug: pageSlug,
        seo: {
          title: pageSEOTitle,
          description: pageSEODescription,
        },
        includeInNavigation: pageIncludeInNavigation,
      })
      applyDraftUpdate(response.draft, selectedPage.id, selectedBlock?.id)
      setPageStatusMessage('Page details saved.')
    } catch (error) {
      setPageErrorMessage(
        error instanceof APIError ? error.message : 'Could not save page',
      )
    } finally {
      setIsSavingPage(false)
    }
  }

  async function handleDeletePage() {
    if (!selectedPage) {
      return
    }
    const confirmed = window.confirm(
      `Delete the page "${selectedPage.title}" from this draft?`,
    )
    if (!confirmed) {
      return
    }

    setIsDeletingPage(true)
    setPageErrorMessage('')
    setPageStatusMessage('')

    try {
      const response = await deletePage(siteId, selectedPage.id)
      applyDraftUpdate(response.draft)
      setPageStatusMessage('Page removed from the draft.')
    } catch (error) {
      setPageErrorMessage(
        error instanceof APIError ? error.message : 'Could not delete page',
      )
    } finally {
      setIsDeletingPage(false)
    }
  }

  async function handleMovePage(pageId: string, direction: -1 | 1) {
    if (!draft) {
      return
    }
    const nextOrder = moveItem(draft.pages, pageId, direction)
    if (!nextOrder) {
      return
    }

    setIsSavingPage(true)
    setPageErrorMessage('')
    setPageStatusMessage('')

    try {
      const response = await reorderPages(
        siteId,
        nextOrder.map((page) => page.id),
      )
      applyDraftUpdate(response.draft, pageId, selectedBlock?.id)
      setPageStatusMessage('Page order updated.')
    } catch (error) {
      setPageErrorMessage(
        error instanceof APIError ? error.message : 'Could not reorder pages',
      )
    } finally {
      setIsSavingPage(false)
    }
  }

  async function handleMoveNavigation(pageId: string, direction: -1 | 1) {
    if (!draft) {
      return
    }
    const currentNavigationPages = getNavigationPages(draft)
    const nextOrder = moveItem(currentNavigationPages, pageId, direction)
    if (!nextOrder) {
      return
    }

    setIsSavingNavigation(true)
    setNavigationErrorMessage('')
    setNavigationStatusMessage('')

    try {
      const response = await reorderSiteNavigation(
        siteId,
        nextOrder.map((page) => page.id),
      )
      applyDraftUpdate(response.draft, selectedPage?.id, selectedBlock?.id)
      setNavigationStatusMessage('Primary navigation order updated.')
    } catch (error) {
      setNavigationErrorMessage(
        error instanceof APIError
          ? error.message
          : 'Could not reorder navigation',
      )
    } finally {
      setIsSavingNavigation(false)
    }
  }

  async function handleSaveBlock(
    props: Record<string, unknown>,
    hidden: boolean,
  ) {
    if (!selectedPage || !selectedBlock) {
      return
    }

    setIsSavingBlock(true)
    setBlockErrorMessage('')
    setBlockStatusMessage('')

    try {
      const response = await updateBlock(
        siteId,
        selectedPage.id,
        selectedBlock.id,
        {
          props,
          hidden,
        },
      )
      applyDraftUpdate(response.draft, selectedPage.id, selectedBlock.id)
      setBlockStatusMessage('Block changes saved.')
    } catch (error) {
      setBlockErrorMessage(
        error instanceof APIError ? error.message : 'Could not save block',
      )
    } finally {
      setIsSavingBlock(false)
    }
  }

  async function handleUploadAsset(event: FormEvent<HTMLFormElement>) {
    event.preventDefault()
    if (!assetFile) {
      setAssetErrorMessage('Choose an image file before uploading.')
      setAssetStatusMessage('')
      return
    }

    setIsUploadingAsset(true)
    setAssetErrorMessage('')
    setAssetStatusMessage('')

    try {
      const ticket = await createAssetUploadURL({
        siteId,
        fileName: assetFile.name,
        contentType: assetFile.type,
        sizeBytes: assetFile.size,
        altText: assetAltText || undefined,
      })

      const uploadHeaders = new Headers(ticket.upload.headers ?? {})
      if (!uploadHeaders.has('Content-Type') && assetFile.type) {
        uploadHeaders.set('Content-Type', assetFile.type)
      }

      const uploadResponse = await fetch(ticket.upload.url, {
        method: ticket.upload.method || 'PUT',
        headers: uploadHeaders,
        body: assetFile,
      })
      if (!uploadResponse.ok) {
        throw new Error(
          `Storage upload failed with status ${uploadResponse.status}`,
        )
      }

      const dimensions = await readImageDimensions(assetFile).catch(() => null)
      const completed = await completeAssetUpload(ticket.asset.id, {
        altText: assetAltText || undefined,
        width: dimensions?.width,
        height: dimensions?.height,
      })

      setSiteAssets((current) => [
        completed.asset,
        ...current.filter((asset) => asset.id !== completed.asset.id),
      ])
      setAssetFile(null)
      setAssetAltText('')
      setAssetInputKey((current) => current + 1)
      setAssetStatusMessage('Asset uploaded and ready for block fields.')
    } catch (error) {
      setAssetErrorMessage(
        error instanceof APIError
          ? error.message
          : error instanceof Error
            ? error.message
            : 'Could not upload asset',
      )
    } finally {
      setIsUploadingAsset(false)
    }
  }

  function handleThemeSelectionChange(
    field: keyof ThemeSelection,
    value: string,
  ) {
    setThemeSelection((current) =>
      current
        ? {
            ...current,
            [field]: value,
          }
        : current,
    )
    setThemeErrorMessage('')
    setThemeStatusMessage('')
  }

  async function handleSaveTheme(event: FormEvent<HTMLFormElement>) {
    event.preventDefault()
    if (!themeSelection) {
      return
    }

    setIsSavingTheme(true)
    setThemeErrorMessage('')
    setThemeStatusMessage('')

    try {
      const response = await updateSiteTheme(siteId, themeSelection)
      setThemeSelection(response.selection)
      setThemeOptions(response.options)
      setDraft((current) =>
        current
          ? {
              ...current,
              theme: response.theme,
            }
          : current,
      )
      setThemeStatusMessage('Theme saved for preview and publish.')
    } catch (error) {
      setThemeErrorMessage(
        error instanceof APIError ? error.message : 'Could not save theme',
      )
    } finally {
      setIsSavingTheme(false)
    }
  }

  async function handleUpdateSubmissionStatus(
    submissionId: string,
    status: FormSubmissionStatus,
  ) {
    setActiveSubmissionId(submissionId)
    setSubmissionErrorMessage('')
    setSubmissionStatusMessage('')

    try {
      const response = await updateFormSubmission(submissionId, { status })
      setFormSubmissions((current) =>
        current.map((submission) =>
          submission.id === submissionId ? response.submission : submission,
        ),
      )
      setSubmissionStatusMessage('Submission status saved.')
    } catch (error) {
      setSubmissionErrorMessage(
        error instanceof APIError
          ? error.message
          : 'Could not update submission status',
      )
    } finally {
      setActiveSubmissionId('')
    }
  }

  async function handleCreateBlock(event: FormEvent<HTMLFormElement>) {
    event.preventDefault()
    const blockTypeToCreate = newBlockType || blockRegistry[0]?.type || ''
    if (!selectedPage || !blockTypeToCreate) {
      return
    }

    setIsCreatingBlock(true)
    setBlockErrorMessage('')
    setBlockStatusMessage('')

    try {
      const response = await createBlock(siteId, selectedPage.id, {
        type: blockTypeToCreate,
      })
      const createdBlock = findNewBlock(
        draft?.pages.find((page) => page.id === selectedPage.id) ?? null,
        response.draft.pages.find((page) => page.id === selectedPage.id) ??
          null,
      )
      applyDraftUpdate(response.draft, selectedPage.id, createdBlock?.id)
      setBlockStatusMessage('Block added to the page.')
    } catch (error) {
      setBlockErrorMessage(
        error instanceof APIError ? error.message : 'Could not add block',
      )
    } finally {
      setIsCreatingBlock(false)
    }
  }

  async function handleDuplicateBlock() {
    if (!selectedPage || !selectedBlock) {
      return
    }

    setIsMutatingBlocks(true)
    setBlockErrorMessage('')
    setBlockStatusMessage('')

    try {
      const response = await duplicateBlock(
        siteId,
        selectedPage.id,
        selectedBlock.id,
      )
      const duplicatedBlock = findNewBlock(
        draft?.pages.find((page) => page.id === selectedPage.id) ?? null,
        response.draft.pages.find((page) => page.id === selectedPage.id) ??
          null,
      )
      applyDraftUpdate(response.draft, selectedPage.id, duplicatedBlock?.id)
      setBlockStatusMessage('Block duplicated.')
    } catch (error) {
      setBlockErrorMessage(
        error instanceof APIError ? error.message : 'Could not duplicate block',
      )
    } finally {
      setIsMutatingBlocks(false)
    }
  }

  async function handleDeleteBlock() {
    if (!selectedPage || !selectedBlock) {
      return
    }
    const confirmed = window.confirm(
      `Delete the ${selectedDefinition?.displayName ?? selectedBlock.type} block?`,
    )
    if (!confirmed) {
      return
    }

    setIsMutatingBlocks(true)
    setBlockErrorMessage('')
    setBlockStatusMessage('')

    try {
      const response = await deleteBlock(
        siteId,
        selectedPage.id,
        selectedBlock.id,
      )
      applyDraftUpdate(response.draft, selectedPage.id)
      setBlockStatusMessage('Block removed from the page.')
    } catch (error) {
      setBlockErrorMessage(
        error instanceof APIError ? error.message : 'Could not delete block',
      )
    } finally {
      setIsMutatingBlocks(false)
    }
  }

  async function handleMoveBlock(direction: -1 | 1) {
    if (!draft || !selectedPage || !selectedBlock) {
      return
    }
    const page = draft.pages.find(
      (candidate) => candidate.id === selectedPage.id,
    )
    if (!page) {
      return
    }
    const nextOrder = moveItem(page.blocks, selectedBlock.id, direction)
    if (!nextOrder) {
      return
    }

    setIsMutatingBlocks(true)
    setBlockErrorMessage('')
    setBlockStatusMessage('')

    try {
      const response = await reorderBlocks(
        siteId,
        selectedPage.id,
        nextOrder.map((block) => block.id),
      )
      applyDraftUpdate(response.draft, selectedPage.id, selectedBlock.id)
      setBlockStatusMessage('Block order updated.')
    } catch (error) {
      setBlockErrorMessage(
        error instanceof APIError ? error.message : 'Could not reorder blocks',
      )
    } finally {
      setIsMutatingBlocks(false)
    }
  }

  async function handleReorderBlocks(blockIds: string[]) {
    if (!selectedPage) return
    setIsMutatingBlocks(true)
    setBlockErrorMessage('')
    setBlockStatusMessage('')

    try {
      const response = await reorderBlocks(siteId, selectedPage.id, blockIds)
      applyDraftUpdate(response.draft, selectedPage.id, selectedBlock?.id)
      setBlockStatusMessage('Blocks reordered.')
    } catch (error) {
      setBlockErrorMessage(
        error instanceof APIError ? error.message : 'Could not reorder blocks',
      )
    } finally {
      setIsMutatingBlocks(false)
    }
  }

  async function handleDropPaletteBlock(blockType: string, targetIndex: number) {
    if (!selectedPage) return
    setIsCreatingBlock(true)
    setBlockErrorMessage('')
    setBlockStatusMessage('')

    try {
      const response = await createBlock(siteId, selectedPage.id, {
        type: blockType,
      })
      const previousPage =
        draft?.pages.find((p) => p.id === selectedPage.id) ?? null
      const nextPage =
        response.draft.pages.find((p) => p.id === selectedPage.id) ?? null
      const createdBlock = findNewBlock(previousPage, nextPage)
      if (!nextPage || !createdBlock) {
        applyDraftUpdate(response.draft, selectedPage.id, createdBlock?.id)
        setBlockStatusMessage('Block added to the page.')
        return
      }

      const visibleBlocks = nextPage.blocks.filter(
        (block) => !block.settings?.hidden,
      )
      const visibleOrder = visibleBlocks.map((block) => block.id)
      const createdVisibleIndex = visibleOrder.findIndex(
        (blockID) => blockID === createdBlock.id,
      )

      if (createdVisibleIndex === -1) {
        applyDraftUpdate(response.draft, selectedPage.id, createdBlock.id)
        setBlockStatusMessage('Block added to the page.')
        return
      }

      const reorderedVisible = [...visibleOrder]
      reorderedVisible.splice(createdVisibleIndex, 1)
      reorderedVisible.splice(
        Math.max(0, Math.min(targetIndex, reorderedVisible.length)),
        0,
        createdBlock.id,
      )

      const hiddenIDs = nextPage.blocks
        .filter((block) => block.settings?.hidden)
        .map((block) => block.id)

      const reordered = await reorderBlocks(siteId, selectedPage.id, [
        ...reorderedVisible,
        ...hiddenIDs,
      ])
      applyDraftUpdate(reordered.draft, selectedPage.id, createdBlock.id)
      setBlockStatusMessage('Block added to the page.')
    } catch (error) {
      setBlockErrorMessage(
        error instanceof APIError ? error.message : 'Could not add block',
      )
    } finally {
      setIsCreatingBlock(false)
    }
  }

  async function handleSiteReprompt(event: FormEvent<HTMLFormElement>) {
    event.preventDefault()
    setIsRepromptingSite(true)
    setRepromptErrorMessage('')
    setRepromptStatusMessage('')

    try {
      const response = await repromptSite(siteId, { prompt: siteReprompt })
      applyDraftUpdate(response.draft)
      setSiteReprompt('')
      setPageReprompt('')
      setRepromptStatusMessage(
        'Site regenerated. The previous draft is available through undo.',
      )
    } catch (error) {
      setRepromptErrorMessage(
        error instanceof APIError ? error.message : 'Could not re-prompt site',
      )
    } finally {
      setIsRepromptingSite(false)
    }
  }

  async function handlePageReprompt(event: FormEvent<HTMLFormElement>) {
    event.preventDefault()
    if (!selectedPage) {
      return
    }

    setIsRepromptingPage(true)
    setRepromptErrorMessage('')
    setRepromptStatusMessage('')

    try {
      const response = await repromptPage(siteId, selectedPage.id, {
        prompt: pageReprompt,
      })
      applyDraftUpdate(response.draft, selectedPage.id)
      setPageReprompt('')
      setRepromptStatusMessage(
        `${selectedPage.title} was regenerated. The previous draft is available through undo.`,
      )
    } catch (error) {
      setRepromptErrorMessage(
        error instanceof APIError ? error.message : 'Could not re-prompt page',
      )
    } finally {
      setIsRepromptingPage(false)
    }
  }

  async function handleUndoReprompt() {
    setIsUndoingReprompt(true)
    setRepromptErrorMessage('')
    setRepromptStatusMessage('')

    try {
      const response = await undoSiteReprompt(siteId)
      applyDraftUpdate(response.draft, selectedPage?.id, selectedBlock?.id)
      setRepromptStatusMessage('Previous draft revision restored.')
    } catch (error) {
      setRepromptErrorMessage(
        error instanceof APIError ? error.message : 'Could not restore draft revision',
      )
    } finally {
      setIsUndoingReprompt(false)
    }
  }

  function handleSelectPage(pageId: string) {
    const page = draft?.pages.find((p) => p.id === pageId) ?? null
    if (!page || !draft) return
    setSelectedPageId(pageId)
    setSelectedBlockId(page.blocks[0]?.id ?? '')
    syncSelectedPageFields(draft, page)
    setBlockErrorMessage('')
    setBlockStatusMessage('')
    setPageErrorMessage('')
    setPageStatusMessage('')
    setRepromptErrorMessage('')
  }

  function handleSelectBlock(blockId: string) {
    if (!draft || !selectedPage) return
    setSelectedBlockId(blockId)
    setBlockErrorMessage('')
    setBlockStatusMessage('')
  }

  async function handleDelete() {
    const confirmed = window.confirm(
      'Delete this site draft? This removes the stored draft and its pages.',
    )
    if (!confirmed) {
      return
    }

    setIsDeleting(true)
    setSiteErrorMessage('')

    try {
      await deleteSite(siteId)
      await navigate({ to: '/app' })
    } catch (error) {
      setSiteErrorMessage(
        error instanceof APIError ? error.message : 'Could not delete site',
      )
      setIsDeleting(false)
    }
  }

  async function handlePublish() {
    setIsPublishing(true)
    setPublishErrorMessage('')
    setPublishStatusMessage('')

    try {
      const response = await publishSite(siteId, { publishNote })
      const versionResponse = await listSiteVersions(siteId)
      setVersions(versionResponse.versions)
      setPublishStatusMessage(
        `Published version ${response.version.versionNumber}.`,
      )
    } catch (error) {
      setPublishErrorMessage(
        error instanceof APIError ? error.message : 'Could not publish site',
      )
    } finally {
      setIsPublishing(false)
    }
  }

  async function handleRollback(version: SiteVersion) {
    if (version.isCurrent) {
      return
    }
    const confirmed = window.confirm(
      `Roll the live site back to version ${version.versionNumber}? The current draft will stay editable.`,
    )
    if (!confirmed) {
      return
    }

    setActiveRollbackVersionId(version.id)
    setPublishErrorMessage('')
    setPublishStatusMessage('')

    try {
      const response = await rollbackSiteVersion(siteId, version.id)
      const versionResponse = await listSiteVersions(siteId)
      setVersions(versionResponse.versions)
      setPublishStatusMessage(
        `Rolled back live site to version ${response.version.versionNumber}.`,
      )
    } catch (error) {
      setPublishErrorMessage(
        error instanceof APIError ? error.message : 'Could not roll back site',
      )
    } finally {
      setActiveRollbackVersionId('')
    }
  }

  if (isLoading) {
    return (
      <div className={ribbonPanel}>
        <p className={text.p}>Loading site...</p>
      </div>
    )
  }

  if (!draft) {
    return (
      <div className={ribbonPanel}>
        <p className={text.error}>{loadErrorMessage || 'Site not found'}</p>
      </div>
    )
  }

  const currentVersion = versions.find((version) => version.isCurrent) ?? null
  const uploadedSiteAssets = siteAssets.filter(
    (asset) => asset.metadata.uploadStatus === 'uploaded',
  )
  const selectedBlockIndex =
    selectedPage && selectedBlock
      ? selectedPage.blocks.findIndex((block) => block.id === selectedBlock.id)
      : -1
  const navigationPages = getNavigationPages(draft)
  const hiddenNavigationPageCount = draft.pages.filter(
    (page) => !draft.navigation.primary.some((item) => item.pageId === page.id),
  ).length
  const externalNavigationCount = draft.navigation.primary.filter(
    (item) => !item.pageId && item.href,
  ).length
  const hasContactForm = draft.pages.some((page) =>
    page.blocks.some((block) => block.type === 'contact_form'),
  )

  return (
    <PuckBuilder
      siteId={siteId}
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
      pageTitle={pageTitle}
      pageSlug={pageSlug}
      pageSEOTitle={pageSEOTitle}
      pageSEODescription={pageSEODescription}
      pageIncludeInNavigation={pageIncludeInNavigation}
      isSavingPage={isSavingPage}
      isDeletingPage={isDeletingPage}
      pages={draft.pages}
      onSelectPage={handleSelectPage}
      onSelectBlock={handleSelectBlock}
      onSaveBlock={handleSaveBlock}
      onCreateBlock={handleCreateBlock}
      onDuplicateBlock={handleDuplicateBlock}
      onDeleteBlock={handleDeleteBlock}
      onMoveBlock={handleMoveBlock}
      onMovePage={handleMovePage}
      onDeletePage={handleDeletePage}
      onChangeNewBlockType={setNewBlockType}
      onSavePage={handleSavePage}
      onSetPageTitle={setPageTitle}
      onSetPageSlug={setPageSlug}
      onSetPageSEOTitle={setPageSEOTitle}
      onSetPageSEODescription={setPageSEODescription}
      onSetPageIncludeInNavigation={setPageIncludeInNavigation}
      onReorderBlocks={handleReorderBlocks}
      onDropPaletteBlock={handleDropPaletteBlock}
      settingsContent={
        <>
          <section className={ribbonPanel}>
            <div className="mb-[22px]">
              <p className={text.eyebrow}>Prompt iteration</p>
              <h2 className={text.h2}>Replace draft direction</h2>
              <p className={cn(text.p, 'mt-2')}>
                Re-prompts replace the targeted content scope. They do not try
                to merge block-by-block edits. Use undo if the new result is
                worse.
              </p>
            </div>

            <div className="grid gap-4">
              <form className={form.grid} onSubmit={handleSiteReprompt}>
                <label htmlFor="site-reprompt" className={text.label}>
                  Whole site prompt
                </label>
                <Textarea
                  id="site-reprompt"
                  rows={4}
                  value={siteReprompt}
                  placeholder="Make the site warmer, tighten the copy, add pricing, and lean harder into workshops."
                  onChange={(event) => setSiteReprompt(event.target.value)}
                />
                <p className={form.hint}>
                  Replaces the generated content across the draft while keeping
                  the site identity and current draft version history.
                </p>
                <Button
                  type="submit"
                  size="sm"
                  disabled={isRepromptingSite || siteReprompt.trim() === ''}
                >
                  {isRepromptingSite ? 'Rebuilding site...' : 'Re-prompt site'}
                </Button>
              </form>

              <form className={form.grid} onSubmit={handlePageReprompt}>
                <label htmlFor="page-reprompt" className={text.label}>
                  {selectedPage
                    ? `${selectedPage.title} page prompt`
                    : 'Page prompt'}
                </label>
                <Textarea
                  id="page-reprompt"
                  rows={4}
                  value={pageReprompt}
                  placeholder="Turn this page into a tighter pricing overview with clearer package framing and fewer sections."
                  onChange={(event) => setPageReprompt(event.target.value)}
                />
                <p className={form.hint}>
                  Replaces the selected page content while keeping its route and
                  position in the site.
                </p>
                <div className={actions.row}>
                  <Button
                    type="submit"
                    size="sm"
                    disabled={
                      !selectedPage ||
                      isRepromptingPage ||
                      pageReprompt.trim() === ''
                    }
                  >
                    {isRepromptingPage
                      ? 'Rebuilding page...'
                      : 'Re-prompt page'}
                  </Button>
                  <Button
                    type="button"
                    size="sm"
                    variant="outline"
                    disabled={isUndoingReprompt}
                    onClick={handleUndoReprompt}
                  >
                    {isUndoingReprompt ? 'Restoring...' : 'Undo last re-prompt'}
                  </Button>
                </div>
              </form>

              {repromptErrorMessage ? (
                <p className={text.error}>{repromptErrorMessage}</p>
              ) : null}
              {repromptStatusMessage ? (
                <p className={text.success}>{repromptStatusMessage}</p>
              ) : null}
            </div>
          </section>

          <section className={ribbonPanel}>
            <div className="mb-[22px]">
              <p className={text.eyebrow}>Navigation</p>
              <h2 className={text.h2}>Set the menu order</h2>
            </div>

            {navigationPages.length > 0 ? (
              <div className="grid gap-3">
                {navigationPages.map((page, index) => (
                  <article
                    key={page.id}
                    className={cn(
                      'grid gap-3 rounded-[14px] border border-border bg-[var(--surface-2)] p-4',
                    )}
                  >
                    <div className="flex items-center justify-between gap-3">
                      <div>
                        <strong className="block text-[var(--paper)]">
                          {page.title}
                        </strong>
                        <small className="text-[var(--paper-muted)]">
                          {page.slug}
                        </small>
                      </div>
                      <div className={actions.row}>
                        <Button
                          type="button"
                          variant="plain"
                          className={actions.inlineLink}
                          disabled={index === 0 || isSavingNavigation}
                          onClick={() => handleMoveNavigation(page.id, -1)}
                        >
                          Earlier
                        </Button>
                        <Button
                          type="button"
                          variant="plain"
                          className={actions.inlineLink}
                          disabled={
                            index === navigationPages.length - 1 ||
                            isSavingNavigation
                          }
                          onClick={() => handleMoveNavigation(page.id, 1)}
                        >
                          Later
                        </Button>
                      </div>
                    </div>
                  </article>
                ))}

                {navigationErrorMessage ? (
                  <p className={text.error}>{navigationErrorMessage}</p>
                ) : null}
                {navigationStatusMessage ? (
                  <p className={text.success}>{navigationStatusMessage}</p>
                ) : null}

                <p className={form.hint}>
                  {hiddenNavigationPageCount > 0
                    ? `${hiddenNavigationPageCount} page${hiddenNavigationPageCount === 1 ? '' : 's'} currently stay out of the primary navigation.`
                    : 'Every page is currently included in the primary navigation.'}
                  {externalNavigationCount > 0
                    ? ` ${externalNavigationCount} external link${externalNavigationCount === 1 ? '' : 's'} still stay appended after the page links.`
                    : ''}
                </p>
              </div>
            ) : (
              <div className={emptyState}>
                <p className={text.p}>
                  Include at least one page in the primary navigation to reorder
                  it.
                </p>
              </div>
            )}
          </section>

          <section className={ribbonPanel}>
            <div className="mb-[22px]">
              <p className={text.eyebrow}>Pages</p>
              <h2 className={text.h2}>Add another page</h2>
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
                Page slug
              </label>
              <Input
                id="new-page-slug"
                value={newPageSlug}
                onChange={(event) => setNewPageSlug(event.target.value)}
                placeholder="/pricing"
              />

              <label className={form.toggle}>
                <input
                  type="checkbox"
                  className="size-4 accent-[var(--thread-teal)]"
                  checked={newPageIncludeInNavigation}
                  onChange={(event) =>
                    setNewPageIncludeInNavigation(event.target.checked)
                  }
                />
                Include this page in primary navigation
              </label>

              <Button
                type="submit"
                size="sm"
                disabled={isCreatingPage || draft.pages.length >= 10}
              >
                {isCreatingPage ? 'Adding page...' : 'Add page'}
              </Button>

              <p className={form.hint}>
                {draft.pages.length >= 10
                  ? 'This draft already has the 10-page MVP limit.'
                  : `${draft.pages.length} of 10 pages currently in this draft.`}
              </p>
            </form>
          </section>

          <section className={ribbonPanel}>
            <div className="mb-[22px]">
              <p className={text.eyebrow}>Theme</p>
              <h2 className={text.h2}>Set the site direction</h2>
            </div>

            {themeSelection && themeOptions ? (
              <form className={form.grid} onSubmit={handleSaveTheme}>
                <div className="rounded-[16px] border border-border bg-[var(--surface-2)] p-4">
                  <div className="grid grid-cols-2 gap-3 max-lg:grid-cols-1">
                    {Object.entries(draft.theme.tokens.colors).map(
                      ([key, value]) => (
                        <div
                          key={key}
                          className="flex items-center gap-3 rounded-[14px] border border-border bg-[var(--surface-1)] px-3 py-2.5"
                        >
                          <span
                            className="size-[34px] shrink-0 rounded-full border border-border shadow-[inset_0_0_0_1px_oklch(7%_0.022_336_/_0.12)]"
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

                <label htmlFor="theme-palette" className={text.label}>
                  Palette
                </label>
                <Select
                  id="theme-palette"
                  value={themeSelection.palette}
                  onChange={(event) =>
                    handleThemeSelectionChange('palette', event.target.value)
                  }
                >
                  {themeOptions.palettes.map((option) => (
                    <option key={option.id} value={option.id}>
                      {option.label}
                    </option>
                  ))}
                </Select>
                <p className={form.hint}>
                  {describeThemeOption(
                    themeOptions.palettes,
                    themeSelection.palette,
                  )}
                </p>

                <label htmlFor="theme-font-preset" className={text.label}>
                  Font preset
                </label>
                <Select
                  id="theme-font-preset"
                  value={themeSelection.fontPreset}
                  onChange={(event) =>
                    handleThemeSelectionChange('fontPreset', event.target.value)
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

                <label htmlFor="theme-section-spacing" className={text.label}>
                  Section spacing
                </label>
                <Select
                  id="theme-section-spacing"
                  value={themeSelection.sectionSpacing}
                  onChange={(event) =>
                    handleThemeSelectionChange(
                      'sectionSpacing',
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

                <label htmlFor="theme-radius" className={text.label}>
                  Corner radius
                </label>
                <Select
                  id="theme-radius"
                  value={themeSelection.radius}
                  onChange={(event) =>
                    handleThemeSelectionChange('radius', event.target.value)
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

                <label htmlFor="theme-button-style" className={text.label}>
                  Button style
                </label>
                <Select
                  id="theme-button-style"
                  value={themeSelection.buttonStyle}
                  onChange={(event) =>
                    handleThemeSelectionChange(
                      'buttonStyle',
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

                <label htmlFor="theme-image-style" className={text.label}>
                  Image style
                </label>
                <Select
                  id="theme-image-style"
                  value={themeSelection.imageStyle}
                  onChange={(event) =>
                    handleThemeSelectionChange(
                      'imageStyle',
                      event.target.value,
                    )
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

                {themeErrorMessage ? (
                  <p className={text.error}>{themeErrorMessage}</p>
                ) : null}
                {themeStatusMessage ? (
                  <p className={text.success}>{themeStatusMessage}</p>
                ) : null}

                <Button type="submit" disabled={isSavingTheme}>
                  {isSavingTheme ? 'Saving theme...' : 'Save theme'}
                </Button>
              </form>
            ) : (
              <div className={emptyState}>
                <p className={text.p}>Loading theme controls...</p>
              </div>
            )}
          </section>

          <section className={ribbonPanel}>
            <div className="mb-[22px]">
              <p className={text.eyebrow}>Assets</p>
              <h2 className={text.h2}>Upload the site image library</h2>
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
                onChange={(event) =>
                  setAssetFile(event.target.files?.[0] ?? null)
                }
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
                {isUploadingAsset ? 'Uploading asset...' : 'Upload asset'}
              </Button>
            </form>

            <div className="mt-5 grid gap-3">
              {siteAssets.length > 0 ? (
                siteAssets.map((asset) => (
                  <article
                    key={asset.id}
                    className="grid gap-3 rounded-[14px] border border-border bg-[var(--surface-2)] p-4"
                  >
                    <div className="grid gap-3 sm:grid-cols-[120px_minmax(0,1fr)] sm:items-start">
                      {asset.metadata.uploadStatus === 'uploaded' ? (
                        <img
                          src={buildDraftAssetURL(asset.id)}
                          alt={
                            asset.altText ||
                            asset.metadata.fileName ||
                            'Uploaded site asset'
                          }
                          className="h-[108px] w-full rounded-[12px] border border-border bg-[var(--surface-1)] object-cover"
                          loading="lazy"
                        />
                      ) : (
                        <div className="grid h-[108px] w-full place-items-center rounded-[12px] border border-dashed border-border bg-[var(--surface-1)] text-sm text-[var(--paper-muted)]">
                          Processing upload
                        </div>
                      )}
                      <div className="grid gap-1">
                        <strong className="text-[var(--paper)]">
                          {asset.metadata.fileName || asset.id}
                        </strong>
                        <small className="text-[var(--paper-muted)]">
                          {asset.metadata.contentType || 'Image'} ·{' '}
                          {formatAssetFileSize(
                            asset.metadata.sizeBytes ||
                              asset.metadata.requestedSizeBytes,
                          )}
                        </small>
                        <small className="text-[var(--paper-muted)]">
                          {describeAssetDimensions(asset)} ·{' '}
                          {asset.metadata.uploadStatus || 'pending'}
                        </small>
                        {asset.altText ? (
                          <p className="m-0 text-sm text-[var(--paper-muted)]">
                            Alt: {asset.altText}
                          </p>
                        ) : null}
                      </div>
                    </div>
                  </article>
                ))
              ) : (
                <div className={emptyState}>
                  <p className={text.p}>
                    No site assets yet. Upload the first image here, then pick
                    it in any asset-enabled block field below.
                  </p>
                </div>
              )}
            </div>
          </section>

          <section className={ribbonPanel}>
            <div className="mb-[22px]">
              <p className={text.eyebrow}>Inquiries</p>
              <h2 className={text.h2}>Review contact form submissions</h2>
            </div>

            {submissionErrorMessage ? (
              <p className={text.error}>{submissionErrorMessage}</p>
            ) : null}
            {submissionStatusMessage ? (
              <p className={text.success}>{submissionStatusMessage}</p>
            ) : null}

            <div className="mt-5 grid gap-3">
              {formSubmissions.length > 0 ? (
                formSubmissions.map((submission) => (
                  <article
                    key={submission.id}
                    className="grid gap-3 rounded-[14px] border border-border bg-[var(--surface-2)] p-4"
                  >
                    <div className="flex items-start justify-between gap-3 max-sm:flex-col">
                      <div>
                        <strong className="block text-[var(--paper)]">
                          {String(
                            submission.payload['name'] ||
                              submission.payload['email'] ||
                              'New inquiry',
                          )}
                        </strong>
                        <small className="text-[var(--paper-muted)]">
                          {submission.pageTitle || 'Stored submission'} ·{' '}
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
                      {Object.entries(submission.payload).map(
                        ([key, value]) => (
                          <div key={key} className="grid gap-1">
                            <strong className="text-sm uppercase tracking-[0.08em] text-[var(--paper-muted)]">
                              {formatSubmissionKey(key)}
                            </strong>
                            <p className="m-0 whitespace-pre-wrap text-[var(--paper)]">
                              {String(value)}
                            </p>
                          </div>
                        ),
                      )}
                    </div>
                  </article>
                ))
              ) : (
                <div className={emptyState}>
                  <p className={text.p}>
                    {hasContactForm
                      ? 'No submissions yet. Published and preview contact forms will start listing messages here.'
                      : 'Add a contact form block to start collecting inquiries.'}
                  </p>
                </div>
              )}
            </div>
          </section>

          <section className={ribbonPanel}>
            <div className="mb-[22px]">
              <p className={text.eyebrow}>Publish</p>
              <h2 className={text.h2}>Release an immutable snapshot</h2>
            </div>

            <label htmlFor="publish-note" className={text.label}>
              Publish note
            </label>
            <Textarea
              id="publish-note"
              name="publishNote"
              rows={3}
              value={publishNote}
              onChange={(event) => setPublishNote(event.target.value)}
              placeholder="What changed in this release?"
            />

            {publishErrorMessage ? (
              <p className={text.error}>{publishErrorMessage}</p>
            ) : null}
            {publishStatusMessage ? (
              <p className={text.success}>{publishStatusMessage}</p>
            ) : null}

            <div className={actions.rowLarge}>
              <Button
                type="button"
                disabled={isPublishing || activeRollbackVersionId !== ''}
                onClick={handlePublish}
              >
                {isPublishing ? 'Publishing...' : 'Publish snapshot'}
              </Button>
              {currentVersion ? (
                <Button asChild variant="plain" className={actions.inlineLink}>
                  <Link
                    to="/public/$siteSlug"
                    params={{ siteSlug: draft.site.slug }}
                  >
                    View live site
                  </Link>
                </Button>
              ) : null}
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
                    className="grid gap-2 rounded-[14px] border border-border bg-[var(--surface-2)] p-4"
                  >
                    <div>
                      <strong>
                        v{version.versionNumber}
                        {version.isCurrent ? ' current' : ''}
                      </strong>
                      <p className="m-0 text-[var(--paper-muted)]">
                        {formatTimestamp(version.createdAt)}
                      </p>
                    </div>
                    {version.publishNote ? (
                      <small className="m-0 text-[var(--paper-muted)]">
                        {version.publishNote}
                      </small>
                    ) : null}
                    {!version.isCurrent ? (
                      <div className={actions.row}>
                        <Button
                          type="button"
                          variant="plain"
                          className={actions.inlineLink}
                          disabled={
                            isPublishing || activeRollbackVersionId !== ''
                          }
                          onClick={() => handleRollback(version)}
                        >
                          {activeRollbackVersionId === version.id
                            ? 'Rolling back...'
                            : 'Roll back live site'}
                        </Button>
                      </div>
                    ) : null}
                  </article>
                ))
              )}
            </div>
          </section>

          <section className={ribbonPanel}>
            <div className="mb-[22px]">
              <p className={text.eyebrow}>Site details</p>
              <h2 className={text.h2}>Rename and reslug the draft</h2>
            </div>

            <form className={form.grid} onSubmit={handleSaveSite}>
              <label htmlFor="site-name" className={text.label}>
                Business name
              </label>
              <Input
                id="site-name"
                name="name"
                value={name}
                onChange={(event) => setName(event.target.value)}
                required
              />

              <label htmlFor="site-slug" className={text.label}>
                Slug
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

              <Button type="submit" disabled={isSavingSite}>
                {isSavingSite ? 'Saving...' : 'Save site'}
              </Button>

              <Button
                type="button"
                variant="outline"
                disabled={isDeleting}
                onClick={handleDelete}
              >
                {isDeleting ? 'Deleting...' : 'Delete draft'}
              </Button>
            </form>
          </section>
        </>
      }
    />
  )
}

function findNewPage(previousDraft: SiteDraft | null, nextDraft: SiteDraft) {
  const previousIDs = new Set(previousDraft?.pages.map((page) => page.id) ?? [])
  return (
    nextDraft.pages.find((page) => !previousIDs.has(page.id)) ??
    nextDraft.pages.at(-1)
  )
}

function findNewBlock(
  previousPage: DraftPage | null,
  nextPage: DraftPage | null,
) {
  if (!nextPage) {
    return null
  }
  const previousIDs = new Set(
    previousPage?.blocks.map((block) => block.id) ?? [],
  )
  return (
    nextPage.blocks.find((block) => !previousIDs.has(block.id)) ??
    nextPage.blocks.at(-1) ??
    null
  )
}

function getNavigationPages(draft: SiteDraft) {
  return draft.navigation.primary
    .map((item) =>
      item.pageId
        ? (draft.pages.find((page) => page.id === item.pageId) ?? null)
        : null,
    )
    .filter((page): page is DraftPage => page !== null)
}

function moveItem<T extends { id: string }>(
  items: T[],
  itemID: string,
  direction: -1 | 1,
) {
  const index = items.findIndex((item) => item.id === itemID)
  const nextIndex = index + direction
  if (index === -1 || nextIndex < 0 || nextIndex >= items.length) {
    return null
  }
  const reordered = [...items]
  ;[reordered[index], reordered[nextIndex]] = [
    reordered[nextIndex],
    reordered[index],
  ]
  return reordered
}

function formatSubmissionKey(value: string) {
  return value.replaceAll('_', ' ').replace(/^./, (char) => char.toUpperCase())
}

function formatTimestamp(value: string) {
  const date = new Date(value)
  if (Number.isNaN(date.getTime())) {
    return value
  }
  return new Intl.DateTimeFormat(undefined, {
    dateStyle: 'medium',
    timeStyle: 'short',
  }).format(date)
}

function describeThemeOption(
  options: Array<{ id: string; description?: string }>,
  selectedID: string,
) {
  return options.find((option) => option.id === selectedID)?.description ?? ''
}

function formatThemeLabel(value: string) {
  return value
    .replace(/([A-Z])/g, ' $1')
    .replace(/^./, (char) => char.toUpperCase())
}
