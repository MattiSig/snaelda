import { afterEach, describe, expect, it } from 'vitest'
import {
  getAppBaseURL,
  resolveHostedPublicSiteContext,
} from './public-site'

describe('public site host detection', () => {
  const originalAppBaseURL = process.env.VITE_APP_BASE_URL

  afterEach(() => {
    if (originalAppBaseURL === undefined) {
      delete process.env.VITE_APP_BASE_URL
    } else {
      process.env.VITE_APP_BASE_URL = originalAppBaseURL
    }
  })

  it('uses the runtime app base url when import meta env is unavailable', () => {
    process.env.VITE_APP_BASE_URL = 'https://web-production-e76d7.up.railway.app'

    expect(getAppBaseURL()).toBe('https://web-production-e76d7.up.railway.app')
    expect(
      resolveHostedPublicSiteContext({
        hostname: 'web-production-e76d7.up.railway.app',
        pagePath: '/',
      }).isHostedPublic,
    ).toBe(false)
  })

  it('treats the www sibling of the app domain as the app', () => {
    process.env.VITE_APP_BASE_URL = 'https://snaelda.io'

    expect(
      resolveHostedPublicSiteContext({
        hostname: 'www.snaelda.io',
        pagePath: '/',
      }).isHostedPublic,
    ).toBe(false)
  })
})
