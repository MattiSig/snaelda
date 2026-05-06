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
