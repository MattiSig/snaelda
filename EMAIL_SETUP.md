# Support Email Setup

Goal: receive `support@snaelda.io` in your personal Gmail (`mattisigur@gmail.com`), and reply *from* `support@snaelda.io` so users see a branded address.

Architecture:

- **Inbound** — Namecheap Email Forwarding (free) forwards `support@snaelda.io` to your Gmail.
- **Outbound** — Gmail's "Send mail as" uses Resend SMTP to send from `support@snaelda.io`.

No code changes required. This is personal-mailbox plumbing next to the app's existing Resend transactional sending.

> **Current DNS reality (checked 2026-07-08).**
> - `snaelda.io` nameservers are at **Namecheap** (`pdns1/pdns2.registrar-servers.com`), and the zone currently has **no MX and no SPF/TXT records**.
> - `snaelda.app` **does not resolve at all** (no NS records). The config default `EMAIL_FROM_ADDRESS=hi@snaelda.app` therefore cannot be a working sender unless production overrides it — check `EMAIL_FROM_ADDRESS` on the Railway API service and which domain is actually verified in Resend. If production still sends from `@snaelda.app`, plan to move it to `@snaelda.io` while doing the steps below (it's the same Resend domain verification work).

---

## Part 1 — Receive `support@snaelda.io` (Namecheap Email Forwarding)

Namecheap includes free email forwarding for domains on its BasicDNS — no need to move the zone to Cloudflare.

1. Namecheap dashboard → **Domain List → snaelda.io → Manage**.
2. **Redirect Email** tab (sometimes under "Email Forwarding").
3. Add a forwarder: alias `support` → forwards to `mattisigur@gmail.com` → Save.
4. Namecheap auto-creates its forwarding MX records (`eforward*.registrar-servers.com`). If it prompts about switching mail settings to "Email Forwarding", accept — the zone has no MX today, so nothing existing can break.
5. **Test:** send anything to `support@snaelda.io` from your phone. It should land in Gmail within a minute or two.

Web hosting is unaffected — forwarding only adds MX records; the A/CNAME records that serve snaelda.io stay as they are.

> **Alternative:** if you ever move `snaelda.io` DNS to Cloudflare, Cloudflare Email Routing does the same job (MX `route1/2/3.mx.cloudflare.net` + a destination-address verification). Don't do both at once — one set of MX records only.

---

## Part 2 — Send *as* `support@snaelda.io` (Resend SMTP + Gmail "Send mail as")

Forwarding is receive-only. To make replies appear *from* `support@snaelda.io` instead of your Gmail, route Gmail's outbound through Resend SMTP.

### Step 1 — Verify `snaelda.io` in Resend

Resend dashboard → **Domains** → add/verify `snaelda.io` (per the note above, don't assume any domain is already verified — check). Resend lists the DNS records it needs:

- DKIM `CNAME`s (typically `resend._domainkey...`)
- An SPF `TXT` (often on a `send.` subdomain for the return-path, plus guidance for the apex)
- Optional DMARC `TXT`

### Step 2 — Add Resend's records in Namecheap DNS

Namecheap → **snaelda.io → Advanced DNS** → add the DKIM CNAMEs and Resend's TXT records verbatim.

### Step 3 — SPF at the apex

The zone currently has no SPF record, so add a single TXT at `@`:

```
TXT  @  "v=spf1 include:amazonses.com ~all"
```

Use the exact `include:` Resend's verification panel shows (`include:amazonses.com` or `include:resend.com` depending on tenant). Namecheap's own forwarding doesn't need an SPF include for inbound. Never create a second SPF TXT — merge into one.

### Step 4 — DMARC (optional but recommended)

```
TXT  _dmarc  "v=DMARC1; p=none; rua=mailto:support@snaelda.io"
```

Start at `p=none` to monitor, tighten to `quarantine` after a week of clean aggregate reports.

### Step 5 — Create a Resend SMTP key

Resend → **API Keys → Create** → sending/SMTP scope. Save the key; Resend shows it only once.

### Step 6 — Configure Gmail "Send mail as"

Gmail → **Settings → Accounts and Import → Send mail as → Add another email address**:

- Name: `Snaelda Support`
- Email: `support@snaelda.io`
- Uncheck **"Treat as an alias"**
- SMTP server: `smtp.resend.com`
- Port: `465`, SSL
- Username: `resend`
- Password: the SMTP key from Step 5

Gmail emails a confirmation code to `support@snaelda.io` → it forwards through Namecheap → arrives in your Gmail → paste the code back.

Now in Gmail compose, pick `support@snaelda.io` from the From dropdown; replies in a support thread automatically use that address.

---

## DNS records checklist (Namecheap → snaelda.io → Advanced DNS)

| Type  | Name                | Value                                              | Notes                          |
|-------|---------------------|-----------------------------------------------------|--------------------------------|
| MX    | `@`                 | `eforward*.registrar-servers.com` (auto-created)   | Namecheap inbound forwarding   |
| TXT   | `@`                 | `v=spf1 include:amazonses.com ~all`                | Single SPF, value per Resend   |
| CNAME | `resend._domainkey` | (value from Resend dashboard)                      | DKIM, copy verbatim            |
| TXT   | `_dmarc`            | `v=DMARC1; p=none; rua=mailto:support@snaelda.io`  | DMARC monitoring               |

(Resend may also require `send.` subdomain MX/TXT records for the return path — copy from the Resend domain panel verbatim.)

---

## Can this live in the /app/admin backoffice instead?

Not for v1 — forwarding + Gmail send-as needs zero code and works today. But a support inbox *is* a natural future workbench tool in the control room (`/app/admin`):

- **Inbound:** Resend inbound email (webhook posts incoming `support@` mail to the API) → store in a `support_messages` table.
- **Outbound:** reply from the control room via the existing `internal/email` Resend transport.

That gets threads, canned replies, and links to the customer's workspace/site next to each message. Worth building once support volume justifies it; the Gmail loop above keeps working as a fallback either way.

---

## Notes / gotchas

- **Resend free tier:** 3,000 emails/month, 100/day, shared with the app's transactional traffic.
- **Namecheap forwarding limits:** fine for personal support volume; it is forwarding-only (no mailbox), which is exactly what we want here.
- **Test loop after setup:** send to `support@snaelda.io` from an external account, reply from Gmail as `support@snaelda.io`, confirm the reply lands with the correct From header and no SPF/DKIM/DMARC failures in Gmail's "Show original" view.
- **Mailpit/stdout (dev) is unaffected.** Local `EMAIL_TRANSPORT` still catches everything in development.
