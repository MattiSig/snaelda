export type APIErrorPayload = {
  error?: string
  code?: string
  message?: string
}

export class APIError extends Error {
  readonly status: number
  readonly payload: APIErrorPayload | null

  constructor(status: number, payload: APIErrorPayload | null) {
    super(payload?.message ?? `API request failed with status ${status}`)
    this.name = 'APIError'
    this.status = status
    this.payload = payload
  }
}

const defaultAPIBaseURL = 'http://localhost:8080'

export function getAPIBaseURL() {
  return import.meta.env.VITE_API_BASE_URL ?? defaultAPIBaseURL
}

export async function apiFetch<T>(
  path: string,
  init: RequestInit = {},
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
    const payload = await response.json().catch(() => null)
    throw new APIError(response.status, payload)
  }

  return response.json() as Promise<T>
}
