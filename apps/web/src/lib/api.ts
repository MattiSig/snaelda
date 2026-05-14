export type APIErrorPayload = {
  error?:
    | string
    | {
        code?: string
        message?: string
      }
  code?: string
  message?: string
}

export class APIError extends Error {
  readonly status: number
  readonly payload: APIErrorPayload | null

  constructor(status: number, payload: APIErrorPayload | null) {
    const nestedMessage =
      typeof payload?.error === 'object' ? payload.error.message : undefined
    super(
      payload?.message ??
        nestedMessage ??
        `API request failed with status ${status}`,
    )
    this.name = 'APIError'
    this.status = status
    this.payload = payload
  }
}

export type AuthUser = {
  id: string
  email: string
  name: string
  workspaceId: string
  workspaceRole: string
}

export type AuthSession = {
  user: AuthUser
  expiresAt?: number
  tokenType?: string
}

export type SiteSummary = {
  id: string
  workspaceId: string
  name: string
  slug: string
  status: string
  defaultLocale: string
  publishedVersionId?: string
  pageCount: number
}

export type SiteDraft = {
  site: {
    id: string
    name: string
    slug: string
    status: string
    defaultLocale?: string
    seo?: {
      title?: string
      description?: string
    }
  }
  theme: {
    version: string
    tokens: {
      colors: Record<string, string>
      typography: Record<string, string>
      layout: Record<string, string>
      shape: Record<string, string>
    }
  }
  navigation: {
    primary: Array<{
      label: string
      pageId?: string
      href?: string
    }>
  }
  pages: Array<{
    id: string
    title: string
    slug: string
    seo?: {
      title?: string
      description?: string
    }
    settings?: Record<string, unknown>
    blocks: Array<{
      id: string
      type: string
      version: string
      props: Record<string, unknown>
      settings?: {
        hidden?: boolean
        anchorId?: string
      }
    }>
  }>
}

export type PublishedSnapshot = {
  schemaVersion: string
  site: {
    id: string
    name: string
    defaultLocale: string
    seo?: {
      title?: string
      description?: string
    }
  }
  theme: SiteDraft['theme']
  navigation: SiteDraft['navigation']
  pages: SiteDraft['pages']
}

export type SiteVersion = {
  id: string
  siteId: string
  versionNumber: number
  createdAt: string
  publishNote?: string
  isCurrent: boolean
}

export type BlockEditorField = {
  name: string
  label: string
  control: string
  valueType?: string
  description?: string
  placeholder?: string
  options?: string[]
  fields?: BlockEditorField[]
  itemFields?: BlockEditorField[]
}

export type BlockDefinition = {
  type: string
  version: string
  displayName: string
  category: string
  defaultProps?: Record<string, unknown>
  editorSchema?: BlockEditorField[]
}

export type GenerationMetadata = {
  prompt: string
  themePreset?: string
  assetsNeeded?: string[]
  assumptions?: string[]
  validationRetryCount?: number
}

export type SiteDraftResponse = {
  draft: SiteDraft
  generation: GenerationMetadata
  blockRegistry: BlockDefinition[]
}

export type SiteRepromptResponse = {
  jobId: string
  draft: SiteDraft
}

export type PublishSiteResponse = {
  version: SiteVersion
  hostname: string
  publicUrl: string
  snapshot: PublishedSnapshot
}

export type RollbackSiteResponse = {
  version: SiteVersion
  hostname: string
  publicUrl: string
}

export type ThemeOption = {
  id: string
  label: string
  description?: string
}

export type ThemeSelection = {
  palette: string
  fontPreset: string
  sectionSpacing: string
  radius: string
  buttonStyle: string
  imageStyle: string
}

export type ThemeEditorCatalog = {
  palettes: ThemeOption[]
  fontPresets: ThemeOption[]
  sectionSpacings: ThemeOption[]
  radii: ThemeOption[]
  buttonStyles: ThemeOption[]
  imageStyles: ThemeOption[]
}

export type ThemeState = {
  theme: SiteDraft['theme']
  selection: ThemeSelection
  options: ThemeEditorCatalog
}

export type AssetMetadata = {
  fileName?: string
  contentType?: string
  requestedSizeBytes?: number
  sizeBytes?: number
  width?: number
  height?: number
  etag?: string
  uploadStatus?: string
  uploadedAt?: string
}

export type AssetRecord = {
  id: string
  workspaceId: string
  siteId?: string
  kind: string
  storageKey: string
  publicUrl?: string
  downloadUrl?: string
  altText?: string
  metadata: AssetMetadata
  createdBy?: string
  createdAt: string
}

export type FormSubmissionStatus = 'new' | 'reviewed' | 'resolved' | 'spam'

export type FormSubmissionRecord = {
  id: string
  siteId: string
  pageId?: string
  blockId?: string
  status: FormSubmissionStatus
  payload: Record<string, unknown>
  createdAt: string
  pageTitle?: string
}

export type AssetUploadTicket = {
  asset: AssetRecord
  upload: {
    url: string
    method: string
    headers?: Record<string, string>
    expiresAt: string
  }
}

export type SiteVersionsResponse = {
  versions: SiteVersion[]
}

export type PublishedSiteResponse = {
  siteSlug: string
  hostname?: string
  publicUrl: string
  version: SiteVersion
  pagePath: string
  page: SiteDraft['pages'][number]
  snapshot: PublishedSnapshot
}

const defaultAPIBaseURL = 'http://localhost:8080'

export function getAPIBaseURL() {
  return viteEnv('VITE_API_BASE_URL') ?? defaultAPIBaseURL
}

function viteEnv(name: string) {
  const meta = import.meta as ImportMeta & {
    env?: Record<string, string | undefined>
  }
  return meta.env?.[name]
}

export async function apiFetch<T>(
  path: string,
  init: RequestInit = {},
  retryOnUnauthorized = true,
): Promise<T> {
  const response = await fetch(new URL(path, getAPIBaseURL()), {
    credentials: 'include',
    headers: {
      Accept: 'application/json',
      ...init.headers,
    },
    ...init,
  })

  if (!response.ok) {
    if (
      response.status === 401 &&
      retryOnUnauthorized &&
      path !== '/api/auth/login' &&
      path !== '/api/auth/refresh'
    ) {
      await refreshAuthSession()
      return apiFetch<T>(path, init, false)
    }

    const payload = await response.json().catch(() => null)
    throw new APIError(response.status, payload)
  }

  if (response.status === 204) {
    return undefined as T
  }

  return response.json() as Promise<T>
}

async function publicAPIRequest<T>(
  path: string,
  init: RequestInit = {},
): Promise<T> {
  const response = await fetch(new URL(path, getAPIBaseURL()), {
    credentials: 'omit',
    headers: {
      Accept: 'application/json',
      ...init.headers,
    },
    ...init,
  })

  if (!response.ok) {
    const payload = await response.json().catch(() => null)
    throw new APIError(response.status, payload)
  }

  if (response.status === 204) {
    return undefined as T
  }

  return response.json() as Promise<T>
}

export async function getCurrentSession() {
  return apiFetch<AuthSession>('/api/auth/me')
}

export async function refreshAuthSession() {
  return apiFetch<AuthSession>(
    '/api/auth/refresh',
    {
      method: 'POST',
    },
    false,
  )
}

export async function login(email: string, name?: string) {
  return apiFetch<AuthSession>('/api/auth/login', {
    method: 'POST',
    headers: {
      'Content-Type': 'application/json',
    },
    body: JSON.stringify({ email, name }),
  })
}

export async function logout() {
  return apiFetch<{ status: string }>('/api/auth/logout', {
    method: 'POST',
  })
}

export async function listSites() {
  return apiFetch<{ sites: SiteSummary[] }>('/api/sites')
}

export async function getSiteDraft(siteId: string) {
  return apiFetch<SiteDraftResponse>(`/api/sites/${siteId}`)
}

export async function createSite(input: {
  name: string
  prompt?: string
  slug?: string
}) {
  return apiFetch<{ draft: SiteDraft }>('/api/sites', {
    method: 'POST',
    headers: {
      'Content-Type': 'application/json',
    },
    body: JSON.stringify(input),
  })
}

export async function generateSite(input: {
  name?: string
  prompt: string
  slug?: string
}) {
  return apiFetch<SiteRepromptResponse>('/api/sites/generate', {
    method: 'POST',
    headers: {
      'Content-Type': 'application/json',
    },
    body: JSON.stringify(input),
  })
}

export async function repromptSite(siteId: string, input: { prompt: string }) {
  return apiFetch<SiteRepromptResponse>(`/api/sites/${siteId}/reprompt`, {
    method: 'POST',
    headers: {
      'Content-Type': 'application/json',
    },
    body: JSON.stringify(input),
  })
}

export async function repromptPage(
  siteId: string,
  pageId: string,
  input: { prompt: string },
) {
  return apiFetch<SiteRepromptResponse>(
    `/api/sites/${siteId}/pages/${pageId}/reprompt`,
    {
      method: 'POST',
      headers: {
        'Content-Type': 'application/json',
      },
      body: JSON.stringify(input),
    },
  )
}

export async function undoSiteReprompt(siteId: string) {
  return apiFetch<{ draft: SiteDraft }>(`/api/sites/${siteId}/undo`, {
    method: 'POST',
  })
}

export async function updateSite(
  siteId: string,
  input: {
    name?: string
    slug?: string
  },
) {
  return apiFetch<{ draft: SiteDraft }>(`/api/sites/${siteId}`, {
    method: 'PATCH',
    headers: {
      'Content-Type': 'application/json',
    },
    body: JSON.stringify(input),
  })
}

export async function getSiteTheme(siteId: string) {
  return apiFetch<ThemeState>(`/api/sites/${siteId}/theme`)
}

export async function updateSiteTheme(
  siteId: string,
  input: Partial<ThemeSelection>,
) {
  return apiFetch<ThemeState>(`/api/sites/${siteId}/theme`, {
    method: 'PATCH',
    headers: {
      'Content-Type': 'application/json',
    },
    body: JSON.stringify(input),
  })
}

export async function createAssetUploadURL(input: {
  siteId: string
  fileName: string
  contentType: string
  sizeBytes: number
  kind?: string
  altText?: string
}) {
  return apiFetch<AssetUploadTicket>('/api/assets/upload-url', {
    method: 'POST',
    headers: {
      'Content-Type': 'application/json',
    },
    body: JSON.stringify(input),
  })
}

export async function completeAssetUpload(
  assetId: string,
  input: {
    altText?: string
    width?: number
    height?: number
  } = {},
) {
  return apiFetch<{ asset: AssetRecord }>('/api/assets/complete', {
    method: 'POST',
    headers: {
      'Content-Type': 'application/json',
    },
    body: JSON.stringify({ assetId, ...input }),
  })
}

export async function listSiteAssets(siteId: string) {
  return apiFetch<{ assets: AssetRecord[] }>(`/api/sites/${siteId}/assets`)
}

export async function listSiteFormSubmissions(siteId: string) {
  return apiFetch<{ submissions: FormSubmissionRecord[] }>(
    `/api/sites/${siteId}/form-submissions`,
  )
}

export async function updateFormSubmission(
  submissionId: string,
  input: {
    status: FormSubmissionStatus
  },
) {
  return apiFetch<{ submission: FormSubmissionRecord }>(
    `/api/form-submissions/${submissionId}`,
    {
      method: 'PATCH',
      headers: {
        'Content-Type': 'application/json',
      },
      body: JSON.stringify(input),
    },
  )
}

export async function updateAsset(
  assetId: string,
  input: {
    altText?: string
  },
) {
  return apiFetch<{ asset: AssetRecord }>(`/api/assets/${assetId}`, {
    method: 'PATCH',
    headers: {
      'Content-Type': 'application/json',
    },
    body: JSON.stringify(input),
  })
}

export async function deleteAsset(assetId: string) {
  return apiFetch<void>(`/api/assets/${assetId}`, {
    method: 'DELETE',
  })
}

export async function createPage(
  siteId: string,
  input: {
    title: string
    slug?: string
    includeInNavigation?: boolean
  },
) {
  return apiFetch<{ draft: SiteDraft }>(`/api/sites/${siteId}/pages`, {
    method: 'POST',
    headers: {
      'Content-Type': 'application/json',
    },
    body: JSON.stringify(input),
  })
}

export async function updatePage(
  siteId: string,
  pageId: string,
  input: {
    title?: string
    slug?: string
    seo?: {
      title?: string
      description?: string
    }
    includeInNavigation?: boolean
  },
) {
  return apiFetch<{ draft: SiteDraft }>(
    `/api/sites/${siteId}/pages/${pageId}`,
    {
      method: 'PATCH',
      headers: {
        'Content-Type': 'application/json',
      },
      body: JSON.stringify(input),
    },
  )
}

export async function deletePage(siteId: string, pageId: string) {
  return apiFetch<{ draft: SiteDraft }>(
    `/api/sites/${siteId}/pages/${pageId}`,
    {
      method: 'DELETE',
    },
  )
}

export async function reorderPages(siteId: string, pageIds: string[]) {
  return apiFetch<{ draft: SiteDraft }>(`/api/sites/${siteId}/pages/reorder`, {
    method: 'POST',
    headers: {
      'Content-Type': 'application/json',
    },
    body: JSON.stringify({ pageIds }),
  })
}

export async function reorderSiteNavigation(siteId: string, pageIds: string[]) {
  return apiFetch<{ draft: SiteDraft }>(
    `/api/sites/${siteId}/navigation/reorder`,
    {
      method: 'POST',
      headers: {
        'Content-Type': 'application/json',
      },
      body: JSON.stringify({ pageIds }),
    },
  )
}

export async function createBlock(
  siteId: string,
  pageId: string,
  input: {
    type: string
    version?: string
  },
) {
  return apiFetch<{ draft: SiteDraft }>(
    `/api/sites/${siteId}/pages/${pageId}/blocks`,
    {
      method: 'POST',
      headers: {
        'Content-Type': 'application/json',
      },
      body: JSON.stringify(input),
    },
  )
}

export async function deleteSite(siteId: string) {
  return apiFetch<unknown>(`/api/sites/${siteId}`, {
    method: 'DELETE',
  })
}

export async function updateBlock(
  siteId: string,
  pageId: string,
  blockId: string,
  input: {
    props?: Record<string, unknown>
    hidden?: boolean
  },
) {
  return apiFetch<{ draft: SiteDraft }>(
    `/api/sites/${siteId}/pages/${pageId}/blocks/${blockId}`,
    {
      method: 'PATCH',
      headers: {
        'Content-Type': 'application/json',
      },
      body: JSON.stringify(input),
    },
  )
}

export async function deleteBlock(
  siteId: string,
  pageId: string,
  blockId: string,
) {
  return apiFetch<{ draft: SiteDraft }>(
    `/api/sites/${siteId}/pages/${pageId}/blocks/${blockId}`,
    {
      method: 'DELETE',
    },
  )
}

export async function duplicateBlock(
  siteId: string,
  pageId: string,
  blockId: string,
) {
  return apiFetch<{ draft: SiteDraft }>(
    `/api/sites/${siteId}/pages/${pageId}/blocks/${blockId}/duplicate`,
    {
      method: 'POST',
    },
  )
}

export async function reorderBlocks(
  siteId: string,
  pageId: string,
  blockIds: string[],
) {
  return apiFetch<{ draft: SiteDraft }>(
    `/api/sites/${siteId}/pages/${pageId}/blocks/reorder`,
    {
      method: 'POST',
      headers: {
        'Content-Type': 'application/json',
      },
      body: JSON.stringify({ blockIds }),
    },
  )
}

export async function publishSite(
  siteId: string,
  input: {
    publishNote?: string
  } = {},
) {
  return apiFetch<PublishSiteResponse>(`/api/sites/${siteId}/publish`, {
    method: 'POST',
    headers: {
      'Content-Type': 'application/json',
    },
    body: JSON.stringify(input),
  })
}

export async function listSiteVersions(siteId: string) {
  return apiFetch<SiteVersionsResponse>(`/api/sites/${siteId}/versions`)
}

export async function rollbackSiteVersion(siteId: string, versionId: string) {
  return apiFetch<RollbackSiteResponse>(
    `/api/sites/${siteId}/rollback/${versionId}`,
    {
      method: 'POST',
    },
  )
}

export async function getPublishedSite(siteSlug: string, pagePath = '/') {
  const search = new URLSearchParams()
  if (pagePath && pagePath !== '/') {
    search.set('path', pagePath)
  }

  const suffix = search.size > 0 ? `?${search.toString()}` : ''
  return publicAPIRequest<PublishedSiteResponse>(
    `/api/public/sites/${siteSlug}${suffix}`,
  )
}

export async function getPublishedSiteByHostname(
  hostname: string,
  pagePath = '/',
) {
  const search = new URLSearchParams({ hostname })
  if (pagePath && pagePath !== '/') {
    search.set('path', pagePath)
  }

  return publicAPIRequest<PublishedSiteResponse>(
    `/api/public/render?${search.toString()}`,
  )
}

export async function submitPublicForm(
  siteId: string,
  blockId: string,
  payload: Record<string, unknown>,
) {
  return publicAPIRequest<{ status: string; message: string }>(
    `/api/public/forms/${siteId}/${blockId}/submit`,
    {
      method: 'POST',
      headers: {
        'Content-Type': 'application/json',
      },
      body: JSON.stringify({ payload }),
    },
  )
}
