import { Link, createFileRoute, useNavigate } from '@tanstack/react-router'
import type { FormEvent } from 'react'
import { useEffect, useState } from 'react'
import { BlockEditor } from '@/components/BlockEditor'
import { Button } from '@/components/ui/button'
import {
  APIError,
  deleteSite,
  getSiteDraft,
  listSiteVersions,
  publishSite,
  type BlockDefinition,
  type SiteDraft,
  type SiteVersion,
  updateBlock,
  updateSite,
} from '@/lib/api'

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
  const [isLoading, setIsLoading] = useState(true)
  const [isSavingSite, setIsSavingSite] = useState(false)
  const [isSavingBlock, setIsSavingBlock] = useState(false)
  const [isDeleting, setIsDeleting] = useState(false)
  const [isPublishing, setIsPublishing] = useState(false)
  const [publishNote, setPublishNote] = useState('')
  const [loadErrorMessage, setLoadErrorMessage] = useState('')
  const [siteErrorMessage, setSiteErrorMessage] = useState('')
  const [siteStatusMessage, setSiteStatusMessage] = useState('')
  const [blockErrorMessage, setBlockErrorMessage] = useState('')
  const [blockStatusMessage, setBlockStatusMessage] = useState('')
  const [publishErrorMessage, setPublishErrorMessage] = useState('')
  const [publishStatusMessage, setPublishStatusMessage] = useState('')

  useEffect(() => {
    let isMounted = true

    Promise.all([getSiteDraft(siteId), listSiteVersions(siteId)])
      .then(([draftResponse, versionResponse]) => {
        if (!isMounted) {
          return
        }
        setDraft(draftResponse.draft)
        setBlockRegistry(draftResponse.blockRegistry)
        setVersions(versionResponse.versions)
        setName(draftResponse.draft.site.name)
        setSlug(draftResponse.draft.site.slug)
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
      setDraft(response.draft)
      setName(response.draft.site.name)
      setSlug(response.draft.site.slug)
      setSiteStatusMessage('Site details saved.')
    } catch (error) {
      setSiteErrorMessage(
        error instanceof APIError ? error.message : 'Could not save site',
      )
    } finally {
      setIsSavingSite(false)
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
      setDraft(response.draft)
      setBlockStatusMessage('Block changes saved.')
    } catch (error) {
      setBlockErrorMessage(
        error instanceof APIError ? error.message : 'Could not save block',
      )
    } finally {
      setIsSavingBlock(false)
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

  return (
    <div className="builder-grid builder-grid--detail">
      <section className="builder-panel ribbon-panel">
        <div className="panel-heading">
          <p className="eyebrow">Site</p>
          <h1>{draft.site.name}</h1>
          <p>
            Pick a page, choose a block, then save changes back through the
            backend validator before previewing the draft.
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

        <div className="page-outline page-outline--detail">
          {draft.pages.map((page) => (
            <article key={page.id} className="page-outline__item page-outline__item--stacked">
              <div className="page-outline__summary">
                <div>
                  <h3>{page.title}</h3>
                  <p>{page.slug}</p>
                </div>
                <strong>{page.blocks.length} blocks</strong>
              </div>

              <div className="block-list">
                {page.blocks.map((block) => {
                  const definition =
                    blockDefinitions.get(`${block.type}@${block.version}`)
                  const isSelected =
                    page.id === selectedPageId && block.id === selectedBlockId

                  return (
                    <button
                      key={block.id}
                      type="button"
                      className={`block-list__item${isSelected ? ' is-selected' : ''}`}
                      onClick={() => {
                        setSelectedPageId(page.id)
                        setSelectedBlockId(block.id)
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
          ))}
        </div>
      </section>

      <div className="builder-sidebar">
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
          </section>
        ) : (
          <section className="editor-panel ribbon-panel">
            <div className="empty-state">
              <p>This page does not have any blocks yet.</p>
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
