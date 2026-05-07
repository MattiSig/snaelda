export type APIErrorPayload = {
  error?: string | {
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

export type SiteDraftResponse = {
  draft: SiteDraft
  blockRegistry: BlockDefinition[]
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
}

export type ThemeEditorCatalog = {
  palettes: ThemeOption[]
  fontPresets: ThemeOption[]
  sectionSpacings: ThemeOption[]
  radii: ThemeOption[]
}

export type ThemeState = {
  theme: SiteDraft['theme']
  selection: ThemeSelection
  options: ThemeEditorCatalog
}

export type SiteVersionsResponse = {
  versions: SiteVersion[]
}

export type PublishedSiteResponse = {
  siteSlug: string
  hostname?: string
  publicUrl: string
  version: SiteVersion
  snapshot: PublishedSnapshot
}

const defaultAPIBaseURL = 'http://localhost:8080'

export function getAPIBaseURL() {
  return import.meta.env.VITE_API_BASE_URL ?? defaultAPIBaseURL
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
  return apiFetch<{ jobId: string; draft: SiteDraft }>('/api/sites/generate', {
    method: 'POST',
    headers: {
      'Content-Type': 'application/json',
    },
    body: JSON.stringify(input),
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
  return apiFetch<{ draft: SiteDraft }>(`/api/sites/${siteId}/pages/${pageId}`, {
    method: 'PATCH',
    headers: {
      'Content-Type': 'application/json',
    },
    body: JSON.stringify(input),
  })
}

export async function deletePage(siteId: string, pageId: string) {
  return apiFetch<{ draft: SiteDraft }>(`/api/sites/${siteId}/pages/${pageId}`, {
    method: 'DELETE',
  })
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

export async function getPublishedSite(siteSlug: string) {
  return apiFetch<PublishedSiteResponse>(`/api/public/sites/${siteSlug}`, {
    credentials: 'omit',
  })
}
