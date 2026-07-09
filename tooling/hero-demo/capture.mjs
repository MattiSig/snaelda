#!/usr/bin/env node
// Playwright capture for the silent hero demo.
//
// Drives the real spin flow against a running web + api stack, records the
// session as WebM via the browser context's recordVideo, logs beat
// timestamps to beats.json, and grabs the first-frame poster.
//
// Output is the *raw master* — the Python edit script consumes it to produce
// the shipping 1280/768 webm+mp4 variants and the final poster.
//
// Usage:
//   node tooling/hero-demo/capture.mjs --headless
//   node tooling/hero-demo/capture.mjs --app-url http://localhost:3000

import { chromium } from 'playwright'
import { mkdirSync, writeFileSync, renameSync, readdirSync } from 'node:fs'
import { join, resolve } from 'node:path'

const DEFAULTS = {
  appUrl: 'http://localhost:3000',
  out: 'apps/web/public/media/hero-demo/_raw',
  prompt:
    'A neighborhood bakery in Gothenburg with online ordering and a booking form for custom cakes',
  refinePrompt: 'Make it warmer and more inviting.',
  width: 1280,
  height: 800,
  headless: false,
}

function parseArgs() {
  const args = { ...DEFAULTS }
  for (let i = 2; i < process.argv.length; i += 1) {
    const arg = process.argv[i]
    const next = process.argv[i + 1]
    if (arg === '--app-url' && next) { args.appUrl = next; i += 1 }
    else if (arg === '--out' && next) { args.out = next; i += 1 }
    else if (arg === '--prompt' && next) { args.prompt = next; i += 1 }
    else if (arg === '--refine-prompt' && next) { args.refinePrompt = next; i += 1 }
    else if (arg === '--width' && next) { args.width = Number(next); i += 1 }
    else if (arg === '--height' && next) { args.height = Number(next); i += 1 }
    else if (arg === '--headless') { args.headless = true }
    else if (arg === '--headed') { args.headless = false }
  }
  return args
}

async function typeLikeHuman(locator, text, perCharMs = 50) {
  await locator.click()
  for (const ch of text) {
    await locator.pressSequentially(ch, { delay: 0 })
    await new Promise((r) => setTimeout(r, perCharMs))
  }
}

async function main() {
  const args = parseArgs()
  const outDir = resolve(args.out)
  mkdirSync(outDir, { recursive: true })

  const beats = []
  const t0 = performance.now()
  const stamp = (name) => {
    const seconds = (performance.now() - t0) / 1000
    beats.push({ name, t: Number(seconds.toFixed(3)) })
    console.log(`  [${seconds.toFixed(2)}s] ${name}`)
  }

  console.log(`Launching Chromium (${args.headless ? 'headless' : 'headed'})...`)
  const browser = await chromium.launch({ headless: args.headless })
  const context = await browser.newContext({
    viewport: { width: args.width, height: args.height },
    deviceScaleFactor: 2,
    recordVideo: {
      dir: outDir,
      size: { width: args.width, height: args.height },
    },
  })
  const page = await context.newPage()

  try {
    // 1. Landing.
    await page.goto(args.appUrl, { waitUntil: 'networkidle' })
    stamp('landing.loaded')

    const promptInput = page.getByLabel('Describe your business')
    await promptInput.waitFor({ state: 'visible' })
    stamp('prompt.focused')

    await typeLikeHuman(promptInput, args.prompt)
    stamp('prompt.typed')

    const spinCta = page.getByRole('button', { name: /spin my site/i })
    await spinCta.click()
    stamp('spin.clicked')

    // 2. Intake — Skip all and generate.
    const skipAll = page.getByRole('button', { name: /skip all and generate/i })
    await skipAll.waitFor({ state: 'visible', timeout: 30_000 })
    stamp('intake.visible')
    await skipAll.click()
    stamp('intake.skipped')

    // 3. Generation progress card visible — wait until we navigate to preview.
    await page.waitForURL(/\/app\/sites\/[^/]+\/preview/, { timeout: 240_000 })
    stamp('preview.loaded')

    // 4. The preview renders inside an iframe (PreviewFrame). Give it a beat to paint.
    const previewFrame = page.locator('iframe').first()
    await previewFrame.waitFor({ state: 'attached', timeout: 30_000 }).catch(() => {})
    await page.waitForTimeout(1500)
    stamp('site.rendered')

    // 5. Scroll the preview iframe to show the full page.
    await page.evaluate(async () => {
      const iframe = document.querySelector('iframe')
      const win = iframe?.contentWindow
      if (!win) return
      const max = win.document.documentElement.scrollHeight
      const stepPx = 5
      for (let y = 0; y <= max - win.innerHeight; y += stepPx) {
        win.scrollTo(0, y)
        await new Promise((r) => setTimeout(r, 14))
      }
      win.scrollTo(0, 0)
    }).catch(() => {})
    stamp('site.scrolled')

    // 6. Refine via the preview-page refinement form.
    const refineInput = page.getByLabel('Refinement prompt')
    if (await refineInput.isVisible({ timeout: 5000 }).catch(() => false)) {
      await typeLikeHuman(refineInput, args.refinePrompt, 45)
      stamp('refine.typed')
      const refineButton = page.getByRole('button', { name: /^refine draft$/i })
      await refineButton.click()
      stamp('refine.submitted')
      // Wait for refinement to complete — Refining... button changes back.
      await page
        .getByRole('button', { name: /^refine draft$/i })
        .waitFor({ state: 'visible', timeout: 240_000 })
      stamp('refine.applied')
      await page.waitForTimeout(800)
    } else {
      console.log('  (refine input not visible — skipped)')
    }

    // 7. Navigate to builder for publish.
    const backToBuilder = page.getByRole('link', { name: /back to builder/i })
    if (await backToBuilder.first().isVisible().catch(() => false)) {
      await backToBuilder.first().click()
      stamp('builder.opened')
      await page.waitForURL(/\/app\/sites\/[^/]+\/?(\?|$)/, { timeout: 15_000 })
    }

    // 8. Publish.
    const publishBtn = page.getByRole('button', { name: /^publish live update$/i })
    if (await publishBtn.first().isVisible({ timeout: 15_000 }).catch(() => false)) {
      // Scroll into view first so the click hits.
      await publishBtn.first().scrollIntoViewIfNeeded()
      await publishBtn.first().click()
      stamp('publish.clicked')
      await page
        .getByText(/published|live update is now live|now live/i)
        .first()
        .waitFor({ state: 'visible', timeout: 60_000 })
        .catch(() => {})
      stamp('publish.confirmed')
    } else {
      console.log('  (publish button not visible — skipped)')
    }

    // 9. Hold a beat on the success state.
    await page.waitForTimeout(1500)
    stamp('capture.end')

    // 10. Poster — clean first-frame: go back to landing and screenshot.
    const posterPath = join(outDir, 'poster.png')
    await page.goto(args.appUrl, { waitUntil: 'networkidle' })
    await page.screenshot({ path: posterPath, type: 'png', fullPage: false })
    console.log(`Poster saved -> ${posterPath}`)
  } finally {
    await page.close()
    await context.close()
    await browser.close()
  }

  // Rename the playwright-generated video to a stable filename.
  const candidates = readdirSync(outDir).filter((f) => f.endsWith('.webm'))
  if (candidates.length > 0) {
    const newest = candidates
      .map((f) => ({ f, t: Number(f.split('.')[0]) || 0 }))
      .sort((a, b) => b.t - a.t)[0].f
    const masterPath = join(outDir, 'master.webm')
    renameSync(join(outDir, newest), masterPath)
    console.log(`Master saved -> ${masterPath}`)
  }

  const beatsPath = join(outDir, 'beats.json')
  writeFileSync(beatsPath, JSON.stringify({ beats, args }, null, 2))
  console.log(`Beats saved -> ${beatsPath}`)
  console.log('\nNext: python3 tooling/hero-demo/edit.py')
}

main().catch((err) => {
  console.error(err)
  process.exit(1)
})
