import { Link, createFileRoute, useNavigate } from '@tanstack/react-router'
import type { FormEvent } from 'react'
import { useEffect, useState } from 'react'
import { BlockEditor } from '@/components/BlockEditor'
import { Button } from '@/components/ui/button'
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
  reorderBlocks,
  reorderPages,
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
  const [publishNote, setPublishNote] = useState('')
  const [themeSelection, setThemeSelection] = useState<ThemeSelection | null>(null)
  const [themeOptions, setThemeOptions] = useState<ThemeEditorCatalog | null>(null)
  const [loadErrorMessage, setLoadErrorMessage] = useState('')
  const [siteErrorMessage, setSiteErrorMessage] = useState('')
  const [siteStatusMessage, setSiteStatusMessage] = useState('')
  const [pageErrorMessage, setPageErrorMessage] = useState('')
  const [pageStatusMessage, setPageStatusMessage] = useState('')
  const [blockErrorMessage, setBlockErrorMessage] = useState('')
  const [blockStatusMessage, setBlockStatusMessage] = useState('')
  const [themeErrorMessage, setThemeErrorMessage] = useState('')
  const [themeStatusMessage, setThemeStatusMessage] = useState('')
  const [isSavingTheme, setIsSavingTheme] = useState(false)
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

  if (isLoading) {
    return (
      <div className="builder-panel ribbon-panel">
        <p>Loading site...</p>
      </div>
    )
  }

  if (!draft) {
    return (
      <div className="builder-panel ribbon-panel">
        <p className="form-error">{loadErrorMessage || 'Site not found'}</p>
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

  return (
    <div className="builder-grid builder-grid--detail">
      <section className="builder-panel ribbon-panel">
        <div className="panel-heading">
          <p className="eyebrow">Site</p>
          <h1>{draft.site.name}</h1>
          <p>
            Manage pages and approved blocks here, then keep using the validated
            preview and publish flow.
          </p>
        </div>

        <dl className="metadata-list">
          <div>
            <dt>Status</dt>
            <dd>{currentVersion ? 'published' : draft.site.status}</dd>
          </div>
          <div>
            <dt>Pages</dt>
            <dd>{draft.pages.length}</dd>
          </div>
          <div>
            <dt>Blocks</dt>
            <dd>{blockCount}</dd>
          </div>
        </dl>

        <div className="site-detail-links">
          <Link
            to="/app/sites/$siteId/preview"
            params={{ siteId }}
            className="site-inline-link"
          >
            Open preview
          </Link>
          {currentVersion ? (
            <Link
              to="/public/$siteSlug"
              params={{ siteSlug: draft.site.slug }}
              className="site-inline-link"
            >
              Open live site
            </Link>
          ) : null}
        </div>

        <form className="editor-panel ribbon-panel add-page-form" onSubmit={handleCreatePage}>
          <div className="panel-heading">
            <div>
              <p className="eyebrow">Pages</p>
              <h2>Add a page</h2>
            </div>
          </div>

          <label htmlFor="new-page-title">Page title</label>
          <input
            id="new-page-title"
            value={newPageTitle}
            onChange={(event) => setNewPageTitle(event.target.value)}
            placeholder="Contact"
            required
          />

          <label htmlFor="new-page-slug">Slug</label>
          <input
            id="new-page-slug"
            value={newPageSlug}
            onChange={(event) => setNewPageSlug(event.target.value)}
            placeholder="/contact"
          />

          <label className="block-editor-toggle">
            <input
              type="checkbox"
              checked={newPageIncludeInNavigation}
              onChange={(event) => setNewPageIncludeInNavigation(event.target.checked)}
            />
            Include this page in site navigation
          </label>

          <Button type="submit" disabled={isCreatingPage}>
            {isCreatingPage ? 'Adding page...' : 'Add page'}
          </Button>
        </form>

        <div className="page-outline page-outline--detail">
          {draft.pages.map((page, index) => {
            const isSelectedPage = page.id === selectedPage?.id
            const isIncludedInNavigation = draft.navigation.primary.some(
              (item) => item.pageId === page.id,
            )

            return (
              <article
                key={page.id}
                className={`page-outline__item page-outline__item--stacked${isSelectedPage ? ' is-selected' : ''}`}
              >
                <div className="page-outline__summary">
                  <button
                    type="button"
                    className="page-outline__select"
                    onClick={() => {
                      setSelectedPageId(page.id)
                      setSelectedBlockId(page.blocks[0]?.id ?? '')
                      syncSelectedPageFields(draft, page)
                      setPageErrorMessage('')
                      setPageStatusMessage('')
                    }}
                  >
                    <div>
                      <h3>{page.title}</h3>
                      <p>{page.slug}</p>
                    </div>
                    <strong>{page.blocks.length} blocks</strong>
                  </button>

                  <div className="page-outline__actions">
                    <button
                      type="button"
                      className="site-inline-link"
                      disabled={index === 0 || isSavingPage}
                      onClick={() => handleMovePage(page.id, -1)}
                    >
                      Earlier
                    </button>
                    <button
                      type="button"
                      className="site-inline-link"
                      disabled={index === draft.pages.length - 1 || isSavingPage}
                      onClick={() => handleMovePage(page.id, 1)}
                    >
                      Later
                    </button>
                  </div>
                </div>

                <div className="page-outline__meta">
                  <span>{isIncludedInNavigation ? 'In navigation' : 'Hidden from navigation'}</span>
                </div>

                <div className="block-list">
                  {page.blocks.map((block) => {
                    const definition =
                      blockDefinitions.get(`${block.type}@${block.version}`)
                    const isSelected =
                      page.id === selectedPage?.id && block.id === selectedBlock?.id

                    return (
                      <button
                        key={block.id}
                        type="button"
                        className={`block-list__item${isSelected ? ' is-selected' : ''}`}
                        onClick={() => {
                          setSelectedPageId(page.id)
                          setSelectedBlockId(block.id)
                          syncSelectedPageFields(draft, page)
                          setBlockErrorMessage('')
                          setBlockStatusMessage('')
                        }}
                      >
                        <div>
                          <span>{definition?.displayName ?? block.type}</span>
                          <small>{block.type}</small>
                        </div>
                        {block.settings?.hidden ? <em>Hidden</em> : <em>Visible</em>}
                      </button>
                    )
                  })}
                </div>
              </article>
            )
          })}
        </div>
      </section>

      <div className="builder-sidebar">
        <section className="editor-panel ribbon-panel">
          <div className="panel-heading">
            <p className="eyebrow">Page details</p>
            <h2>{selectedPage ? selectedPage.title : 'Choose a page'}</h2>
            <p>
              Edit the current page title, slug, SEO, and navigation inclusion
              before publishing the next snapshot.
            </p>
          </div>

          {selectedPage ? (
            <form className="auth-panel" onSubmit={handleSavePage}>
              <label htmlFor="page-title">Page title</label>
              <input
                id="page-title"
                value={pageTitle}
                onChange={(event) => setPageTitle(event.target.value)}
                required
              />

              <label htmlFor="page-slug">Slug</label>
              <input
                id="page-slug"
                value={pageSlug}
                onChange={(event) => setPageSlug(event.target.value)}
                required
              />

              <label htmlFor="page-seo-title">SEO title</label>
              <input
                id="page-seo-title"
                value={pageSEOTitle}
                onChange={(event) => setPageSEOTitle(event.target.value)}
              />

              <label htmlFor="page-seo-description">SEO description</label>
              <textarea
                id="page-seo-description"
                rows={4}
                value={pageSEODescription}
                onChange={(event) => setPageSEODescription(event.target.value)}
              />

              <label className="block-editor-toggle">
                <input
                  type="checkbox"
                  checked={pageIncludeInNavigation}
                  onChange={(event) => setPageIncludeInNavigation(event.target.checked)}
                />
                Include this page in the primary navigation
              </label>

              {pageErrorMessage ? <p className="form-error">{pageErrorMessage}</p> : null}
              {pageStatusMessage ? (
                <p className="form-success">{pageStatusMessage}</p>
              ) : null}

              <div className="builder-actions">
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
            <div className="empty-state">
              <p>Select a page from the outline to edit its details.</p>
            </div>
          )}
        </section>

        <section className="editor-panel ribbon-panel">
          <div className="panel-heading">
            <p className="eyebrow">Theme</p>
            <h2>Set the site direction</h2>
            <p>
              Keep the visual system inside the safe theme contract while tuning
              the palette, type, spacing, and corner feel.
            </p>
          </div>

          {themeSelection && themeOptions ? (
            <form className="auth-panel" onSubmit={handleSaveTheme}>
              <div className="theme-preview-card">
                <div className="theme-preview-card__swatches">
                  {Object.entries(draft.theme.tokens.colors).map(([key, value]) => (
                    <div key={key} className="theme-swatch">
                      <span
                        className="theme-swatch__chip"
                        style={{ backgroundColor: value }}
                      />
                      <div>
                        <strong>{formatThemeLabel(key)}</strong>
                        <small>{value}</small>
                      </div>
                    </div>
                  ))}
                </div>
              </div>

              <label htmlFor="theme-palette">Palette</label>
              <select
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
              </select>
              <p className="field-hint">
                {describeThemeOption(themeOptions.palettes, themeSelection.palette)}
              </p>

              <label htmlFor="theme-font-preset">Font preset</label>
              <select
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
              </select>
              <p className="field-hint">
                {describeThemeOption(themeOptions.fontPresets, themeSelection.fontPreset)}
              </p>

              <label htmlFor="theme-section-spacing">Section spacing</label>
              <select
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
              </select>
              <p className="field-hint">
                {describeThemeOption(
                  themeOptions.sectionSpacings,
                  themeSelection.sectionSpacing,
                )}
              </p>

              <label htmlFor="theme-radius">Corner radius</label>
              <select
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
              </select>
              <p className="field-hint">
                {describeThemeOption(themeOptions.radii, themeSelection.radius)}
              </p>

              {themeErrorMessage ? <p className="form-error">{themeErrorMessage}</p> : null}
              {themeStatusMessage ? (
                <p className="form-success">{themeStatusMessage}</p>
              ) : null}

              <Button type="submit" disabled={isSavingTheme}>
                {isSavingTheme ? 'Saving theme...' : 'Save theme'}
              </Button>
            </form>
          ) : (
            <div className="empty-state">
              <p>Loading theme controls...</p>
            </div>
          )}
        </section>

        <section className="editor-panel ribbon-panel">
          <div className="panel-heading">
            <p className="eyebrow">Blocks</p>
            <h2>{selectedPage ? `Add to ${selectedPage.title}` : 'Choose a page first'}</h2>
          </div>

          {selectedPage ? (
            <form className="auth-panel" onSubmit={handleCreateBlock}>
              <label htmlFor="new-block-type">Approved block type</label>
              <select
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
              </select>

              <Button type="submit" disabled={isCreatingBlock || !newBlockType}>
                {isCreatingBlock ? 'Adding block...' : 'Add block'}
              </Button>
            </form>
          ) : (
            <div className="empty-state">
              <p>Select a page before adding new blocks.</p>
            </div>
          )}
        </section>

        {selectedBlock ? (
          <section className="ribbon-panel">
            <BlockEditor
              key={selectedBlock.id}
              block={selectedBlock}
              definition={selectedDefinition}
              isSaving={isSavingBlock}
              errorMessage={blockErrorMessage}
              statusMessage={blockStatusMessage}
              onSave={handleSaveBlock}
            />

            <div className="builder-actions builder-actions--panel">
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
          <section className="editor-panel ribbon-panel">
            <div className="empty-state">
              <p>{selectedPage ? 'This page does not have any blocks yet.' : 'Select a page to work with its blocks.'}</p>
            </div>
          </section>
        )}

        <section className="editor-panel ribbon-panel">
          <div className="panel-heading">
            <p className="eyebrow">Publish</p>
            <h2>Release an immutable snapshot</h2>
            <p>
              Publish stores the current validated draft in `site_versions` and
              serves the public route from that snapshot.
            </p>
          </div>

          <label htmlFor="publish-note">Publish note</label>
          <textarea
            id="publish-note"
            name="publishNote"
            rows={3}
            value={publishNote}
            onChange={(event) => setPublishNote(event.target.value)}
            placeholder="What changed in this release?"
          />

          {publishErrorMessage ? (
            <p className="form-error">{publishErrorMessage}</p>
          ) : null}
          {publishStatusMessage ? (
            <p className="form-success">{publishStatusMessage}</p>
          ) : null}

          <div className="publish-actions">
            <Button type="button" disabled={isPublishing} onClick={handlePublish}>
              {isPublishing ? 'Publishing...' : 'Publish snapshot'}
            </Button>
            {currentVersion ? (
              <Link
                to="/public/$siteSlug"
                params={{ siteSlug: draft.site.slug }}
                className="site-inline-link"
              >
                View live site
              </Link>
            ) : null}
          </div>

          <div className="version-list">
            {versions.length === 0 ? (
              <div className="empty-state">
                <p>No published versions yet.</p>
              </div>
            ) : (
              versions.map((version) => (
                <article key={version.id} className="version-list__item">
                  <div>
                    <strong>
                      v{version.versionNumber}
                      {version.isCurrent ? ' current' : ''}
                    </strong>
                    <p>{formatTimestamp(version.createdAt)}</p>
                  </div>
                  {version.publishNote ? <small>{version.publishNote}</small> : null}
                </article>
              ))
            )}
          </div>
        </section>

        <section className="editor-panel ribbon-panel">
          <div className="panel-heading">
            <p className="eyebrow">Site details</p>
            <h2>Rename and reslug the draft</h2>
          </div>

          <form className="auth-panel" onSubmit={handleSaveSite}>
            <label htmlFor="site-name">Business name</label>
            <input
              id="site-name"
              name="name"
              value={name}
              onChange={(event) => setName(event.target.value)}
              required
            />

            <label htmlFor="site-slug">Slug</label>
            <input
              id="site-slug"
              name="slug"
              value={slug}
              onChange={(event) => setSlug(event.target.value)}
              required
            />

            {siteErrorMessage ? <p className="form-error">{siteErrorMessage}</p> : null}
            {siteStatusMessage ? (
              <p className="form-success">{siteStatusMessage}</p>
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
