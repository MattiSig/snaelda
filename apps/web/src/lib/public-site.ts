import { createServerFn } from '@tanstack/react-start'

export type HostedPublicSiteContext = {
  isHostedPublic: boolean
  hostname: string
  pagePath: string
}

export const getHostedPublicSiteContext = createServerFn({ method: 'GET' }).handler(async () => {
  const { getRequestHost, getRequestUrl } = await import('@tanstack/start-server-core')

  return resolveHostedPublicSiteContext({
    hostname: getRequestHost({ xForwardedHost: true }),
    pagePath: getRequestUrl({
      xForwardedHost: true,
      xForwardedProto: true,
    }).pathname,
  })
})

export function resolveHostedPublicSiteContext(input: {
  hostname: string
  pagePath: string
}): HostedPublicSiteContext {
  const hostname = normalizeHost(input.hostname)

  return {
    isHostedPublic: hostname !== '' && hostname !== normalizeHost(getAppBaseURLHost()),
    hostname,
    pagePath: normalizePagePath(input.pagePath),
  }
}

export function getAppBaseURL() {
  return import.meta.env.VITE_APP_BASE_URL ?? 'http://localhost:3000'
}

export function getAppBaseURLHost() {
  try {
    return new URL(getAppBaseURL()).host
  } catch {
    return 'localhost:3000'
  }
}

export function normalizeHost(value: string) {
  return value.trim().toLowerCase().replace(/\.$/, '')
}

export function normalizePagePath(value: string) {
  const trimmed = value.trim()
  if (trimmed === '' || trimmed === '/') {
    return '/'
  }

  const prefixed = trimmed.startsWith('/') ? trimmed : `/${trimmed}`
  const collapsed = prefixed.replace(/\/{2,}/g, '/')
  return collapsed === '/' ? '/' : collapsed.replace(/\/+$/, '')
}
