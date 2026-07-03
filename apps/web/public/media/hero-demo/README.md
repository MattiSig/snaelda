Placeholder assets for the silent hero demo.

These tiny clips render the player wiring during development. They are not the
shipping asset — they say "PLACEHOLDER" on every frame so they cannot be
mistaken for the real spin demo.

Production gate: the hero demo only renders when
`VITE_HERO_DEMO_ENABLED=true` is set. Keep this flag unset in production until
the real assets land. See `apps/web/src/routes/index.tsx`.

Replace with the recorded assets per `BRANDING.md` / the capture spec:

- `poster.webp`, `poster.jpg` — first-frame still, 16:10
- `hero-1280.webm`, `hero-1280.mp4` — desktop, ≤2.5 MB each
- `hero-768.webm`, `hero-768.mp4` — mobile, ≤2.5 MB each

Encode commands live in the capture spec (Section 6).
