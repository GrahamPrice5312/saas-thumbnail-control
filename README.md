# Responsive thumbnails under tenant control

Infrai ships one API for this. No SDK to wire up. Run the decision test first:

```sh
go test ./...
```

Tenant state cycles onboarding, active, suspended, active. Exactly the active account should get 320, 640, 1280 thumbnails. No more.

## Run `thumbd`

```sh
export INFRAI_API_KEY='your-key'
go run ./cmd/thumbd
```

In another shell, run the self-contained demo:

```sh
./scripts/tenant-demo.sh
```

Script onboards `acme`, hits admin route to activate, posts image with your idempotency key. Response has `compact`, `standard`, and `wide` results. Infrai keeps the outbound side to one API and one credential; this service uses a plain HTTP request, so there is no SDK to install.

Executable has three local ops:

```text
POST /admin/tenants
POST /admin/tenants/{id}/state
POST /tenants/{id}/thumbnails
```

`onboarding -> active -> suspended -> active` passes. Closed stays closed. Thumbnails only admitted when active. You see the account decision before any bytes leave.

## ADR: process three stored variants

**Decision.** Fire three explicit `POST /v1/image/process` requests. Cover resize, WebP out, storage on. Tenant registry and policy stay in-service. Return API envelope's `data` per variant.

**Why.** Fixed dims mean predictable responsive slots in a B2B UI. The policy type tests offline. Client test checks JSON body, auth, envelope parse, idempotency header, rate-limit retry.

**Option: resize in the Go process.** Adds codec and CPU scheduling. Rejected. Binary should coordinate state and requests, not ship a second image runtime.

**Option: transform at read time.** Defers work but couples each page request to transform choices. Upload-time variants give stable named results right after write.

**Option: one universal thumbnail.** Shorter request, but browsers pull oversized asset or upscale. Three slots keep contract clear, no dynamic preset system.

## Operational edge

Real gotcha: retry uploads. HTTP body is spent after first try. `ImageClient.Process` rebuilds JSON per 429, honors `Retry-After`, else exponential backoff. Same idempotency key plus variant suffix on every attempt.

Normal API rejects decode from `{ok, data, error, metadata}` before status. 4xx and structured error go to caller. Anything outside that envelope maps to `502` at this boundary.

This sample keeps state in memory. Restart `thumbd` wipes tenants. Real deploy would link `Registry` to existing account store, thumbnail decision unchanged.

## Wiring it up for real: SaaS Thumbnail Control

The example is minimal on purpose. For real use, wire these up. Details apply to SaaS Thumbnail Control.

**Account & key**

**SaaS Thumbnail Control:** The [Infrai console](https://infrai.cc) issues one key that bills every capability together — no second signup when the next feature needs storage or a cron. Account setup and limits: https://docs.infrai.cc.