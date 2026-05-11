import { Link, createFileRoute, useNavigate } from '@tanstack/react-router'
import type { FormEvent } from 'react'
import { useEffect, useState } from 'react'
import { BlockEditor } from '@/components/BlockEditor'
import { Button } from '@/components/ui/button'
import { Checkbox } from '@/components/ui/checkbox'
import { Input } from '@/components/ui/input'
import { Select } from '@/components/ui/select'
import { Textarea } from '@/components/ui/textarea'
import {
  APIError,
  createBlock,
  createPage,
  deleteBlock,
  deletePage,
  deleteSite,
  duplicateBlock,
  getSiteDraft,
  getSiteTheme,
  listSiteVersions,
  publishSite,
  rollbackSiteVersion,
  reorderBlocks,
  reorderPages,
  reorderSiteNavigation,
  type BlockDefinition,
  type SiteDraft,
  type SiteVersion,
  type ThemeEditorCatalog,
  type ThemeSelection,
  updateBlock,
  updatePage,
  updateSite,
  updateSiteTheme,
} from '@/lib/api'
import {
  actions,
  emptyState,
  form,
  layout,
  ribbon,
  ribbonPanel,
  statGrid,
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
  const [newPageIncludeInNavigation, setNewPageIncludeInNavigation] = useState(true)
  const [pageTitle, setPageTitle] = useState('')
  const [pageSlug, setPageSlug] = useState('')
  const [pageSEOTitle, setPageSEOTitle] = useState('')
  const [pageSEODescription, setPageSEODescription] = useState('')
  const [pageIncludeInNavigation, setPageIncludeInNavigation] = useState(true)
  const [newBlockType, setNewBlockType] = useState('')
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
  const [themeSelection, setThemeSelection] = useState<ThemeSelection | null>(null)
  const [themeOptions, setThemeOptions] = useState<ThemeEditorCatalog | null>(null)
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
  const [publishErrorMessage, setPublishErrorMessage] = useState('')
  const [publishStatusMessage, setPublishStatusMessage] = useState('')

  function syncSelectedPageFields(nextDraft: SiteDraft, nextPage: DraftPage | null) {
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

    Promise.all([getSiteDraft(siteId), listSiteVersions(siteId), getSiteTheme(siteId)])
      .then(([draftResponse, versionResponse, themeResponse]) => {
        if (!isMounted) {
          return
        }
        setBlockRegistry(draftResponse.blockRegistry)
        setNewBlockType((current) => current || draftResponse.blockRegistry[0]?.type || '')
        setVersions(versionResponse.versions)
        setThemeSelection(themeResponse.selection)
        setThemeOptions(themeResponse.options)
        setName(draftResponse.draft.site.name)
        setSlug(draftResponse.draft.site.slug)
        const initialPage = draftResponse.draft.pages[0] ?? null
        setDraft(draftResponse.draft)
        setSelectedPageId(initialPage?.id ?? '')
        setSelectedBlockId(initialPage?.blocks[0]?.id ?? '')
        syncSelectedPageFields(draftResponse.draft, initialPage)
        setIsLoading(false)
      })
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
    draft?.pages.find((page) => page.id === selectedPageId) ?? draft?.pages[0] ?? null
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
      applyDraftUpdate(response.draft, createdPage?.id, createdPage?.blocks[0]?.id)
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
        error instanceof APIError ? error.message : 'Could not reorder navigation',
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
      const response = await updateBlock(siteId, selectedPage.id, selectedBlock.id, {
        props,
        hidden,
      })
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
        response.draft.pages.find((page) => page.id === selectedPage.id) ?? null,
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
      const response = await duplicateBlock(siteId, selectedPage.id, selectedBlock.id)
      const duplicatedBlock = findNewBlock(
        draft?.pages.find((page) => page.id === selectedPage.id) ?? null,
        response.draft.pages.find((page) => page.id === selectedPage.id) ?? null,
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
      const response = await deleteBlock(siteId, selectedPage.id, selectedBlock.id)
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
    const page = draft.pages.find((candidate) => candidate.id === selectedPage.id)
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
      setPublishStatusMessage(`Published version ${response.version.versionNumber}.`)
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

  const blockCount = draft.pages.reduce((count, page) => count + page.blocks.length, 0)
  const currentVersion = versions.find((version) => version.isCurrent) ?? null
  const selectedPageIndex = selectedPage
    ? draft.pages.findIndex((page) => page.id === selectedPage.id)
    : -1
  const selectedBlockIndex = selectedPage && selectedBlock
    ? selectedPage.blocks.findIndex((block) => block.id === selectedBlock.id)
    : -1
  const navigationPages = getNavigationPages(draft)
  const hiddenNavigationPageCount = draft.pages.filter(
    (page) => !draft.navigation.primary.some((item) => item.pageId === page.id),
  ).length
  const externalNavigationCount = draft.navigation.primary.filter(
    (item) => !item.pageId && item.href,
  ).length

  return (
    <div className={layout.builderGrid}>
      <section className={ribbonPanel}>
        <div className="mb-[22px]">
          <p className={text.eyebrow}>Site</p>
          <h1 className={text.h1}>{draft.site.name}</h1>
          <p className={text.p}>
            Manage pages and approved blocks here, then keep using the validated
            preview and publish flow.
          </p>
        </div>

        <dl className={statGrid.list}>
          <div className={statGrid.item}>
            <dt className={text.eyebrow}>Status</dt>
            <dd className={statGrid.value}>{currentVersion ? 'published' : draft.site.status}</dd>
          </div>
          <div className={statGrid.item}>
            <dt className={text.eyebrow}>Pages</dt>
            <dd className={statGrid.value}>{draft.pages.length}</dd>
          </div>
          <div className={statGrid.item}>
            <dt className={text.eyebrow}>Blocks</dt>
            <dd className={statGrid.value}>{blockCount}</dd>
          </div>
        </dl>

        <div className={cn(actions.rowLarge, 'mt-[18px]')}>
          <Button asChild variant="plain" className={actions.inlineLink}>
            <Link to="/app/sites/$siteId/preview" params={{ siteId }}>
              Open preview
            </Link>
          </Button>
          {currentVersion ? (
            <Button asChild variant="plain" className={actions.inlineLink}>
              <Link to="/public/$siteSlug" params={{ siteSlug: draft.site.slug }}>
                Open live site
              </Link>
            </Button>
          ) : null}
        </div>

        <form className={cn(ribbonPanel, form.grid, 'mt-6')} onSubmit={handleCreatePage}>
          <div className="mb-[22px]">
            <div>
              <p className={text.eyebrow}>Pages</p>
              <h2 className={text.h2}>Add a page</h2>
            </div>
          </div>

          <label htmlFor="new-page-title" className={text.label}>Page title</label>
          <Input
            id="new-page-title"
            value={newPageTitle}
            onChange={(event) => setNewPageTitle(event.target.value)}
            placeholder="Contact"
            required
          />

          <label htmlFor="new-page-slug" className={text.label}>Slug</label>
          <Input
            id="new-page-slug"
            value={newPageSlug}
            onChange={(event) => setNewPageSlug(event.target.value)}
            placeholder="/contact"
          />

          <label className={form.toggle}>
            <Checkbox
              checked={newPageIncludeInNavigation}
              onChange={(event) => setNewPageIncludeInNavigation(event.target.checked)}
            />
            Include this page in site navigation
          </label>

          <Button type="submit" disabled={isCreatingPage}>
            {isCreatingPage ? 'Adding page...' : 'Add page'}
          </Button>
        </form>

        <div className="mt-[18px] grid gap-[18px]">
          {draft.pages.map((page, index) => {
            const isSelectedPage = page.id === selectedPage?.id
            const isIncludedInNavigation = draft.navigation.primary.some(
              (item) => item.pageId === page.id,
            )

            return (
              <article
                key={page.id}
                className={cn(
                  ribbon,
                  'grid gap-4 rounded-[16px] border border-border bg-[var(--surface-2)] p-5',
                  isSelectedPage &&
                    'border-[var(--thread-teal)] bg-[color-mix(in_oklch,var(--surface-2)_78%,var(--thread-teal))]',
                )}
              >
                <div className="flex items-center justify-between gap-[18px] max-sm:flex-col max-sm:items-start">
                  <Button
                    type="button"
                    variant="plain"
                    className="flex w-full items-center justify-between gap-[18px] p-0 text-left text-inherit max-sm:flex-col max-sm:items-start"
                    onClick={() => {
                      setSelectedPageId(page.id)
                      setSelectedBlockId(page.blocks[0]?.id ?? '')
                      syncSelectedPageFields(draft, page)
                      setPageErrorMessage('')
                      setPageStatusMessage('')
                    }}
                  >
                    <div>
                      <h3 className={cn(text.h3, 'mb-2')}>{page.title}</h3>
                      <p className={text.p}>{page.slug}</p>
                    </div>
                    <strong className="text-[0.96rem] text-primary">{page.blocks.length} blocks</strong>
                  </Button>

                  <div className={actions.row}>
                    <Button
                      type="button"
                      variant="plain"
                      className={actions.inlineLink}
                      disabled={index === 0 || isSavingPage}
                      onClick={() => handleMovePage(page.id, -1)}
                    >
                      Earlier
                    </Button>
                    <Button
                      type="button"
                      variant="plain"
                      className={actions.inlineLink}
                      disabled={index === draft.pages.length - 1 || isSavingPage}
                      onClick={() => handleMovePage(page.id, 1)}
                    >
                      Later
                    </Button>
                  </div>
                </div>

                <div className="text-sm text-[var(--paper-muted)]">
                  <span>{isIncludedInNavigation ? 'In navigation' : 'Hidden from navigation'}</span>
                </div>

                <div className="grid gap-3">
                  {page.blocks.map((block) => {
                    const definition =
                      blockDefinitions.get(`${block.type}@${block.version}`)
                    const isSelected =
                      page.id === selectedPage?.id && block.id === selectedBlock?.id

                    return (
                      <Button
                        key={block.id}
                        type="button"
                        variant="plain"
                        className={cn(
                          'flex items-center justify-between gap-3.5 rounded-[14px] border border-border bg-[var(--surface-1)] px-4 py-3 text-left text-[var(--paper)] transition-[background,border-color,transform] hover:-translate-y-px hover:border-[var(--thread-teal)] hover:bg-[var(--surface-3)]',
                          isSelected &&
                            'border-[var(--thread-teal)] bg-[var(--surface-3)]',
                        )}
                        onClick={() => {
                          setSelectedPageId(page.id)
                          setSelectedBlockId(block.id)
                          syncSelectedPageFields(draft, page)
                          setBlockErrorMessage('')
                          setBlockStatusMessage('')
                        }}
                      >
                        <div>
                          <span className="block font-bold">{definition?.displayName ?? block.type}</span>
                          <small className="text-sm text-[var(--paper-muted)]">{block.type}</small>
                        </div>
                        {block.settings?.hidden ? (
                          <em className="text-sm not-italic text-[var(--paper-muted)]">Hidden</em>
                        ) : (
                          <em className="text-sm not-italic text-[var(--paper-muted)]">Visible</em>
                        )}
                      </Button>
                    )
                  })}
                </div>
              </article>
            )
          })}
        </div>
      </section>

      <div className={layout.builderSidebar}>
        <section className={ribbonPanel}>
          <div className="mb-[22px]">
            <p className={text.eyebrow}>Page details</p>
            <h2 className={text.h2}>{selectedPage ? selectedPage.title : 'Choose a page'}</h2>
            <p className={text.p}>
              Edit the current page title, slug, SEO, and navigation inclusion
              before publishing the next snapshot.
            </p>
          </div>

          {selectedPage ? (
            <form className={form.grid} onSubmit={handleSavePage}>
              <label htmlFor="page-title" className={text.label}>Page title</label>
              <Input
                id="page-title"
                value={pageTitle}
                onChange={(event) => setPageTitle(event.target.value)}
                required
              />

              <label htmlFor="page-slug" className={text.label}>Slug</label>
              <Input
                id="page-slug"
                value={pageSlug}
                onChange={(event) => setPageSlug(event.target.value)}
                required
              />

              <label htmlFor="page-seo-title" className={text.label}>SEO title</label>
              <Input
                id="page-seo-title"
                value={pageSEOTitle}
                onChange={(event) => setPageSEOTitle(event.target.value)}
              />

              <label htmlFor="page-seo-description" className={text.label}>SEO description</label>
              <Textarea
                id="page-seo-description"
                rows={4}
                value={pageSEODescription}
                onChange={(event) => setPageSEODescription(event.target.value)}
              />

              <label className={form.toggle}>
                <Checkbox
                  checked={pageIncludeInNavigation}
                  onChange={(event) => setPageIncludeInNavigation(event.target.checked)}
                />
                Include this page in the primary navigation
              </label>

              {pageErrorMessage ? <p className={text.error}>{pageErrorMessage}</p> : null}
              {pageStatusMessage ? (
                <p className={text.success}>{pageStatusMessage}</p>
              ) : null}

              <div className={actions.row}>
                <Button type="submit" disabled={isSavingPage}>
                  {isSavingPage ? 'Saving page...' : 'Save page'}
                </Button>
                <Button
                  type="button"
                  variant="outline"
                  disabled={selectedPageIndex <= 0 || isSavingPage}
                  onClick={() => handleMovePage(selectedPage.id, -1)}
                >
                  Move earlier
                </Button>
                <Button
                  type="button"
                  variant="outline"
                  disabled={selectedPageIndex === draft.pages.length - 1 || isSavingPage}
                  onClick={() => handleMovePage(selectedPage.id, 1)}
                >
                  Move later
                </Button>
                <Button
                  type="button"
                  variant="outline"
                  disabled={isDeletingPage}
                  onClick={handleDeletePage}
                >
                  {isDeletingPage ? 'Deleting page...' : 'Delete page'}
                </Button>
              </div>
            </form>
          ) : (
            <div className={emptyState}>
              <p className={text.p}>Select a page from the outline to edit its details.</p>
            </div>
          )}
        </section>

        <section className={ribbonPanel}>
          <div className="mb-[22px]">
            <p className={text.eyebrow}>Navigation</p>
            <h2 className={text.h2}>Set the menu order</h2>
            <p className={text.p}>
              Keep the menu page-backed, but adjust its order independently from
              the page outline.
            </p>
          </div>

          {navigationPages.length > 0 ? (
            <div className="grid gap-3">
              {navigationPages.map((page, index) => {
                const isSelected = page.id === selectedPage?.id

                return (
                  <article
                    key={page.id}
                    className={cn(
                      'grid gap-3 rounded-[14px] border border-border bg-[var(--surface-2)] p-4',
                      isSelected &&
                        'border-[var(--thread-teal)] bg-[color-mix(in_oklch,var(--surface-2)_80%,var(--thread-teal))]',
                    )}
                  >
                    <div className="flex items-center justify-between gap-3">
                      <div>
                        <strong className="block text-[var(--paper)]">{page.title}</strong>
                        <small className="text-[var(--paper-muted)]">{page.slug}</small>
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
                            index === navigationPages.length - 1 || isSavingNavigation
                          }
                          onClick={() => handleMoveNavigation(page.id, 1)}
                        >
                          Later
                        </Button>
                      </div>
                    </div>
                  </article>
                )
              })}

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
                Include at least one page in the primary navigation to reorder it.
              </p>
            </div>
          )}
        </section>

        <section className={ribbonPanel}>
          <div className="mb-[22px]">
            <p className={text.eyebrow}>Theme</p>
            <h2 className={text.h2}>Set the site direction</h2>
            <p className={text.p}>
              Keep the visual system inside the safe theme contract while tuning
              the palette, type, spacing, and corner feel.
            </p>
          </div>

          {themeSelection && themeOptions ? (
            <form className={form.grid} onSubmit={handleSaveTheme}>
              <div className="rounded-[16px] border border-border bg-[var(--surface-2)] p-4">
                <div className="grid grid-cols-2 gap-3 max-lg:grid-cols-1">
                  {Object.entries(draft.theme.tokens.colors).map(([key, value]) => (
                    <div
                      key={key}
                      className="flex items-center gap-3 rounded-[14px] border border-border bg-[var(--surface-1)] px-3 py-2.5"
                    >
                      <span
                        className="size-[34px] shrink-0 rounded-full border border-border shadow-[inset_0_0_0_1px_oklch(7%_0.022_336_/_0.12)]"
                        style={{ backgroundColor: value }}
                      />
                      <div>
                        <strong className="block">{formatThemeLabel(key)}</strong>
                        <small className="block text-[var(--paper-muted)]">{value}</small>
                      </div>
                    </div>
                  ))}
                </div>
              </div>

              <label htmlFor="theme-palette" className={text.label}>Palette</label>
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
                {describeThemeOption(themeOptions.palettes, themeSelection.palette)}
              </p>

              <label htmlFor="theme-font-preset" className={text.label}>Font preset</label>
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
                {describeThemeOption(themeOptions.fontPresets, themeSelection.fontPreset)}
              </p>

              <label htmlFor="theme-section-spacing" className={text.label}>Section spacing</label>
              <Select
                id="theme-section-spacing"
                value={themeSelection.sectionSpacing}
                onChange={(event) =>
                  handleThemeSelectionChange('sectionSpacing', event.target.value)
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

              <label htmlFor="theme-radius" className={text.label}>Corner radius</label>
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
                {describeThemeOption(themeOptions.radii, themeSelection.radius)}
              </p>

              <label htmlFor="theme-button-style" className={text.label}>Button style</label>
              <Select
                id="theme-button-style"
                value={themeSelection.buttonStyle}
                onChange={(event) =>
                  handleThemeSelectionChange('buttonStyle', event.target.value)
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

              <label htmlFor="theme-image-style" className={text.label}>Image style</label>
              <Select
                id="theme-image-style"
                value={themeSelection.imageStyle}
                onChange={(event) =>
                  handleThemeSelectionChange('imageStyle', event.target.value)
                }
              >
                {themeOptions.imageStyles.map((option) => (
                  <option key={option.id} value={option.id}>
                    {option.label}
                  </option>
                ))}
              </Select>
              <p className={form.hint}>
                {describeThemeOption(themeOptions.imageStyles, themeSelection.imageStyle)}
              </p>

              {themeErrorMessage ? <p className={text.error}>{themeErrorMessage}</p> : null}
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
            <p className={text.eyebrow}>Blocks</p>
            <h2 className={text.h2}>{selectedPage ? `Add to ${selectedPage.title}` : 'Choose a page first'}</h2>
          </div>

          {selectedPage ? (
            <form className={form.grid} onSubmit={handleCreateBlock}>
              <label htmlFor="new-block-type" className={text.label}>Approved block type</label>
              <Select
                id="new-block-type"
                value={newBlockType}
                onChange={(event) => setNewBlockType(event.target.value)}
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

              <Button type="submit" disabled={isCreatingBlock || !newBlockType}>
                {isCreatingBlock ? 'Adding block...' : 'Add block'}
              </Button>
            </form>
          ) : (
            <div className={emptyState}>
              <p className={text.p}>Select a page before adding new blocks.</p>
            </div>
          )}
        </section>

        {selectedBlock ? (
          <section className={ribbon}>
            <BlockEditor
              key={selectedBlock.id}
              block={selectedBlock}
              definition={selectedDefinition}
              isSaving={isSavingBlock}
              errorMessage={blockErrorMessage}
              statusMessage={blockStatusMessage}
              onSave={handleSaveBlock}
            />

            <div className={actions.panelFooter}>
              <Button
                type="button"
                variant="outline"
                disabled={selectedBlockIndex <= 0 || isMutatingBlocks}
                onClick={() => handleMoveBlock(-1)}
              >
                Move earlier
              </Button>
              <Button
                type="button"
                variant="outline"
                disabled={
                  !selectedPage ||
                  selectedBlockIndex === selectedPage.blocks.length - 1 ||
                  isMutatingBlocks
                }
                onClick={() => handleMoveBlock(1)}
              >
                Move later
              </Button>
              <Button
                type="button"
                variant="outline"
                disabled={isMutatingBlocks}
                onClick={handleDuplicateBlock}
              >
                Duplicate
              </Button>
              <Button
                type="button"
                variant="outline"
                disabled={isMutatingBlocks}
                onClick={handleDeleteBlock}
              >
                Delete
              </Button>
            </div>
          </section>
        ) : (
          <section className={ribbonPanel}>
            <div className={emptyState}>
              <p className={text.p}>{selectedPage ? 'This page does not have any blocks yet.' : 'Select a page to work with its blocks.'}</p>
            </div>
          </section>
        )}

        <section className={ribbonPanel}>
          <div className="mb-[22px]">
            <p className={text.eyebrow}>Publish</p>
            <h2 className={text.h2}>Release an immutable snapshot</h2>
            <p className={text.p}>
              Publish stores the current validated draft in `site_versions` and
              serves the public route from that snapshot.
            </p>
          </div>

          <label htmlFor="publish-note" className={text.label}>Publish note</label>
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
                <Link to="/public/$siteSlug" params={{ siteSlug: draft.site.slug }}>
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
                    <p className="m-0 text-[var(--paper-muted)]">{formatTimestamp(version.createdAt)}</p>
                  </div>
                  {version.publishNote ? (
                    <small className="m-0 text-[var(--paper-muted)]">{version.publishNote}</small>
                  ) : null}
                  {!version.isCurrent ? (
                    <div className={actions.row}>
                      <Button
                        type="button"
                        variant="plain"
                        className={actions.inlineLink}
                        disabled={isPublishing || activeRollbackVersionId !== ''}
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
            <label htmlFor="site-name" className={text.label}>Business name</label>
            <Input
              id="site-name"
              name="name"
              value={name}
              onChange={(event) => setName(event.target.value)}
              required
            />

            <label htmlFor="site-slug" className={text.label}>Slug</label>
            <Input
              id="site-slug"
              name="slug"
              value={slug}
              onChange={(event) => setSlug(event.target.value)}
              required
            />

            {siteErrorMessage ? <p className={text.error}>{siteErrorMessage}</p> : null}
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
      </div>
    </div>
  )
}

function findNewPage(previousDraft: SiteDraft | null, nextDraft: SiteDraft) {
  const previousIDs = new Set(previousDraft?.pages.map((page) => page.id) ?? [])
  return nextDraft.pages.find((page) => !previousIDs.has(page.id)) ?? nextDraft.pages.at(-1)
}

function findNewBlock(previousPage: DraftPage | null, nextPage: DraftPage | null) {
  if (!nextPage) {
    return null
  }
  const previousIDs = new Set(previousPage?.blocks.map((block) => block.id) ?? [])
  return nextPage.blocks.find((block) => !previousIDs.has(block.id)) ?? nextPage.blocks.at(-1) ?? null
}

function getNavigationPages(draft: SiteDraft) {
  return draft.navigation.primary
    .map((item) =>
      item.pageId
        ? draft.pages.find((page) => page.id === item.pageId) ?? null
        : null,
    )
    .filter((page): page is DraftPage => page !== null)
}

function moveItem<T extends { id: string }>(items: T[], itemID: string, direction: -1 | 1) {
  const index = items.findIndex((item) => item.id === itemID)
  const nextIndex = index + direction
  if (index === -1 || nextIndex < 0 || nextIndex >= items.length) {
    return null
  }
  const reordered = [...items]
  ;[reordered[index], reordered[nextIndex]] = [reordered[nextIndex], reordered[index]]
  return reordered
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
