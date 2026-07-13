# Decision 4: Headless capture as a browserless container in a separate Railway project

## Status

Accepted

## Decision

Ship the Spec 21 v1.1 headless capability as an off-the-shelf **browserless/chromium container deployed in its own Railway project**, called by the Go API over token-authenticated HTTPS. Do not write a custom capture service, do not build an SSRF egress proxy, and do not run Chromium inside the API container.

- The capture service is the stock `ghcr.io/browserless/chromium` image with configuration only: `TOKEN`, `MAX_CONCURRENT_SESSIONS=2`, `TIMEOUT=25000` (the Spec 21 render budget), downloads disabled.
- The API calls two REST endpoints synchronously from the respin pipeline: `POST /screenshot` (PNG — before/after view, composed OG image) and `POST /content` (rendered HTML — the headless fetch fallback for JS-walled sites). One small Go client behind `CAPTURE_URL` / `CAPTURE_TOKEN` env vars.
- The service lives in a **separate Railway project**, not merely a separate service in the platform project.

## Context

Spec 21 defers headless rendering to v1.1 and defines its security contract: the browser executes attacker-supplied JavaScript, so it must be sandboxed with no path to internal services, killed on a render budget, and capped in concurrency. The original contract phrased the network guard as "all egress forced through the SSRF-guarded proxy."

Four workstreams converge on the same capability: the before/after source screenshot and the shareable OG money shot (the GTM centerpiece), the fetch fallback for client-rendered sites, and computed-style brand-color sampling. QA against real Squarespace sources (2026-07) confirmed brand fidelity is the top gap, raising the priority of everything that feeds it.

Railway private networking spans a single project. A browser container inside the platform project could be steered by in-page JavaScript to reach `api.railway.internal` or Postgres over the private mesh; a browser in its own project can reach only the public internet. A full egress proxy would close the remaining gap (in-page JS probing arbitrary public hosts from our egress IP), but that risk is identical to what plain fetch already accepts, and the proxy is significant custom infrastructure for no new guarantee.

Alternatives rejected:

- **Custom Playwright/chromedp service** — request interception still cannot fully stop DNS rebinding without a real proxy, so it adds owned code without closing the gap that project isolation closes.
- **Chromium in the API container** — couples the API's memory and deploy lifecycle to Chrome and puts attacker JS on the private network.
- **Third-party screenshot APIs** — no rendered-HTML fallback, and prospect URLs leave our infrastructure.

## Consequences

- Network isolation substitutes for the egress-proxy requirement; Spec 21's Headless Capture Service section is updated to record this. `ValidatePublicURL` still pre-checks every URL before capture, so the browser is never pointed at a private target.
- Zero capture code to own; the operational surface is one Railway project (~1–2 GB RAM, roughly $5–10/month) plus two env vars on the API.
- Captures run synchronously inside the respin runner's existing concurrency slots with a ~30s client timeout; capture failure is a soft degradation to the v1 after-only behavior, never a failed import.
- The same service later serves computed-style color sampling, which supersedes CSS-parsing heuristics for brand color when it lands.
- If capture volume outgrows the single container, the exit path is scaling the browserless service or revisiting a managed capture API — the Go client seam keeps either swap local.
