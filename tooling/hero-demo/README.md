# Hero demo capture & edit pipeline

Two scripts that produce the silent hero demo on the landing page.

```
tooling/hero-demo/
  capture.mjs    Playwright drives the real spin flow, records WebM, logs beat timestamps
  edit.py        ffmpeg cuts, speed-ramps the gen wait, burns captions, derives 1280/768 webm+mp4
```

The shipping assets land in `apps/web/public/media/hero-demo/`. The landing
route only renders the demo section when `VITE_HERO_DEMO_ENABLED=true`.

## 1. Capture

```bash
npm i -D playwright
npx playwright install chromium

make dev-up && make api          # one shell
npm run web:dev                  # another shell

node tooling/hero-demo/capture.mjs \
  --app-url http://localhost:3000 \
  --prompt 'A neighborhood bakery in Gothenburg with online ordering and a booking form for custom cakes'
```

The script writes to `apps/web/public/media/hero-demo/_raw/`:

- `master.webm` — the recording (Playwright outputs VP8/WebM at real time)
- `beats.json` — beat timestamps the editor uses to find the gen-wait segment
- `poster.png` — clean first-frame screenshot

If selectors drift, edit `capture.mjs` — beat names must stay stable
(`landing.loaded`, `prompt.focused`, `prompt.typed`, `spin.clicked`,
`generation.visible`, `site.rendered`, `site.scrolled`, `refine.*`,
`publish.*`, `capture.end`) because the editor reads them by name.

## 2. Edit & encode

```bash
python3 tooling/hero-demo/edit.py --target-duration 18
```

Produces, in `apps/web/public/media/hero-demo/`:

- `hero-1280.{webm,mp4}` — desktop variants, ≤2.5 MB
- `hero-768.{webm,mp4}` — mobile variants, ≤2.5 MB
- `poster.{webp,jpg}` — first-frame still

The editor:

1. Trims into `[pre | wait | post]` around `spin.clicked` and `site.rendered`.
2. Applies `setpts` only to the wait segment so the speed-ramp is surgical.
3. Concatenates, burns the four short captions at the right times, fades the
   last 0.3s back onto the start for a seamless loop, strips audio.
4. Derives all four variants with adaptive CRF — bumps `crf` if a variant is
   over 2.5 MB, hard-fails over 4 MB.

If the predicted output duration falls outside 15–22s the script warns; either
adjust `--target-duration` or re-capture with a faster typing speed.

## 3. Ship

Replace the placeholder assets in `apps/web/public/media/hero-demo/` with the
fresh outputs and flip `VITE_HERO_DEMO_ENABLED=true` in production. The
placeholders in this folder are deliberately labelled "PLACEHOLDER" so
mis-ship is loud.

## Re-capture rules of thumb

- Re-record whenever the spin flow's UI changes — the script is the source of
  truth, not the recording.
- Beat names are an API. Renaming `site.rendered` breaks `edit.py`.
- Use a clean demo account; the recording captures whatever loads.
