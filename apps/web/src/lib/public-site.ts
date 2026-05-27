import { createServerFn } from '@tanstack/react-start'

export type HostedPublicSiteContext = {
  isHostedPublic: boolean
  hostname: string
  pagePath: string
}

export const getHostedPublicSiteContext = createServerFn({
  method: 'GET',
}).handler(async () => {
  const { getRequestHost, getRequestUrl } =
    await import('@tanstack/start-server-core')

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
    isHostedPublic: hostname !== '' && !getAppBaseURLHosts().has(hostname),
    hostname,
    pagePath: normalizePagePath(input.pagePath),
  }
}

export function getAppBaseURL() {
  const meta = import.meta as ImportMeta & {
    env?: Record<string, string | undefined>
  }
  if (meta.env?.VITE_APP_BASE_URL) {
    return meta.env.VITE_APP_BASE_URL
  }
  if (typeof process !== 'undefined' && process.env?.VITE_APP_BASE_URL) {
    return process.env.VITE_APP_BASE_URL
  }
  return 'http://localhost:3000'
}

export function getAppBaseURLHost() {
  try {
    return new URL(getAppBaseURL()).host
  } catch {
    return 'localhost:3000'
  }
}

export function getAppBaseURLHosts() {
  const host = normalizeHost(getAppBaseURLHost())
  const hosts = new Set<string>()
  if (host === '') {
    return hosts
  }
  hosts.add(host)

  const [hostname, port = ''] = host.split(':', 2)
  if (!hostname.startsWith('www.') && hostname.includes('.')) {
    hosts.add(`www.${hostname}${port ? `:${port}` : ''}`)
  }

  return hosts
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
